package logx

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/libgo/pool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

// zerolog type alias
type (
	Level = zerolog.Level
)

// Level
const (
	TraceLevel = zerolog.TraceLevel
	DebugLevel = zerolog.DebugLevel
	InfoLevel  = zerolog.InfoLevel
	WarnLevel  = zerolog.WarnLevel
	ErrorLevel = zerolog.ErrorLevel
	FatalLevel = zerolog.FatalLevel
	Disabled   = zerolog.Disabled
)

// default to stdout
var logger = zerolog.New(StdWriter(StdConfig{Level: TraceLevel})).With().Timestamp().Logger()

func init() {
	// base config
	zerolog.MessageFieldName = "msg"
	zerolog.TimestampFieldName = "ts"
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	// No need error field, all in msg
	zerolog.ErrorFieldName = "err"
	zerolog.ErrorStackFieldName = "est"
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	// env config
	envLevel := func(key string) Level {
		v := os.Getenv(key)
		switch v {
		case "error", "Error", "ERROR", "ERR", "E":
			return ErrorLevel
		case "warn", "Warn", "WARN", "WRN", "W":
			return WarnLevel
		case "info", "Info", "INFO", "INF", "I":
			return InfoLevel
		case "debug", "Debug", "DEBUG", "DBG", "D":
			return DebugLevel
		case "trace", "Trace", "TRACE", "TRC", "T":
			return TraceLevel
		default:
			return TraceLevel
		}
	}

	envBool := func(key string) bool {
		v := os.Getenv(key)
		return v == "true" || v == "1" || v == "True" || v == "TRUE"
	}

	envInt := func(key string) int {
		v := os.Getenv(key)
		i, _ := strconv.Atoi(v)
		return i
	}

	// set Global Level
	SetGlobalLevel(envLevel("LOGX_GLOBAL_LEVEL"))

	w := []Writer{}

	// redis writer
	if rdsDSN, rdsKey := os.Getenv("LOGX_REDIS_DSN"), os.Getenv("LOGX_REDIS_KEY"); rdsDSN != "" && rdsKey != "" {
		w = append(w, RedisWriter(RedisConfig{
			Level:  envLevel("LOGX_REDIS_LEVEL"),
			Async:  envBool("LOGX_REDIS_ASYNC"),
			DSN:    rdsDSN,
			LogKey: rdsKey,
		}))
	}

	// file writer
	if fileName := os.Getenv("LOGX_FILE_NAME"); fileName != "" {
		w = append(w, FileWriter(FileConfig{
			Level:      envLevel("LOGX_FILE_LEVEL"),
			Async:      envBool("LOGX_FILE_ASYNC"),
			Filename:   fileName,
			Format:     os.Getenv("LOGX_FILE_FORMAT"),
			MaxSize:    envInt("LOGX_FILE_MAXSIZE"),
			MaxAge:     envInt("LOGX_FILE_MAXAGE"),
			MaxBackups: envInt("LOGX_FILE_MAXBACKUPS"),
			LocalTime:  envBool("LOGX_FILE_LOCALTIME"),
			Compress:   envBool("LOGX_FILE_COMPRESS"),
		}))
	}

	// std writer
	if envBool("LOGX_STD_ON") {
		w = append(w, StdWriter(StdConfig{
			Level: envLevel("LOGX_STD_LEVEL"),
			Async: envBool("LOGX_STD_ASYNC"),
		}))
	}

	SetOutput(w...)
}

var ouputList = []func() error{}

// Close resource FILO
func Close() error {
	for i := len(ouputList) - 1; i >= 0; i-- {
		ouputList[i]()
	}
	return nil
}

// Close is helper func for logger impl Close method.
func (l *Log) Close() error {
	return Close()
}

type Log struct {
	kvpair       []interface{}
	skip         int
	callerEnable bool
	stackEnable  bool
}

var logPool = &sync.Pool{
	New: func() interface{} {
		return &Log{
			kvpair: make([]interface{}, 0, 16),
		}
	},
}

func newLog() *Log {
	l := logPool.Get().(*Log)

	l.kvpair = l.kvpair[:0]
	l.skip = 0
	l.callerEnable = false
	l.stackEnable = false

	return l
}

func putLog(l *Log) {
	logPool.Put(l)
}

// Logger return *Log from pool.
func Logger() *Log {
	return newLog()
}

type Writer interface {
	io.WriteCloser
	WriteLevel(Level, []byte) (int, error)
}

// SetOutput set multi log writer, all SetXXX method are non-thread safe.
func SetOutput(w ...Writer) {
	switch len(w) {
	case 0:
		return
	case 1:
		ouputList = append(ouputList, w[0].Close)
		logger = logger.Output(w[0])
	default:
		wList := make([]io.Writer, len(w))
		for i := range w {
			wList[i] = w[i].(io.Writer)
			ouputList = append(ouputList, w[i].Close)
		}

		logger = logger.Output(zerolog.MultiLevelWriter(wList...))
	}
}

// SetGlobalLevel set global log level.
func SetGlobalLevel(l Level) {
	zerolog.SetGlobalLevel(l)
}

var globalCallerEnable = false

// SetGlobalCaller set global caller status
func SetGlobalCaller(b bool) {
	globalCallerEnable = b
}

type ctxKey struct{}

func FromContext(ctx context.Context) *Log {
	if l, ok := ctx.Value(ctxKey{}).(*Log); ok {
		// make copy of Log
		return l.Copy()
	}

	return newLog()
}

func (l *Log) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func (l *Log) Copy() *Log {
	nl := *l
	nl.kvpair = append([]interface{}{}, nl.kvpair...)
	return &nl
}

// Attach add global kv to logger, this is NOT thread safe.
func Attach(kv map[string]interface{}) {
	logger = logger.With().Fields(kv).Logger()
}

// Attach is helper func for logger impl SetAttach method.
func (l *Log) Attach(kv map[string]interface{}) {
	Attach(kv)
}

// Deprecated, using Attach instead.
func SetAttach(kv map[string]interface{}) {
	Attach(kv)
}

// Deprecated, using Attach instead.
func (l *Log) SetAttach(kv map[string]interface{}) {
	Attach(kv)
}

func TraceEnabled() bool {
	return logger.Trace().Enabled()
}

func (l *Log) TraceEnabled() bool {
	return logger.Trace().Enabled()
}

func DebugEnabled() bool {
	return logger.Debug().Enabled()
}

func (l *Log) DebugEnabled() bool {
	return logger.Debug().Enabled()
}

func Trace(v string) {
	newLog().levelLog(TraceLevel, v)
}

func Tracef(format string, v ...interface{}) {
	newLog().levelLog(TraceLevel, format, v...)
}

func Debug(v string) {
	newLog().levelLog(DebugLevel, v)
}

func Debugf(format string, v ...interface{}) {
	newLog().levelLog(DebugLevel, format, v...)
}

func Info(v string) {
	newLog().levelLog(InfoLevel, v)
}

func Infof(format string, v ...interface{}) {
	newLog().levelLog(InfoLevel, format, v...)
}

func Warn(v string) {
	newLog().levelLog(WarnLevel, v)
}

func Warnf(format string, v ...interface{}) {
	newLog().levelLog(WarnLevel, format, v...)
}

// Error v if error value, it will try print e.stack
// using special %_- as error stack
func Error(v interface{}) {
	newLog().levelLog(ErrorLevel, "%_-", v)
}

func Errorf(format string, v ...interface{}) {
	newLog().levelLog(ErrorLevel, format, v...)
}

// Fatal v if error value, it will try print e.stack
// using special %_- as error stack
func Fatal(v interface{}) {
	newLog().levelLog(FatalLevel, "%_-", v)
}

func Fatalf(format string, v ...interface{}) {
	newLog().levelLog(FatalLevel, format, v...)
}

// Deprecated
// KVPair need allocate for type convert, using KV instead.
func KVPair(kv map[string]interface{}) *Log {
	l := newLog()
	for k, v := range kv {
		l.kvpair = append(l.kvpair, k, v)
	}
	return l
}

// KV should be paired, and key should be string
func KV(k interface{}, v ...interface{}) *Log {
	return newLog().KV(k, v...)
}

func (l *Log) KV(k interface{}, v ...interface{}) *Log {
	l.kvpair = append(append(l.kvpair, k), v...)
	return l
}

func Skip(n ...int) *Log {
	return newLog().Skip(n...)
}

func (l *Log) Skip(n ...int) *Log {
	if len(n) != 0 {
		l.skip = n[0]
	}
	return l
}

func Caller() *Log {
	return newLog().Caller()
}

func (l *Log) Caller() *Log {
	l.callerEnable = true
	return l
}

func Stack() *Log {
	return newLog().Stack()
}

func (l *Log) Stack() *Log {
	l.stackEnable = true
	return l
}

func (l *Log) Trace(v string) {
	l.levelLog(TraceLevel, v)
}

func (l *Log) Tracef(format string, v ...interface{}) {
	l.levelLog(TraceLevel, format, v...)
}

func (l *Log) Debug(v string) {
	l.levelLog(DebugLevel, v)
}

func (l *Log) Debugf(format string, v ...interface{}) {
	l.levelLog(DebugLevel, format, v...)
}

func (l *Log) Info(v string) {
	l.levelLog(InfoLevel, v)
}

func (l *Log) Infof(format string, v ...interface{}) {
	l.levelLog(InfoLevel, format, v...)
}

func (l *Log) Warn(v string) {
	l.levelLog(WarnLevel, v)
}

func (l *Log) Warnf(format string, v ...interface{}) {
	l.levelLog(WarnLevel, format, v...)
}

// Error v if error value, it will try print e.stack
// using special %_- as error stack
func (l *Log) Error(v interface{}) {
	l.levelLog(ErrorLevel, "%_-", v)
}

func (l *Log) Errorf(format string, v ...interface{}) {
	l.levelLog(ErrorLevel, format, v...)
}

// Fatal v if error value, it will try print e.stack
// using special %_- as error stack
func (l *Log) Fatal(v interface{}) {
	l.levelLog(FatalLevel, "%_-", v)
}

func (l *Log) Fatalf(format string, v ...interface{}) {
	l.levelLog(FatalLevel, format, v...)
}

func (l *Log) levelLog(lv Level, format string, v ...interface{}) {
	evt := logger.WithLevel(lv)

	// TODO assert cost optimize
	for i, ln := 0, len(l.kvpair); i < ln; i = i + 2 {
		key, ok := l.kvpair[i].(string)
		if !ok {
			key = fmt.Sprint(l.kvpair[i])
		}
		switch vv := l.kvpair[i+1].(type) {
		case string:
			evt.Str(key, vv)
		case float64:
			evt.Float64(key, vv)
		case int64:
			evt.Int64(key, vv)
		case int:
			evt.Int(key, vv)
		case zerolog.LogObjectMarshaler:
			evt.Object(key, vv)
		default:
			evt.Interface(key, vv)
		}
	}

	if globalCallerEnable || l.callerEnable {
		_, file, line, ok := runtime.Caller(2 + l.skip)
		if ok {
			evt.Str("caller", file+":"+strconv.Itoa(line))
		}
	}

	// call stack
	if l.stackEnable {
		evt.Str("cst", TakeStacktrace(l.skip))
	}

	vl := len(v)

	if format == "%_-" && vl != 0 {
		if ve, ok := v[0].(error); ok {
			evt.Stack().Err(ve)
			format = "%v"
		} else {
			format = "%+v"
		}
	}

	if vl == 0 {
		evt.Msg(format)
	} else {
		evt.Msgf(format, v...)
	}

	putLog(l)

	if lv == FatalLevel {
		Close()
		os.Exit(1)
	}
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
	skip := 4
	if len(optionalSkip) != 0 {
		skip += optionalSkip[0]
	}

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

		buff.AppendString(frame.File)

		buff.AppendByte(':')
		buff.AppendInt(int64(frame.Line))
	}

	return buff.String()
}
