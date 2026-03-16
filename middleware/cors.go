package middleware

import (
	"net/http"
	"strings"
)

type corsConfig struct {
	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
}

type CORSOption func(*corsConfig)

func WithAllowedOrigins(origins ...string) CORSOption {
	return func(c *corsConfig) {
		c.allowedOrigins = origins
	}
}

func WithAllowedMethods(methods ...string) CORSOption {
	return func(c *corsConfig) {
		c.allowedMethods = methods
	}
}

func WithAllowedHeaders(headers ...string) CORSOption {
	return func(c *corsConfig) {
		c.allowedHeaders = headers
	}
}

func CORS(opts ...CORSOption) func(http.Handler) http.Handler {
	// 1. Set the defaults
	cfg := &corsConfig{
		allowedOrigins: []string{"*"},
		allowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		allowedHeaders: []string{"Content-Type", "Authorization", "X-Request-ID"},
	}

	// 2. Apply any options passed in by the user
	for _, opt := range opts {
		opt(cfg)
	}

	// 3. Pre-compute the comma-separated strings for the headers to save CPU time per request
	originsStr := strings.Join(cfg.allowedOrigins, ", ")
	methodsStr := strings.Join(cfg.allowedMethods, ", ")
	headersStr := strings.Join(cfg.allowedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 4. Always set the Origin hedaer on every response first
			w.Header().Set("Access-Control-Allow-Origin", originsStr)

			// 5. Check if it's a Preflight (OPTIONS) request
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", methodsStr)
				w.Header().Set("Access-Control-Allow-Headers", headersStr)
				w.WriteHeader(http.StatusNoContent) // 204 No Content
				return                              // CRITICAL: Stop here, do not call next.ServeHTTP!
			}

			// 6. It's a normal request, pass it down the chain
			next.ServeHTTP(w, r)
		})
	}
}
