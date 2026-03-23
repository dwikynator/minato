package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dwikynator/minato/merr"
	"github.com/dwikynator/minato/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// claimsKey is a test context key for injecting/extracting user claims.
type claimsKey struct{}

// successValidator always succeeds and injects "test-user" into the context.
func successValidator(ctx context.Context, token string) (context.Context, error) {
	return context.WithValue(ctx, claimsKey{}, "test-user"), nil
}

// failValidator always rejects with a merr error (simulates expired/invalid JWT).
func failValidator(ctx context.Context, token string) (context.Context, error) {
	return nil, merr.Unauthorized("EXPIRED_TOKEN", "token has expired")
}

// genericFailValidator rejects with a plain Go error (simulates a non-merr error).
func genericFailValidator(ctx context.Context, token string) (context.Context, error) {
	return nil, errors.New("something went wrong")
}

// callInterceptor is a test helper that invokes the auth interceptor directly.
func callInterceptor(
	t *testing.T,
	interceptor grpc.UnaryServerInterceptor,
	ctx context.Context,
	method string,
) (any, error) {
	t.Helper()
	info := &grpc.UnaryServerInfo{FullMethod: method}
	return interceptor(ctx, "request", info, func(ctx context.Context, req any) (any, error) {
		// Extract the enriched value to prove context was forwarded
		return ctx.Value(claimsKey{}), nil
	})
}

func TestAuthInterceptor_SkipPath(t *testing.T) {
	interceptor := middleware.AuthInterceptor(
		middleware.WithAuthSkipPaths("/auth.v1.AuthService/Login"),
		middleware.WithAuthValidator(failValidator), // would fail if not skipped
	)

	// Even without metadata, the skip path should succeed.
	resp, err := callInterceptor(t, interceptor, context.Background(), "/auth.v1.AuthService/Login")
	if err != nil {
		t.Fatalf("expected skip path to succeed, got: %v", err)
	}
	_ = resp
}

func TestAuthInterceptor_MissingMetadata(t *testing.T) {
	interceptor := middleware.AuthInterceptor(
		middleware.WithAuthValidator(successValidator),
	)

	_, err := callInterceptor(t, interceptor, context.Background(), "/pkg.Svc/Protected")
	if err == nil {
		t.Fatal("expected error for missing metadata")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestAuthInterceptor_MissingAuthHeader(t *testing.T) {
	interceptor := middleware.AuthInterceptor(
		middleware.WithAuthValidator(successValidator),
	)

	// metadata present, but no "authorization" key
	md := metadata.New(map[string]string{"x-request-id": "abc"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := callInterceptor(t, interceptor, ctx, "/pkg.Svc/Protected")
	if err == nil {
		t.Fatal("expected error for missing authorization header")
	}
}

func TestAuthInterceptor_InvalidScheme(t *testing.T) {
	interceptor := middleware.AuthInterceptor(
		middleware.WithAuthValidator(successValidator),
	)

	md := metadata.New(map[string]string{"authorization": "Basic dXNlcjpwYXNz"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := callInterceptor(t, interceptor, ctx, "/pkg.Svc/Protected")
	if err == nil {
		t.Fatal("expected error for non-Bearer scheme")
	}
}

func TestAuthInterceptor_ValidatorRejects(t *testing.T) {
	interceptor := middleware.AuthInterceptor(
		middleware.WithAuthValidator(failValidator),
	)

	md := metadata.New(map[string]string{"authorization": "Bearer expired-token"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := callInterceptor(t, interceptor, ctx, "/pkg.Svc/Protected")
	if err == nil {
		t.Fatal("expected validator rejection")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestAuthInterceptor_ValidatorRejects_GenericError(t *testing.T) {
	interceptor := middleware.AuthInterceptor(
		middleware.WithAuthValidator(genericFailValidator),
	)

	md := metadata.New(map[string]string{"authorization": "Bearer some-token"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := callInterceptor(t, interceptor, ctx, "/pkg.Svc/Protected")
	if err == nil {
		t.Fatal("expected validator rejection")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", st.Code())
	}
}

func TestAuthInterceptor_Success(t *testing.T) {
	interceptor := middleware.AuthInterceptor(
		middleware.WithAuthValidator(successValidator),
	)

	md := metadata.New(map[string]string{"authorization": "Bearer valid-jwt-token"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := callInterceptor(t, interceptor, ctx, "/pkg.Svc/Protected")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if resp != "test-user" {
		t.Fatalf("expected enriched context with 'test-user', got %v", resp)
	}
}

func TestAuthHTTP_SkipPath(t *testing.T) {
	mw := middleware.AuthHTTP(
		middleware.WithAuthSkipPaths("/api/v1/auth/login"),
		middleware.WithAuthValidator(failValidator),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skip path, got %d", rec.Code)
	}
}

func TestAuthHTTP_MissingHeader(t *testing.T) {
	mw := middleware.AuthHTTP(
		middleware.WithAuthValidator(successValidator),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthHTTP_InvalidScheme(t *testing.T) {
	mw := middleware.AuthHTTP(
		middleware.WithAuthValidator(successValidator),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthHTTP_ValidatorRejects(t *testing.T) {
	mw := middleware.AuthHTTP(
		middleware.WithAuthValidator(failValidator),
	)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthHTTP_Success(t *testing.T) {
	mw := middleware.AuthHTTP(
		middleware.WithAuthValidator(successValidator),
	)

	var gotUser any
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Context().Value(claimsKey{})
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotUser != "test-user" {
		t.Fatalf("expected enriched context with 'test-user', got %v", gotUser)
	}
}

func TestAuthInterceptor_PanicsWithoutValidator(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when no validator is provided")
		}
	}()
	middleware.AuthInterceptor() // no WithAuthValidator — should panic
}

func TestAuthHTTP_PanicsWithoutValidator(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when no validator is provided")
		}
	}()
	middleware.AuthHTTP() // no WithAuthValidator — should panic
}
