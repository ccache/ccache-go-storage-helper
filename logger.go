// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package storagehelper

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Logger writes timestamped diagnostic messages to an optional file.
type Logger struct {
	mu   sync.Mutex
	file *os.File
}

// NewLogger creates a Logger that appends to logFile. Logging is disabled if
// logFile is empty or cannot be opened.
func NewLogger(logFile string) *Logger {
	l := &Logger{}
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err == nil {
			l.file = f
		}
	}
	return l
}

// Logf writes a formatted, timestamped message. It is safe for concurrent use.
func (l *Logger) Logf(format string, args ...any) {
	if l.file == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "[%s] %s\n", timestamp, msg)
}

// Close closes the log file if logging is enabled.
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}
