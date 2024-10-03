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
// Created: October 1, 2024

package router

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// awaitContextDone waits for the context's cancellation or completion signal (ctx.Done()).
// Once the context is done, it gracefully shuts down the router. If an error occurs during
// the shutdown process, it logs the error message.
func (r *Router) awaitContextDone() {
	<-r.ctx.Done()
	if err := r.Shutdown(); err != nil {
		fmt.Errorf("router failed to shutdown: %s", err.Error())
	}
}

// isClientDisconnected checks if the given error is indicative of a client disconnection.
func isClientDisconnected(err error) bool {

	if err == nil {
		return false
	}

	// Check for specific error types or error messages
	if errors.Is(err, io.EOF) {
		return true
	}

	// Handle common network errors that indicate client disconnection
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Check error messages for common disconnection cases
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "broken pipe") || strings.Contains(errMsg, "connection reset by peer") {
		return true
	}

	return false
}

// corsMiddleware sets up CORS configuration
func corsMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"*"},
		AllowCredentials: true,
		MaxAge:           time.Hour,
	})
}

// noRouteHandler is a custom handler for 404 Not Found responses
func noRouteHandler(c *gin.Context) {
	c.String(http.StatusNotFound, "endpoint not found")
}
