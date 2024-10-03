// Copyright (c) 2024 Alan Beebe [www.alanbeebe.com]
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// Created: September 30, 2024

package logger

import (
	"fmt"
	"os"
)

// New creates a new Logger instance for the specified service.
func New(serviceName string) *Logger {
	return &Logger{}
}

// Debug logs a message at DEBUG level.
func (l *Logger) Debug(message string) {
	fmt.Printf(message)
}

// Debugf logs a formatted message at DEBUG level.
func (l *Logger) Debugf(message string, args ...interface{}) {
	fmt.Printf(message, args)
}

// Error logs a message at ERROR level.
func (l *Logger) Error(message string) {
	fmt.Println(fmt.Errorf(message).Error())
}

// Errorf logs a formatted message at ERROR level.
func (l *Logger) Errorf(message string, args ...any) {
	err := fmt.Errorf(message, args...)
	fmt.Println(err.Error())
}

// Fatal logs a message at CRITICAL level and exits the application.
func (l *Logger) Fatal(message string) {
	fmt.Printf(message)
	os.Exit(1)
}

// Fatalf logs a formatted message at CRITICAL level and exits the application.
func (l *Logger) Fatalf(message string, args ...interface{}) {
	fmt.Printf(message, args)
	os.Exit(1)
}

// Info logs a message at INFO level.
func (l *Logger) Info(message string) {
	fmt.Printf(message)
	fmt.Println(message)
}

// Infof logs a formatted message at INFO level.
func (l *Logger) Infof(message string, args ...interface{}) {
	fmt.Printf(message, args)
	fmt.Printf(message, args)
}

// Warn logs a message at WARN level.
func (l *Logger) Warn(message string) {
	fmt.Printf(message)
}

// Warnf logs a formatted message at WARN level.
func (l *Logger) Warnf(message string, args ...interface{}) {
	fmt.Printf(message, args)
}
