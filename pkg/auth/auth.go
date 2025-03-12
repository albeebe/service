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
// Created: October 2, 2024

package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// New initializes and returns a new Auth instance after validating the config.
func New(ctx context.Context, config Config) (*Auth, error) {

	// Ensure the context is not nil
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	// Confirm the config is valid
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Initialize the Auth struct
	a := Auth{
		ctx:          ctx,
		authProvider: config.AuthProvider,
		errorChan:    make(chan error),
		keys:         map[string]*Key{},
	}

	return &a, nil
}

// Start initializes the auth service and begins a periodic refresh using a ticker.
// This function ensures that the service is started only once and returns an error channel
// that reports any issues encountered during refreshes. Consumers of this function
// must listen to the returned error channel to prevent it from blocking when errors occur.
func (a *Auth) Start() chan error {

	// Ensure the auth service is started only once
	a.start.Do(func() {
		// Initialize the tickers for periodic key and access token refresh.
		a.refreshKeysTicker = time.NewTicker(time.Second)
		a.refreshAccessTokenTicker = time.NewTicker(time.Second)

		// Start a goroutine to handle periodic refresh and graceful shutdown on context cancellation.
		go func() {
			defer a.refreshKeysTicker.Stop() // Ensure the ticker is stopped when the goroutine exits.
			// Immediately refresh the keys
			if err := a.refreshKeys(); err != nil {
				a.errorChan <- err
			}
			for {
				select {
				case <-a.refreshAccessTokenTicker.C:
					// Refresh the access token if it's time
					if a.nextAccessTokenRefresh != nil && a.nextAccessTokenRefresh.Before(time.Now()) {
						a.refreshAccessToken()
					}
				case <-a.refreshKeysTicker.C:
					// Refresh the keys if it's time
					if time.Now().After(a.nextKeyRefresh) {
						if err := a.refreshKeys(); err != nil {
							a.errorChan <- err
						}
					}
				case <-a.ctx.Done():
					a.shutdown()
					return
				}
			}
		}()
	})

	return a.errorChan
}

// Authenticate checks the provided HTTP request for a valid Bearer token in the Authorization header.
// If the token is missing, malformed, or invalid, it returns false, a reason, and an error.
// The reason is only set when the request cannot be authenticated, and it is designed to be sent back
// to the client to provide feedback on why authentication failed.
func (a *Auth) Authenticate(r *http.Request) (authenticated bool, reason string, err error) {

	// Ensure the request is not nil
	if r == nil {
		return false, "", errors.New("request is nil")
	}

	// Extract the bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false, "missing authorization header", nil
	}
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return false, "malformed authorization header", nil
	}
	token := strings.TrimSpace(authHeader[7:])
	if token == "" {
		return false, "missing bearer token", nil
	}

	// Validate the token
	isValid, reason, err := a.validateJWT(token)
	if err != nil {
		return false, "", fmt.Errorf("failed to validate jwt: %w", err)
	}

	return isValid, reason, nil
}

// Authorize checks if the request meets the given permission.
// Returns true if authorized, false otherwise, and an error if the request is invalid or the check fails.
func (a *Auth) Authorize(r *http.Request, permission string) (authorized bool, err error) {

	// Ensure the request is not
	if r == nil {
		return false, errors.New("request is nil")
	}

	// Delegate the authorization check to the auth provider
	return a.authProvider.AuthorizeRequest(r, permission)
}

// IsServiceRequest checks whether the given HTTP request originates from a service.
// It delegates the request to the underlying AuthProvider to perform the service request check.
func (a *Auth) IsServiceRequest(r *http.Request) bool {
	return a.authProvider.IsServiceRequest(r)
}

// ExtractBearerToken extracts the Bearer token from the Authorization header of an HTTP request.
// It returns the token and a boolean indicating whether the token was successfully extracted.
func ExtractBearerToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return "", false
	}
	token := strings.TrimSpace(authHeader[7:])
	if len(token) == 0 {
		return "", false
	}
	return token, true
}
