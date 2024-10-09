# environment

`environment` is a Go library that simplifies managing environment variables. It prompts for missing variables in local development, while in production, all required variables must be set, enforcing strict configuration for both environments.

## Features

- **Seamless Integration for Local and Production Environments**: In production, missing environment variables return an error, while in local environments, users are prompted for missing values and can either provide their own values or use defaults.
- **Automatic Struct Population**: Automatically populates a provided struct with environment variable values.
- **Type Safety**: Supports multiple types (`bool`, `float64`, `int64`, `string`), ensuring that values are correctly typed and validated.
- **.env File Support**: Automatically loads environment variables from a `.env` file if available.

## Installation

```bash
go get github.com/albeebe/service/pkg/environment
```

## Usage

### Define Your Struct

Create a struct that defines your application's environment variables. Field names must exactly match the corresponding environment variables (case-sensitive). Use the default tag to specify default values for each field. 

_The default value is only used when the application is running locally and the environment variable is missing. The user will be prompted to use the default or specify their own value:_

```go
type EnvVars struct {
    LOG_LEVEL  string  `default:"INFO"`
    DEBUG_MODE bool    `default:"false"`
    THRESHOLD  float64 `default:"0.8"`
    TIMEOUT    int64   `default:"30"`
}
```

### Initialize the Environment

Call the Initialize function, passing a pointer to your struct and a boolean flag (`runningInProduction`) indicating whether the application is running in a production environment. The function will automatically populate the struct fields with values from environment variables.
```go
package main

import (
    "fmt"
    "cloud.google.com/go/compute/metadata"
    "github.com/albeebe/service/pkg/environment"
)

func main() {
    var envVars EnvVars
	runningInProduction := metadata.OnGCE()
    err := environment.Initialize(&envVars, runningInProduction)
    if err != nil {
        fmt.Printf("Error initializing environment: %v\n", err)
        return
    }

    fmt.Printf("Environment Variables: %+v\n", envVars)
}
```

In local environments, missing environment variables prompt users to provide values (or accept defaults), which are saved in a `.env` file for future runs. In production environments, missing environment variables will trigger an error to enforce strict configuration.

### Running Locally: Example

When running locally, if an environment variable like `LOG_LEVEL` is missing, the program will prompt:

```
Missing Environment Variables

You are seeing this message because the service is running locally. In production, an error would have been returned.

To run this service locally, please provide a value for each environment variable, or press [Enter] to use the default.


LOG_LEVEL
 └── INFO: 
```

After providing the value, it will be saved in the `.env` file for future runs.

## Supported Field Types

- `bool`
- `float64`
- `int64`
- `string`

Fields with unsupported types or fields without a `default` tag will result in an error.

## Error Handling

The `Initialize` function returns an error if it encounters issues such as:
- Missing `default` tags.
- Invalid values for the specified types.
- Uninitialized environment variables in a production environment.

In local environments, the user will be prompted for missing environment variables. 

_After the values are saved in the `.env` file, simply rerun the application and the environment variables will automatically load._

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
