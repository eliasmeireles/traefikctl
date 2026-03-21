package logger

import (
	"fmt"
	"log"
	"os"
)

var (
	debugEnabled bool
	infoLogger   = log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime)
	warnLogger   = log.New(os.Stdout, "[WARN] ", log.Ldate|log.Ltime)
	errorLogger  = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime)
	debugLogger  = log.New(os.Stdout, "[DEBUG] ", log.Ldate|log.Ltime)
)

func SetDebug(enabled bool) {
	debugEnabled = enabled
}

func Info(format string, args ...interface{}) {
	infoLogger.Output(2, fmt.Sprintf(format, args...))
}

func Warn(format string, args ...interface{}) {
	warnLogger.Output(2, fmt.Sprintf(format, args...))
}

func Error(format string, args ...interface{}) {
	errorLogger.Output(2, fmt.Sprintf(format, args...))
}

func Debug(format string, args ...interface{}) {
	if debugEnabled {
		debugLogger.Output(2, fmt.Sprintf(format, args...))
	}
}
