package custom_logger

import (
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

// Log levels
const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	FATAL
)

// Convert LogLevel to string
func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARNING", "ERROR", "FATAL"}[l]
}

// Logger represents a custom logger
type Logger struct {
	level  LogLevel
	output io.Writer
}

// NewLogger creates a new Logger with the specified minimum log level
func NewLogger(level LogLevel, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}
	return &Logger{
		level:  level,
		output: output,
	}
}

// log formats and writes a log message if the log level is sufficient
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	prefix := fmt.Sprintf("[%s] [%s] ", timestamp, level)
	message := fmt.Sprintf(format, args...)

	fmt.Fprintf(l.output, "%s%s\n", prefix, message)

	// If it's a fatal message, exit the program
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	l.log(WARNING, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits the program
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}
