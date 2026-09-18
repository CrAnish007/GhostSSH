package log

import (
	"fmt"
	"io"
	"log"
)

// Logger defines the methods used by the project logger.
type Logger interface {
	Trace(format string, args ...any)
	Debug(format string, args ...any)
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
}

const (
	TRACE LogLevel = "TRACE"
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
)

type LogLevel string

func (level LogLevel) getVerbosity() int {
	switch level {
	case TRACE:
		return 5
	case DEBUG:
		return 4
	case INFO:
		return 3
	case WARN:
		return 2
	case ERROR:
		return 1
	default:
		return 0
	}
}

type logger struct {
	i     *log.Logger
	level LogLevel
}

// NewLogger creates a logger that implements the Logger interface.
func NewLogger(level LogLevel, out io.Writer) *logger {
	return &logger{
		i:     log.New(out, "", log.LstdFlags),
		level: level,
	}
}

func (l *logger) print(level LogLevel, format string, args ...any) {
	if level.getVerbosity() > l.level.getVerbosity() {
		return
	}
	l.i.Printf("[%s] %s\n", level, fmt.Sprintf(format, args...))
}

// SetVerbosity
func (l *logger) SetVerbosity(level LogLevel) {
	l.level = level
}

func (l *logger) Trace(format string, args ...any) {
	l.print(TRACE, format, args...)
}

func (l *logger) Debug(format string, args ...any) {
	l.print(DEBUG, format, args...)
}

func (l *logger) Info(format string, args ...any) {
	l.print(INFO, format, args...)
}

func (l *logger) Warn(format string, args ...any) {
	l.print(WARN, format, args...)
}

func (l *logger) Error(format string, args ...any) {
	l.print(ERROR, format, args...)
}
