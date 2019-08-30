env
```
LOGX_GLOBAL_LEVEL

LOGX_REDIS_DSN
LOGX_REDIS_KEY
LOGX_REDIS_LEVEL
LOGX_REDIS_ASYNC

LOGX_FILE_NAME
LOGX_FILE_FORMAT
LOGX_FILE_LEVEL
LOGX_FILE_ASYNC
LOGX_FILE_MAXSIZE
LOGX_FILE_MAXAGE
LOGX_FILE_MAXBACKUPS
LOGX_FILE_LOCALTIME
LOGX_FILE_COMPRESS

LOGX_STD_ON
LOGX_STD_LEVEL
LOGX_STD_ASYNC
```

```
package main

import (
	"errors"
	"os"

	"github.com/libgo/logx"
)

func main() {
	// global kv, NOT thread safe
	logx.SetAttach(map[string]interface{}{
		"svc":  "test",
		"mode": "dev",
	})

	// global output
	logx.SetOutput(os.Stdout)

	// multi output with different level
	logx.SetOutput(logx.RedisWriter(logx.RedisConfig{
		DSN:    "redis://:@127.0.0.1:6379",
		LogKey: "log:test",
		Level:  InfoLevel,
		Async:  true,
	}), logx.StdWriter(logx.StdConfig{Level: DebugLevel}))

	// global level
	logx.SetGlobalLevel(logx.InfoLevel)

	logx.Debug("hello")

	err := errors.New("some error")
	logx.Caller().Error(err)

	logger := logx.Logger()
	logger.Info("hello world")
}
```


go test -v . -test.bench=".*"
```
goos: darwin
goarch: amd64
pkg: github.com/libgo/logx
BenchmarkBase-8     	300000000	         5.96 ns/op	       0 B/op	       0 allocs/op
BenchmarkDebug-8    	50000000	        36.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkDebugf-8   	50000000	        30.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkKV-8       	20000000	        84.2 ns/op	      16 B/op	       1 allocs/op
BenchmarkSetKV-8    	30000000	        55.6 ns/op	      16 B/op	       1 allocs/op
PASS
```