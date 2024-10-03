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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var errorAlgInvalid = errors.New("alg is invalid")
var errorAlgMissing = errors.New("alg is missing")
var errorKidMissing = errors.New("kid is missing")
var errorKeyNotFound = errors.New("key not found")

// awaitKey checks if a key with the given `kid` exists. If found, it returns the key and `true`.
// If the key doesn't exist but keys are currently being refreshed, it waits for up to 5 seconds,
// checking every second for the key to become available. If the key is not found and no refresh
// is happening, or the wait times out, it returns `nil` and `false`.
func (a *Auth) awaitKey(kid string) (*Key, bool) {

	const maxWaitDuration = 5 * time.Second
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timeout := time.After(maxWaitDuration)

	for {
		// Check if the key is available
		key, ok := a.keyWithID(kid)
		if ok {
			return key, true
		}

		// Check if the keys are being refreshed
		a.mux.RLock()
		isRefreshing := time.Now().After(a.nextKeyRefresh)
		a.mux.RUnlock()
		if !isRefreshing {
			return nil, false
		}

		// Wait to check again
		select {
		case <-ticker.C: // Check again if the key is available
		case <-timeout: // Timed out waiting for the key
			return nil, false
		}
	}
}

// keyWithID retrieves a key from the internal map by its ID in a thread-safe manner
// using a read lock. Returns the key and a boolean indicating if the key was found.
func (a *Auth) keyWithID(id string) (*Key, bool) {
	a.mux.RLock()
	defer a.mux.RUnlock()
	key, ok := a.keys[id]
	return key, ok
}

// refreshKeys fetches new keys from the auth provider, validates them,
// and updates the internal key map and next refresh time in a thread-safe manner.
func (a *Auth) refreshKeys() error {

	// Fetch the keys from the auth provider
	keys, nextRefresh, err := a.authProvider.RefreshKeys()
	if err != nil {
		return fmt.Errorf("authProvider failed to refresh keys: %w", err)
	}

	// Validate the keys and place them into a map using key.Kid as the map key
	// for efficient retrieval of individual keys
	keyMap := make(map[string]*Key, len(keys))
	for _, key := range keys {
		if err := key.Validate(); err != nil {
			return fmt.Errorf("key is invalid: %w", err)
		}
		keyMap[key.Kid] = key
	}

	// Update keys and next refresh time with mutex protection
	a.mux.Lock()
	defer a.mux.Unlock()
	a.nextKeyRefresh = nextRefresh
	a.keys = keyMap

	return nil
}

// shutdown stops the key refresh ticker and safely closes the error channel,
// ensuring idempotency and avoiding potential panics.
func (a *Auth) shutdown() error {

	// Stop the ticker if it exists
	if a.refreshKeysTicker != nil {
		a.refreshKeysTicker.Stop()
		a.refreshKeysTicker = nil // Ensure idempotency
	}

	// Close the error channel if it's not already closed
	select {
	case <-a.errorChan:
		// Channel already closed, do nothing
	default:
		close(a.errorChan)
	}

	return nil
}

// validateJWT parses and validates a JWT, ensuring it has the required headers,
// verifies the token signature against a stored key, and checks for common JWT-related
// errors such as expiration, malformed tokens, and invalid signatures.
// The 'reason' returned is designed to be safe for returning to the client, providing
// informative yet non-sensitive details about validation failures without exposing
// internal errors or sensitive information that could assist an attacker.
func (a *Auth) validateJWT(tokenString string) (isValid bool, reason string, err error) {

	// Parse, validate, and verify the tokens signature
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the token has the required headers
		alg, ok := token.Header["alg"].(string)
		if !ok || alg == "" {
			return nil, errorAlgMissing
		}
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, errorKidMissing
		}

		// Get the key the token was signed with
		key, ok := a.awaitKey(kid)
		if !ok || key == nil {
			return nil, errorKeyNotFound
		}

		// Confirm the tokens algorithm matches the keys algorithm
		if strings.ToLower(key.Alg) != strings.ToLower(alg) {
			return nil, errorAlgInvalid
		}

		// Parse the key
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(key.Pem))
		if err != nil {
			return nil, fmt.Errorf("failed to parse RSA public key from key.pem: %w", err)
		}

		// Return the key
		return publicKey, nil
	})

	// Handle errors
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return false, "token is expired", nil
		case errors.Is(err, jwt.ErrTokenMalformed):
			return false, "token is malformed", nil
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return false, "token is not valid yet", nil
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return false, "token signature is invalid", nil
		case errors.Is(err, jwt.ErrTokenUsedBeforeIssued):
			return false, "token used before being issued", nil
		case errors.Is(err, errorAlgInvalid):
			return false, "value for 'alg' header is invalid", nil
		case errors.Is(err, errorAlgMissing):
			return false, "token header is missing an 'alg' value", nil
		case errors.Is(err, errorKidMissing):
			return false, "token header is missing a 'kid' value", nil
		case errors.Is(err, errorKeyNotFound):
			return false, "key not found", nil
		default:
			return false, "", err
		}
	}

	// Check if the token is not valid
	if !token.Valid {
		return false, "token is not valid", nil
	}

	return true, "", nil
}
