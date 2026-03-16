package middleware

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
)

type recoveryConfig struct {
	printer LogPrinter
}

type RecoveryOption func(*recoveryConfig)

// WithRecoveryPrinter allows injecting a custom logger for panic reporting.
func WithRecoveryPrinter(p LogPrinter) RecoveryOption {
	return func(c *recoveryConfig) {
		c.printer = p
	}
}

func Recovery(opts ...RecoveryOption) func(http.Handler) http.Handler {
	cfg := &recoveryConfig{
		printer: &defaultLogPrinter{},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 1. Safety net
			defer func() {
				// 2. Catch the panic!
				// recover() returns nil if there was no panic.
				// If it returns something, a panic occurred.
				if err := recover(); err != nil {
					// 3. Log the panic!
					// We want to log the error itself AND the stack trace
					// so we can debug what crashed
					cfg.printer.Error("panic recovered",
						"err", err,
						"stack", string(debug.Stack()),
						"request_id", RequestIDFromContext(r.Context()),
					)

					// 4. Return a clean 500 JSON response to the user
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					// We don't expose the internal error details to the client
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Internal Server Error",
					})
				}
			}()

			// 5. Normal Operation: Call the next handler in the chain
			// If this panics, the defer above will catch it
			next.ServeHTTP(w, r)
		})
	}
}
