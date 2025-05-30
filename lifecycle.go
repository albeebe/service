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
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"cloud.google.com/go/cloudsqlconn/mysql/mysql"
	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/storage"
	"github.com/albeebe/service/pkg/logger"
	"github.com/albeebe/service/pkg/pubsub"
	"github.com/albeebe/service/pkg/router"
	"google.golang.org/api/option"
)

// initializeLogger sets up the structured logger for the service, configuring it based on the environment.
// In production, it uses Google Cloud Logging with the Info level to capture operational logs.
// In development, it defaults to a local console logger with the Debug level for more verbose output.
func (s *Service) initializeLogger() error {
	var err error

	// Choose logger configuration based on the environment
	if runningInProduction() {
		// Set up Google Cloud logging for production
		s.Log, err = logger.NewGoogleCloudLogger(s.Context, logger.Config{
			GCPProjectID: s.internal.config.GCPProjectID,
			LogName:      "service-log",
			Level:        slog.LevelInfo, // Use Info level for production
		})
	} else {
		// Set up development logging for non-production environments
		s.Log, err = logger.NewDevelopmentLogger(s.Context, logger.Config{
			Level: slog.LevelDebug, // Use Debug level for development
		})
	}

	return err
}

// setup initializes various components of the service concurrently to enhance performance.
func (s *Service) setup() error {

	// Define the components we want to set up
	type Component struct {
		Name     string
		Function func() error
	}
	components := []Component{
		{"Cloud SQL", s.setupCloudSQL},
		{"Cloud Storage", s.setupCloudStorage},
		{"Cloud Tasks", s.setupCloudTasks},
		{"IAM Client", s.setupIAMClient},
		{"Pub/Sub", s.setupPubSub},
		{"Router", s.setupRouter},
	}

	// Set up the various components concurrently to enhance performance
	wg := sync.WaitGroup{}
	errCh := make(chan error, len(components))
	for _, component := range components {
		wg.Add(1)
		go func(c Component) {
			defer wg.Done()
			if err := c.Function(); err != nil {
				errCh <- fmt.Errorf("failed to set up %s: %w", c.Name, err)
			}
		}(component)
	}

	// Wait for the components to finish setting up
	go func() {
		wg.Wait()
		close(errCh)
	}()
	var finalErr error
	for err := range errCh {
		if err != nil {
			if finalErr == nil {
				finalErr = err
			}
		}
	}

	return finalErr
}

// setupCloudSQL initializes the Cloud SQL database connection using the provided configuration.
// If the Cloud SQL connection string is not configured (i.e., empty), the function skips the
// database setup and returns early with no error, as Cloud SQL is considered optional.
func (s *Service) setupCloudSQL() (err error) {

	// Return early if Cloud SQL connection is not configured
	if s.internal.config.CloudSQLConnection == "" {
		return nil
	}

	// Set up the driver
	mysqlDriver := "mysql-driver"
	_, err = mysql.RegisterDriver(mysqlDriver)

	// Open the connection to the database
	dsn := fmt.Sprintf("%s:@%s(%s)/%s?parseTime=true", s.internal.config.CloudSQLUser, mysqlDriver, s.internal.config.CloudSQLConnection, s.internal.config.CloudSQLDatabase)
	s.DB, err = sql.Open(mysqlDriver, dsn)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	// Verify the connection to the database
	if err := s.DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

// setupCloudStorage creates a new Cloud Storage client using the specified Google credentials.
func (s *Service) setupCloudStorage() (err error) {
	opts := []option.ClientOption{
		option.WithCredentials(s.GoogleCredentials),
	}
	s.CloudStorageClient, err = storage.NewClient(s.Context, opts...)
	return err
}

// setupCloudTasks initializes the Cloud Tasks client for the service.
func (s *Service) setupCloudTasks() (err error) {
	s.CloudTasksClient, err = cloudtasks.NewClient(s.Context)
	return err
}

// setupIAMClient initializes the IAM (Identity and Access Management) client for the service.
func (s *Service) setupIAMClient() (err error) {
	s.IAMClient, err = credentials.NewIamCredentialsClient(s.Context)
	return err
}

// setupPubSub creates a new Pub/Sub client for the service using the provided GCP project ID.
func (s *Service) setupPubSub() (err error) {
	s.internal.pubsub, err = pubsub.New(s.Context, pubsub.Config{
		GCPProjectID: s.internal.config.GCPProjectID,
	})
	return err
}

// setupRouter initializes the HTTP router for the service.
func (s *Service) setupRouter() (err error) {

	noRouteHandler := func(w http.ResponseWriter, r *http.Request) {
		sendResponse(w, http.StatusNotFound, "not found")
	}

	s.internal.router, err = router.NewRouter(s.Context, router.Config{
		Host:           s.internal.config.Host,
		NoRouteHandler: &noRouteHandler,
		Cors: &router.Cors{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"*"},
			AllowHeaders:     []string{"*"},
			ExposeHeaders:    []string{"*"},
			AllowCredentials: true,
			MaxAge:           time.Hour,
		},
	})
	return err
}

// startAuthService starts the auth service and blocks, listening for errors
// and logging them. It only returns when the service's context is canceled
// and should be run in a goroutine.
func (s *Service) startAuthService() {
	for {
		select {
		case err := <-s.internal.auth.Start():
			if err != nil {
				s.Log.Error("failed to start auth service", slog.Any("error", err))
			}
		case <-s.Context.Done():
			return
		}
	}
}

// teardown gracefully shuts down multiple service components concurrently within a specified timeout.
// If the process exceeds the timeout, it stops waiting and returns. Any errors encountered are collected,
// with the first error being returned, if any.
func (s *Service) teardown(timeout time.Duration) error {

	// Define the components we want to tear down
	type Component struct {
		Name     string
		Function func() error
	}
	components := []Component{
		{"Router", s.teardownRouter},
	}

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	wg := sync.WaitGroup{}
	errCh := make(chan error, len(components))

	// Launch the teardown process for each component in a separate goroutine
	for _, component := range components {
		wg.Add(1)
		go func(c Component) {
			defer wg.Done()
			if err := c.Function(); err != nil {
				select {
				case errCh <- fmt.Errorf("failed to tear down %s: %w", c.Name, err):
				case <-ctx.Done():
					return
				}
			}
		}(component)
	}

	// Close the error channel when all goroutines are done
	go func() {
		wg.Wait()
		close(errCh)
	}()

	var finalErr error

	// Wait until either the timeout occurs or all components have finished tearing down, whichever happens first
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		finalErr = err
	}

	// Drain the remaining errors from the error channel
	for err := range errCh {
		if err != nil && finalErr == nil {
			finalErr = err
		}
	}

	// Teardown CloudSQL followed by flushing the logger.
	// We delay tearing down the CloudSQL (database) and flushing the logger until last
	// because other components may still require access to the database or logging
	// capabilities during their own teardown processes. By shutting down the database
	// and flushing logs last, we ensure that any necessary resources remain available
	// and that all logged messages are written out before completing the overall shutdown.
	if err := s.teardownCloudSQL(); err != nil {
		s.Log.Error("failed to tear down CloudSQL", slog.Any("error", err))
	}
	if err := s.flushLogger(); err != nil {
		s.Log.Error("failed to flush the logger", slog.Any("error", err))
	}

	return finalErr
}

// teardownCloudSQL gracefully closes the Cloud SQL database connection if it is open.
func (s *Service) teardownCloudSQL() (err error) {
	if s.DB != nil {
		if err := s.DB.Close(); err != nil {
			return err
		}
		s.DB = nil
	}
	return nil
}

// flushLogger ensures that all pending log entries are flushed to their destination
// (such as Google Cloud Logging)
func (s *Service) flushLogger() (err error) {
	if s.Log != nil {
		err = logger.FlushLogger(s.Log)
	}
	return
}

// teardownRouter gracefully shuts down the router, immediately stopping it from accepting
// new incoming connections while allowing existing connections to complete before returning.
func (s *Service) teardownRouter() (err error) {
	if s.internal.router != nil {
		return s.internal.router.Shutdown()
	}
	return nil
}
