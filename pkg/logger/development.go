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
// Created: October 10, 2024

package logger

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"
)

// ANSI color codes for different log levels
const (
	DebugColor = "\033[36m" // Cyan for Debug
	InfoColor  = "\033[32m" // Green for Info
	WarnColor  = "\033[33m" // Yellow for Warn
	ErrorColor = "\033[31m" // Red for Error
	ResetColor = "\033[0m"  // Reset to default terminal color
)

// Enabled reports whether the provided log level is enabled for this handler.
func (h *DevelopmentHandler) Enabled(_ context.Context, level slog.Level) bool {
	// Returns true if the log level is equal to or higher than the handler's log level.
	return level >= h.level
}

// Handle processes log records for development use, printing them to the console with a timestamp,
// the appropriate color based on log level, and a reset color afterward. It also includes any
// structured key-value data associated with the log record. For error logs, it attempts to append
// the relevant file and line number where the log was generated.
func (h *DevelopmentHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format the current time with millisecond precision and include log level in the message
	timeStamp := time.Now().Format("15:04:05.000")
	var messageBuilder strings.Builder
	messageBuilder.WriteString(fmt.Sprintf("[%s] [%s] %s", timeStamp, r.Level.String(), r.Message))

	// Collect structured data from slog.Record using strings.Builder for efficiency
	var attrsBuilder strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		attrsBuilder.WriteString(fmt.Sprintf("%s=%v ", a.Key, a.Value))
		return true // Continue iterating over all attributes
	})

	// Combine message with structured data if available
	attrs := attrsBuilder.String()
	if attrs != "" {
		messageBuilder.WriteString(" | " + attrs) // Append structured data to the message
	}

	// If the log level is an error, print the call stack starting from the first frame outside of the logger
	// This captures and prints multiple stack frames, providing deeper context for where the error originated
	if r.Level == slog.LevelError {
		for x := 3; x < 10; x++ { // Limit stack frame search to a reasonable depth
			_, file, line, ok := runtime.Caller(x)
			if ok {
				messageBuilder.WriteString(fmt.Sprintf("\n   └── (file: %s, line: %d)", file, line))
			}
		}
	}

	// Print the log message with the appropriate color based on log level
	message := messageBuilder.String() // Final message built
	switch r.Level {
	case slog.LevelDebug:
		fmt.Println(DebugColor + message + ResetColor)
	case slog.LevelInfo:
		fmt.Println(InfoColor + message + ResetColor)
	case slog.LevelWarn:
		fmt.Println(WarnColor + message + ResetColor)
	case slog.LevelError:
		fmt.Println(ErrorColor + message + ResetColor)
	default:
		fmt.Println(ResetColor + message + ResetColor) // Handle unknown log levels gracefully
	}

	return nil // Return nil as there are no errors to handle in this context
}

// WithAttrs is required to satisfy the slog.Handler interface.
// This method would typically return a new handler with additional attributes,
// but since attribute handling is not needed, it returns the original handler unchanged.
func (h *DevelopmentHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Returning the handler unchanged, as attribute handling is not required.
	return h
}

// WithGroup is required to satisfy the slog.Handler interface.
// This method would typically return a new handler that groups log attributes,
// but since grouping is not needed, it returns the original handler unchanged.
func (h *DevelopmentHandler) WithGroup(name string) slog.Handler {
	// Returning the handler unchanged, as log grouping is not required.
	return h
}

// Flush is a required handler method for the slog.Handler interface.
// In a development environment, there is no buffered output to flush, so this method simply returns nil.
func (h *DevelopmentHandler) Flush() error {
	return nil
}
