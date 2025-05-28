package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	LevelDebug LogLevel = iota // 0
	LevelInfo                  // 1
	LevelWarn                  // 2
	LevelError                 // 3
	LevelFatal                 // 4
)

// String returns the string representation of a LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger provides a simple, level-based logging utility.
type Logger struct {
	minLevel LogLevel
	mu       sync.Mutex // Mutex to ensure thread-safe logging
}

// NewLogger creates and returns a new Logger instance.
// It reads the LOG_LEVEL environment variable, defaulting to INFO if not set or invalid.
func NewLogger() *Logger {
	logLevelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	minLevel := LevelInfo // Default log level

	switch logLevelStr {
	case "DEBUG":
		minLevel = LevelDebug
	case "INFO":
		minLevel = LevelInfo
	case "WARN":
		minLevel = LevelWarn
	case "ERROR":
		minLevel = LevelError
	case "FATAL":
		minLevel = LevelFatal
	default:
		log.Printf("[WARN] Invalid or unset LOG_LEVEL environment variable '%s'. Defaulting to INFO.", logLevelStr)
	}

	// Configure the standard logger to show file and line number
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return &Logger{
		minLevel: minLevel,
	}
}

// SetMinLevel sets the minimum log level for the logger.
func (l *Logger) SetMinLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// logf prints a log message if its level is greater than or equal to the logger's minimum level.
func (l *Logger) logf(level LogLevel, format string, v ...interface{}) {
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Prepend the log level to the message
	prefix := fmt.Sprintf("[%s] ", level.String())
	log.Output(3, prefix+fmt.Sprintf(format, v...)) // Use Output to correctly set caller depth
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(msg string) {
	l.logf(LevelDebug, msg)
}

// Debugf logs a formatted message at DEBUG level.
func (l *Logger) Debugf(format string, v ...interface{}) {
	l.logf(LevelDebug, format, v...)
}

// Info logs a message at INFO level.
func (l *Logger) Info(msg string) {
	l.logf(LevelInfo, msg)
}

// Infof logs a formatted message at INFO level.
func (l *Logger) Infof(format string, v ...interface{}) {
	l.logf(LevelInfo, format, v...)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(msg string) {
	l.logf(LevelWarn, msg)
}

// Warnf logs a formatted message at WARN level.
func (l *Logger) Warnf(format string, v ...interface{}) {
	l.logf(LevelWarn, format, v...)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(msg string) {
	l.logf(LevelError, msg)
}

// Errorf logs a formatted message at ERROR level.
func (l *Logger) Errorf(format string, v ...interface{}) {
	l.logf(LevelError, format, v...)
}

// Fatal logs a message at FATAL level and then exits the application.
func (l *Logger) Fatal(msg string) {
	l.logf(LevelFatal, msg)
	os.Exit(1)
}

// Fatalf logs a formatted message at FATAL level and then exits the application.
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.logf(LevelFatal, format, v...)
	os.Exit(1)
}
