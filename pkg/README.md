# Package Index

This folder contains various Go packages that are used throughout the service library. Each package provides specific functionality and can be reused as needed.

## Available Packages

### 1. [environment](environment)
Simplifies managing environment variables. It prompts for missing variables in local development, while in production, all required variables must be set, enforcing strict configuration for both environments.

### 2. [gcpcredentials](gcpcredentials)
Simplifies the process of obtaining and managing Google Cloud credentials. It provides utility functions to initialize credentials, extract email addresses associated with the credentials, and handle both local and production environments (running on Google Cloud).

### 3. [pubsub](pubsub)
Simplifies interacting with Google Cloud Pub/Sub.. It enables publishing messages to Pub/Sub topics, managing topics efficiently, and validating incoming Google Pub/Sub HTTP requests with token authentication.