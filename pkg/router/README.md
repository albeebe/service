# router

`router` is a Go package that provides an HTTP/2-enabled router with CORS support and a customizable 404 handler. The package leverages the [Gin framework](https://gin-gonic.com/) internally to handle routing and middleware. However, Gin is not exposed externally, which allows for the easy replacement of the underlying framework in the future without affecting the external API of the package.

## Features

- **HTTP/2 Support**
  - The router comes with HTTP/2 support out of the box, using the `h2c` package to provide cleartext HTTP/2 handling.
- **CORS Support**
  - CORS policies are configurable via the `Config` struct, allowing control over origins, headers, methods, and credentials.
- **Graceful Shutdown**
  - The router listens to context cancellation signals to shut down gracefully, ensuring ongoing connections are closed properly.
- **Custom 404 Handler**
  - You can define a custom handler for undefined routes, allowing more control over the application's behavior when a route is not found.

## Installation

```bash
go get github.com/albeebe/service/pkg/router
```

## Usage

### Create a New Router

To create and configure a new router, use the `NewRouter` function. It requires a `context.Context` for managing the lifecycle and a `Config` struct for customization.

```go
ctx := context.Background()

config := Config{
    Host: "localhost:8080",
    Cors: &Cors{
        AllowOrigins: []string{"*"},
        AllowMethods: []string{"GET", "POST"},
    },
}

router, err := NewRouter(ctx, config)
if err != nil {
    log.Fatalf("failed to create router: %v", err)
}
```

### Register Routes

You can register route handlers using the `RegisterHandler` function, which takes an HTTP method, path, and handler function. The handler function is a standard `http.HandlerFunc`.

```go
router.RegisterHandler("GET", "/ping", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("pong"))
})
```

### Start the Server

To start the server, call the `ListenAndServe` method. This will start the server in a separate goroutine and return a channel for capturing any errors.

```go
errChan := router.ListenAndServe()

// Capture any errors
if err := <-errChan; err != nil {
    log.Fatalf("server error: %v", err)
}
```

### Send HTTP Responses

You can use the `SendResponse` helper function to send responses to clients, including setting headers and streaming the body.

```go
headers := http.Header{
    "Content-Type": []string{"application/json"},
}
body := io.NopCloser(strings.NewReader(`{"message": "Hello, World!"}`))

SendResponse(w, http.StatusOK, headers, body)
```

### Graceful Shutdown

The router listens for context cancellation to shut down the server gracefully. This happens automatically when the context passed to `NewRouter` is canceled.

To explicitly shut down the router, you can call:

```go
router.Shutdown()
```

## Internals

This package uses the Gin framework internally for routing and middleware but does not expose Gin-specific functionality directly. This abstraction allows flexibility for future changes, such as swapping Gin for another framework, without affecting the router's public API.

## Example

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"github.com/albeebe/service/pkg/router"
)

func main() {
    ctx := context.Background()
    
    // Define a custom NoRouteHandler
    noRouteHandler := func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/plain")
        w.WriteHeader(http.StatusNotFound)
        w.Write([]byte("not found"))
    }
    
    // Create the config with the custom NoRouteHandler
    config := router.Config{
        Host: "localhost:8080",
        Cors: &router.Cors{
            AllowOrigins: []string{"*"},
            AllowMethods: []string{"GET", "POST"},
        },
        NoRouteHandler: &noRouteHandler,
    }
    
    // Initialize the router
    r, err := router.NewRouter(ctx, config)
    if err != nil {
        log.Fatalf("failed to create router: %v", err)
    }
    
    // Register some example routes
    r.RegisterHandler("GET", "/ping", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("pong"))
    })
    
    // Start the server
    errChan := r.ListenAndServe()
    
    log.Printf("Server running at %s", config.Host)
    
    // Capture any server errors
    if err := <-errChan; err != nil {
        log.Fatalf("server error: %v", err)
    }
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
