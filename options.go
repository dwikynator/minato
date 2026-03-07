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

func WithAddr(addr string) Option {
	return func(c *config) {
		c.addr = addr
	}
}

func WithShutdownTimeout(d time.Duration) Option {
	return func(c *config) {
		c.shutdownTimeout = d
	}
}

func WithReadHeaderTimeout(d time.Duration) Option {
	return func(c *config) {
		c.readHeaderTimeout = d
	}
}

func WithIdleTimeout(d time.Duration) Option {
	return func(c *config) {
		c.idleTimeout = d
	}
}

func WithHealthCheck() Option {
	return func(c *config) {
		c.healthCheck = true
	}
}

func WithMetrics() Option {
	return func(c *config) {
		c.metrics = true
	}
}

func WithReadinessCheck(name string, fn func(ctx context.Context) error) Option {
	return func(c *config) {
		c.readinessChecks = append(c.readinessChecks, namedCheck{name: name, fn: fn})
	}
}

func WithCloser(name string, fn func() error) Option {
	return func(c *config) {
		c.closers = append(c.closers, namedCloser{name: name, fn: fn})
	}
}
