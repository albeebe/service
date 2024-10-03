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
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// NewRouter initializes a new HTTP/2-compatible Gin router with CORS support
// and a custom 404 handler. It validates the provided config and sets up a
// server that gracefully shuts down when the context is canceled.
func NewRouter(ctx context.Context, config Config) (*Router, error) {

	// Ensure the context is not nil
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	// Validate the provided configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Initialize the Router struct
	router := &Router{ctx: ctx}

	// Set Gin mode to release
	gin.SetMode(gin.ReleaseMode)

	// Create a new Gin router with HTTP/2 support
	router.ginRouter = gin.New()
	router.ginRouter.UseH2C = true

	// Set up a custom 404 route
	router.ginRouter.NoRoute(noRouteHandler)

	// Apply CORS middleware
	router.ginRouter.Use(corsMiddleware())

	// Set up the HTTP server
	router.server = &http.Server{
		Addr: config.Host,
		Handler: h2c.NewHandler(
			router.ginRouter,
			&http2.Server{},
		),
	}

	// Gracefully shutdown the server when the context is canceled
	go router.awaitContextDone()

	return router, nil
}

// ListenAndServe starts the server in a separate goroutine and returns a channel
// that will capture any error from starting the server.
func (r *Router) ListenAndServe() chan error {
	errorChan := make(chan error)
	go func() {
		errorChan <- r.server.ListenAndServe()
	}()
	return errorChan
}

func (r *Router) AddHandler(method, relativePath string, handler func(*gin.Context)) error {

	switch strings.ToUpper(method) {
	case "DELETE":
		r.ginRouter.DELETE(relativePath, handler)
	case "GET":
		r.ginRouter.GET(relativePath, handler)
	case "HEAD":
		r.ginRouter.HEAD(relativePath, handler)
	case "PATCH":
		r.ginRouter.PATCH(relativePath, handler)
	case "POST":
		r.ginRouter.POST(relativePath, handler)
	case "PUT":
		r.ginRouter.PUT(relativePath, handler)
	default:
		return fmt.Errorf("Invalid http method '%s' for path '%s'", strings.ToUpper(method), relativePath)
	}
	return nil
}

// SendResponse sends an HTTP response with the provided status code, headers,
// and body content to the client using the Gin context. It streams the body
// data in chunks, ensures headers are set correctly, and handles client disconnection
// or errors during streaming.
func SendResponse(c *gin.Context, statusCode int, headers http.Header, body io.ReadCloser) error {

	// Set the HTTP status code
	c.Status(statusCode)

	// Set the headers
	for key, values := range headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// If the body is provided, stream it to the client and ensure it gets closed
	if body != nil {
		defer body.Close()
		buf := make([]byte, 4096)
		for {
			n, err := body.Read(buf)
			if n > 0 {
				if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
					if isClientDisconnected(writeErr) {
						return nil
					}
					return writeErr
				}
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
		}
	}
	return nil
}

// Shutdown gracefully shuts down the server, waiting for ongoing connections to finish.
func (r *Router) Shutdown() error {
	return r.server.Shutdown(context.Background())
}
