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

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/albeebe/service/internal/credentials"
	"github.com/albeebe/service/internal/environment"
	"github.com/albeebe/service/internal/logger"
	"github.com/albeebe/service/internal/router"
	"github.com/albeebe/service/pkg/auth"
	"github.com/gin-gonic/gin"
)

// Initialize loads the environment variables specified in the provided spec struct.
// In a local development environment, if any variables are missing, the user is
// prompted to enter the missing values. In a production environment, if required
// variables are not set, the function returns an error, indicating that the
// configuration is incomplete and the service should not start until the issue
// is resolved.
func Initialize(spec interface{}) error {
	return environment.Initialize(spec)
}

// New initializes a new service instance with a service name, and configuration.
// It validates the configuration, sets up Google Cloud credentials,
// and prepares the service for use. Returns a configured Service or an error on failure.
func New(serviceName string, config Config) (*Service, error) {

	// Validate the configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config is invalid: %w", err)
	}

	// Configure service
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		Context: ctx,
		Name:    serviceName,
		Log:     logger.New(serviceName),
		internal: &internal{
			cancel: cancel,
			config: &config,
		},
	}

	// Load the credentials
	var err error
	s.GoogleCredentials, err = credentials.NewGoogleCredentials(ctx, credentials.Config{
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/sqlservice.admin",
			"https://www.googleapis.com/auth/devstorage.full_control",
		},
	})
	if err != nil {
		return nil, err
	}

	// Set up the services components
	if err := s.setup(); err != nil {
		return nil, fmt.Errorf("failed to set up the service: %w", err)
	}

	return s, nil
}

// Run starts the service and blocks, waiting for an OS termination signal, context cancellation,
// or an error from the internal router. When termination begins—whether due to an interrupt
// signal, context cancellation, or an error—the provided terminating function is called. The
// function returns only after the service has gracefully shut down.
func (s *Service) Run(terminating func(error)) {

	// Start the auth service
	if s.internal.auth != nil {
		go s.startAuthService()
	}

	// Set up a channel to listen for the terminate signals from the OS
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(terminate)

	// Block until we get a terminate signal, or the context is canceled
	select {
	case <-terminate:
		terminating(nil)
	case <-s.Context.Done():
		terminating(nil)
	case err := <-s.internal.router.ListenAndServe():
		terminating(err)
	}

	// Cancel the context to initiate the graceful shutdown
	s.internal.cancel()

	// Create a channel to signal when teardown is complete
	teardownComplete := make(chan error)

	// Begin teardown in a separate goroutine allowing up to 5 seconds to gracefully teardown
	go func() {
		defer close(teardownComplete)
		teardownComplete <- s.teardown(5 * time.Second)
	}()

	// Wait for teardown to complete, or return immediately if a second signal is received
	select {
	case <-terminate:
		return
	case err := <-teardownComplete:
		if err != nil {
			s.Log.Errorf("teardown completed with an error: %w", err)
		}
	}
}

// Shutdown initiates an immediate graceful shutdown by canceling the service's context,
// signaling all components to stop their operations. This method triggers the shutdown
// process but does not block or wait for the service to fully stop.
func (s *Service) Shutdown() {
	s.internal.cancel()
}

// AddAuthenticatedEndpoint registers an HTTP endpoint that requires authentication.
// It wraps the provided handler to authenticate requests before passing them through.
// If the service was initialized without an AuthProvider, it logs a fatal error and exits.
// If authentication fails or the handler encounters an error, appropriate HTTP responses are returned.
func (s *Service) AddAuthenticatedEndpoint(method, relativePath string, handler func(*Service, *http.Request) *HTTPResponse) {

	// Confirm an AuthProvider exists
	if s.internal.auth == nil {
		s.Log.Fatal("AddAuthenticatedEndpoint requires the service to be initialized with an AuthProvider")
	}

	// Middleware to wrap the handler for request authentication. It authenticates the request,
	// injects the relevant service into the handler, and manages the process of sending the response.
	wrappedHandler := func(c *gin.Context) {
		// Authenticate the request
		authenticated, reason, err := s.internal.auth.Authenticate(c.Request)
		if err != nil {
			s.Log.Errorf("failed to authenticated request: %w", err)
			sendResponse(c, 500, "internal server error")
			return
		}
		if !authenticated {
			message := "unauthorized"
			if reason != "" {
				message += ": " + reason
			}
			sendResponse(c, 401, message)
			return
		}

		// Send the request to the handler and handle the response
		resp := handler(s, c.Request)
		if resp == nil {
			sendResponse(c, 501, "internal server error")
			return
		}
		if err := router.SendResponse(c, resp.StatusCode, resp.Headers, resp.Body); err != nil {
			s.Log.Errorf("failed to send response: %w", err)
		}
	}

	// Register the wrapped handler to the router to handle requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.AddHandler(method, relativePath, wrappedHandler); err != nil {
		s.Log.Fatalf("failed to add endpoint [%s %s]: %w", method, relativePath, err)
	}
}

// AddCloudTask registers a new POST endpoint at the specified relativePath to handle
// incoming Google Cloud Tasks. In production, it verifies the authenticity of the request,
// while in local or non-production environments, request verification is skipped.
func (s *Service) AddCloudTask(relativePath string, handler func(*Service, *http.Request) error) {

	// wrappedHandler is the middleware that processes the incoming request.
	wrappedHandler := func(c *gin.Context) {

		// Verify the request if running in a production environment.
		// This step ensures that the request comes from Google Cloud Tasks.
		if runningInProduction() {
			if err := verifyGoogleRequest(s.Context, c.Request); err != nil {
				// Respond with a 403 Forbidden status if verification fails.
				sendResponse(c, http.StatusForbidden, "forbidden")
				return
			}
		}

		// Invoke the provided handler function with the request.
		// If the handler returns an error, log it and respond with a 500 Internal Server Error status.
		if err := handler(s, c.Request); err != nil {
			s.Log.Errorf("failed to handle request: %w", err.Error)
			sendResponse(c, http.StatusInternalServerError, "internal server error")
			return
		}

		// If the handler succeeds, respond with a 200 OK status.
		sendResponse(c, http.StatusOK, "OK")
	}

	// Register the wrapped handler to the router to handle POST requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.AddHandler("POST", relativePath, wrappedHandler); err != nil {
		s.Log.Fatalf("failed to add Cloud Task [POST %s]: %w", relativePath, err)
	}
}

// AddCronjob registers a new POST endpoint at the specified relativePath to handle
// incoming Google Cloud Scheduler requests. In production, it verifies the authenticity of the
// request, while in local or non-production environments, request verification is skipped.
func (s *Service) AddCronjob(relativePath string, handler func(*Service, *http.Request) error) {

	// wrappedHandler is the middleware that processes the incoming request.
	wrappedHandler := func(c *gin.Context) {

		// Verify the request if running in a production environment.
		// This step ensures that the request comes from Google Cloud Scheduler.
		if runningInProduction() {
			if err := verifyGoogleRequest(s.Context, c.Request); err != nil {
				// Respond with a 403 Forbidden status if verification fails.
				sendResponse(c, http.StatusForbidden, "forbidden")
				return
			}
		}

		// Invoke the provided handler function with the request.
		// If the handler returns an error, log it and respond with a 500 Internal Server Error status.
		if err := handler(s, c.Request); err != nil {
			s.Log.Errorf("failed to handle request: %w", err.Error)
			sendResponse(c, http.StatusInternalServerError, "internal server error")
			return
		}

		// If the handler succeeds, respond with a 200 OK status.
		sendResponse(c, http.StatusOK, "OK")
	}

	// Register the wrapped handler to the router to handle POST requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.AddHandler("POST", relativePath, wrappedHandler); err != nil {
		s.Log.Fatalf("failed to add Cloud Scheduler [POST %s]: %w", relativePath, err)
	}
}

// AddEndpoint registers a new HTTP endpoint with the specified method (e.g., "GET", "POST")
// and relative path. It wraps the provided handler function so that the current Service
// instance is passed into the handler when the endpoint is invoked.
// If an error occurs while registering the endpoint, the function will log the error
// and terminate the program.
func (s *Service) AddEndpoint(method, relativePath string, handler func(*Service, *http.Request) *HTTPResponse) {

	// Wrap the handler, so we can pass the service to it and handle sending the response
	wrappedHandler := func(c *gin.Context) {
		resp := handler(s, c.Request)
		if resp == nil {
			resp = Text(500, "internal server error")
		}
		if err := router.SendResponse(c, resp.StatusCode, resp.Headers, resp.Body); err != nil {
			s.Log.Errorf("failed to send response: %w", err)
		}
	}

	// Register the wrapped handler to the router to handle requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.AddHandler(method, relativePath, wrappedHandler); err != nil {
		s.Log.Fatalf("failed to add endpoint [%s %s]: %w", method, relativePath, err)
	}
}

func (s *Service) AddServiceEndpoint(method, relativePath string, handler func(*Service, *http.Request) *HTTPResponse) {

	// Confirm an AuthProvider exists
	if s.internal.auth == nil {
		s.Log.Fatal("AddServiceEndpoint requires the service to be initialized with an AuthProvider")
	}

	// Middleware to wrap the handler for request authentication. It authenticates the request,
	// injects the relevant service into the handler, and manages the process of sending the response.
	wrappedHandler := func(c *gin.Context) {
		// Authenticate the request
		authenticated, reason, err := s.internal.auth.Authenticate(c.Request)
		if err != nil {
			s.Log.Errorf("failed to authenticated request: %w", err)
			sendResponse(c, 500, "internal server error")
			return
		}
		if !authenticated {
			message := "unauthorized"
			if reason != "" {
				message += ": " + reason
			}
			sendResponse(c, 401, message)
			return
		}

		// Verify the request is from a service
		if isVerified := s.internal.auth.IsServiceRequest(c.Request); !isVerified {
			sendResponse(c, 403, "forbidden: restricted to services")
			return
		}

		// Send the request to the handler and handle the response
		resp := handler(s, c.Request)
		if resp == nil {
			sendResponse(c, 501, "internal server error")
			return
		}
		if err := router.SendResponse(c, resp.StatusCode, resp.Headers, resp.Body); err != nil {
			s.Log.Errorf("failed to send response: %w", err)
		}
	}

	// Register the wrapped handler to the router to handle requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.AddHandler(method, relativePath, wrappedHandler); err != nil {
		s.Log.Fatalf("failed to add endpoint [%s %s]: %w", method, relativePath, err)
	}
}

// AddPubSub registers a new POST endpoint at the specified relativePath to handle incoming
// Pub/Sub messages. In production, it verifies the authenticity of the request, while in
// local or non-production environments, request verification is skipped. The function
// decodes the Pub/Sub message and invokes the provided handler function with the decoded message.
func (s *Service) AddPubSub(relativePath string, handler func(*Service, PubSubMessage) error) {

	// wrappedHandler is the middleware that processes the incoming request.
	wrappedHandler := func(c *gin.Context) {

		// Verify the request if running in a production environment.
		// This step ensures that the request comes from Google Pub/Sub.
		if runningInProduction() {
			if err := verifyGoogleRequest(s.Context, c.Request); err != nil {
				// Respond with a 403 Forbidden status if verification fails.
				sendResponse(c, http.StatusForbidden, "forbidden")
				return
			}
		}

		// Decode the incoming JSON payload, which contains the Pub/Sub message envelope.
		type Envelope struct {
			Message struct {
				Data        string    `json:"data"`
				MessageID   string    `json:"messageId"`
				PublishTime time.Time `json:"publishTime"`
			} `json:"message"`
		}
		var envelope Envelope

		// If the JSON binding fails, log the error and respond with a 400 Bad Request status.
		if err := c.BindJSON(&envelope); err != nil {
			s.Log.Errorf("failed to decode message envelope: %w", err)
			sendResponse(c, http.StatusBadRequest, "bad request")
			return
		}

		// Decode the base64-encoded data from the Pub/Sub message.
		message := PubSubMessage{
			ID:        envelope.Message.MessageID,
			Published: envelope.Message.PublishTime,
			Data:      nil,
		}
		var err error
		message.Data, err = base64.StdEncoding.DecodeString(envelope.Message.Data)

		// If data decoding fails, log the error and respond with a 400 Bad Request status.
		if err != nil {
			s.Log.Errorf("failed to decode message data: %w", err)
			sendResponse(c, http.StatusBadRequest, "bad request")
			return
		}

		// Invoke the provided handler function with the decoded Pub/Sub message.
		// If the handler returns an error, log it and respond with a 500 Internal Server Error status.
		if err := handler(s, message); err != nil {
			s.Log.Errorf("failed to handle message: %w", err.Error)
			sendResponse(c, http.StatusInternalServerError, "internal server error")
			return
		}

		// If the handler succeeds, respond with a 200 OK status.
		sendResponse(c, http.StatusOK, "OK")
	}

	// Register the wrapped handler to the router to handle POST requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.AddHandler("POST", relativePath, wrappedHandler); err != nil {
		s.Log.Fatalf("failed to add PubSub [POST %s]: %w", relativePath, err)
	}
}

// AuthClient returns an *http.Client that automatically attaches JWT tokens to requests
// and refreshes them as needed. It requires the service to have been initialized with an AuthProvider.
func (s *Service) AuthClient() (*http.Client, error) {
	// Check that the service has an initialized AuthProvider
	if s.internal.auth == nil {
		return nil, errors.New("AddServiceEndpoint requires the service to be initialized with an AuthProvider")
	}

	// Retrieve the http.Client from the AuthProvider
	client, err := s.internal.auth.Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, nil
}

// ParseClaimsFromRequest extracts the JWT from the Authorization header of the request,
// decodes the payload, and unmarshals it into the provided claims struct WITHOUT VERIFYING THE SIGNATURE.
func ParseClaimsFromRequest(r *http.Request, claims interface{}) error {
	// Extract the Bearer token
	token, ok := auth.ExtractBearerToken(r)
	if !ok {
		return errors.New("failed to extract bearer token")
	}

	// Split the token into its components (header, payload, signature)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid JWT token format")
	}

	// Base64-decode the payload (JWT uses base64url encoding without padding)
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("failed to decode payload: %w", err)
	}

	// Unmarshal the payload into the claims
	if err := json.Unmarshal(decoded, claims); err != nil {
		return fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	return nil
}
