package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/dwikynator/minato"
	"github.com/dwikynator/minato/merr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthValidatorFunc is the signature that client business logic must implement.
// It receives the raw JWT token string (Bearer prefix already stripped).
// It must return an enriched context (e.g. with user claims) or an error.
type AuthValidatorFunc func(ctx context.Context, token string) (context.Context, error)

// authConfig holds the authentication middleware configuration.
// Fields are private — the only way to set them is via AuthOption functions.
type authConfig struct {
	// skipPaths stores full gRPC method paths ("/pkg.Svc/Method") and/or
	// HTTP paths ("/api/v1/auth/login") that bypass authentication.
	// Using a map gives O(1) lookup per request.
	skipPaths map[string]struct{}

	// validator is the client-injected business logic for token validation.
	validator AuthValidatorFunc
}

// AuthOption is a functional option for configuring the Auth middleware.
type AuthOption func(*authConfig)

// WithAuthSkipPaths registers paths that do NOT require authentication.
//
// For gRPC: use the full method path, e.g. "/auth.v1.AuthService/Login".
// For HTTP: use the URL path, e.g. "/api/v1/auth/login".
//
// Paths are checked with an O(1) map lookup — no regex or prefix matching overhead.
func WithAuthSkipPaths(paths ...string) AuthOption {
	return func(c *authConfig) {
		for _, p := range paths {
			c.skipPaths[p] = struct{}{}
		}
	}
}

// WithAuthValidator registers the token validation function.
//
// The function receives:
//   - ctx: the current request context
//   - token: the raw token string (e.g. "eyJhbGciOi..."), Bearer prefix already stripped
//
// It must return:
//   - A (possibly enriched) context — e.g. with user claims injected via context.WithValue
//   - An error if the token is invalid/expired/revoked
//
// Performance note: heavy initialization (JWT parser allocation, key loading,
// connection pooling) should be done OUTSIDE this function and captured via closure.
func WithAuthValidator(fn AuthValidatorFunc) AuthOption {
	return func(c *authConfig) {
		c.validator = fn
	}
}

// buildAuthConfig applies options and validates that required fields are set.
// Panics at startup (not at request time) — fail fast on misconfiguration.
func buildAuthConfig(opts []AuthOption) *authConfig {
	cfg := &authConfig{
		skipPaths: make(map[string]struct{}),
	}
	for _, o := range opts {
		o(cfg)
	}
	if cfg.validator == nil {
		panic("minato/middleware: auth middleware requires WithAuthValidator")
	}
	return cfg
}

// AuthInterceptor returns a gRPC UnaryServerInterceptor that:
//  1. Skips authentication for methods registered via WithAuthSkipPaths.
//  2. Extracts the "authorization" value from incoming gRPC metadata.
//  3. Validates the Bearer scheme and strips the prefix.
//  4. Delegates to the client-provided validator for token verification.
//  5. Passes the enriched context to downstream handlers.
//
// Error passthrough: if the validator returns a *merr.Error or any error with
// a GRPCStatus() method, it is passed directly to the client. Otherwise, the
// error is wrapped as codes.Unauthenticated.
func AuthInterceptor(opts ...AuthOption) grpc.UnaryServerInterceptor {
	cfg := buildAuthConfig(opts)

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 1. Skip authentication for public endpoints.
		if _, ok := cfg.skipPaths[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		// 2. Extract "authorization" from gRPC metadata.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, merr.Unauthorized("MISSING_TOKEN", "missing metadata")
		}

		vals := md.Get("authorization")
		if len(vals) == 0 {
			return nil, merr.Unauthorized("MISSING_TOKEN", "missing authorization header")
		}

		// 3. Validate Bearer scheme and strip prefix.
		authHeader := vals[0]
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return nil, merr.Unauthorized("INVALID_TOKEN", "authorization header must use Bearer scheme")
		}
		token := authHeader[7:] // len("Bearer ") == 7; avoids TrimPrefix allocation

		// 4. Delegate to client validator.
		ctx, err := cfg.validator(ctx, token)
		if err != nil {
			// If the validator returns a gRPC-aware error (e.g., *merr.Error),
			// pass it through verbatim so the client controls the error shape.
			if _, ok := status.FromError(err); ok {
				return nil, err
			}
			// Fallback: wrap unknown errors as Unauthenticated.
			return nil, merr.Unauthorized("INVALID_TOKEN", "invalid or expired token")
		}

		// 5. Continue with enriched context.
		return handler(ctx, req)
	}
}

// AuthHTTP returns an HTTP middleware that:
//  1. Skips authentication for paths registered via WithAuthSkipPaths.
//  2. Extracts the "Authorization" header from the HTTP request.
//  3. Validates the Bearer scheme and strips the prefix.
//  4. Delegates to the client-provided validator for token verification.
//  5. Passes the enriched context to downstream handlers.
//
// Error responses: returns 401 JSON if token is missing, invalid scheme, or
// validator rejects.
func AuthHTTP(opts ...AuthOption) func(http.Handler) http.Handler {
	cfg := buildAuthConfig(opts)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Skip authentication for public paths.
			if _, ok := cfg.skipPaths[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}

			// 2. Extract Authorization header.
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeUnauthorized(w, "MISSING_TOKEN", "missing authorization header")
				return
			}

			// 3. Validate Bearer scheme and strip prefix.
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeUnauthorized(w, "INVALID_TOKEN", "authorization header must use Bearer scheme")
				return
			}
			token := authHeader[7:]

			// 4. Delegate to client validator.
			ctx, err := cfg.validator(r.Context(), token)
			if err != nil {
				writeUnauthorized(w, "INVALID_TOKEN", "invalid or expired token")
				return
			}

			// 5. Continue with enriched context.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeUnauthorized writes a 401 JSON error response matching Minato's
// unified error shape from the merr/gateway package.
func writeUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	// Matches the gateway error shape: {"error":{"code":"...","message":"..."}}
	fmt.Fprintf(w, `{"error":{"code":%q,"message":%q}}`, code, message)
}

// AuthPlugin bundles AuthHTTP (HTTP) and AuthInterceptor (gRPC) into a single
// minato.Plugin for dual-transport registration via srv.UsePlugin().
//
// This is the recommended entry point when your server handles both pure HTTP
// and gRPC traffic.
//
// Example:
//
//	v := auth.NewTokenValidator(publicKey, blacklist)
//	srv.UsePlugin(middleware.AuthPlugin(
//	    middleware.WithAuthSkipPaths(
//	        "/auth.v1.AuthService/Login",     // gRPC
//	        "/api/v1/auth/login",             // HTTP
//	    ),
//	    middleware.WithAuthValidator(v.Validate),
//	))
func AuthPlugin(opts ...AuthOption) minato.Plugin {
	return minato.Plugin{
		HTTP: AuthHTTP(opts...),
		GRPC: AuthInterceptor(opts...),
	}
}
