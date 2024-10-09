# gcpcredentials

`gcpcredentials` is a Go package that simplifies the process of obtaining and managing Google Cloud credentials. It provides utility functions to initialize credentials, extract email addresses associated with the credentials, and handle both local and production environments (running on Google Cloud).

## Features

- **Initialize Google Cloud Credentials**
  - Obtain default Google credentials based on the provided configuration.
- **Extract Email**
  - Retrieve the email address associated with the credentials, whether the code is running on Google Cloud or locally.

## Installation

```bash
go get github.com/albeebe/service/pkg/gcpcredentials
```

## Usage

### Initializing Credentials

Use `NewCredentials` to initialize Google Cloud credentials:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/albeebe/service/pkg/gcpcredentials"
)

func main() {
	ctx := context.Background()
	config := gcpcredentials.Config{
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
	}

	creds, err := gcpcredentials.NewCredentials(ctx, config)
	if err != nil {
		log.Fatalf("Failed to initialize credentials: %v", err)
	}
}
```

### Extracting Email from Credentials

To extract the email associated with the credentials:

```go
email, err := gcpcredentials.ExtractEmail(creds)
if err != nil {
	log.Fatalf("Failed to extract email: %v", err)
}

fmt.Println("Email extracted frm credentials:", email)
```

## Configuration

The `Config` struct allows you to specify scopes when initializing credentials:

```go
config := gcpcredentials.Config{
    Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
