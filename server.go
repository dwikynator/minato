package minato

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
)

type Server struct {
	config  *config
	router  *Router
	httpSrv *http.Server
}

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

func (s *Server) Use(middlewares ...func(http.Handler) http.Handler) {
	s.router.Use(middlewares...)
}

func (s *Server) Router() *Router {
	return s.router
}

func (s *Server) Run() error {
	s.httpSrv.Handler = s.router

	// TODO: register /healthz, /readyz, and /metrics routes here
	// based on s.config.healthCheck and s.config.metrics

	log.Printf("minato: server listening on %s\n", s.config.addr)

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
			log.Println("error closing", closers[i].name, " : ", err)
		}
	}

	return nil
}
