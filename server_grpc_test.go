package minato

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

func TestWithGRPCAddr(t *testing.T) {
	addr := ":9090"
	s := New(WithGRPCAddr(addr))

	if s.config.grpcAddr != addr {
		t.Errorf("expected grpcAddr to be %s, got %s", addr, s.config.grpcAddr)
	}
}
func TestRegisterGRPC(t *testing.T) {
	s := New()
	called := false
	fn := func(srv grpc.ServiceRegistrar) {
		called = true
	}

	s.RegisterGRPC(fn)

	if len(s.config.grpcServices) != 1 {
		t.Fatalf("expected 1 grpc service registered, got %d", len(s.config.grpcServices))
	}

	s.config.grpcServices[0](nil)
	if !called {
		t.Error("registered GRPC service function was not called")
	}
}

func TestRegisterGateway(t *testing.T) {
	s := New()
	stub := func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
		return nil
	}

	s.RegisterGateway(stub)

	if len(s.config.gatewayRegFns) != 1 {
		t.Errorf("expected 1 gateway registration function, got %d", len(s.config.gatewayRegFns))
	}
}

func TestUsePlugin(t *testing.T) {
	s := New()
	p := Plugin{
		HTTP: func(next http.Handler) http.Handler {
			return next
		},
		GRPC: func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		},
	}

	s.UsePlugin(p)

	// Verify it was added to the gRPC interceptors
	if len(s.config.grpcUnaryInts) != 1 {
		t.Errorf("expected 1 grpc unary interceptor, got %d", len(s.config.grpcUnaryInts))
	}
}

func TestGRPCMode_HealthCheckRegistered(t *testing.T) {
	s := New(WithGRPCAddr(":0"), WithHealthCheck())

	// registerInfraRoutes should have been set up to be called.
	// We can verify by checking the router handles /healthz.
	s.registerInfraRoutes()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /healthz, got %d", rec.Code)
	}
}

func TestGRPCMode_ReadinessCheckRegistered(t *testing.T) {
	s := New(
		WithGRPCAddr(":0"),
		WithHealthCheck(),
		WithReadinessCheck("test-dep", func(ctx context.Context) error {
			return nil
		}),
	)

	s.registerInfraRoutes()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	s.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /readyz, got %d", rec.Code)
	}
}

func TestGRPCMode_HealthCheckDisabledByDefault(t *testing.T) {
	s := New(WithGRPCAddr(":0"))

	s.registerInfraRoutes()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.router.ServeHTTP(rec, req)

	// Should NOT be 200 — healthCheck is disabled by default
	if rec.Code == http.StatusOK {
		t.Error("/healthz should not be registered when WithHealthCheck is not set")
	}
}
