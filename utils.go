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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/albeebe/service/internal/router"
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

// Textf formats a string with the provided arguments and sets the HTTP response
// with the given status code and the formatted plain text body.
// It is a variant of the Text function that supports formatted text using fmt.Sprintf.
func Textf(statusCode int, text string, args ...any) *HTTPResponse {
	return Text(statusCode, fmt.Sprintf(text, args))
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

// InternalServerError returns an HTTP 500 response with a standard "internal server error" message.
func InternalServerError() *HTTPResponse {
	return Text(500, "internal server error")
}

// UnmarshalJSONBody reads the JSON-encoded body of an HTTP request and unmarshals it into the provided target.
// It returns an error if the request body is empty or if the JSON decoding fails.
func UnmarshalJSONBody(r *http.Request, target interface{}) error {
	// Ensure the request body is not nil or empty
	if r.Body == nil {
		return errors.New("request body is missing")
	}
	defer r.Body.Close()

	// Create a new JSON decoder and decode the body into the target
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		if err == io.EOF {
			return errors.New("request body is empty")
		}
		// Provide more detailed error messages for common JSON decoding issues
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("malformed JSON at position %d", syntaxError.Offset)
		case errors.As(err, &unmarshalTypeError):
			return fmt.Errorf("unexpected JSON type for field %s", unmarshalTypeError.Field)
		default:
			return fmt.Errorf("failed to decode JSON body: %w", err)
		}
	}

	// Ensure there's no additional data after the valid JSON (e.g., extra commas)
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body contains additional unexpected data")
	}

	return nil
}

// Returns true if we're currently running on GCP
func runningInProduction() bool {
	return metadata.OnGCE()
}

// sendResponse is a helper function that simplifies sending HTTP responses
// with a given status code and message.
func sendResponse(w http.ResponseWriter, statusCode int, message string) {
	response := Text(statusCode, message)
	router.SendResponse(w, response.StatusCode, response.Headers, response.Body)
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
