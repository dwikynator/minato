package minato

import (
	"context"
	"time"
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
}

type namedCheck struct {
	name string
	fn   func(ctx context.Context) error
}

type namedCloser struct {
	name string
	fn   func() error
}

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
