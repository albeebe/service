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
	"errors"
	"fmt"
	"log/slog"

	"cloud.google.com/go/logging"
)

// NewDevelopmentLogger sets up a logger for development environments using a custom slog handler.
// This logger prints log entries to the console in a human-readable format. It includes:
// - Timestamps with millisecond precision.
// - Log levels (DEBUG, INFO, WARN, ERROR) in brackets for easy identification.
// - The log message itself.
// - Structured key-value data (attributes) when provided, appended after the message.
// - For ERROR-level logs, a stack trace is included, showing the file and line number of the error origin.
// This logging setup is useful for local development as it makes it easier to spot issues,
// read structured data, and debug errors directly from the console output.
func NewDevelopmentLogger(ctx context.Context, config Config) (*slog.Logger, error) {
	// Create a custom slog handler for development logging
	handler := &DevelopmentHandler{
		level: config.Level, // Set the logging level based on the provided config
	}

	// Return a new slog.Logger using the custom development handler
	return slog.New(handler), nil
}

// NewGoogleCloudLogger sets up a logger for Google Cloud Logging.
// It validates the provided configuration, initializes a Google Cloud Logging client,
// creates a custom slog handler for Google Cloud Logging, and returns an slog.Logger.
func NewGoogleCloudLogger(ctx context.Context, config Config) (*slog.Logger, error) {
	// Validate the provided configuration
	if config.GCPProjectID == "" {
		return nil, errors.New("GCP project ID is missing in config")
	}
	if config.LogName == "" {
		return nil, errors.New("log name is missing in config")
	}

	// Initialize Google Cloud Logging client with the provided context
	client, err := logging.NewClient(ctx, config.GCPProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Cloud Logging client: %w", err)
	}

	// Create a Google Cloud logger with the specified log name
	googleLogger := client.Logger(config.LogName)

	// Create a custom slog handler for Google Cloud Logging
	handler := &GoogleCloudLoggingHandler{
		logger:         googleLogger,
		level:          config.Level, // Set the logging level based on the provided config
		serviceName:    config.ServiceName,
		serviceVersion: config.ServiceVersion,
	}

	// Return a new slog.Logger using the custom Google Cloud Logging handler
	return slog.New(handler), nil
}

// FlushLogger attempts to flush the logs for the provided slog.Logger.
// It supports flushing for loggers using either GoogleCloudLoggingHandler or DevelopmentHandler.
// If the logger does not support flushing, an error is returned.
func FlushLogger(l *slog.Logger) error {
	if l == nil {
		return errors.New("logger is nil")
	}

	// Attempt to flush if the handler is GoogleCloudLoggingHandler
	if handler, ok := l.Handler().(*GoogleCloudLoggingHandler); ok {
		return handler.Flush()
	}

	// Attempt to flush if the handler is DevelopmentHandler
	if handler, ok := l.Handler().(*DevelopmentHandler); ok {
		return handler.Flush()
	}

	// Return an error because the logger does not support flushing
	return errors.New("logger does not support flushing")
}
