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

package router

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Router wraps the HTTP server, managing routing and lifecycle (graceful shutdown).
type Router struct {
	ctx       context.Context // Manages router lifecycle (shutdown on cancel)
	ginRouter *gin.Engine     // Gin engine for routing HTTP requests
	server    *http.Server    // HTTP server handling requests and shutdown
}

// Cors defines the structure for configuring Cross-Origin Resource Sharing (CORS) settings.
// These settings control how a server responds to requests from different origins,
// specifying what is allowed in terms of origins, methods, headers, etc. For fields
// like AllowOrigins, AllowMethods, and AllowHeaders, setting the value to "*" will
// allow any value (e.g., any origin, any method, or any header).
type Cors struct {
	// AllowOrigins is a list of origins that are allowed to access the server's resources.
	// You can use "*" as a wildcard to allow all origins.
	// Example: ["https://example.com", "https://another-site.com"] or ["*"] to allow all.
	AllowOrigins []string

	// AllowMethods is a list of HTTP methods that are allowed for cross-origin requests.
	// You can use "*" as a wildcard to allow all methods.
	// Example: ["GET", "POST", "PUT"] or ["*"] to allow all methods.
	AllowMethods []string

	// AllowHeaders is a list of headers that the server allows in cross-origin requests.
	// You can use "*" as a wildcard to allow all headers.
	// Example: ["Content-Type", "Authorization", "X-Custom-Header"] or ["*"] to allow all headers.
	AllowHeaders []string

	// ExposeHeaders is a list of headers that the browser is allowed to access in the response
	// from the server during a CORS request. You can use "*" as a wildcard to allow access to all headers.
	// Example: ["X-Response-Time", "Content-Length"] or ["*"] to expose all headers.
	ExposeHeaders []string

	// AllowCredentials indicates whether credentials (like cookies, HTTP authentication, etc.)
	// are allowed in cross-origin requests. Note: If AllowOrigins is set to "*", this must be false
	// as credentials cannot be used with a wildcard origin.
	// Example: true to allow credentials to be sent with the request, false otherwise.
	AllowCredentials bool

	// MaxAge is the duration for which the results of a preflight request (OPTIONS) can be cached
	// by the browser.
	// Example: time.Hour()
	MaxAge time.Duration
}

// Config holds the server configuration options, including the host address,
// CORS settings, and a custom handler for undefined routes.
type Config struct {
	Host           string                                        // Server host address
	Cors           *Cors                                         // CORS configuration
	NoRouteHandler *func(w http.ResponseWriter, r *http.Request) // Custom handler for undefined routes
}

// validate checks the Config struct for required fields and
// returns an error if any required fields are missing
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is empty")
	}
	return nil
}
