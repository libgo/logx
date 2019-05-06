package logx

import (
	"log"
	"time"

	"github.com/go-redis/redis"
)

// RedisConfig is conf for redis writer.
type RedisConfig struct {
	Level  Level
	Async  bool
	DSN    string
	LogKey string
	client *redis.Client
}

// RedisWriter writer log to redis.
func RedisWriter(conf RedisConfig) Writer {
	if conf.LogKey == "" {
		conf.LogKey = "log:basic"
	}

	opt, err := redis.ParseURL(conf.DSN)
	if err != nil {
		panic(err)
	}

	conf.client = redis.NewClient(opt)

	if conf.Async {
		wr := NewAsyncWriter(conf.Level, conf, 1000, 10*time.Millisecond, func(missed int) {
			log.Printf("Redis Writer dropped %d messages", missed)
		})

		return wr
	}

	return conf
}

// Write write data to writer
func (r RedisConfig) Write(p []byte) (n int, err error) {
	return len(p), r.client.RPush(r.LogKey, p).Err()
}

// WriteLevel write data to writer with level info provided
func (r RedisConfig) WriteLevel(level Level, p []byte) (n int, err error) {
	if level < r.Level {
		return len(p), nil
	}

	return r.Write(p)
}

// Close close redis
func (r RedisConfig) Close() error {
	return r.client.Close()
}
