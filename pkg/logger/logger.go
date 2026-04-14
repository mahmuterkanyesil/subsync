package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type level int

const (
    LevelDebug level = iota
    LevelInfo
    LevelWarn
    LevelError
)

var current level = LevelInfo

func Init() {
    lv := strings.ToLower(os.Getenv("LOG_LEVEL"))
    switch lv {
    case "debug":
        current = LevelDebug
    case "info":
        current = LevelInfo
    case "warn":
        current = LevelWarn
    case "error":
        current = LevelError
    default:
        current = LevelInfo
    }
}

func enabled(l level) bool {
    return l >= current
}

func Debug(format string, v ...interface{}) {
    if !enabled(LevelDebug) {
        return
    }
    log.Output(2, fmt.Sprintf("DEBUG: "+format, v...))
}

func Info(format string, v ...interface{}) {
    if !enabled(LevelInfo) {
        return
    }
    log.Output(2, fmt.Sprintf("INFO: "+format, v...))
}

func Warn(format string, v ...interface{}) {
    if !enabled(LevelWarn) {
        return
    }
    log.Output(2, fmt.Sprintf("WARN: "+format, v...))
}

func Error(format string, v ...interface{}) {
    if !enabled(LevelError) {
        return
    }
    log.Output(2, fmt.Sprintf("ERROR: "+format, v...))
}

// SetLevel allows programmatic override of the level
func SetLevel(l string) {
    os.Setenv("LOG_LEVEL", strings.ToLower(l))
    Init()
}
