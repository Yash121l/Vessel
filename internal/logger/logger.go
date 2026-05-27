package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	debugMode bool
	logFile   *os.File
	outWriter io.Writer
)

// Init initializes the logger. If debug is true, it creates/appends to a log file.
// If debug is false, it does not write to any file.
func Init(debug bool, dataDir string) {
	debugMode = debug
	if !debug {
		// Normal mode: no log file created. Standard log package will discard or output to stderr/stdout as needed,
		// but let's make sure we don't write to any file.
		outWriter = io.Discard
		log.SetOutput(outWriter)
		return
	}

	// Debug mode: create log file
	logPath := filepath.Join(dataDir, "vessel.log")
	// Try creating the data directory if it doesn't exist
	_ = os.MkdirAll(dataDir, 0755)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fallback to local vessel.log if dataDir is not writable
		logPath = "vessel.log"
		f, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}

	if err == nil {
		logFile = f
		// Write to both the log file and stdout so the running process still shows logs
		outWriter = io.MultiWriter(os.Stdout, f)
		log.SetOutput(outWriter)
		log.SetFlags(0) // We will format timestamps ourselves for custom styling
		Infof("Debug logging enabled. Writing to %s", logPath)
	} else {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		outWriter = os.Stdout
		log.SetOutput(outWriter)
	}
}

// Close closes the log file if open
func Close() {
	if logFile != nil {
		_ = logFile.Close()
	}
}

// IsDebug returns true if debug logging is enabled
func IsDebug() bool {
	return debugMode
}

// GetWriter returns the writer for custom log redirection (like Gin)
func GetWriter() io.Writer {
	if outWriter == nil {
		return io.Discard
	}
	return outWriter
}

func logMessage(level, format string, v ...interface{}) {
	if !debugMode {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, v...)
	log.Printf("[%s] [%s] %s\n", timestamp, level, msg)
}

func Infof(format string, v ...interface{}) {
	logMessage("INFO", format, v...)
}

func Errorf(format string, v ...interface{}) {
	logMessage("ERROR", format, v...)
}

func Debugf(format string, v ...interface{}) {
	logMessage("DEBUG", format, v...)
}
