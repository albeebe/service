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
// Created: October 3, 2024

package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// NewAuthClient creates a new HTTP client with an AuthClient as the transport,
// allowing access token injection on each request.
func (a *Auth) NewAuthClient() (*http.Client, error) {
	ac := AuthClient{
		roundTripper: http.DefaultTransport,
		auth:         a,
	}
	return &http.Client{
		Transport: &ac,
	}, nil
}

// RoundTrip intercepts the HTTP request to inject an access token and then forwards it
// using the configured roundTripper. It handles request body cleanup and ensures
// a valid access token is acquired within a timeout.
func (ac *AuthClient) RoundTrip(r *http.Request) (*http.Response, error) {

	// Ensure the request body is closed if it is not nil
	defer func() {
		if r.Body != nil {
			_ = r.Body.Close()
		}
	}()

	// Get the access token with a timeout
	accessToken, err := ac.getAccessTokenWithTimeout(time.Second * 30)
	if err != nil {
		return nil, fmt.Errorf("failed to get an access token: %w", err)
	}
	if accessToken == nil || len(accessToken.Token) == 0 {
		return nil, errors.New("an access token was expected but not received")
	}

	// Attach the access token to the request
	r.Header.Set("Authorization", "Bearer "+accessToken.Token)

	// Execute the request, ensuring roundTripper is not nil
	if ac.roundTripper == nil {
		return nil, fmt.Errorf("roundTripper is not initialized")
	}
	return ac.roundTripper.RoundTrip(r)
}

// getAccessTokenWithTimeout attempts to retrieve an access token, either from cache or by refreshing it,
// within the specified timeout. If a refresh is needed, multiple requests will wait for the result of
// a single token refresh operation using singleflight to ensure only one refresh happens at a time.
func (ac *AuthClient) getAccessTokenWithTimeout(timeout time.Duration) (*AccessToken, error) {

	// If we already have a valid access token, return it immediately
	if token, ok := ac.auth.getAccessToken(); ok {
		return token, nil
	}

	// Create channels for result and error
	resultCh := make(chan *AccessToken, 1)
	errCh := make(chan error, 1)

	go func() {
		// Use singleflight to ensure only one token refresh happens
		v, err, _ := ac.auth.tokenRefresher.Do("getAccessTokenWithTimeout", func() (interface{}, error) {
			return ac.auth.refreshAccessToken()
		})

		// Send the result or error to the appropriate channel
		if err != nil {
			errCh <- err
		} else {
			resultCh <- v.(*AccessToken)
		}
	}()

	// Wait for either a successful token refresh, an error, or a timeout.
	select {
	case token := <-resultCh:
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for token refresh")
	}
}
