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
	"io"
	"log/slog"
	"net/http"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/storage"
	"github.com/albeebe/service/pkg/auth"
	"github.com/albeebe/service/pkg/pubsub"
	"github.com/albeebe/service/pkg/router"
	"golang.org/x/oauth2/google"
)

type EndpointHandler func(*Service, *http.Request) *HTTPResponse

type PubSubHandler func(*Service, PubSubMessage) error

type Service struct {
	Context            context.Context
	CloudStorageClient *storage.Client
	CloudTasksClient   *cloudtasks.Client
	GoogleCredentials  *google.Credentials
	IAMClient          *credentials.IamCredentialsClient
	DB                 *sql.DB
	Log                *slog.Logger
	Name               string
	internal           *internal
}

type Config struct {
	CloudSQLConnection string // Cloud SQL instance connection string in the format "project:region:instance"
	CloudSQLDatabase   string // Name of the specific database within the Cloud SQL instance
	CloudSQLUser       string // Username for accessing the Cloud SQL database
	GCPProjectID       string // Google Cloud Platform Project ID where the service is deployed
	Host               string // The host address where the service listens for incoming requests (e.g., ":8080")
	ServiceAccount     string // Service account email used for authentication with GCP resources
}

type HTTPResponse struct {
	StatusCode int           // The HTTP status code of the response (e.g., 200, 404)
	Headers    http.Header   // The headers of the HTTP response (e.g., Content-Type, Set-Cookie)
	Body       io.ReadCloser // The response body, allowing streaming of the content and efficient memory usage
}

type PubSubMessage struct {
	ID        string    `json:"id"`        // Unique identifier for the message.
	Published time.Time `json:"published"` // Time the message was published.
	Data      []byte    `json:"data"`      // Data payload of the message as a byte slice.
}

type State struct {
	Starting    func()          // Called when the service is starting
	Running     func()          // Called when the service is running
	Terminating func(err error) // Called when the service is terminating, with an optional error if it was due to a failure
}

type internal struct {
	auth   *auth.Auth
	cancel context.CancelFunc
	config *Config
	pubsub *pubsub.PubSub
	router *router.Router
}

// validate checks the Config struct for required fields and
// returns an error if any required fields are missing
func (config *Config) validate() error {

	if config.CloudSQLConnection != "" {
		if config.CloudSQLDatabase == "" {
			return fmt.Errorf("CloudSQLDatabase must be provided when CloudSQLConnection is specified")
		} else if config.CloudSQLUser == "" {
			return fmt.Errorf("CloudSQLUser must be provided when CloudSQLConnection is specified")
		}
	}

	if config.CloudSQLDatabase != "" {
		if config.CloudSQLConnection == "" {
			return fmt.Errorf("CloudSQLConnection must be provided when CloudSQLDatabase is specified")
		} else if config.CloudSQLUser == "" {
			return fmt.Errorf("CloudSQLUser must be provided when CloudSQLDatabase is specified")
		}
	}

	if config.CloudSQLUser != "" {
		if config.CloudSQLConnection == "" {
			return fmt.Errorf("CloudSQLConnection must be provided when CloudSQLUser is specified")
		} else if config.CloudSQLDatabase == "" {
			return fmt.Errorf("CloudSQLDatabase must be provided when CloudSQLUser is specified")
		}
	}

	if config.GCPProjectID == "" {
		return fmt.Errorf("GCPProjectID is empty")
	}

	if config.Host == "" {
		return fmt.Errorf("Host is empty")
	}

	if config.ServiceAccount == "" {
		return fmt.Errorf("ServiceAccount is empty")
	}

	return nil
}
