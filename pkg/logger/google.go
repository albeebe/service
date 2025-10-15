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
	"runtime"
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
//
// NOTE: For Error Reporting ingestion, we add `serviceContext` and `context.reportLocation`
// when severity is ERROR or higher. We also set Entry.SourceLocation.
func (h *GoogleCloudLoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	// 1) Collect attributes from slog.Record
	attributes := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attributes[a.Key] = a.Value.Any()
		return true
	})

	// 2) Compute source location (prefer slog's source if available; fallback to runtime.Caller)
	var file string
	var line int
	var function string
	if src := r.Source(); src != nil { // available if AddSource: true on the slog logger
		file = src.File
		line = src.Line
		function = src.Function
	} else if pc, f, l, ok := runtime.Caller(5); ok { // adjust depth if needed
		file = f
		line = l
		if fn := runtime.FuncForPC(pc); fn != nil {
			function = fn.Name()
		}
	}

	// 3) If error, attach a stack (helps with grouping even if reportLocation is present)
	if r.Level >= slog.LevelError {
		// Only add if caller didn't already set one
		if _, ok := attributes["stack_trace"]; !ok {
			attributes["stack_trace"] = string(debug.Stack())
		}
	}

	// 4) Base payload (always present)
	payload := map[string]any{
		"message":    r.Message,
		"attributes": attributes,
	}

	// 5) For ERROR and above, add the fields that Error Reporting expects
	if r.Level >= slog.LevelError {
		// Service name/version: keep the service stable across releases
		service := h.serviceName
		if service == "" {
			service = "unknown-service"
		}
		version := h.serviceVersion

		payload["serviceContext"] = map[string]any{
			"service": service,
			"version": version,
		}

		// Either a stack trace in message OR context.reportLocation is required.
		// We provide reportLocation (stack trace is already in attributes).
		payload["context"] = map[string]any{
			"reportLocation": map[string]any{
				"filePath":     file,
				"lineNumber":   line,
				"functionName": function,
			},
		}
	}

	// 6) Build and send the Logging entry
	entry := logging.Entry{
		Severity: h.mapSeverity(r.Level),
		Payload:  payload,
	}

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
