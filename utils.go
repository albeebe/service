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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/albeebe/service/internal/router"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"
)

// Text sets the HTTP response with the provided status code and plain text body.
func Text(statusCode int, text string) *HTTPResponse {
	r := &HTTPResponse{
		Headers: http.Header{},
	}
	r.StatusCode = statusCode
	r.Headers.Set("Content-Type", "text/plain")
	r.Body = io.NopCloser(strings.NewReader(text))
	return r
}

// JSON sets the HTTP response with the provided status code and a JSON-encoded
// body generated from the provided object. If an error occurs during the JSON
// encoding process (e.g., unsupported types or invalid data), the function
// gracefully handles it by setting the response body to `null`. The response
// is streamed using a pipe to avoid loading the entire JSON payload into memory
// at once, making it suitable for handling large objects.
func JSON(statusCode int, obj interface{}) *HTTPResponse {
	r := &HTTPResponse{
		Headers: http.Header{},
	}
	r.StatusCode = statusCode
	r.Headers.Set("Content-Type", "application/json")
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		err := json.NewEncoder(pw).Encode(obj)
		if err != nil {
			pw.Write([]byte(`null`))
		}
	}()
	r.Body = pr
	return r
}

// Returns true if we're currently running on GCP
func runningInProduction() bool {
	return metadata.OnGCE()
}

// sendResponse is a helper function that simplifies sending HTTP responses
// in the Gin context with a given status code and message.
func sendResponse(c *gin.Context, statusCode int, message string) {
	response := Text(statusCode, message)
	router.SendResponse(c, response.StatusCode, response.Headers, response.Body)
}

// verifyGoogleRequest validates an incoming HTTP request by checking its Authorization
// header for a Bearer token. It ensures the token is properly formatted, verifies
// the token using Google's ID token validation, and compares the request's host and
// path against the audience in the token to ensure they match. Returns an error if
// any validation step fails.
func verifyGoogleRequest(ctx context.Context, r *http.Request) error {

	// Extract the bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return fmt.Errorf("invalid authorization header")
	}
	token := strings.TrimSpace(authHeader[7:])
	if token == "" {
		return fmt.Errorf("missing bearer token")
	}

	// Validate the token
	payload, err := idtoken.Validate(ctx, token, "")
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	// Compare the request's host and path with the audience's host and path
	audienceURL, err := url.Parse(payload.Audience)
	if err != nil {
		return fmt.Errorf("failed to parse payload.Audience: %w", err)
	}
	if r.Host != audienceURL.Host || r.URL.Path != audienceURL.Path {
		return fmt.Errorf("token audience does not match request")
	}

	return nil
}
