package log

import (
	"fmt"
	"log"
)

func NewSimpleLogger(name string) Logger {
	return &SimpleLogger{Name: name, Level: LogLevel}
}

// A simple implement of Logger, using Go standard library log
type SimpleLogger struct {
	Name  string
	Level int
}

func (l *SimpleLogger) logging(context string, content string, level int) {
	if level <= l.Level {
		name := l.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		log.Printf("<%s> %-7s %-20s: %s\n", context, fmt.Sprintf("[%s]", levels[level]), name, content)
	}
}

func (l *SimpleLogger) Fatal(context string, content string, args ...interface{}) {
	l.logging(context, fmt.Sprintf(content, args...), Fatal)
}

func (l *SimpleLogger) Error(context string, content string, args ...interface{}) {
	l.logging(context, fmt.Sprintf(content, args...), Error)
}

func (l *SimpleLogger) Info(context string, content string, args ...interface{}) {
	l.logging(context, fmt.Sprintf(content, args...), Info)
}

func (l *SimpleLogger) Debug(context string, content string, args ...interface{}) {
	l.logging(context, fmt.Sprintf(content, args...), Debug)
}

func (l *SimpleLogger) Trace(context string, content string, args ...interface{}) {
	l.logging(context, fmt.Sprintf(content, args...), Trace)
}

func init() {
	New = NewSimpleLogger
}
