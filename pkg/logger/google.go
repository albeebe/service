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
	"context"
	"log/slog"
	"runtime/debug"

	"cloud.google.com/go/logging"
)

// Enabled reports whether the provided log level is enabled for this handler.
func (h *GoogleCloudLoggingHandler) Enabled(_ context.Context, level slog.Level) bool {
	// Returns true if the log level is equal to or higher than the handler's log level.
	return level >= h.level
}

// Handle processes a slog.Record by converting it into a Google Cloud Logging entry.
// It extracts the log message and any associated structured attributes (key-value pairs),
// maps the slog log level to Google Cloud Logging severity, and forwards the log entry
// to Google Cloud Logging.
func (h *GoogleCloudLoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Create a map to hold the structured data (attributes)
	attributes := make(map[string]interface{})

	// Collect attributes from slog.Record
	r.Attrs(func(a slog.Attr) bool {
		attributes[a.Key] = a.Value.Any()
		return true // Continue iterating over all attributes
	})

	// Add stack trace to attributes for errors
	if r.Level == slog.LevelError {
		attributes["stack_trace"] = string(debug.Stack())
	}

	// Create a Google Cloud Logging entry with the log message and structured data
	entry := logging.Entry{
		Severity: h.mapSeverity(r.Level), // Map slog levels to Google Cloud severity levels
		Payload: map[string]interface{}{
			"message":    r.Message,  // Add the message as part of the payload
			"attributes": attributes, // Add structured data as part of the payload
		},
	}

	// Log the entry to Google Cloud
	h.logger.Log(entry)
	return nil
}

// WithAttrs is required to satisfy the slog.Handler interface.
// This method would typically return a new handler with additional attributes,
// but since attribute handling is not needed, it returns the original handler unchanged.
func (h *GoogleCloudLoggingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Returning the handler unchanged, as attribute handling is not required.
	return h
}

// WithGroup is required to satisfy the slog.Handler interface.
// This method would typically return a new handler that groups log attributes,
// but since grouping is not needed, it returns the original handler unchanged.
func (h *GoogleCloudLoggingHandler) WithGroup(name string) slog.Handler {
	// Returning the handler unchanged, as log grouping is not required.
	return h
}

// Flush sends any buffered log entries to Google Cloud Logging and waits for all logs
// to be fully processed. It ensures that logs are properly flushed before shutting down
// the service or completing operations that depend on log delivery.
func (h *GoogleCloudLoggingHandler) Flush() error {
	return h.logger.Flush()
}

// mapSeverity maps slog levels to Google Cloud Logging severity levels
func (h *GoogleCloudLoggingHandler) mapSeverity(level slog.Level) logging.Severity {
	switch level {
	case slog.LevelDebug:
		return logging.Debug
	case slog.LevelInfo:
		return logging.Info
	case slog.LevelWarn:
		return logging.Warning
	case slog.LevelError:
		return logging.Error
	default:
		return logging.Default
	}
}
