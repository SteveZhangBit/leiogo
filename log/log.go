package log

type Logger interface {
	Fatal(context string, content string, args ...interface{})
	Error(context string, content string, args ...interface{})
	Info(context string, content string, args ...interface{})
	Debug(context string, content string, args ...interface{})
	Trace(context string, content string, args ...interface{})
}

const (
	Fatal = iota
	Error
	Info
	Debug
	Trace
)

var (
	LogLevel = Debug
	levels   = [...]string{"FATAL", "ERROR", "INFO", "DEBUG", "TRACE"}
)

var New func(name string) Logger
