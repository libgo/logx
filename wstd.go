package logx

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// StdConfig is conf for console std writer.
type StdConfig struct {
	Level Level
	Async bool
	o     io.WriteCloser
	w     zerolog.ConsoleWriter
}

// StdWriter write log to console
func StdWriter(conf StdConfig) Writer {
	if conf.o == nil {
		conf.o = os.Stdout
	}

	conf.w = zerolog.ConsoleWriter{
		Out:             conf.o,
		FormatTimestamp: consoleTimeFormatter,
	}

	if conf.Async {
		wr := NewAsyncWriter(conf.Level, conf, 1000, 10*time.Millisecond, func(missed int) {
			log.Printf("Std Writer dropped %d messages", missed)
		})

		return wr
	}

	return conf
}

func consoleTimeFormatter(i interface{}) string {
	if i, ok := i.(json.Number); ok {
		s, err := i.Int64()
		if err == nil {
			return time.Unix(s, 0).Format(time.RFC3339)
		}
	}

	return fmt.Sprint(i)
}

// Write write data to writer
func (s StdConfig) Write(p []byte) (n int, err error) {
	return s.w.Write(p)
}

// WriteLevel write data to writer with level info provided
func (s StdConfig) WriteLevel(level Level, p []byte) (n int, err error) {
	if level < s.Level {
		return len(p), nil
	}

	return s.Write(p)
}

func (s StdConfig) Close() error {
	return s.o.Close()
}
