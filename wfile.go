package logx

import (
	"io"
	"log"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileConfig is conf for file writer.
type FileConfig struct {
	Level      Level
	Async      bool
	Format     string
	Filename   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
	LocalTime  bool
	Compress   bool
	w          io.WriteCloser
}

// FileWriter write log to file.
func FileWriter(conf FileConfig) Writer {
	conf.w = &lumberjack.Logger{
		Filename:   conf.Filename,
		MaxSize:    conf.MaxSize,
		MaxBackups: conf.MaxBackups,
		MaxAge:     conf.MaxAge,
		LocalTime:  conf.LocalTime,
		Compress:   conf.Compress,
	}

	// wrapped as std out
	if conf.Format == "TEXT" || conf.Format == "text" {
		conf.w = StdWriter(StdConfig{
			Level: conf.Level,
			o:     conf.w,
		})
	}

	if conf.Async {
		wr := NewAsyncWriter(conf.Level, conf, 1000, 10*time.Millisecond, func(missed int) {
			log.Printf("File Writer dropped %d messages", missed)
		})

		return wr
	}

	return conf
}

func (f *FileConfig) Init() error {
	f.w = &lumberjack.Logger{
		Filename:   f.Filename,
		MaxSize:    f.MaxSize,
		MaxBackups: f.MaxBackups,
		MaxAge:     f.MaxAge,
		LocalTime:  f.LocalTime,
		Compress:   f.Compress,
	}

	// wrapped as std out
	if f.Format == "TEXT" || f.Format == "text" {
		w := &StdConfig{
			Level: f.Level,
			Async: f.Async,
			o:     f.w,
		}
		f.w = w
	}
	return nil
}

// Write write data to writer
func (f FileConfig) Write(p []byte) (n int, err error) {
	return f.w.Write(p)
}

// WriteLevel write data to writer with level info provided
func (f FileConfig) WriteLevel(level Level, p []byte) (n int, err error) {
	if level < f.Level {
		return len(p), nil
	}

	return f.Write(p)
}

func (f FileConfig) Close() error {
	return f.w.Close()
}
