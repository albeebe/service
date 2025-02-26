# Package Index

This folder contains various Go packages that are used throughout the service library. Each package provides specific functionality and can be reused as needed.

## Available Packages

### [auth](auth)
Provides robust authentication and authorization functionalities for HTTP services. It includes JWT validation, role and permission-based authorization, and automatic key and token refresh mechanisms. Additionally, the library provides middleware to make it easy for your services to make authenticated requests to other services, simplifying secure inter-service communication in distributed systems.

<hr>

### [environment](environment)
Simplifies managing environment variables. It prompts for missing variables in local development, while in production, all required variables must be set, enforcing strict configuration for both environments.

<hr>

### [credentials](credentials)
Simplifies initializing cloud credentials and extracting associated email addresses. **Currently supports Google Cloud Platform (GCP)**, but is designed with a path in mind to support other cloud providers in the future.

<hr>

### [logger](logger)
Provides structured logging with support for development and production environments, including Google Cloud Logging integration.

<hr>

### [pubsub](pubsub)
Simplifies interacting with Google Cloud Pub/Sub.. It enables publishing messages to Pub/Sub topics, managing topics efficiently, and validating incoming Google Pub/Sub HTTP requests with token authentication.

<hr>

### [router](router)
Provides an HTTP/2-enabled router with CORS support and a customizable 404 handler. The package leverages the [Gin framework](https://gin-gonic.com/) internally to handle routing and middleware. However, Gin is not exposed externally, which allows for the easy replacement of the underlying framework in the future without affecting the external API of the package.
