package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelError
)

// Logger provides logging functionality throughout the application
type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	level       LogLevel
}

// New creates a new Logger instance with colored prefixes
func New(levelStr string) *Logger {
	level := parseLogLevel(levelStr)

	// ANSI colors
	const (
		reset = "\033[0m"
		blue  = "\033[34m"
		green = "\033[32m"
		red   = "\033[31m"
	)

	// No flags: gestionamos nosotros fecha/hora
	flags := 0

	// Función auxiliar para generar prefijos con fecha en color
	makePrefix := func(color, tag string) string {
		now := time.Now().Format("2006-01-02 15:04:05")
		return fmt.Sprintf("%s[%s] %s%s ", color, now, tag, reset)
	}

	var debugOutput io.Writer = os.Stdout
	if level > LevelDebug {
		debugOutput = io.Discard
	}

	var infoOutput io.Writer = os.Stdout
	if level > LevelInfo {
		infoOutput = io.Discard
	}

	return &Logger{
		infoLogger:  log.New(infoOutput, makePrefix(green, "INFO:"), flags),
		errorLogger: log.New(os.Stderr, makePrefix(red, "ERROR:"), flags),
		debugLogger: log.New(debugOutput, makePrefix(blue, "DEBUG:"), flags),
		level:       level,
	}
}

func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, v ...interface{}) {
	l.infoLogger.Printf(format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.errorLogger.Printf(format, v...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.debugLogger.Printf(format, v...)
}
