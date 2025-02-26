# pubsub

`pubsub`  is a Go package that simplifies interacting with Google Cloud Pub/Sub.. It enables publishing messages to Pub/Sub topics, managing topics efficiently, and validating incoming Google Pub/Sub HTTP requests with token authentication.

## Features

- **Google Pub/Sub Integration**
  - Easy to publish messages to Google Pub/Sub topics.
- **Automatic Topic Management**
  - Caches topics to minimize redundant Pub/Sub client operations.
- **Request Validation**
  - Validates incoming Google Pub/Sub HTTP requests by verifying the authorization token and audience.

## Installation

```bash
go get github.com/albeebe/service/pkg/pubsub
```

## Usage

### Initializing the Pub/Sub Client

To get started, initialize the Pub/Sub client by providing your Google Cloud Project ID.

```go
package main

import (
    "context"
    "log"

    "github.com/albeebe/service/pkg/pubsub"
)

func main() {
    ctx := context.Background()
    config := pubsub.Config{
        GCPProjectID: "your-gcp-project-id",
    }

    pubsubClient, err := pubsub.New(ctx, config)
    if err != nil {
        log.Fatalf("failed to initialize pubsub client: %v", err)
    }

    log.Println("PubSub client initialized successfully")
}
```

### Publishing Messages

You can easily publish messages to a Pub/Sub topic using the `Publish` method. The message can be a string, byte slice, or any struct that can be serialized to JSON.

```go
msgID, err := pubsubClient.Publish("my-topic", "Hello, Pub/Sub!")
if err != nil {
    log.Fatalf("failed to publish message: %v", err)
}

log.Printf("Message published successfully with ID: %s", msgID)
```

### Validating Pub/Sub HTTP Requests

To validate incoming HTTP requests from Google Pub/Sub, you can use the `ValidateGooglePubSubRequest` method. This function verifies the request's Authorization header by checking for a valid Bearer token, which is validated using Google's ID token validation mechanism. The function also checks the token's audience.

#### Audience Validation

- **Provided Audience**: If you pass a specific `audience` string to the validation method, it ensures the ID token's audience matches the provided value.

- **Blank Audience**: If the `audience` argument is left blank, the method will automatically compare the token's audience to the request's `Host` and `Path`. This ensures the request is intended for your service without needing to provide an explicit audience.

Here's how you can use it:

```go
func pubsubHandler(w http.ResponseWriter, r *http.Request) {
    ctx := context.Background()

    if err := pubsub.ValidateGooglePubSubRequest(ctx, r, "your-audience"); err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Handle the valid request
    w.WriteHeader(http.StatusOK)
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
