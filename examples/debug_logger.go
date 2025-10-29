// debug_logger.go: example debug logging with existing Logger interface
//
// Copyright (c) 2025 AGILira - A. Giordano
// Series: an AGILira fragment
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"
	"time"

	"github.com/agilira/balios"
)

// ConsoleLogger implements Logger for console output with debug support
type ConsoleLogger struct {
	logger *log.Logger
}

func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{
		logger: log.New(log.Writer(), "[BALIOS] ", log.LstdFlags|log.Lmicroseconds),
	}
}

func (l *ConsoleLogger) Debug(msg string, keyvals ...interface{}) {
	l.logger.Printf("DEBUG %s %v", msg, keyvals)
}

func (l *ConsoleLogger) Info(msg string, keyvals ...interface{}) {
	l.logger.Printf("INFO %s %v", msg, keyvals)
}

func (l *ConsoleLogger) Warn(msg string, keyvals ...interface{}) {
	l.logger.Printf("WARN %s %v", msg, keyvals)
}

func (l *ConsoleLogger) Error(msg string, keyvals ...interface{}) {
	l.logger.Printf("ERROR %s %v", msg, keyvals)
}

func main() {
	// Create cache with debug mode enabled
	cache := balios.NewCache(balios.Config{
		MaxSize:   100,
		DebugMode: true,
		Logger:    NewConsoleLogger(),
	})

	// Perform operations to see debug output
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Get("key1")
	cache.Get("key3") // miss
	cache.Delete("key1")

	// Wait a bit to see all debug output
	time.Sleep(100 * time.Millisecond)
}
