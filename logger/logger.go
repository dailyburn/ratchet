// Package logger is a simple but customizable logger used by ratchet.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
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

// LoggerFlag can be set to to log.Lshortfile or log.Llongfile
// to make the logger print file and line number
var LoggerFlag = 0

const (
	Llongfile  = log.Llongfile
	Lshortfile = log.Lshortfile
)

var defaultLogger = log.New(os.Stdout, "", log.LstdFlags)

// prefixCaller prepends caller info to variadic the argument v
func prefixCaller(v ...interface{}) []interface{} {
	_, file, line, ok := runtime.Caller(2)
	if ok {
		args := make([]interface{}, len(v)+1)
		if LoggerFlag&(log.Lshortfile) != 0 {
			file = path.Base(file)
		}
		args[0] = fmt.Sprintf("%v:%v:", file, line)
		for i, arg := range v {
			args[i+1] = arg
		}
		return args
	}
	return v
}

// Debug logs output when LogLevel is set to at least Debug level
func Debug(v ...interface{}) {
	if LoggerFlag&(log.Lshortfile|log.Llongfile) != 0 {
		v = prefixCaller(v...)
	}
	logit(LevelDebug, v...)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelDebug, nil, v...)
	}
}

// Info logs output when LogLevel is set to at least Info level
func Info(v ...interface{}) {
	if LoggerFlag&(log.Lshortfile|log.Llongfile) != 0 {
		v = prefixCaller(v...)
	}
	logit(LevelInfo, v...)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelInfo, nil, v...)
	}
}

// Error logs output when LogLevel is set to at least Error level
func Error(v ...interface{}) {
	if LoggerFlag&(log.Lshortfile|log.Llongfile) != 0 {
		v = prefixCaller(v...)
	}
	logit(LevelError, v...)
	if Notifier != nil {
		trace := make([]byte, 4096)
		runtime.Stack(trace, true)
		Notifier.RatchetNotify(LevelError, trace, v...)
	}
}

// ErrorWithoutTrace logs output when LogLevel is set to at least Error level
// but doesn't send the stack trace to Notifier. This is useful only when
// using a RatchetNotifier implementation.
func ErrorWithoutTrace(v ...interface{}) {
	if LoggerFlag&(log.Lshortfile|log.Llongfile) != 0 {
		v = prefixCaller(v...)
	}
	logit(LevelError, v...)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelError, nil, v...)
	}
}

// Status logs output when LogLevel is set to at least Status level
// Status output is high-level status events like stages starting/completing.
func Status(v ...interface{}) {
	if LoggerFlag&(log.Lshortfile|log.Llongfile) != 0 {
		v = prefixCaller(v...)
	}
	logit(LevelStatus, v...)
	if Notifier != nil {
		Notifier.RatchetNotify(LevelStatus, nil, v...)
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
