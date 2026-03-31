package minato

import (
	"context"
	"net/http"
	"time"

	runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

type config struct {
	addr              string
	shutdownTimeout   time.Duration
	readHeaderTimeout time.Duration
	idleTimeout       time.Duration
	healthCheck       bool
	metrics           bool
	readinessChecks   []namedCheck
	closers           []namedCloser
	logger            Logger

	// gRPC mode - zero values mean gRPC mode is disabled.
	grpcAddr       string
	grpcServices   []GRPCServiceFunc
	grpcServerOpts []grpc.ServerOption
	grpcUnaryInts  []grpc.UnaryServerInterceptor
	grpcStreamInts []grpc.StreamServerInterceptor
	gatewayRegFns  []GatewayRegisterFunc
	gatewayMuxOpts []runtime.ServeMuxOption
	grpcReflection bool
}

type namedCheck struct {
	name string
	fn   func(ctx context.Context) error
}

type namedCloser struct {
	name string
	fn   func() error
}

// Plugin bundles the HTTP middleware form and the gRPC unary interceptor form
// of the same cross-cutting concern into a single registerable unit.
type Plugin struct {
	HTTP func(http.Handler) http.Handler
	GRPC grpc.UnaryServerInterceptor
}

// GRPCServiceFunc is a callback that registers a gRPC service implementation
// against the server's grpc.ServiceRegistrar.
type GRPCServiceFunc func(s grpc.ServiceRegistrar)

// GatewayRegisterFunc is a callback that registers a gRPC gateway handler
// produced by protoc-gen-grpc-gateway.
type GatewayRegisterFunc func(
	ctx context.Context,
	mux *runtime.ServeMux,
	endpoint string,
	opts []grpc.DialOption,
) error

// Option defines a functional configuration option for the Minato Server.
type Option func(*config)

func defaultConfig() *config {
	return &config{
		addr:              ":8080",
		shutdownTimeout:   30 * time.Second,
		readHeaderTimeout: 5 * time.Second,
		idleTimeout:       60 * time.Second,
		healthCheck:       false,
		metrics:           false,
		logger:            &defaultLogger{},
	}
}

// WithAddr sets the TCP address for the server to listen on.
// Defaults to ":8080"
func WithAddr(addr string) Option {
	return func(c *config) {
		c.addr = addr
	}
}

// WithShutdownTimeout sets the deadline for graceful shutdown.
// Defaults to 30 seconds.
func WithShutdownTimeout(d time.Duration) Option {
	return func(c *config) {
		c.shutdownTimeout = d
	}
}

// WithReadHeaderTimeout sets the amount of time allowed to read request headers.
// Defaults to 5 seconds.
func WithReadHeaderTimeout(d time.Duration) Option {
	return func(c *config) {
		c.readHeaderTimeout = d
	}
}

// WithIdleTimeout sets the maximum amount of time to wait for the next request
// when keep-alives are enabled.
// Defaults to 60 seconds.
func WithIdleTimeout(d time.Duration) Option {
	return func(c *config) {
		c.idleTimeout = d
	}
}

// WithHealthCheck enables automatic registration of /healthz and /readyz endpoints.
func WithHealthCheck() Option {
	return func(c *config) {
		c.healthCheck = true
	}
}

// WithMetrics enables automatic registration of the Prometheus /metrics endpoint.
func WithMetrics() Option {
	return func(c *config) {
		c.metrics = true
	}
}

// WithReadinessCheck registers a named dependency check for the /readyz endpoint.
func WithReadinessCheck(name string, fn func(ctx context.Context) error) Option {
	return func(c *config) {
		c.readinessChecks = append(c.readinessChecks, namedCheck{name: name, fn: fn})
	}
}

// WithCloser registers a teardown function that will be called during graceful shutdown.
func WithCloser(name string, fn func() error) Option {
	return func(c *config) {
		c.closers = append(c.closers, namedCloser{name: name, fn: fn})
	}
}

// WithLogger allows injecting a custom Logger implementation for the server.
// If not provided, it defaults to standard log/slog.
func WithLogger(l Logger) Option {
	return func(c *config) {
		c.logger = l
	}
}

// WithGRPCAddr sets the TCP address for the gRPC server and enables gRPC mode.
func WithGRPCAddr(addr string) Option {
	return func(c *config) {
		c.grpcAddr = addr
	}
}

// WithGRPCUnaryInterceptor appends one or more unary interceptors to the gRPC server.
// Deprecated: prefer using WithGRPCServerOption with grpc.ChainUnaryInterceptor instead.
func WithGRPCUnaryInterceptor(interceptors ...grpc.UnaryServerInterceptor) Option {
	return func(c *config) {
		c.grpcUnaryInts = append(c.grpcUnaryInts, interceptors...)
	}
}

// WithGRPCServerOption appends one or more arbitrary gRPC ServerOptions.
// Useful for adding StatsHandlers or custom interceptor chains.
func WithGRPCServerOption(opts ...grpc.ServerOption) Option {
	return func(c *config) {
		c.grpcServerOpts = append(c.grpcServerOpts, opts...)
	}
}

// WithGRPCStreamInterceptor appends one or more stream interceptors to the gRPC server.
func WithGRPCStreamInterceptor(interceptors ...grpc.StreamServerInterceptor) Option {
	return func(c *config) {
		c.grpcStreamInts = append(c.grpcStreamInts, interceptors...)
	}
}

// WithGatewayMuxOptions forwards options directly to runtime.NewServeMux.
func WithGatewayMuxOptions(opts ...runtime.ServeMuxOption) Option {
	return func(c *config) {
		c.gatewayMuxOpts = append(c.gatewayMuxOpts, opts...)
	}
}

// WithGRPCReflection enables gRPC server reflection.
// Keep disabled by default and enable only when needed.
func WithGRPCReflection() Option {
	return func(c *config) {
		c.grpcReflection = true
	}
}
