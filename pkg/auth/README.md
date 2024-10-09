# auth

`auth` is a Go library providing robust authentication and authorization functionalities for HTTP services. It includes JWT validation, role and permission-based authorization, and automatic key and token refresh mechanisms. Additionally, the library provides middleware to make it easy for your services to make authenticated requests to other services, simplifying secure inter-service communication in distributed systems.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Getting Started](#getting-started)
    - [Implementing AuthProvider](#implementing-authprovider)
    - [Initializing Auth](#initializing-auth)
    - [Starting the Auth Service](#starting-the-auth-service)
- [Usage](#usage)
    - [Authenticating Requests](#authenticating-requests)
    - [Authorizing Requests](#authorizing-requests)
    - [Creating an Authenticated HTTP Client](#creating-an-authenticated-http-client)
    - [Extracting Bearer Tokens](#extracting-bearer-tokens)
- [Types and Interfaces](#types-and-interfaces)
    - [AuthProvider Interface](#authprovider-interface)
    - [AuthRequirements](#authrequirements)
    - [AccessToken](#accesstoken)
    - [Key](#key)
- [Error Handling](#error-handling)
- [License](#license)
- [Contributing](#contributing)

## Features

- **JWT Authentication**: Validate JSON Web Tokens (JWT) with signature verification.
- **Role and Permission-Based Authorization**: Fine-grained access control using roles and permissions.
- **Automatic Refresh Mechanisms**: Periodic refresh of authentication keys and access tokens.
- **Custom Authentication Providers**: Support for custom logic via the `AuthProvider` interface.
- **Helper Functions**: Utilities for token extraction and authorization requirement definitions.
- **HTTP Client Middleware**: HTTP client that automatically injects access tokens into requests.

## Installation

```bash
go get github.com/albeebe/service/pkg/auth
```

## Getting Started

### Implementing AuthProvider

To use the library, you need to implement the `AuthProvider` interface, which defines methods for authorization checks and token/key refresh logic.

```go
type MyAuthProvider struct {
    // Fields for your provider
}

func (p *MyAuthProvider) AuthorizeRequest(r *http.Request, authRequirements auth.AuthRequirements) (isAuthorized bool, err error) {
    // Implement your authorization logic
    return true, nil
}

func (p *MyAuthProvider) IsServiceRequest(r *http.Request) (isService bool) {
    // Determine if the request is from a service
    return false
}

func (p *MyAuthProvider) RefreshAccessToken() (accessToken *auth.AccessToken, nextRefresh time.Time, err error) {
    // Refresh and return a new access token
    return &auth.AccessToken{
        Token:   "new-access-token",
        Expires: time.Now().Add(1 * time.Hour),
    }, time.Now().Add(55 * time.Minute), nil
}

func (p *MyAuthProvider) RefreshKeys() (keys []*auth.Key, nextRefresh time.Time, err error) {
    // Fetch and return new authentication keys
    keys := []*auth.Key{
        {
            Kid: "key-id",
            Iat: time.Now().Unix(),
            Exp: time.Now().Add(24 * time.Hour).Unix(),
            Alg: "RS256",
            Pem: "public-key-in-pem-format",
        },
    }
    return keys, time.Now().Add(12 * time.Hour), nil
}
```

### Initializing Auth

Create a new `Auth` instance by providing a context and configuration with your `AuthProvider`.

```go
ctx := context.Background()
authProvider := &MyAuthProvider{}

config := auth.Config{
    AuthProvider: authProvider,
}

authInstance, err := auth.New(ctx, config)
if err != nil {
    log.Fatalf("Failed to initialize auth: %v", err)
}
```

### Starting the Auth Service

Start the auth service to initialize periodic refresh routines for keys and tokens.

```go
errorChan := authInstance.Start()

// Handle errors from the error channel
go func() {
    for err := range errorChan {
        log.Printf("Auth error: %v", err)
    }
}()
```

## Usage

### Authenticating Requests

Use the `Authenticate` method to validate incoming HTTP requests by checking the bearer token in the `Authorization` header.

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    authenticated, reason, err := authInstance.Authenticate(r)
    if err != nil {
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    if !authenticated {
        http.Error(w, reason, http.StatusUnauthorized)
        return
    }
    // Proceed with handling the authenticated request
}
```

### Authorizing Requests

Use the `Authorize` method to check if the authenticated request has the required roles or permissions.

```go
authRequirements := auth.AuthRequirements{
    AnyRole:        []string{"admin", "editor"},
    AllPermissions: []string{"read", "write"},
}

authorized, err := authInstance.Authorize(r, authRequirements)
if err != nil {
    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    return
}
if !authorized {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
// Proceed with handling the authorized request
```

Alternatively, use helper functions to define requirements:

```go
authorized, err := authInstance.Authorize(r, auth.AnyRole("admin", "editor"))
```

### Creating an Authenticated HTTP Client

Create an HTTP client that automatically injects the access token into requests using `NewAuthClient`.

```go
httpClient, err := authInstance.NewAuthClient()
if err != nil {
    log.Fatalf("Failed to create auth client: %v", err)
}

resp, err := httpClient.Get("https://api.example.com/protected-resource")
if err != nil {
    log.Fatalf("Request failed: %v", err)
}
defer resp.Body.Close()
// Process the response
```

### Extracting Bearer Tokens

Extract the bearer token from an HTTP request using `ExtractBearerToken`.

```go
token, ok := auth.ExtractBearerToken(r)
if !ok {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
// Use the token as needed
```

## Types and Interfaces

### AuthProvider Interface

Defines methods that custom authentication providers must implement.

```go
type AuthProvider interface {
    // AuthorizeRequest checks if the request meets the specified authorization requirements.
    // It returns true if the request is authorized, otherwise false, and an error if something goes wrong.
    AuthorizeRequest(r *http.Request, authRequirements AuthRequirements) (isAuthorized bool, err error)

    // IsServiceRequest checks whether the given HTTP request originates from a service.
    // It returns true if the request is identified as a service request, otherwise false.
    IsServiceRequest(r *http.Request) (isService bool)

    // RefreshAccessToken refreshes the current access token.
    // It returns the new access token, the time for the next refresh, and an error if the operation fails.
    RefreshAccessToken() (accessToken *AccessToken, nextRefresh time.Time, err error)

    // RefreshKeys retrieves updated authentication keys and the scheduled time for the next key refresh.
    // It returns a slice of keys, the time for the next refresh, and an error if the operation fails.
    RefreshKeys() (keys []*Key, nextRefresh time.Time, err error)}

```

### AuthRequirements

Specifies roles and permissions required for authorization.

```go
type AuthRequirements struct {
    AnyRole        []string // At least one role must be present
    AllPermissions []string // All permissions must be granted
}
```

#### Helper Functions

- `auth.AnyRole(roles ...string) AuthRequirements`: Requires at least one of the specified roles.
- `auth.AllPermissions(permissions ...string) AuthRequirements`: Requires all specified permissions.

### AccessToken

Represents an access token with its expiration time.

```go
type AccessToken struct {
    Token   string    // The access token string
    Expires time.Time // The token's expiration time
}
```

### Key

Represents an authentication key used for token verification.

```go
type Key struct {
    Kid string // Unique identifier for the key
    Iat int64  // Issued-at time (Unix timestamp)
    Exp int64  // Expiration time (Unix timestamp)
    Alg string // Algorithm used (e.g., "RS256")
    Pem string // RSA public key in PEM format
}
```

## Error Handling

The `Start` method returns an error channel that reports issues encountered during refresh operations. It's important to listen to this channel to handle errors appropriately.

```go
errorChan := authInstance.Start()

go func() {
    for err := range errorChan {
        // Handle errors such as logging or retry mechanisms
        log.Printf("Auth error: %v", err)
    }
}()
```

When using `Authenticate` and `Authorize`, check for errors and handle unauthorized or forbidden responses accordingly.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Feel free to open an issue or submit a pull request with any proposed changes.
