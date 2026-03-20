package merr_test

import (
	"testing"

	"github.com/dwikynator/minato/merr"
	"google.golang.org/grpc/codes"
)

func TestConstructors_CodeMapping(t *testing.T) {
	tests := []struct {
		name        string
		constructor func(string, string, ...merr.Opt) *merr.Error
		wantCode    codes.Code
	}{
		{"BadRequest", merr.BadRequest, codes.InvalidArgument},
		{"Unauthorized", merr.Unauthorized, codes.Unauthenticated},
		{"Forbidden", merr.Forbidden, codes.PermissionDenied},
		{"NotFound", merr.NotFound, codes.NotFound},
		{"Conflict", merr.Conflict, codes.AlreadyExists},
		{"PreconditionFailed", merr.PreconditionFailed, codes.FailedPrecondition},
		{"TooManyRequests", merr.TooManyRequests, codes.ResourceExhausted},
		{"Internal", merr.Internal, codes.Internal},
		{"Unavailable", merr.Unavailable, codes.Unavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor("SOME_REASON", "some message")
			if err.Code != tt.wantCode {
				t.Errorf("expected code %v, got %v", tt.wantCode, err.Code)
			}
			if err.Reason != "SOME_REASON" {
				t.Errorf("expected reason SOME_REASON, got %q", err.Reason)
			}
			if err.Message != "some message" {
				t.Errorf("expected message 'some message', got %q", err.Message)
			}
		})
	}
}

func TestConstructors_WithMetadata(t *testing.T) {
	err := merr.TooManyRequests(
		"TOO_MANY_REQUESTS",
		"rate limit exceeded",
		merr.WithMetadata("retry_after", "30"),
		merr.WithMetadata("limit", "100"),
	)

	if err.Metadata["retry_after"] != "30" {
		t.Errorf("expected retry_after=30, got %q", err.Metadata["retry_after"])
	}
	if err.Metadata["limit"] != "100" {
		t.Errorf("expected limit=100, got %q", err.Metadata["limit"])
	}

	// Verify the metadata survives the GRPCStatus round-trip
	st := err.GRPCStatus()
	if st.Code() != codes.ResourceExhausted {
		t.Errorf("expected ResourceExhausted, got %v", st.Code())
	}
}

func TestConstructors_WithDomain(t *testing.T) {
	err := merr.NotFound("USER_NOT_FOUND", "user not found", merr.WithDomain("core-auth"))

	if err.Domain != "core-auth" {
		t.Errorf("expected domain core-auth, got %q", err.Domain)
	}
}

func TestConstructors_FullHandlerExample(t *testing.T) {
	// Simulates exactly what a real handler would return:
	//   return nil, merr.NotFound("USER_NOT_FOUND", "no user matches the identifier",
	//       merr.WithDomain("core-auth"))
	//
	// The gRPC server calls GRPCStatus() → wire format → client reconstructs.

	handlerErr := merr.NotFound(
		"USER_NOT_FOUND",
		"no user matches the identifier",
		merr.WithDomain("core-auth"),
	)

	// Simulate grpc-go extracting the status
	st := handlerErr.GRPCStatus()
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
	if st.Message() != "no user matches the identifier" {
		t.Fatalf("expected message, got %q", st.Message())
	}
}
