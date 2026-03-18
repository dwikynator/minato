package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/dwikynator/minato"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// RecoverInterceptor is the gRPC unary interceptor form of recovery.
func RecoveryInterceptor(opts ...RecoveryOption) grpc.UnaryServerInterceptor {
	cfg := &recoveryConfig{printer: &defaultLogPrinter{}}
	for _, o := range opts {
		o(cfg)
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				cfg.printer.Error("panic recovered",
					"err", r,
					"stack", string(debug.Stack()),
					"request_id", RequestIDFromContext(ctx),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		resp, err = handler(ctx, req)
		return resp, err
	}
}

// RecoveryPlugin bundles Recovery (HTTP) and RecoveryInterceptor (gRPC).
func RecoveryPlugin(opts ...RecoveryOption) minato.Plugin {
	return minato.Plugin{
		HTTP: Recovery(opts...),
		GRPC: RecoveryInterceptor(opts...),
	}
}
