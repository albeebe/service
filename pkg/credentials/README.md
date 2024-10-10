# credentials

`credentials` is a Go library to simplify initializing cloud credentials and extracting associated email addresses. **Currently supports Google Cloud Platform (GCP)**, but is designed with a path in mind to support other cloud providers in the future.

## Features

- **Initialize Cloud Credentials**: Easily set up credentials with specified scopes for GCP.
- **Retrieve Associated Email**: Extract the email address linked to the credentials.
- **Environment Agnostic**: Automatically detects and adapts to GCP environments and local setups.
- **Extensible Design**: Built with future support for other cloud platforms in mind.

## Installation

```bash
go get github.com/albeebe/pkg/credentials
```

## Requirements

- Google Cloud SDK (for local development)

## Usage

### Initializing Google Credentials

Create a configuration with the desired scopes and initialize the credentials for GCP.

```go
ctx := context.Background()
config := credentials.Config{
    Scopes: []string{
        "https://www.googleapis.com/auth/cloud-platform",
    },
}

creds, err := credentials.NewGoogleCredentials(ctx, config)
if err != nil {
    fmt.Printf("Error initializing credentials: %v\n", err)
    return
}
```

### Retrieving Email from Google Credentials

Get the email address associated with the credentials.

```go
email, err := credentials.EmailFromGoogleCredentials(creds)
if err != nil {
    fmt.Printf("Error retrieving email: %v\n", err)
    return
}

fmt.Printf("Authenticated as: %s\n", email)
```

### Full Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/albeebe/service/pkg/credentials"
)

func main() {
    // Set up the context and configuration.
    ctx := context.Background()
    config := credentials.Config{
        Scopes: []string{
            "https://www.googleapis.com/auth/cloud-platform",
        },
    }

    // Initialize credentials.
    creds, err := credentials.NewGoogleCredentials(ctx, config)
    if err != nil {
        fmt.Printf("Error initializing credentials: %v\n", err)
        return
    }

    // Retrieve the associated email address.
    email, err := credentials.EmailFromGoogleCredentials(creds)
    if err != nil {
        fmt.Printf("Error retrieving email: %v\n", err)
        return
    }

    fmt.Printf("Authenticated as: %s\n", email)
}
```

### Handling Different Environments

The library handles credential retrieval differently based on the environment:

- **Google Cloud Platform (GCP)**: Retrieves a JWT from the metadata server.
- **Local Development**: Uses the Google SDK to retrieve the ID token from the credentials.

### Extensibility

While the library currently supports GCP, it is designed with a clear path to support other cloud providers in the future.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
