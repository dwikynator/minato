# Minato

Minato is an opinionated, fast, and feature-rich HTTP server framework for Go, designed for building production-ready microservices with minimal boilerplate.

## Features

- **Graceful Shutdown**: Built-in signal handling and dependency teardown.
- **Observability**: Structured logging (`log/slog`), Request ID generation, and automatic Prometheus metrics.
- **Resilience**: Panic recovery middleware to keep your server alive.
- **Health Checks**: Automatic `/healthz` (liveness) and `/readyz` (readiness) endpoints with concurrent dependency checking.
- **Security**: Highly configurable CORS middleware.
- **Routing**: Clean, `chi`-backed routing with sub-router groups.

## Installation

```bash
go get github.com/dwikynator/minato
```

## Quick Start

Here is a minimal example of a Minato server with all the bells and whistles:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dwikynator/minato"
	"github.com/dwikynator/minato/middleware"
)

func main() {
	// 1. Configure the server
	server := minato.New(
		minato.WithAddr(":8080"),
		minato.WithHealthCheck(),
		minato.WithMetrics(),
		minato.WithReadinessCheck("database", checkDatabase),
		minato.WithCloser("database", func() error {
			fmt.Println("Closing database connection...")
			return nil
		}),
	)

	// 2. Register Global Middleware
	server.Use(middleware.RequestID())
	server.Use(middleware.Recovery())
	server.Use(middleware.Logger(
		middleware.WithBodyLogging(true),
	))
	server.Use(middleware.CORS(
		middleware.WithAllowedOrigins("*"),
	))

	// 3. Register Routes
	server.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// 4. Start the server (blocks until SIGINT/SIGTERM)
	if err := server.Run(); err != nil {
		panic(err)
	}
}

func checkDatabase(ctx context.Context) error {
	// Check connection here
	return nil
}
```

## License

MIT License
