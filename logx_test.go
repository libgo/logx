package logx

import (
	"context"
	"errors"
	"os"
	"testing"

	pkgerr "github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	defer Close()
	c := m.Run()
	os.Exit(c)
}

func TestFormat(t *testing.T) {
	Debug("%2F")
}

func TestBasic(t *testing.T) {
	Debug("debug")
	Debugf("debug %s", "fmt")
	Info("info")
	Infof("info %s", "fmt")
	Warn("warn")
	Warnf("warn %s", "fmt")
	Error("error")
	Errorf("error %s", "fmt")
	// Fatal("fatal")
	// Fatalf("fatal %s", "fmt")
}

func TestGlobalAttach(t *testing.T) {
	SetAttach(map[string]interface{}{
		"a": 1,
		"b": 2,
	})

	Debug("global attach")
}

func TestEnabled(t *testing.T) {
	// reset
	defer SetGlobalLevel(DebugLevel)

	if !DebugEnabled() {
		t.Fatalf("debug level should be enabled.")
	}

	lg := Logger()
	if !lg.DebugEnabled() {
		t.Fatalf("debug level should be enabled.")
	}

	SetGlobalLevel(InfoLevel)

	if DebugEnabled() {
		t.Fatalf("debug should not be enabled.")
	}

	if lg.DebugEnabled() {
		t.Fatalf("debug should not be enabled.")
	}
}

func TestCaller(t *testing.T) {
	Caller().Debug("caller")
	SetGlobalCaller(true)
	Debug("gc1")
	KV("a", "b").KV("x", "y").Debug("gc2")
	SetGlobalCaller(false)
}

func TestStack(t *testing.T) {
	err := pkgerr.Wrap(errors.New("error message"), "from error")
	Stack().Error(err)

	Error(errors.New("hello"))

	Stack().Error("hello")
}

func TestContext(t *testing.T) {
	ctx := context.Background()

	ori := KV("c1", 1).KV("c2", 2)
	ctx = ori.WithContext(ctx)

	l := FromContext(ctx)
	l.KV("c3", 3)
	l.Debug("ctx")

	ori.KV("c4", 4).KV("c5", 5).Debug("ori")

	l.Debug("ctx")
}

func TestKV(t *testing.T) {
	KV("a", 1).KV("b", 2).Debugf("hello, %s", "world")

	// override ?
	KV("a", 1).KV("a", 2).Debugf("hello, %s", "world")
}

func TestLog2Redis(t *testing.T) {
	SetOutput(RedisWriter(RedisConfig{
		DSN:    "redis://:@127.0.0.1:6379",
		LogKey: "log:test",
		Level:  InfoLevel,
		Async:  true,
	}), StdWriter(StdConfig{Level: DebugLevel}))
	Debug("only show in console")
	Info("hello info")
	Error("hello error")
	Stack().Info("stack")
}

func TestLog2File(t *testing.T) {
	SetOutput(FileWriter(FileConfig{
		Filename: "x.log",
		Format:   "text",
	}))
	Info("123")
}

// go test -v . -test.bench=".*"

func BenchmarkKV(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			KV("a", 1, "b", 2).Trace("1").
				KV("c", 3).
				KV("d", 4).
				KV("x", "y").
				KV("m", "n").
				Debugf("hello %s", "world")
		}
	})
}

func BenchmarkKVPair(b *testing.B) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			KVPair(map[string]interface{}{"a": 1, "b": 2}).
				Trace("1").
				KV("c", 3).
				KV("d", 4).
				KV("x", "y").
				KV("m", "n").
				Debugf("hello %s", "world")
		}
	})
}
