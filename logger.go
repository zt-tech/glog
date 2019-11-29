package glog

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/valyala/fasttemplate"
	"github.com/zt-tech/glog/color"
)

const (
	// ContextError error
	ContextError = "context_error"
	// ContextAppID appID
	ContextAppID = "context_app_id"
)

type (
	// LoggerConfig defines the config for Logger middleware.
	LoggerConfig struct {
		Skip map[string]struct{}
		// Tags to construct the logger format.
		//
		// - time_unix
		// - time_unix_nano
		// - time_rfc3339
		// - time_rfc3339_nano
		// - time_custom
		// - remote_ip
		// - uri
		// - host
		// - method
		// - path
		// - query
		// - protocol
		// - referer
		// - user_agent
		// - status
		// - level
		// - error
		// - app_id
		// - latency (In nanoseconds)
		// - latency_human (Human readable)
		// - body
		// - response
		// - header:<NAME>
		// - query:<NAME>
		// - form:<NAME>

		//
		// Example "${remote_ip} ${status}"
		//
		// Optional. Default value DefaultLoggerConfig.Format.
		Format string `yaml:"format"`

		// Optional. Default value DefaultLoggerConfig.CustomTimeFormat.
		CustomTimeFormat string `yaml:"custom_time_format"`

		// Output is a writer where logs in JSON format are written.
		// Optional. Default value os.Stdout.
		Output io.Writer

		template *fasttemplate.Template
		colorer  *color.Color
		pool     *sync.Pool
	}

	bodyLogWriter struct {
		gin.ResponseWriter
		body *bytes.Buffer
	}
)

var (
	// DefaultLoggerConfig is the default Logger middleware config.
	DefaultLoggerConfig = LoggerConfig{
		Format: `{"time":"${time_rfc3339_nano}","id":"${id}","remote_ip":"${remote_ip}",` +
			`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
			`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02 15:04:05.00000",
		Output:           os.Stdout,
		colorer:          color.New(),
	}
)

// LoggerWithConfig returns a Logger middleware with config.
// See: `Logger()`.
func LoggerWithConfig(config LoggerConfig) gin.HandlerFunc {
	if config.Format == "" {
		config.Format = DefaultLoggerConfig.Format
	}
	if config.Output == nil {
		config.Output = DefaultLoggerConfig.Output
	}
	config.template = fasttemplate.New(config.Format, "${", "}")
	config.colorer = color.New()
	config.colorer.SetOutput(config.Output)
	config.pool = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 256))
		},
	}
	return func(ctx *gin.Context) {
		bodyBytes, _ := ctx.GetRawData()
		// Restore the io.ReadCloser to its original state
		ctx.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		path := ctx.Request.URL.Path
		raw := ctx.Request.URL.RawQuery
		start := time.Now()
		resBody := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: ctx.Writer}
		ctx.Writer = resBody

		ctx.Next()
		level := "info"
		err, ok := ctx.Get(ContextError)
		if ok {
			level = "error"
		}
		errInfo, _ := json.Marshal(err)
		if _, ok := config.Skip[path]; !ok {
			stop := time.Now()

			buf := config.pool.Get().(*bytes.Buffer)
			buf.Reset()
			defer config.pool.Put(buf)
			re := regexp.MustCompile("\n *|\"password.*\":\".+?\",*")
			if _, err := config.template.ExecuteFunc(buf, func(w io.Writer, tag string) (int, error) {
				switch tag {
				case "time_unix":
					return buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
				case "time_unix_nano":
					return buf.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
				case "time_rfc3339":
					return buf.WriteString(time.Now().Format(time.RFC3339))
				case "time_rfc3339_nano":
					return buf.WriteString(time.Now().Format(time.RFC3339Nano))
				case "time_custom":
					return buf.WriteString(time.Now().Format(config.CustomTimeFormat))
				case "remote_ip":
					return buf.WriteString(ctx.ClientIP())
				case "host":
					return buf.WriteString(ctx.Request.Host)
				case "uri":
					return buf.WriteString(ctx.Request.RequestURI)
				case "method":
					return buf.WriteString(ctx.Request.Method)
				case "path":
					if path == "" {
						path = "/"
					}
					return buf.WriteString(path)
				case "query":
					return buf.WriteString(raw)
				case "protocol":
					return buf.WriteString(ctx.Request.Proto)
				case "referer":
					return buf.WriteString(ctx.Request.Referer())
				case "user_agent":
					return buf.WriteString(ctx.Request.UserAgent())
				case "status":
					n := ctx.Writer.Status()
					s := config.colorer.Green(n)
					switch {
					case n >= 500:
						s = config.colorer.Red(n)
					case n >= 400:
						s = config.colorer.Yellow(n)
					case n >= 300:
						s = config.colorer.Cyan(n)
					}
					return buf.WriteString(s)
				case "app_id":
					appID, _ := ctx.Get(ContextError)
					return buf.WriteString(appID.(string))
				case "level":
					return buf.WriteString(level)
				case "error":
					return buf.Write(errInfo)
				case "latency":
					l := stop.Sub(start)
					return buf.WriteString(strconv.FormatInt(int64(l), 10))
				case "latency_human":
					return buf.WriteString(stop.Sub(start).String())
				case "body":
					return buf.WriteString(re.ReplaceAllString(string(bodyBytes), ""))
				case "response":
					return buf.WriteString(re.ReplaceAllString(resBody.body.String(), ""))
				default:
					switch {
					case strings.HasPrefix(tag, "header:"):
						return buf.Write([]byte(ctx.Request.Header.Get(tag[7:])))
					case strings.HasPrefix(tag, "query:"):
						return buf.Write([]byte(ctx.Query(tag[6:])))
					case strings.HasPrefix(tag, "form:"):
						return buf.Write([]byte(ctx.Request.FormValue(tag[5:])))
					case strings.HasPrefix(tag, "cookie:"):
						cookie, err := ctx.Cookie(tag[7:])
						if err == nil {
							return buf.Write([]byte(cookie))
						}
					}
				}
				return 0, nil
			}); err != nil {
				return
			}

			_, _ = config.Output.Write(buf.Bytes())
			return
		}
	}
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
