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
	"log/slog"

	"cloud.google.com/go/logging"
)

// Config holds configuration details for setting up logging.
type Config struct {
	GCPProjectID   string     // GCPProjectID is the Google Cloud Project ID where logs will be sent.
	ServiceName    string     // ServiceName identifies the service in Error Reporting and groups related errors together.
	ServiceVersion string     // ServiceVersion specifies the version or revision of the service for Error Reporting.
	LogName        string     // LogName is the name of the log stream where entries will be written.
	Level          slog.Level // Level is the minimum log level that will be captured (e.g., DEBUG, INFO).
}

// DevelopmentHandler is a custom handler for slog used in development environments.
// It outputs logs to the console with formatted messages and structured data.
type DevelopmentHandler struct {
	level slog.Level // Level is the minimum log level at which logs will be printed to the console.
}

// GoogleCloudLoggingHandler is a custom handler for slog used to send logs to Google Cloud Logging.
type GoogleCloudLoggingHandler struct {
	logger         *logging.Logger // logger is the Google Cloud Logger instance used to send log entries.
	level          slog.Level      // level is the minimum log level at which logs will be sent to Google Cloud.
	serviceName    string          // serviceName identifies the service in Error Reporting and groups related errors together.
	serviceVersion string          // serviceVersion specifies the version or revision of the service for Error Reporting.
}
