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

	"cloud.google.com/go/logging"
)

// Config holds configuration details
type Config struct {
	GCPProjectID string
	Production   bool
	LogName      string
}

// Custom Google Cloud Logging handler for slog
type GoogleCloudLoggingHandler struct {
	logger *logging.Logger
}

// validate checks the Config struct for required fields and
// returns an error if any required fields are missing
func (c *Config) Validate() error {

	if c.GCPProjectID == "" {
		return fmt.Errorf("GCPProjectID is empty")
	}

	if c.LogName == "" {
		return fmt.Errorf("LogName is empty")
	}

	return nil
}
