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
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/albeebe/service/pkg/auth"
	"github.com/albeebe/service/pkg/environment"
	"github.com/albeebe/service/pkg/gcpcredentials"
	"github.com/albeebe/service/pkg/pubsub"
	"github.com/albeebe/service/pkg/router"
	"github.com/golang-jwt/jwt/v5"
)

// Initialize loads the environment variables specified in the provided spec struct.
// In a local development environment, if any variables are missing, the user is
// prompted to enter the missing values. In a production environment, if required
// variables are not set, the function returns an error, indicating that the
// configuration is incomplete and the service should not start until the issue
// is resolved.
func Initialize(spec interface{}) error {
	return environment.Initialize(spec, runningInProduction())
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
		internal: &internal{
			cancel: cancel,
			config: &config,
		},
	}

	// Initialize the logger
	if err := s.initializeLogger(); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	s.Log.Error("Just testing an error", slog.Any("error", errors.New("My Error")))
	// Load the credentials
	var err error
	s.GoogleCredentials, err = gcpcredentials.NewCredentials(ctx, gcpcredentials.Config{
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

// Run starts the service and blocks, waiting for an OS signal, context cancellation, or an error.
// Lifecycle callbacks from the State struct are invoked at each stage:
// - `Starting`: Called when the service starts.
// - `Running`: Called when the service is running.
// - `Terminating`: Called during shutdown, with an error if one triggered the termination.
//
// The function returns only after the service has gracefully shut down.
func (s *Service) Run(state State) {

	if state.Starting != nil {
		state.Starting()
	}

	// Start the auth service
	if s.internal.auth != nil {
		go s.startAuthService()
	}

	// Set up a channel to listen for the terminate signals from the OS
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(terminate)

	// Block until we get a terminate signal, or the context is canceled
	if state.Running != nil {
		state.Running()
	}
	select {
	case <-terminate:
		if state.Terminating != nil {
			state.Terminating(nil)
		}
	case <-s.Context.Done():
		if state.Terminating != nil {
			state.Terminating(nil)
		}
	case err := <-s.internal.router.ListenAndServe():
		if state.Terminating != nil {
			state.Terminating(err)
		}
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
			s.Log.Error("teardown completed with an error", slog.Any("error", err))
		}
	}
}

// Shutdown initiates an immediate graceful shutdown by canceling the service's context,
// signaling all components to stop their operations. This method triggers the shutdown
// process but does not block or wait for the service to fully stop.
func (s *Service) Shutdown() {
	s.internal.cancel()
}

// Config returns the current configuration of the service.
// It provides access to the internal configuration stored in the service.
func (s *Service) Config() *Config {
	return s.internal.config
}

// AddAuthProvider initializes the authentication provider for the service.
func (s *Service) AddAuthProvider(authProvider auth.AuthProvider) error {
	var err error
	s.internal.auth, err = auth.New(s.Context, auth.Config{
		AuthProvider: authProvider,
	})
	return err
}

// AddAuthenticatedEndpoint registers an HTTP endpoint that requires authentication
// and optionally authorization. It first authenticates the request, and if authorization
// requirements (roles or permissions) are provided, it checks them before passing
// the request to the handler.
// If the service was initialized without an AuthProvider, it logs a fatal error and exits.
// If authentication fails, a 401 Unauthorized response is returned. If authorization
// requirements are provided and the request fails authorization, a 403 Forbidden response is returned.
// In case of an internal error during processing, a 500 Internal Server Error is returned.
func (s *Service) AddAuthenticatedEndpoint(method, relativePath string, handler func(*Service, *http.Request) *HTTPResponse, authRequirements ...auth.AuthRequirements) {

	// Confirm an AuthProvider exists
	if s.internal.auth == nil {
		s.Log.Error("AddAuthenticatedEndpoint requires the service to be initialized with an AuthProvider")
		os.Exit(1)
	}

	// Middleware to wrap the handler for request authentication. It authenticates the request,
	// injects the relevant service into the handler, and manages the process of sending the response.
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		// Authenticate the request
		authenticated, reason, err := s.internal.auth.Authenticate(r)
		if err != nil {
			s.Log.Error("failed to authenticated request", slog.Any("error", err))
			sendResponse(w, 500, "internal server error")
			return
		}
		if !authenticated {
			message := "unauthorized"
			if reason != "" {
				message += ": " + reason
			}
			sendResponse(w, 401, message)
			return
		}

		// Authorize the request
		requirements := auth.AuthRequirements{}
		for _, r := range authRequirements {
			requirements.AnyRole = append(requirements.AnyRole, r.AnyRole...)
			requirements.AllPermissions = append(requirements.AllPermissions, r.AllPermissions...)
		}
		authorized, err := s.internal.auth.Authorize(r, requirements)
		if err != nil {
			s.Log.Error("failed to authorize request", slog.Any("error", err))
			sendResponse(w, 500, "internal server error")
			return
		}
		if !authorized {
			sendResponse(w, 403, "forbidden")
			return
		}

		// Send the request to the handler and handle the response
		resp := handler(s, r)
		if resp == nil {
			sendResponse(w, 500, "internal server error")
			return
		}
		if err := router.SendResponse(w, resp.StatusCode, resp.Headers, resp.Body); err != nil {
			s.Log.Error("failed to send response", slog.Any("error", err))
		}
	}

	// Register the wrapped handler to the router to handle requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.RegisterHandler(method, relativePath, wrappedHandler); err != nil {
		s.Log.Error("failed to register handler", slog.Any("error", err), slog.Any("method", method), slog.Any("relative_path", relativePath))
		os.Exit(1)
	}
}

// AddCloudTaskEndpoint registers a new POST endpoint at the specified relativePath to handle
// incoming Google Cloud Tasks. In production, it verifies the authenticity of the request,
// while in local or non-production environments, request verification is skipped.
func (s *Service) AddCloudTaskEndpoint(relativePath string, handler func(*Service, *http.Request) error) {

	// wrappedHandler is the middleware that processes the incoming request.
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {

		// Verify the request if running in a production environment.
		// This step ensures that the request comes from Google Cloud Tasks.
		if runningInProduction() {
			if err := verifyGoogleRequest(s.Context, r); err != nil {
				// Respond with a 403 Forbidden status if verification fails.
				sendResponse(w, http.StatusForbidden, "forbidden")
				return
			}
		}

		// Invoke the provided handler function with the request.
		// If the handler returns an error, log it and respond with a 500 Internal Server Error status.
		if err := handler(s, r); err != nil {
			s.Log.Error("failed to handle request", slog.Any("error", err))
			sendResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// If the handler succeeds, respond with a 200 OK status.
		sendResponse(w, http.StatusOK, "OK")
	}

	// Register the wrapped handler to the router to handle POST requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.RegisterHandler("POST", relativePath, wrappedHandler); err != nil {
		s.Log.Error("failed to add Cloud Task", slog.Any("error", err), slog.Any("relative_path", relativePath))
	}
}

// AddCloudSchedulerEndpoint registers a new POST endpoint at the specified relativePath to handle
// incoming Google Cloud Scheduler requests. In production, it verifies the authenticity of the
// request, while in local or non-production environments, request verification is skipped.
func (s *Service) AddCloudSchedulerEndpoint(relativePath string, handler func(*Service, *http.Request) error) {

	// wrappedHandler is the middleware that processes the incoming request.
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {

		// Verify the request if running in a production environment.
		// This step ensures that the request comes from Google Cloud Scheduler.
		if runningInProduction() {
			if err := verifyGoogleRequest(s.Context, r); err != nil {
				// Respond with a 403 Forbidden status if verification fails.
				sendResponse(w, http.StatusForbidden, "forbidden")
				return
			}
		}

		// Invoke the provided handler function with the request.
		// If the handler returns an error, log it and respond with a 500 Internal Server Error status.
		if err := handler(s, r); err != nil {
			s.Log.Error("failed to handle request", slog.Any("error", err))
			sendResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// If the handler succeeds, respond with a 200 OK status.
		sendResponse(w, http.StatusOK, "OK")
	}

	// Register the wrapped handler to the router to handle POST requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.RegisterHandler("POST", relativePath, wrappedHandler); err != nil {
		s.Log.Error("failed to add Cronjob", slog.Any("error", err), slog.Any("relative_path", relativePath))
		os.Exit(1)

	}
}

// AddPublicEndpoint registers a new HTTP endpoint with the specified method (e.g., "GET", "POST")
// and relative path. It wraps the provided handler function so that the current Service
// instance is passed into the handler when the endpoint is invoked.
// This endpoint does not require authentication.
// If an error occurs while registering the endpoint, the function will log the error
// and terminate the program.
func (s *Service) AddPublicEndpoint(method, relativePath string, handler func(*Service, *http.Request) *HTTPResponse) {

	// Wrap the handler, so we can pass the service to it and handle sending the response
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		resp := handler(s, r)
		if resp == nil {
			resp = Text(500, "internal server error")
		}
		if err := router.SendResponse(w, resp.StatusCode, resp.Headers, resp.Body); err != nil {
			s.Log.Error("failed to send response", slog.Any("error", err))
		}
	}

	// Register the wrapped handler to the router to handle requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.RegisterHandler(method, relativePath, wrappedHandler); err != nil {
		s.Log.Error("failed to register handler", slog.Any("error", err), slog.Any("method", method), slog.Any("relative_path", relativePath))
		os.Exit(1)
	}
}

// AddServiceEndpoint registers an HTTP endpoint that requires authentication and is restricted to service requests only.
// It first authenticates the request, ensuring that only valid credentials are allowed, and then verifies
// that the request comes specifically from a trusted service before invoking the handler.
//
// If the service was initialized without an AuthProvider, it logs a fatal error and exits.
// If authentication fails, a 401 Unauthorized response is returned. If the request is not verified as coming
// from a service, a 403 Forbidden response is returned indicating that access is restricted to services.
//
// Optionally, authorization requirements (roles or permissions) can be specified and are checked after
// authentication. If the authorization requirements are not met, a 403 Forbidden response is returned.
//
// The handler function receives the Service instance and the HTTP request, and returns an HTTPResponse.
// In case of an internal error during processing, a 500 Internal Server Error is returned.
// This endpoint is intended for use by other services and ensures only authenticated and verified service requests
// are permitted.
func (s *Service) AddServiceEndpoint(method, relativePath string, handler func(*Service, *http.Request) *HTTPResponse, authRequirements ...auth.AuthRequirements) {

	// Confirm an AuthProvider exists
	if s.internal.auth == nil {
		s.Log.Error("AddServiceEndpoint requires the service to be initialized with an AuthProvider")
		os.Exit(1)
	}

	// Middleware to wrap the handler for request authentication. It authenticates the request,
	// injects the relevant service into the handler, and manages the process of sending the response.
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {
		// Authenticate the request
		authenticated, reason, err := s.internal.auth.Authenticate(r)
		if err != nil {
			s.Log.Error("failed to authenticated request", slog.Any("error", err))
			sendResponse(w, 500, "internal server error")
			return
		}
		if !authenticated {
			message := "unauthorized"
			if reason != "" {
				message += ": " + reason
			}
			sendResponse(w, 401, message)
			return
		}

		// Verify the request is from a service
		if isVerified := s.internal.auth.IsServiceRequest(r); !isVerified {
			sendResponse(w, 403, "forbidden: restricted to services")
			return
		}

		// Authorize the request
		requirements := auth.AuthRequirements{}
		for _, r := range authRequirements {
			requirements.AnyRole = append(requirements.AnyRole, r.AnyRole...)
			requirements.AllPermissions = append(requirements.AllPermissions, r.AllPermissions...)
		}
		authorized, err := s.internal.auth.Authorize(r, requirements)
		if err != nil {
			s.Log.Error("failed to authorize request", slog.Any("error", err))
			sendResponse(w, 500, "internal server error")
			return
		}
		if !authorized {
			sendResponse(w, 403, "forbidden")
			return
		}

		// Send the request to the handler and handle the response
		resp := handler(s, r)
		if resp == nil {
			sendResponse(w, 501, "internal server error")
			return
		}
		if err := router.SendResponse(w, resp.StatusCode, resp.Headers, resp.Body); err != nil {
			s.Log.Error("failed to send response", slog.Any("error", err))
		}
	}

	// Register the wrapped handler to the router to handle requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.RegisterHandler(method, relativePath, wrappedHandler); err != nil {
		s.Log.Error("failed to register handler", slog.Any("error", err), slog.Any("method", method), slog.Any("relative_path", relativePath))
		os.Exit(1)
	}
}

// AddPubSubEndpoint registers a new POST endpoint at the specified relativePath to handle incoming
// Pub/Sub messages. In production, it verifies the authenticity of the request, while in
// local or non-production environments, request verification is skipped. The function
// decodes the Pub/Sub message and invokes the provided handler function with the decoded message.
func (s *Service) AddPubSubEndpoint(relativePath string, handler func(*Service, PubSubMessage) error) {

	// wrappedHandler is the middleware that processes the incoming request.
	wrappedHandler := func(w http.ResponseWriter, r *http.Request) {

		// Verify the request if running in a production environment.
		// This step ensures that the request comes from Google Pub/Sub.
		if runningInProduction() {
			if err := pubsub.ValidateGooglePubSubRequest(s.Context, r, ""); err != nil {
				// Respond with a 403 Forbidden status if verification fails.
				sendResponse(w, http.StatusForbidden, "forbidden")
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

		// If the JSON unmarshalling fails, log the error and respond with a 400 Bad Request status.
		if err := UnmarshalJSONBody(r, &envelope); err != nil {
			s.Log.Error("failed to decode message envelope", slog.Any("error", err))
			sendResponse(w, http.StatusBadRequest, "bad request")
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
			s.Log.Error("failed to decode message data", slog.Any("error", err))
			sendResponse(w, http.StatusBadRequest, "bad request")
			return
		}

		// Invoke the provided handler function with the decoded Pub/Sub message.
		// If the handler returns an error, log it and respond with a 500 Internal Server Error status.
		if err := handler(s, message); err != nil {
			s.Log.Error("failed to handle message", slog.Any("error", err))
			sendResponse(w, http.StatusInternalServerError, "internal server error")
			return
		}

		// If the handler succeeds, respond with a 200 OK status.
		sendResponse(w, http.StatusOK, "OK")
	}

	// Register the wrapped handler to the router to handle POST requests on the given relativePath.
	// Log a fatal error if the handler registration fails.
	if err := s.internal.router.RegisterHandler("POST", relativePath, wrappedHandler); err != nil {
		s.Log.Error("failed to register handler", slog.Any("error", err), slog.Any("relative_path", relativePath))
		os.Exit(1)
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
	client, err := s.internal.auth.NewAuthClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, nil
}

// GenerateGoogleIDToken generates a Google ID token for a given audience.
// It uses a service account to create the token, either by impersonating the account
// in non-production environments or by querying the metadata server in production.
func (s *Service) GenerateGoogleIDToken(audience string) (string, error) {

	// Ensure a service account is configured
	if len(s.internal.config.ServiceAccount) == 0 {
		return "", errors.New("GenerateGoogleIDToken requires a service account to be configured")
	}

	// Validate that an audience is provided
	if len(audience) == 0 {
		return "", errors.New("audience is required")
	}

	// If not running in production, use the IAM client to impersonate the service account
	if !runningInProduction() {
		if s.IAMClient == nil {
			return "", errors.New("IAMClient is not initialized")
		}

		// Generate ID token using the IAM client.
		idTokenResp, err := s.IAMClient.GenerateIdToken(context.Background(), &credentialspb.GenerateIdTokenRequest{
			Name:         fmt.Sprintf("projects/-/serviceAccounts/%s", s.internal.config.ServiceAccount),
			Audience:     audience,
			IncludeEmail: true,
		})
		if err != nil {
			return "", fmt.Errorf("failed to generate ID token: %w", err)
		}

		// Return the generated ID token.
		return idTokenResp.Token, nil
	}

	// In production, retrieve the ID token from the metadata server.
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience="+url.QueryEscape(audience), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create metadata server request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve ID token from metadata server: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	idToken := string(body)

	// Parse the ID token without validating it, to extract claims.
	token, _, err := new(jwt.Parser).ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse ID token: %w", err)
	}

	// Extract and validate claims from the token, especially the email claim.
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("failed to parse claims from ID token")
	}
	email, ok := claims["email"].(string)
	if !ok || len(email) == 0 {
		return "", errors.New("ID token does not contain an email claim")
	}

	// Validate that the email matches the configured service account.
	if strings.ToLower(s.internal.config.ServiceAccount) != strings.ToLower(email) {
		return "", errors.New("service account email does not match the configured service account")
	}

	return idToken, nil
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

// PublishToPubSub sends a message to the specified Pub/Sub topic.
// It returns the message ID or an error if the operation fails.
func (s *Service) PublishToPubSub(topic string, message interface{}) (string, error) {
	return s.internal.pubsub.Publish(topic, message)
}
