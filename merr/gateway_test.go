package merr_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dwikynator/minato/merr"
	runtime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// helper to invoke the error handler and capture the response
func invokeGatewayError(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	mux := runtime.NewServeMux()
	marshaler := &runtime.JSONPb{}

	merr.GatewayErrorHandler(context.Background(), mux, marshaler, rec, req, err)
	return rec
}

func TestGatewayErrorHandler_WithMerrError(t *testing.T) {
	// Simulate: handler returns merr.NotFound(...)
	merrErr := merr.NotFound(
		"USER_NOT_FOUND",
		"no user matches the identifier",
		merr.WithDomain("core-auth"),
		merr.WithMetadata("user_id", "abc-123"),
	)

	// grpc-go converts via GRPCStatus().Err() on the wire
	grpcErr := merrErr.GRPCStatus().Err()

	rec := invokeGatewayError(t, grpcErr)

	// 1. HTTP status code
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}

	// 2. Parse response body
	var body struct {
		Error struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Details map[string]string `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if body.Error.Code != "USER_NOT_FOUND" {
		t.Errorf("expected code USER_NOT_FOUND, got %q", body.Error.Code)
	}
	if body.Error.Message != "no user matches the identifier" {
		t.Errorf("unexpected message: %q", body.Error.Message)
	}
	if body.Error.Details["domain"] != "core-auth" {
		t.Errorf("expected domain core-auth, got %q", body.Error.Details["domain"])
	}
	if body.Error.Details["user_id"] != "abc-123" {
		t.Errorf("expected user_id abc-123, got %q", body.Error.Details["user_id"])
	}
}

func TestGatewayErrorHandler_PlainGRPCError(t *testing.T) {
	// When a handler returns a plain status.Error (no detail), the code falls
	// back to the gRPC code name.
	grpcErr := status.Error(codes.Internal, "something broke")

	rec := invokeGatewayError(t, grpcErr)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if body.Error.Code != "Internal" {
		t.Errorf("expected fallback code 'Internal', got %q", body.Error.Code)
	}
	if body.Error.Message != "something broke" {
		t.Errorf("unexpected message: %q", body.Error.Message)
	}
}

func TestGatewayErrorHandler_ContentType(t *testing.T) {
	grpcErr := status.Error(codes.NotFound, "not found")
	rec := invokeGatewayError(t, grpcErr)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected application/json content type, got %q", ct)
	}
}
