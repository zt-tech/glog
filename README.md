# glog

Gin框架logger组件的包，参看echo框架logger包

### 字段

- time_unix
- time_unix_nano
- time_rfc3339
- time_rfc3339_nano
- time_custom
- remote_ip
- uri
- host
- method
- path
- query
- protocol
- referer
- user_agent
- status
- level  
- error
- app_id
- latency (In nanoseconds)
- latency_human (Human readable)
- body
- response
- header:<NAME>
- query:<NAME>
- form:<NAME>

**注** level默认为`info`；使用 `error`、`app_id`请设置centext上下文对应上下文key为`context_error`、`context_app_id`

### 使用

```go
Engine.Use(glog.LoggerWithConfig(glog.LoggerConfig{
    CustomTimeFormat:"2006-01-02 15:04:05",
    Format: `{"time":"${time_custom}","timestamp":"${time_unix}","remote_ip":"${remote_ip}","host":"${host}",` +
        `"method":"${method}","uri":"${uri}","status":${status},"error":"${error}",` +
        `"latency_human":"${latency_human}","query":"${query}","body":"${body}","response":"${response}","name":"${query:name}"},` + "\n",
    Output: os.Stdout,
}))
```

### 结果

```json
{
    "time": "2019-11-29 18:53:20",
    "timestamp": "1575024800",
    "remote_ip": "127.0.0.1",
    "host": "127.0.0.1:8083",
    "method": "POST",
    "uri": "/api/user/info?id=1234&name=Manu",
    "status": 200,
    "error": "null",
    "latency_human": "52.303472ms",
    "query": "id=1234&name=Manu",
    "body": "",
    "response": "",
    "name": "Manu"
}
```
