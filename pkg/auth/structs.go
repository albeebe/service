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
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type Auth struct {
	ctx                      context.Context    // Context for managing request lifetimes
	accessToken              *AccessToken       // The current access token
	authProvider             AuthProvider       // Provider for handling authentication logic
	errorChan                chan error         // Channel for reporting errors during operations
	keys                     map[string]*Key    // Cached keys used for authentication
	mux                      sync.RWMutex       // Mutex for synchronizing access to shared resources
	nextAccessTokenRefresh   *time.Time         // Time for the next access token refresh
	nextKeyRefresh           time.Time          // Time for the next key refresh
	refreshAccessTokenTicker *time.Ticker       // Ticker for periodic access token refresh
	refreshKeysTicker        *time.Ticker       // Ticker for periodic key refresh
	start                    sync.Once          // Ensures the start logic is executed only once
	tokenRefresher           singleflight.Group // Group to manage single access token refresh in-flight
}

type AuthClient struct {
	auth         *Auth             // Reference to the auth instance
	roundTripper http.RoundTripper // HTTP RoundTripper to intercept and modify requests
}

type Config struct {
	AuthProvider AuthProvider // Provider for authentication logic configuration
}

type AccessToken struct {
	Token   string    // The access token string
	Expires time.Time // The token's expiration time
}

type Key struct {
	Kid string `json:"kid"` // Kid is the unique identifier for the key.
	Iat int64  `json:"iat"` // Iat is the issued-at time in Unix time (seconds since the epoch).
	Exp int64  `json:"exp"` // Exp is the expiration time in Unix time (seconds since the epoch).
	Alg string `json:"alg"` // Alg specifies the algorithm used with the key (e.g., "RS256").
	Pem string `json:"pem"` // Key contains the RSA public key in PEM format.
}

// validate checks the Config struct for required fields and
// returns an error if any required fields are missing
func (c *Config) Validate() error {

	if c.AuthProvider == nil {
		return fmt.Errorf("an AuthProvider is required")
	}
	return nil
}

// validate checks the Key struct for required fields and
// returns an error if any required fields are missing
func (k *Key) Validate() error {
	if k.Kid == "" {
		return fmt.Errorf("kid is empty")
	}
	if k.Iat == 0 {
		return fmt.Errorf("iat is zero")
	}
	if k.Exp == 0 {
		return fmt.Errorf("exp is zero")
	}
	if k.Alg == "" {
		return fmt.Errorf("alg is empty")
	}
	if k.Pem == "" {
		return fmt.Errorf("pem is empty")
	}
	return nil
}
