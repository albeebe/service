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

package gcpcredentials

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang-jwt/jwt"
	"golang.org/x/oauth2/google"
)

// NewCredentials initializes Google Cloud credentials based on the provided configuration.
// It validates the configuration, retrieves the default credentials for the given scopes,
// and returns them. If any step fails, it returns an error.
func NewCredentials(ctx context.Context, config Config) (*google.Credentials, error) {
	// Validate the provided configuration.
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Retrieve the default Google credentials based on the provided scopes.
	creds, err := google.FindDefaultCredentials(ctx, config.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to find default credentials: %w", err)
	}

	return creds, nil
}

// ExtractEmail returns the email address associated with the given Google credentials.
// It handles both production environments (running on Google Cloud) and local development environments.
func ExtractEmail(creds *google.Credentials) (string, error) {
	const metadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=https://www.google.com"
	var identityToken string

	// Check if the code is running on Google Cloud (Google Compute Platform).
	if metadata.OnGCE() {
		// Retrieve the JWT from the metadata server.
		req, err := http.NewRequest("GET", metadataURL, nil)
		if err != nil {
			return "", fmt.Errorf("unable to create request to metadata server: %w", err)
		}
		req.Header.Set("Metadata-Flavor", "Google")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve metadata from server: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("unable to read metadata response: %w", err)
		}
		identityToken = string(body)
	} else {
		// Running locally: retrieve the ID token from the credentials.
		token, err := creds.TokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("failed to retrieve token: %w", err)
		}

		idToken, ok := token.Extra("id_token").(string)
		if !ok {
			return "", fmt.Errorf("id_token not found in token extras")
		}
		identityToken = idToken
	}

	// Parse the JWT to extract the email address.
	email, err := extractEmailFromJWT(identityToken)
	if err != nil {
		return "", fmt.Errorf("failed to extract email from JWT: %w", err)
	}

	return email, nil
}

// extractEmailFromJWT parses a JWT and extracts the email from its claims.
// This function assumes that the JWT contains an "email" claim.
func extractEmailFromJWT(identityToken string) (string, error) {
	// Parse the JWT without verifying the signature.
	token, _, err := new(jwt.Parser).ParseUnverified(identityToken, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("unable to parse JWT: %w", err)
	}

	// Extract claims from the token.
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("unable to parse JWT claims")
	}

	// Retrieve the email from the claims.
	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return "", fmt.Errorf("email not found in JWT claims")
	}

	return email, nil
}
