# Whoami

A simple HTTP service that returns information about incoming requests. Useful for testing and debugging.

## Features

- Returns request headers, IP address, and other metadata
- Lightweight and fast
- Health check endpoint at `/health`
- Configurable response name

## Configuration

- **Port**: The port to expose the service on (default: 8080)
- **Service Name**: Custom name to display in responses
- **Version**: Docker image version to use

## Endpoints

- `/` - Main endpoint showing request information
- `/health` - Health check endpoint
- `/api` - JSON formatted response
- `/bench` - Benchmarking endpoint

## Usage

After installation, access the service at:
- http://localhost:8080 (or your configured port)

The service will display:
- Client IP address
- Request headers
- HTTP method and path
- Host information
- Any custom name configured

## Notes

This is primarily a testing/debugging tool and should not be exposed to the internet without proper security considerations.
