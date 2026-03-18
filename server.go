// Package minato provides an opinionated, feature-rich Go server framework for
// building production-ready HTTP and gRPC-gateway microservices with minimal
// boilerplate. It is built on top of standard library net/http and chi, with
// lightweight core routing and additional overhead only when optional
// middleware is enabled.
package minato

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/dwikynator/minato/health"
	runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// Server represents the Minato HTTP server instance, wrapping an internal http.Server and Router
type Server struct {
	config  *config
	router  *Router
	httpSrv *http.Server
}

// New creates a new Minato Server instance with the provided options.
func New(opts ...Option) *Server {
	cfg := defaultConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	r := newRouter()

	return &Server{
		config: cfg,
		router: r,
		httpSrv: &http.Server{
			Addr:              cfg.addr,
			ReadHeaderTimeout: cfg.readHeaderTimeout,
			IdleTimeout:       cfg.idleTimeout,
			// ReadTimeout:  intentionally omitted (0 = unlimited) — per-request via context
			// WriteTimeout: intentionally omitted (0 = unlimited) — per-request via context
		},
	}
}

// Run starts the server. If a gRPC address is configured, it starts in gRPC mode
// with both gRPC and HTTP/REST endpoints. Otherwise, it starts in standard HTTP mode.
func (s *Server) Run() error {
	if s.config.grpcAddr != "" {
		return s.runGRPCMode()
	}
	return s.runHTTP()
}

// Use registers global middleware that will be applied to all routes.
func (s *Server) Use(middlewares ...func(http.Handler) http.Handler) {
	s.router.Use(middlewares...)
}

// Router returns the underlying Minato Router for registering routes.
func (s *Server) Router() *Router {
	return s.router
}

// runHTTP starts the HTTP server, handles graceful shutdown on SIGINT and SIGTERM,
// and executes any registered closers before exiting.
func (s *Server) runHTTP() error {
	s.httpSrv.Handler = s.router

	if s.config.healthCheck {
		// Convert slice of named checks into a map for the readiness handler
		checks := make(map[string]health.CheckFunc)
		for _, nc := range s.config.readinessChecks {
			checks[nc.name] = nc.fn
		}
		s.router.Get("/healthz", health.Liveness())
		s.router.Get("/readyz", health.Readiness(checks))
	}

	if s.config.metrics {
		// promphttp.Handler() returns a standard http.Handler that
		// exposes Go runtime metrics and any custom metrics registered
		s.router.Get("/metrics", promhttp.Handler().ServeHTTP)
	}

	s.config.logger.Info("minato:server starting", "addr", s.config.addr)

	serverErr := make(chan error, 1)

	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.shutdownTimeout)
	defer cancel()
	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	closers := s.config.closers
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i].fn(); err != nil {
			s.config.logger.Error("error shutting down dependency", "name", closers[i].name, "error", err)
		}
	}

	return nil
}

// UsePlugin applies the HTTP form of each plugin to the HTTP middleware chain
// and the gRPC form to the gRPC interceptor chain.
func (s *Server) UsePlugin(plugins ...Plugin) {
	for _, p := range plugins {
		if p.HTTP != nil {
			s.router.Use(p.HTTP)
		}
		if p.GRPC != nil {
			s.config.grpcUnaryInts = append(s.config.grpcUnaryInts, p.GRPC)
		}
	}
}

// UseGRPC appends gRPC unary interceptors directly (without an HTTP counterpart)
func (s *Server) UseGRPC(interceptors ...grpc.UnaryServerInterceptor) {
	s.config.grpcUnaryInts = append(s.config.grpcUnaryInts, interceptors...)
}

// RegisterGRPC registers a gRPC service implementation.
func (s *Server) RegisterGRPC(fn GRPCServiceFunc) {
	s.config.grpcServices = append(s.config.grpcServices, fn)
}

// RegisterGateway registers a generated grpc-gateway handler for HTTP<->gRPC translation.
func (s *Server) RegisterGateway(fn GatewayRegisterFunc) {
	s.config.gatewayRegFns = append(s.config.gatewayRegFns, fn)
}

func (s *Server) runGRPCMode() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 1. Build the gRPC server with interceptors
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(s.config.grpcUnaryInts...),
		grpc.ChainStreamInterceptor(s.config.grpcStreamInts...),
	)

	// 2. Register gRPC service implementation
	for _, fn := range s.config.grpcServices {
		fn(grpcSrv)
	}

	if s.config.grpcReflection {
		reflection.Register(grpcSrv)
	}

	// 3. Build the grpc-gateway HTTP mux (self-dials grpcAddr on loopback)
	gwMux := runtime.NewServeMux(s.config.gatewayMuxOpts...)
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	for _, fn := range s.config.gatewayRegFns {
		if err := fn(ctx, gwMux, s.config.grpcAddr, dialOpts); err != nil {
			return fmt.Errorf("minato: gateway registration failed: %w", err)
		}
	}

	// 4. Mount gateway mux so HTTP middleware chain applies to it.
	s.router.Mount("/", gwMux)
	s.httpSrv.Handler = s.router

	// 5. Start both servers concurrently
	serverErr := make(chan error, 1)

	go func() {
		lis, err := net.Listen("tcp", s.config.grpcAddr)
		if err != nil {
			serverErr <- fmt.Errorf("minato: grpc listen: %w", err)
			return
		}
		s.config.logger.Info("minato:grpc server starting", "addr", s.config.grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			serverErr <- err
		}
	}()

	go func() {
		s.config.logger.Info("minato:gateway starting", "addr", s.config.addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// 6. Block until signal or fatal error
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
	}

	// 7. Graceful shutdown - HTTP first, then gRPC , then closers
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.shutdownTimeout)
	defer cancel()

	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		s.config.logger.Error("minato: http shutdown error", "error", err)
	}
	grpcSrv.GracefulStop()

	closers := s.config.closers
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i].fn(); err != nil {
			s.config.logger.Error("error shutting down dependency", "name", closers[i].name, "error", err)
		}
	}

	return nil

}
