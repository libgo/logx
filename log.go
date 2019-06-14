package logx

import (
	"context"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/libgo/pool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Zerolog type alias
type (
	Level    = zerolog.Level
	Event    = zerolog.Event
	Hook     = zerolog.Hook
	HookFunc = zerolog.HookFunc
)

// Sampler alias
type (
	Sampler      = zerolog.Sampler
	BasicSampler = zerolog.BasicSampler
)

// Level
const (
	DebugLevel = zerolog.DebugLevel
	InfoLevel  = zerolog.InfoLevel
	WarnLevel  = zerolog.WarnLevel
	ErrorLevel = zerolog.ErrorLevel
	FatalLevel = zerolog.FatalLevel
	Disabled   = zerolog.Disabled
)

// Log struct.
type Log struct {
	mu           *sync.RWMutex
	zl           *zerolog.Logger
	sampler      zerolog.Sampler
	depth        int
	callerEnable bool
	stackEnable  bool
	kv           []interface{} // len must be even
}

func init() {
	w := []Writer{}

	getLevel := func(key string) Level {
		v := os.Getenv(key)
		switch v {
		case "info", "Info", "INFO", "INF", "I":
			return InfoLevel
		case "warn", "Warn", "WARN", "WRN", "W":
			return WarnLevel
		case "error", "Error", "ERROR", "ERR", "E":
			return ErrorLevel
		default:
			return DebugLevel
		}
	}

	getBool := func(key string) bool {
		v := os.Getenv(key)
		return v == "true" || v == "1" || v == "True" || v == "TRUE"
	}

	getInt := func(key string) int {
		v := os.Getenv(key)
		i, _ := strconv.Atoi(v)
		return i
	}

	// set Global Level
	SetGlobalLevel(getLevel("LOGX_GLOBAL_LEVEL"))

	// redis writer
	if rdsDSN, rdsKey := os.Getenv("LOGX_REDIS_DSN"), os.Getenv("LOGX_REDIS_KEY"); rdsDSN != "" && rdsKey != "" {
		w = append(w, RedisWriter(RedisConfig{
			Level:  getLevel("LOGX_REDIS_LEVEL"),
			Async:  getBool("LOGX_REDIS_ASYNC"),
			DSN:    rdsDSN,
			LogKey: rdsKey,
		}))
	}

	// file writer
	if fileName := os.Getenv("LOGX_FILE_NAME"); fileName != "" {
		w = append(w, FileWriter(FileConfig{
			Level:      getLevel("LOGX_FILE_LEVEL"),
			Async:      getBool("LOGX_FILE_ASYNC"),
			Filename:   fileName,
			Format:     os.Getenv("LOGX_FILE_FORMAT"),
			MaxSize:    getInt("LOGX_FILE_MAXSIZE"),
			MaxAge:     getInt("LOGX_FILE_MAXAGE"),
			MaxBackups: getInt("LOGX_FILE_MAXBACKUPS"),
			LocalTime:  getBool("LOGX_FILE_LOCALTIME"),
			Compress:   getBool("LOGX_FILE_COMPRESS"),
		}))
	}

	// std writer
	if getBool("LOGX_STD_ON") {
		w = append(w, StdWriter(StdConfig{
			Level: getLevel("LOGX_STD_LEVEL"),
			Async: getBool("LOGX_STD_ASYNC"),
		}))
	}

	// using std writer as default is no writer
	if len(w) == 0 {
		w = append(w, StdWriter(StdConfig{}))
	}

	SetOutput(w...)
}

var logger = Log{
	mu: &sync.RWMutex{},
	zl: &log.Logger,
	kv: make([]interface{}, 0, 8),
}

// prefixSize is used internally to trim the user specific path from the
// front of the returned filenames from the runtime call stack.
var prefixSize int

func init() {
	zerolog.MessageFieldName = "msg"
	zerolog.TimestampFieldName = "ts"
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	_, file, _, ok := runtime.Caller(0)
	if file == "?" {
		return
	}
	if ok {
		size := len(file)
		suffix := len("github.com/libgo/logx/log.go")
		prefixSize = len(strings.TrimSuffix(file[:size-suffix], "vendor/")) // remove vendor
	}
}

// Logger return copy of default logger
func Logger() *Log {
	l := logger
	return &l
}

type ctxKey struct{}

func Ctx(ctx context.Context) Log {
	if l, ok := ctx.Value(ctxKey{}).(Log); ok {
		return l
	}

	return logger
}

func (l Log) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

type Writer interface {
	io.WriteCloser
	WriteLevel(Level, []byte) (int, error)
}

// SetOutput set multi log writer, careful, all SetXXX method are non-thread safe.
func SetOutput(w ...Writer) {
	switch len(w) {
	case 0:
		return
	case 1:
		log.Logger = log.Logger.Output(w[0])
		asyncWaitList = append(asyncWaitList, w[0].Close)
	default:
		wList := make([]io.Writer, len(w))
		for i := range w {
			wList[i] = w[i].(io.Writer)
			asyncWaitList = append(asyncWaitList, w[i].Close)
		}

		log.Logger = log.Logger.Output(zerolog.MultiLevelWriter(wList...))
	}
}

var globalCallerEnable = false

// SetGlobalCaller set global caller status
func SetGlobalCaller(b bool) {
	globalCallerEnable = b
}

// CallDepth set call depth for show line number.
var CallDepth = 2

// SetGlobalLevel set global log level.
func SetGlobalLevel(l Level) {
	zerolog.SetGlobalLevel(l)
}

// SetAttach add global kv to logger, this is NOT thread safe.
func SetAttach(kv map[string]interface{}) {
	log.Logger = log.With().Fields(kv).Logger()
}

// Deprecated: SetAttachment using SetAttach instead
func SetAttachment(kv map[string]interface{}) {
	SetAttach(kv)
}

// SetAttach is helper func for logger impl SetAttach method.
func (l *Log) SetAttach(kv map[string]interface{}) {
	SetAttach(kv)
}

// Deprecated: SetAttachment using SetAttach instead
func (l *Log) SetAttachment(kv map[string]interface{}) {
	SetAttach(kv)
}

func Debug(v string) {
	l := logger
	l.depth++
	l.Debug(v)
}

func Debugf(format string, v ...interface{}) {
	l := logger
	l.depth++
	l.Debugf(format, v...)
}

func DebugEnabled() bool {
	return log.Logger.Debug().Enabled()
}

func Info(v string) {
	l := logger
	l.depth++
	l.Info(v)
}

func Infof(format string, v ...interface{}) {
	l := logger
	l.depth++
	l.Infof(format, v...)
}

func Warn(v string) {
	l := logger
	l.depth++
	l.Warn(v)
}

func Warnf(format string, v ...interface{}) {
	l := logger
	l.depth++
	l.Warnf(format, v...)
}

func Error(v interface{}) {
	l := logger
	l.depth++
	l.Error(v)
}

func Errorf(format string, v ...interface{}) {
	l := logger
	l.depth++
	l.Errorf(format, v...)
}

func Fatal(v interface{}) {
	l := logger
	l.depth++
	l.Fatal(v)
}

func Fatalf(format string, v ...interface{}) {
	l := logger
	l.depth++
	l.Fatalf(format, v...)
}

func KV(k string, v interface{}) Log {
	l := logger
	l.SetKV(k, v)
	return l
}

func (l Log) KV(k string, v interface{}) Log {
	l.SetKV(k, v)
	return l
}

func KVPair(kv map[string]interface{}) Log {
	l := logger
	l.mu.Lock()
	for k, v := range kv {
		l.kv = append(l.kv, k, v)
	}
	l.mu.Unlock()
	return l
}

// SetKV change kv slice
func (l *Log) SetKV(k string, v interface{}) {
	l.mu.Lock()
	l.kv = append(l.kv, k, v)
	l.mu.Unlock()
}

func Trace(v string) Log {
	l := logger
	l.SetKV("tid", v)
	return l
}

func (l Log) Trace(v string) Log {
	l.SetKV("tid", v)
	return l
}

func Caller() Log {
	l := logger
	l.callerEnable = true
	return l
}

func (l Log) Caller() Log {
	l.callerEnable = true
	return l
}

func WithStack() Log {
	l := logger
	l.stackEnable = true
	return l
}

func (l Log) WithStack() Log {
	l.stackEnable = true
	return l
}

func WithHook(h Hook) Log {
	l := logger
	hl := l.zl.Hook(h)
	l.zl = &hl
	return l
}

func (l Log) WithHook(h Hook) Log {
	hl := l.zl.Hook(h)
	l.zl = &hl
	return l
}

func WithHookFunc(h HookFunc) Log {
	l := logger
	hl := l.zl.Hook(h)
	l.zl = &hl
	return l
}

func (l Log) WithHookFunc(h HookFunc) Log {
	hl := l.zl.Hook(h)
	l.zl = &hl
	return l
}

func Sample(sampler Sampler) Log {
	l := logger
	l.sampler = sampler
	return l
}

func Skip(n int) Log {
	l := logger
	l.depth += n
	return l
}

func (l Log) Skip(n int) Log {
	l.depth += n
	return l
}

func (l Log) DebugEnabled() bool {
	return l.zl.Debug().Enabled()
}

func (l Log) Debug(v string) {
	l.depth++
	l.Debugf(v)
}

func (l Log) Debugf(format string, v ...interface{}) {
	l.levelLog(DebugLevel, format, v...)
}

func (l Log) Info(v string) {
	l.depth++
	l.Infof(v)
}

func (l Log) Infof(format string, v ...interface{}) {
	l.levelLog(InfoLevel, format, v...)
}

func (l Log) Warn(v string) {
	l.depth++
	l.Warnf(v)
}

func (l Log) Warnf(format string, v ...interface{}) {
	l.levelLog(WarnLevel, format, v...)
}

func (l Log) Error(v interface{}) {
	l.depth++
	l.Errorf("%v", v)
}

func (l Log) Errorf(format string, v ...interface{}) {
	l.levelLog(ErrorLevel, format, v...)
}

func (l Log) Fatal(v interface{}) {
	l.depth++
	l.Fatalf("%v", v)
}

func (l Log) Fatalf(format string, v ...interface{}) {
	l.levelLog(FatalLevel, format, v...)
}

func (l Log) levelLog(lv Level, format string, v ...interface{}) {
	evt := l.zl.WithLevel(lv)

	if l.sampler != nil {
		s := l.zl.Sample(l.sampler)
		evt = s.WithLevel(lv)
	}

	l.mu.RLock()
	for i, ln := 0, len(l.kv); i < ln; i = i + 2 {
		switch vv := l.kv[i+1].(type) {
		case string:
			evt.Str(l.kv[i].(string), vv)
		case float64:
			evt.Float64(l.kv[i].(string), vv)
		case int64:
			evt.Int64(l.kv[i].(string), vv)
		case int:
			evt.Int(l.kv[i].(string), vv)
		default:
			evt.Interface(l.kv[i].(string), l.kv[i+1])
		}
	}
	l.mu.RUnlock()

	if globalCallerEnable || l.callerEnable {
		_, file, line, ok := runtime.Caller(CallDepth + l.depth)
		if ok {
			if prefixSize != 0 && len(file) > prefixSize {
				file = file[prefixSize:]
			}
			file += ":" + strconv.Itoa(line)
			evt.Str("caller", file)
		}
	}

	if l.stackEnable {
		evt.Str("stack", TakeStacktrace(l.depth))
	}

	evt.Msgf(format, v...)

	// Close then exit
	switch lv {
	case FatalLevel:
		Close()
		os.Exit(1)
	}
}

var asyncWaitList = []func() error{}

// TODO reverse close
func Close() error {
	for i := range asyncWaitList {
		asyncWaitList[i]()
	}
	return nil
}

var stacktracePool = sync.Pool{
	New: func() interface{} {
		return newProgramCounters(64)
	},
}

type programCounters struct {
	pcs []uintptr
}

func newProgramCounters(size int) *programCounters {
	return &programCounters{make([]uintptr, size)}
}

var bufferPool = pool.NewBytesPool()

// TakeStacktrace is helper func to take snap short of stack trace.
func TakeStacktrace(optionalSkip ...int) string {
	skip := 0
	if len(optionalSkip) != 0 {
		skip = optionalSkip[0]
	}
	skip += CallDepth + 2

	buff := bufferPool.Get()
	defer buff.Free()

	programCounters := stacktracePool.Get().(*programCounters)
	defer stacktracePool.Put(programCounters)

	var numFrames int
	for {
		// Skip the call to runtime.Counters and takeStacktrace so that the
		// program counters start at the caller of takeStacktrace.
		numFrames = runtime.Callers(skip, programCounters.pcs)
		if numFrames < len(programCounters.pcs) {
			break
		}
		// Don't put the too-short counter slice back into the pool; this lets
		// the pool adjust if we consistently take deep stacktraces.
		programCounters = newProgramCounters(len(programCounters.pcs) * 2)
	}

	frames := runtime.CallersFrames(programCounters.pcs[:numFrames])

	// Note: On the last iteration, frames.Next() returns false, with a valid
	// frame, but we ignore this frame. The last frame is a a runtime frame which
	// adds noise, since it's only either runtime.main or runtime.goexit.
	i := 0
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		if i != 0 {
			buff.AppendByte('\n')
		}
		i++
		buff.AppendString(frame.Function)
		buff.AppendByte('\n')
		buff.AppendByte('\t')

		if prefixSize != 0 && len(frame.File) > prefixSize {
			frame.File = frame.File[prefixSize:]
		}
		buff.AppendString(frame.File)

		buff.AppendByte(':')
		buff.AppendInt(int64(frame.Line))
	}

	return buff.String()
}
