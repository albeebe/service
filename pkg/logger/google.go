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

	"cloud.google.com/go/logging"
)

// Enabled checks if the log level is enabled for this handler.
func (h *GoogleCloudLoggingHandler) Enabled(_ context.Context, level slog.Level) bool {
	// Only log if the level is equal to or higher than the handler's log level
	return true
}

// Implement the slog.Handler interface
func (h *GoogleCloudLoggingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Convert slog.Record to a Google Cloud log entry
	entry := logging.Entry{
		Severity: logging.Info, // Map slog levels to Google Cloud severity levels
		Payload:  r.Message,    // Use slog's message
	}

	h.logger.Log(entry) // Forward the log entry to Google Cloud Logging
	return nil
}

func (h *GoogleCloudLoggingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Customize as needed
	return h
}

func (h *GoogleCloudLoggingHandler) WithGroup(name string) slog.Handler {
	// Customize as needed
	return h
}
