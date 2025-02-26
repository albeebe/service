# logger

`logger` is a Go library providing structured logging with support for development and production environments, including Google Cloud Logging integration.

## Overview

This package simplifies logging in Go applications by providing a unified interface for both development and production environments. In development, logs are printed to the console in a human-readable format with colors and timestamps. In production, logs are sent to Google Cloud Logging with structured data.

## Features

- Easy setup for both development and production logging.
- Structured logging with key-value pairs.
- Supports log levels (DEBUG, INFO, WARN, ERROR).
- Colorful console output in development.
- Integration with Google Cloud Logging.
- Error stack traces in development mode for easier debugging.

## Installation

```bash
go get github.com/albeebe/service/pkg/logger
```

## Usage

### Setting up Development Logging

```go
package main

import (
    "context"
    "log/slog"

    "github.com/albeebe/service/pkg/logger"
)

func main() {
    ctx := context.Background()

    config := logger.Config{
        Level: slog.LevelDebug, // Set desired log level
    }

    log, err := logger.NewDevelopmentLogger(ctx, config)
    if err != nil {
        panic(err)
    }

    // Use the logger
    log.Info("Application started", "version", "1.0.0")
    log.Debug("Debugging info", "details", "some debug info")
    log.Error("An error occurred", "error", err)
}
```

### Setting up Google Cloud Logging

```go
package main

import (
    "context"
    "log/slog"

    "github.com/albeebe/service/pkg/logger"
)

func main() {
    ctx := context.Background()

    config := logger.Config{
        GCPProjectID: "your-gcp-project-id",
        LogName:      "your-log-name",
        Level:        slog.LevelInfo, // Set desired log level
    }

    log, err := logger.NewGoogleCloudLogger(ctx, config)
    if err != nil {
        panic(err)
    }

    // Use the logger
    log.Info("Application started", "version", "1.0.0")
    log.Warn("Potential issue detected", "details", "some warning")
    log.Error("An error occurred", "error", err)
}
```

## Configuration

### Config Struct

The `Config` struct is used to configure the logger:

```go
type Config struct {
    GCPProjectID string     // Google Cloud Project ID
    LogName      string     // Name of the log stream
    Level        slog.Level // Minimum log level to capture (e.g., DEBUG, INFO)
}
```

- **GCPProjectID**: Required for Google Cloud Logging; specify your Google Cloud Project ID.
- **LogName**: Required for Google Cloud Logging; specify the name of the log stream.
- **Level**: Sets the minimum level of logs to capture.

## Logging Levels

The logger supports the following log levels:

- `slog.LevelDebug`: Debug-level messages, typically only of interest during development.
- `slog.LevelInfo`: Informational messages that highlight the progress of the application.
- `slog.LevelWarn`: Potentially harmful situations which still allow the application to continue running.
- `slog.LevelError`: Error events that might still allow the application to continue running.

## Flushing Logs

For production logging to Google Cloud, it's important to flush the logs before exiting the application to ensure all logs are properly sent.

```go
if err := logger.FlushLogger(log); err != nil {
    panic(err)
}
```

## Examples

### Logging with Structured Data

You can include key-value pairs in your log messages:

```go
log.Info("User login", "username", "johndoe", "method", "oauth")
```

### Error Logging with Stack Trace (Development Mode)

In development mode, when logging errors, a stack trace is included to help with debugging:

```go
err := errors.New("something went wrong")
log.Error("an error occurred", "error", err)
```
Outputs:

```diff
[11:10:53.442] [ERROR] something went wrong | error=an error occurred
   └── (file: /path/to/project/service/main.go, line: 86)
   └── (file: /path/to/project/service-demo/main.go, line: 61)
```

This will output the error message along with a stack trace showing the file and line numbers.

## Prerequisites

### Google Cloud Logging

To use Google Cloud Logging, you need:

- A Google Cloud Platform project.
- Proper authentication setup (e.g., service account with the `Logging > Logs Writer` role).
- Application credentials configured in your environment (e.g., `GOOGLE_APPLICATION_CREDENTIALS` environment variable pointing to your service account key file).

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
