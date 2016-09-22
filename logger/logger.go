// Package logger is a simple but customizable logger used by ratchet.
package logger

import (
	"io"
	"log"
	"os"
	"runtime"
)

// Ordering the importance of log information. See LogLevel below.
const (
	LevelDebug = iota
	LevelInfo
	LevelError
	LevelStatus
	LevelSilent
)

// RatchetNotifier is an interface for receiving log events. See the
// Notifier variable.
type RatchetNotifier interface {
	RatchetNotify(lvl int, trace []byte, v ...interface{})
}

// Notifier can be set to receive log events in your external
// implementation code. Useful for doing custom alerting, etc.
var Notifier RatchetNotifier

// LogLevel can be set to one of:
// logger.LevelDebug, logger.LevelInfo, logger.LevelError, logger.LevelStatus, or logger.LevelSilent
var LogLevel = LevelInfo

var defaultLogger = log.New(os.Stdout, "", log.LstdFlags)

// Debug logs output when LogLevel is set to at least Debug level
func Debug(v ...interface{}) {
	logit(LevelDebug, v)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelDebug, nil, v)
	}
}

// Info logs output when LogLevel is set to at least Info level
func Info(v ...interface{}) {
	logit(LevelInfo, v)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelInfo, nil, v)
	}
}

// Error logs output when LogLevel is set to at least Error level
func Error(v ...interface{}) {
	logit(LevelError, v)
	if Notifier != nil {
		trace := make([]byte, 4096)
		runtime.Stack(trace, true)
		Notifier.RatchetNotify(LevelError, trace, v)
	}
}

// ErrorWithoutTrace logs output when LogLevel is set to at least Error level
// but doesn't send the stack trace to Notifier. This is useful only when
// using a RatchetNotifier implementation.
func ErrorWithoutTrace(v ...interface{}) {
	logit(LevelError, v)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelError, nil, v)
	}
}

// Status logs output when LogLevel is set to at least Status level
// Status output is high-level status events like stages starting/completing.
func Status(v ...interface{}) {
	logit(LevelStatus, v)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelStatus, nil, v)
	}
}

func logit(lvl int, v ...interface{}) {
	if lvl >= LogLevel {
		defaultLogger.Println(v...)
	}
}

// SetLogfile can be used to log to a file as well as Stdoud.
func SetLogfile(filepath string) {
	f, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err.Error())
	}
	out := io.MultiWriter(os.Stdout, f)
	SetOutput(out)
}

// SetOutput allows setting log output to any custom io.Writer.
func SetOutput(out io.Writer) {
	defaultLogger = log.New(out, "", log.LstdFlags)
}
