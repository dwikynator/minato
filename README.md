# Minato

Minato is an opinionated, fast, and feature-rich HTTP server framework for Go, designed for building production-ready microservices with minimal boilerplate.

## Features

- **Graceful Shutdown**: Built-in signal handling and dependency teardown.
- **Observability**: Structured logging (`log/slog`), Request ID generation, and automatic Prometheus metrics.
- **Resilience**: Panic recovery middleware to keep your server alive.
- **Health Checks**: Automatic `/healthz` (liveness) and `/readyz` (readiness) endpoints with concurrent dependency checking.
- **Security**: Highly configurable CORS middleware.
- **Routing**: Clean, `chi`-backed routing with sub-router groups.
- **gRPC Mode (Optional)**: Run gRPC (`:9090`) and HTTP/JSON gateway (`:8080`) in one process via `grpc-gateway`.
- **Cross-Transport Middleware Plugins**: Apply the same concern to HTTP middleware and gRPC interceptors with `UsePlugin(...)`.

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

## gRPC + HTTP Gateway Mode

Enable gRPC mode with `WithGRPCAddr(...)`, register your gRPC services, and register generated gateway handlers.

```go
package main

import (
	"log"

	"github.com/dwikynator/minato"
	greeterpb "github.com/dwikynator/minato/_example/grpc/grpc/greeter/v1"
	"github.com/dwikynator/minato/_example/grpc/handler"
	"github.com/dwikynator/minato/middleware"
	"google.golang.org/grpc"
)

func main() {
	server := minato.New(
		minato.WithAddr(":8080"),     // HTTP gateway
		minato.WithGRPCAddr(":9090"), // gRPC server
		minato.WithGRPCReflection(),  // optional; useful for grpcurl/dev tooling
	)

	server.UsePlugin(
		middleware.RequestIDPlugin(),
		middleware.LoggerPlugin(),
		middleware.RecoveryPlugin(),
	)
	server.Use(middleware.CORS()) // HTTP-only middleware

	server.RegisterGRPC(func(s grpc.ServiceRegistrar) {
		greeterpb.RegisterGreeterServiceServer(s, handler.NewGreeterHandler())
	})
	server.RegisterGateway(greeterpb.RegisterGreeterServiceHandlerFromEndpoint)

	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
```

### Registering Multiple Services

Call `RegisterGRPC` and `RegisterGateway` once per service:

```go
server.RegisterGRPC(func(s grpc.ServiceRegistrar) {
	userpb.RegisterUserServiceServer(s, userHandler)
})
server.RegisterGateway(userpb.RegisterUserServiceHandlerFromEndpoint)

server.RegisterGRPC(func(s grpc.ServiceRegistrar) {
	orderpb.RegisterOrderServiceServer(s, orderHandler)
})
server.RegisterGateway(orderpb.RegisterOrderServiceHandlerFromEndpoint)
```

### Quick Verification

```bash
# REST via gateway
curl -s -X POST http://localhost:8080/v1/greet \
  -H "Content-Type: application/json" \
  -d '{"name":"Minato"}'

# Direct gRPC via reflection (when WithGRPCReflection is enabled)
grpcurl -plaintext \
  -d '{"name":"Minato"}' \
  localhost:9090 greeter.v1.GreeterService/SayHello
```

If reflection is disabled, `grpcurl` still works by providing proto descriptors:

```bash
grpcurl -plaintext \
  -import-path _example/grpc/proto \
  -import-path _example/grpc/proto/third_party/googleapis \
  -proto greeter.proto \
  -d '{"name":"Minato"}' \
  localhost:9090 greeter.v1.GreeterService/SayHello
```

## License

MIT License
