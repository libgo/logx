package logx

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
)

func TestMain(m *testing.M) {
	m.Run()
	Close()
}

func TestBasic(t *testing.T) {
	Debug("msg debug")
	Debugf("msg debug %s", "format")
	Info("msg info")
	Infof("msg info %s", "format")
	Warn("msg warn")
	Warnf("msg warn %s", "format")
	Error("msg error")
	Errorf("msg error %s", "format")
}

func TestSetAttachment(t *testing.T) {
	l := Logger()
	SetAttachment(map[string]interface{}{"global1": "1", "global2": "2"})
	Debug("hello")
	l.SetKV("set-attach-only-1", 1)
	l.SetKV("global1", "override")
	l.KV("set-attach-only-2", "2").Debug("world")
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

func TestLogger(t *testing.T) {
	Debug("123")
	l := Logger()
	l.SetKV("test-logger-1", "v1")
	l.KV("test-logger-2", "v2").Debug("123")
	Debug("123")
}

func TestKV(t *testing.T) {
	KV("test-kv-1", "v1").KV("test-kv-2", "v2").Debug("k1,k2")
	KV("k1", "v1").Debug("k1 only")

	l1 := Logger()
	l1.SetKV("l1", 1)
	l1.Debug("l1")

	Debug("bare")

	for i := 0; i < 3; i++ {
		go func(x int) {
			KVPair(map[string]interface{}{
				"p1": x,
			}).KV("k", "v").Debug("p1 diff")
		}(i)
	}

	wg := &sync.WaitGroup{}
	wg.Add(200)
	go func() {
		logger1 := Logger()
		for i := 0; i < 100; i++ {
			go func(x int) {
				defer wg.Done()
				seq := fmt.Sprint(x)
				logger1.SetKV("k"+seq, "v"+seq)
			}(i)
		}
		wg.Wait()
		logger1.Debug("logger1")
	}()

	go func() {
		logger2 := Logger()
		for i := 0; i < 100; i++ {
			go func(x int) {
				defer wg.Done()
				seq := fmt.Sprint(x)
				logger2.SetKV("a"+seq, "b"+seq)
			}(i)
		}
		wg.Wait()
		logger2.Debug("logger2")
	}()

	KV("x1", "v1").KV("x2", "v2").Debug("x1,x2")

	<-time.After(time.Microsecond * 500)

	wg.Wait()
}

func TestTrace(t *testing.T) {
	Trace("uuid-uuid-uuid-uuid1").Info("trace1")
	Trace("uuid-uuid-uuid-uuid2").Error("trace2")
	KV("x", "y").Trace("uuid-uuid-uuid-uuid").Error("trace test3")
}

func TestCaller(t *testing.T) {
	Caller().Info("caller1")
	KV("x", "y").Caller().Errorf("caller2")
}

var sLog = Sample(&BasicSampler{N: 2})

func TestSample(t *testing.T) {
	sLog.KV("x", "y").Debug("shown1")
	sLog.KV("m", "n").Debug("hidden1")
	sLog.Debug("shown2")
	sLog.KV("k", "v").Debug("hidden2")
	sLog.KV("k", "v").Debug("shown3")
	sLog.KV("k", "v").Debugf("hidden3")
}

func TestWithStack(t *testing.T) {
	WithStack().Debug("hello")
	KV("stack", "x").WithStack().Debug("hello")
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
	WithStack().Info("stack")
}

func TestLog2File(t *testing.T) {
	SetOutput(FileWriter(FileConfig{
		Filename: "x.log",
		Format:   "text",
	}))
}

func TestSetGlobalLevel(t *testing.T) {
	SetGlobalLevel(InfoLevel)
}

func RunX(e *Event, level Level, msg string) {
	e.Str("xyz", "123").Str("m", "n")
}

func TestHook(t *testing.T) {
	WithHookFunc(RunX).KV("xyz", "abc").Info("111")
}

// go test -v --count=1 -test.bench=".*"

func BenchmarkBase(b *testing.B) {
	SetGlobalLevel(InfoLevel)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Debug().Str("key", "v").Msg("MSG")
	}
}

func BenchmarkDebug(b *testing.B) {
	SetGlobalLevel(InfoLevel)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Debug("MSG")
	}
}

func BenchmarkDebugf(b *testing.B) {
	SetGlobalLevel(InfoLevel)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Debugf("%s", "MSG")
	}
}

func BenchmarkKV(b *testing.B) {
	SetGlobalLevel(InfoLevel)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		KV("a", "b").Debugf("hello, %s", "world")
	}
}

func BenchmarkSetKV(b *testing.B) {
	SetGlobalLevel(InfoLevel)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l := Logger()
		l.SetKV("a", "b")
	}
}
