package minato

import (
	"context"
	"net/http"
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
