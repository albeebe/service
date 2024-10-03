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

package credentials

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang-jwt/jwt"
	"golang.org/x/oauth2/google"
)

// New initializes and returns a Credentials struct by validating the given
// configuration and retrieving the default Google credentials. It also extracts
// the associated email from the credentials. If any step fails, it returns an error.
func NewGoogleCredentials(ctx context.Context, config Config) (creds *google.Credentials, err error) {

	// Validate the provided configuration
	if err = config.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	// Retrieve default Google credentials based on the provided scopes
	creds, err = google.FindDefaultCredentials(ctx, config.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}

	return creds, nil
}

// Email retrieves the email address associated with the Google credentials.
// It handles both production environments (service accounts) and local environments (user accounts).
func Email(creds *google.Credentials) (string, error) {
	const metadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=https://www.google.com"
	var identityToken string

	// Check if running in production
	if runningInProduction() {
		// Get the JWT from the metadata server
		req, err := http.NewRequest("GET", metadataURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create metadata request: %w", err)
		}
		req.Header.Set("Metadata-Flavor", "Google")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve metadata: %w", err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read metadata response: %w", err)
		}
		identityToken = string(body)
	} else {
		// Get the token when running locally
		t, err := creds.TokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("failed to retrieve token: %w", err)
		}
		idToken, ok := t.Extra("id_token").(string)
		if !ok {
			return "", fmt.Errorf("id_token not found in token extras")
		}
		identityToken = idToken
	}

	// Parse the JWT to extract the email
	token, _, err := new(jwt.Parser).ParseUnverified(identityToken, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse JWT: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("failed to parse JWT claims")
	}
	email, ok := claims["email"].(string)
	if !ok || len(email) == 0 {
		return "", fmt.Errorf("JWT does not contain a valid email")
	}
	return email, nil
}

// runningInProduction returns TRUE if the service is running in production
func runningInProduction() bool {
	return metadata.OnGCE()
}
