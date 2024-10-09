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

package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ps "cloud.google.com/go/pubsub"
	"google.golang.org/api/idtoken"
)

// New creates a new PubSub instance, initializing the Pub/Sub client.
// Returns a pointer to PubSub or an error if initialization fails.
func New(ctx context.Context, config Config) (*PubSub, error) {

	// Ensure the context is not nil
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize the Pub/Sub client
	client, err := ps.NewClient(ctx, config.GCPProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Pub/Sub client: %w", err)
	}

	// Return a new PubSub instance with an empty topic map
	return &PubSub{
		ctx:    ctx,
		Client: client,
		Topics: make(map[string]*ps.Topic),
	}, nil
}

// Publish sends a message to the specified Pub/Sub topic.
// It returns the message ID or an error if the operation fails.
func (p *PubSub) Publish(topic string, message interface{}) (string, error) {
	// Ensure the client is initialized
	if p.Client == nil {
		return "", errors.New("Pub/Sub client is not initialized")
	}

	// Check if the topic already exists, otherwise, create and cache it
	p.Mux.RLock()
	t, exists := p.Topics[topic]
	p.Mux.RUnlock()
	if !exists {
		p.Mux.Lock()
		// Ensure no one created it in the meantime
		if t = p.Topics[topic]; t == nil {
			t = p.Client.Topic(topic)
			p.Topics[topic] = t
		}
		p.Mux.Unlock()
	}

	// Serialize the message into bytes
	data, err := p.serializeMessage(message)
	if err != nil {
		return "", fmt.Errorf("failed to serialize message: %w", err)
	}

	// Ensure the topic exists
	if t == nil {
		return "", errors.New("Pub/Sub topic is not initialized")
	}

	// Publish the message and return the message ID or an error
	result := t.Publish(p.ctx, &ps.Message{Data: data})
	msgID, err := result.Get(p.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to publish message: %w", err)
	}
	return msgID, nil
}

// ValidateGooglePubSubRequest validates an incoming HTTP request from Google Pub/Sub
// by checking its Authorization header for a Bearer token. It ensures that the token
// is well-formed, verifies it using Google's ID token validation, and optionally
// compares the token's audience with a provided audience string. If no audience is
// provided, the request's host and path are compared to the audience in the token.
// Returns an error if any validation step fails.
func ValidateGooglePubSubRequest(ctx context.Context, r *http.Request, audience string) error {

	// Extract the Authorization header and ensure it contains a Bearer token.
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return fmt.Errorf("missing or malformed authorization header")
	}

	// Remove the "Bearer " prefix and trim any surrounding whitespace.
	token := strings.TrimSpace(authHeader[7:])
	if token == "" {
		return fmt.Errorf("authorization header contains no token")
	}

	// Validate the ID token using Google's token validation method.
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	// If no audience is provided, verify the token's audience matches the request's host and path.
	if audience == "" {
		audienceURL, err := url.Parse(payload.Audience)
		if err != nil {
			return fmt.Errorf("failed to parse token audience: %w", err)
		}
		// Ensure the host and path in the request match the token audience's host and path.
		if r.Host != audienceURL.Host || r.URL.Path != audienceURL.Path {
			return fmt.Errorf("request host and path do not match token audience")
		}
	}

	return nil
}

// serializeMessage converts the message to a byte slice based on its type.
// Supports string, []byte, or marshals other types into JSON.
func (p *PubSub) serializeMessage(message interface{}) ([]byte, error) {
	switch v := message.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return json.Marshal(v)
	}
}
