package middleware_test

import (
	"context"
	"testing"

	"github.com/dwikynator/minato/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoveryInterceptor_CatchesPanic(t *testing.T) {
	interceptor := middleware.RecoveryInterceptor()

	// Create a handler that panics
	panicHandler := func(ctx context.Context, req any) (any, error) {
		panic("unexpected nil pointer")
	}

	resp, err := interceptor(
		context.Background(),
		nil, // req
		&grpc.UnaryServerInfo{FullMethod: "/test.Service/PanicMethod"},
		panicHandler,
	)

	// Should NOT have panicked — interceptor caught it
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}

	// Should return an Internal error
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal, got %v", st.Code())
	}
	if st.Message() != "internal server error" {
		t.Errorf("expected 'internal server error', got %q", st.Message())
	}
}

func TestRecoveryInterceptor_PassesThrough(t *testing.T) {
	interceptor := middleware.RecoveryInterceptor()

	// Create a normal handler
	normalHandler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test.Service/NormalMethod"},
		normalHandler,
	)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected 'ok', got %v", resp)
	}
}

func TestRecoveryInterceptor_HandlerReturnsError(t *testing.T) {
	interceptor := middleware.RecoveryInterceptor()

	// Create a handler that returns a normal gRPC error (not a panic)
	errorHandler := func(ctx context.Context, req any) (any, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}

	resp, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/test.Service/ErrorMethod"},
		errorHandler,
	)

	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	// Should pass through the original error, not transform it
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}
