package minato

import "net/http"

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
