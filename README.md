# Service Library for Go

`service` is a Go library built to streamline the development of services on Google Cloud Platform. By minimizing boilerplate code and automating much of the setup, it allows you to focus on your business logic without getting bogged down in infrastructure details. The result? A clean, readable main.go file that acts like an index, offering a clear and concise overview of your service — self-documenting and easy to maintain. Whether you're setting up authentication, Pub/Sub, or handling Cloud Tasks, this library is designed to make your services simple, scalable, and production-ready from the start.

## Features

- **Simplified Initialization**: Load environment variables and configurations with ease, handling both local development and production environments gracefully.
- **Streamlined Service Setup**: Quickly set up common GCP services like Cloud SQL, Pub/Sub, and authentication with minimal code.
- **Dependency Injection**: Access shared resources like databases, logs, and Google Cloud clients through dependency injection, promoting cleaner code and easier testing.
- **Clean Entry Point**: Keep your `main.go` file concise and readable, resembling an index that outlines the service's structure.
- **Built-in Middleware**: Includes authentication and authorization middleware, request handling, and error management out of the box.
- **Production-Ready**: Designed with production best practices, including graceful shutdowns, context handling, and proper error logging.

## Getting Started

### Installation

```bash
go get github.com/albeebe/service
```

### Basic Usage

Here's how you can set up a simple service using this library:

```go
package main

import (
    "net/http"

    "github.com/albeebe/service"
)

// All the environment variables are defined here
type EnvVars struct {
  GCP_PROJECT_ID       string `default:"my-project"`
  SERVICE_ACCOUNT      string `default:"my-service@my-project.iam.gserviceaccount.com"`
  CLOUD_SQL_CONNECTION string `default:"my-service:us-east4:shared"`
  CLOUD_SQL_DATABASE   string `default:"my-database"`
  CLOUD_SQL_USER       string `default:"my-user"`
  HOST                 string `default:":8080"`
}

func main() {

  // Load all the environment variables and confirm they're set
  env := EnvVars{}
  if err := service.Initialize(&env); err != nil {
    panic(err)
  }

  // Create the service
  s, err := service.New("my-service", service.Config{
    CloudSQLConnection: env.CLOUD_SQL_CONNECTION,
    CloudSQLDatabase:   env.CLOUD_SQL_DATABASE,
    CloudSQLUser:       env.CLOUD_SQL_USER,
    GCPProjectID:       env.GCP_PROJECT_ID,
    Host:               env.HOST,
    ServiceAccount:     env.SERVICE_ACCOUNT,
  })
  if err != nil {
    panic(fmt.Errorf("failed to create service with err: %s", err.Error()))
  }

  // Add an auth provider to handle authentication
  s.AddAuthProvider(authprovider.New(s))

  // Add endpoints that do not require authentication
  s.AddPublicEndpoint("GET", "/", endpoints.GetRoot)

  // Add endpoints that require authentication
  s.AddAuthenticatedEndpoint("GET", "/authenticated", endpoints.GetAuthenticated)
  s.AddAuthenticatedEndpoint("GET", "/role", endpoints.GetRole, auth.AnyRole("viewer", "editor"))
  s.AddAuthenticatedEndpoint("GET", "/permissions", endpoints.GetPermissions, auth.AllPermissions("project.create"))

  // Add websocket endpoints
  s.AddWebsocket("/websocket", endpoints.Websocket)
  
  // Add endpoints that only services can access
  s.AddServiceEndpoint("GET", "/service", endpoints.GetService)

  // Add endpoints for Pub/Sub subscriptions
  s.AddPubSubEndpoint("_pubsub/demo", pubsub.Demo)

  // Add endpoints for Cloud Tasks
  s.AddCloudTaskEndpoint("/_tasks/demo", tasks.Demo)

  // Add endpoints for Cloud Scheduler
  s.AddCloudSchedulerEndpoint("_scheduled/demo", scheduled.Demo)

  // Begin accepting requests. Blocks until the service terminates.
  s.Run(service.State{
    Starting: func() {
      s.Log.Info("Service starting...")
    },
    Running: func() {
      s.Log.Info("Service running...")
    },
    Terminating: func(err error) {
      if err != nil {
        s.Log.Info("Service terminating with error...", slog.String("error", err.Error()))
      } else {
        s.Log.Info("Service terminating...")
      }
    },
  })
}
```

### Explanation

- **Configuration**: Define all necessary configurations in one place.
- **Service Initialization**: Initialize your service with `service.New`, handling all setup internally.
- **Dependency Injection**: Access shared resources like the database (`s.DB`), logger (`s.Log`), and Google Cloud clients directly from the service instance.
- **Adding Endpoints**: Use methods like `AddPublicEndpoint` and `AddAuthenticatedEndpoint` to register handlers.
- **Running the Service**: Call `Run` with lifecycle callbacks for starting, running, and terminating states.

## Detailed Features

### Simplified Initialization

The `Initialize` function loads environment variables based on a provided specification. It prompts for missing variables during local development and ensures all required variables are set in production.

```go
func Initialize(spec interface{}) error
```

### Service Creation

Create a new service instance with `New`, which handles configuration validation and sets up GCP credentials.

```go
func New(serviceName string, config Config) (*Service, error)
```

### Dependency Injection

The library utilizes dependency injection to provide access to shared resources throughout your application. This includes:

- **Database Connection (`s.DB`)**: Access your Cloud SQL database connection.
- **Logger (`s.Log`)**: Use the built-in structured logger for consistent logging.
- **Google Cloud Clients**: Access clients for services like Pub/Sub, Cloud Storage, and Cloud Tasks.
- **Context (`s.Context`)**: Use the service's context for cancellation and timeout handling.

This approach promotes cleaner code by avoiding global variables and making it easier to write unit tests.

### Clean `main.go`

By abstracting away the boilerplate, your `main.go` remains clean and self-documenting, making it easy to understand the service structure at a glance.

### Adding Endpoints

- **Public Endpoints**: For handlers that don't require authentication.

  ```go
  func (s *Service) AddPublicEndpoint(method, path string, handler func(*Service, *http.Request) *HTTPResponse)
  ```

- **Authenticated Endpoints**: For handlers that require authentication and optional authorization.

  ```go
  func (s *Service) AddAuthenticatedEndpoint(method, path string, handler func(*Service, *http.Request) *HTTPResponse, authRequirements ...auth.AuthRequirements)
  ```

- **Websocket Endpoints**: For handlers that want to automatically upgrade HTTP requests to WebSocket connections.

  ```go
  func (s *Service) AddWebsocketEndpoint(relativePath string, handler func(*Service, *websocket.Conn))
  ```
  
- **Service Endpoints**: For internal service-to-service communication with strict authentication.

  ```go
  func (s *Service) AddServiceEndpoint(method, path string, handler func(*Service, *http.Request) *HTTPResponse, authRequirements ...auth.AuthRequirements)
  ```

- **Cloud Task Endpoints**: Specifically for handling Cloud Tasks.

  ```go
  func (s *Service) AddCloudTaskEndpoint(path string, handler func(*Service, *http.Request) error)
  ```

- **Cloud Scheduler Endpoints**: For handling scheduled tasks via Cloud Scheduler.

  ```go
  func (s *Service) AddCloudSchedulerEndpoint(path string, handler func(*Service, *http.Request) error)
  ```

- **Pub/Sub Endpoints**: For processing Pub/Sub messages.

  ```go
  func (s *Service) AddPubSubEndpoint(path string, handler func(*Service, PubSubMessage) error)
  ```

### Authentication and Authorization

Set up authentication providers and middleware effortlessly.

```go
func (s *Service) AddAuthProvider(authProvider auth.AuthProvider) error
```

### Accessing Shared Resources

Access various clients and utilities provided by the service instance:

- **Database Client**:

  ```go
  db := s.DB
  ```

- **Logger**:

  ```go
  s.Log.Info("Logging information.")
  ```

- **Pub/Sub Client**:

  ```go
  messageID, err := s.PublishToPubSub("topic-name", messageData)
  ```

- **Cloud Storage Client**:

  ```go
  storageClient := s.CloudStorageClient
  ```

- **Cloud Tasks Client**:

  ```go
  tasksClient := s.CloudTasksClient
  ```

### Graceful Shutdown

Handles OS signals and context cancellations to terminate the service gracefully.

### Utility Functions

- **Response Helpers**:

  ```go
  func Text(statusCode int, text string) *HTTPResponse
  func JSON(statusCode int, obj interface{}) *HTTPResponse
  ```

- **Request Parsing**:

  ```go
  func UnmarshalJSONBody(r *http.Request, target interface{}) error
  ```

### Example with Dependency Injection

```go
func endpointHandler(s *service.Service, r *http.Request) *service.HTTPResponse {
    // Access the database
    db := s.DB
    // Use the database connection
    rows, err := db.Query("SELECT * FROM data_table")
    if err != nil {
        s.Log.Error("Database query failed.", err)
        return service.InternalServerError()
    }
    defer rows.Close()

    // Process data...
    data := []DataModel{}
    for rows.Next() {
        var item DataModel
        if err := rows.Scan(&item.ID, &item.Value); err != nil {
            s.Log.Error("Row scan failed.", err)
            return service.InternalServerError()
        }
        data = append(data, item)
    }

    // Log the operation
    s.Log.Info("Data retrieved successfully.")

    // Return the response
    return service.JSON(http.StatusOK, data)
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
