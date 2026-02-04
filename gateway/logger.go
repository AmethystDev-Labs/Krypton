package gateway

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

type LogLevel int32

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var logLevel int32 = int32(LevelInfo)

func SetLogLevel(level LogLevel) {
	atomic.StoreInt32(&logLevel, int32(level))
}

func SetLogLevelFromEnv() {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KRYPTON_LOG_LEVEL")))
	switch v {
	case "debug":
		SetLogLevel(LevelDebug)
	case "info":
		SetLogLevel(LevelInfo)
	case "warn", "warning":
		SetLogLevel(LevelWarn)
	case "error":
		SetLogLevel(LevelError)
	}
}

func Debugf(format string, args ...interface{}) {
	logf(LevelDebug, "DEBUG", colorCyan, format, args...)
}

func Infof(format string, args ...interface{}) {
	logf(LevelInfo, "INFO", colorGreen, format, args...)
}

func Warnf(format string, args ...interface{}) {
	logf(LevelWarn, "WARN", colorYellow, format, args...)
}

func Errorf(format string, args ...interface{}) {
	logf(LevelError, "ERROR", colorRed, format, args...)
}

func logf(level LogLevel, label string, color string, format string, args ...interface{}) {
	if level < LogLevel(atomic.LoadInt32(&logLevel)) {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s[%s]%s (%s) %s\n", color, label, colorReset, ts, msg)
}

const (
	colorReset  = "\x1b[0m"
	colorRed    = "\x1b[31m"
	colorGreen  = "\x1b[32m"
	colorYellow = "\x1b[33m"
	colorCyan   = "\x1b[36m"
)
