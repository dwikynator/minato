package middleware

import (
	"context"
	"net/http"

	"github.com/dwikynator/minato"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID returns a middleware that generates a unique request ID for each request.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = uuid.New().String()
			}

			ctx := context.WithValue(r.Context(), requestIDKey, id)

			r.Header.Set("Grpc-Metadata-X-Request-Id", id)

			r = r.WithContext(ctx)
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDInterceptor is the gRPC unary interceptor form of RequestID
// It reads x-request-id from incoming gRPC metadata ; generates a UUID if absent;
func RequestIDInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-request-id"); len(vals) > 0 {
				id = vals[0]
			}
		}
		if id == "" {
			id = uuid.New().String()
		}
		ctx = context.WithValue(ctx, requestIDKey, id)
		return handler(ctx, req)
	}
}

// RequestIDPlugin bundles RequestID (HTTP) and RequestIDInterceptor (gRPC).
func RequestIDPlugin() minato.Plugin {
	return minato.Plugin{
		HTTP: RequestID(),
		GRPC: RequestIDInterceptor(),
	}
}

// RequestIDFromContext returns the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}
