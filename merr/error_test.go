package merr_test

import (
	"errors"
	"testing"

	"github.com/dwikynator/minato/merr"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestError_Error(t *testing.T) {
	e := &merr.Error{
		Code:    codes.NotFound,
		Message: "user not found",
		Reason:  "USER_NOT_FOUND",
		Domain:  "minato",
	}

	got := e.Error()
	if got != "NotFound: user not found [USER_NOT_FOUND]" {
		t.Errorf("unexpected Error() string: %s", got)
	}
}

func TestError_ErrorWithoutReason(t *testing.T) {
	e := &merr.Error{
		Code:    codes.Internal,
		Message: "something broke",
	}

	got := e.Error()
	if got != "Internal: something broke" {
		t.Errorf("unexpected Error() string: %s", got)
	}
}

func TestError_GRPCStatus(t *testing.T) {
	e := &merr.Error{
		Code:    codes.NotFound,
		Message: "user not found",
		Reason:  "USER_NOT_FOUND",
		Domain:  "minato",
		Metadata: map[string]string{
			"user_id": "abc-123",
		},
	}

	st := e.GRPCStatus()

	// 1. Verify code
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}

	// 2. Verify message
	if st.Message() != "user not found" {
		t.Errorf("expected 'user not found', got %q", st.Message())
	}

	// 3. Verify details contain ErrorInfo
	details := st.Details()
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}

	info, ok := details[0].(*errdetails.ErrorInfo)
	if !ok {
		t.Fatalf("expected *errdetails.ErrorInfo, got %T", details[0])
	}

	if info.Reason != "USER_NOT_FOUND" {
		t.Errorf("expected reason USER_NOT_FOUND, got %q", info.Reason)
	}
	if info.Domain != "minato" {
		t.Errorf("expected domain minato, got %q", info.Domain)
	}
	if info.Metadata["user_id"] != "abc-123" {
		t.Errorf("expected metadata user_id=abc-123, got %q", info.Metadata["user_id"])
	}
}

func TestError_GRPCStatusRoundTrip(t *testing.T) {
	// Simulates what grpc-go does: it calls GRPCStatus(), serializes,
	// then the client reconstructs from the wire format.
	e := &merr.Error{
		Code:    codes.PermissionDenied,
		Message: "account suspended",
		Reason:  "ACCOUNT_SUSPENDED",
		Domain:  "minato",
	}

	// Convert to standard grpc error (this is what grpc-go does internally)
	grpcErr := e.GRPCStatus().Err()

	// Reconstruct from the error (this is what a client does)
	st, ok := status.FromError(grpcErr)
	if !ok {
		t.Fatal("expected status.FromError to succeed")
	}

	if st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", st.Code())
	}

	details := st.Details()
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(details))
	}

	info, ok := details[0].(*errdetails.ErrorInfo)
	if !ok {
		t.Fatalf("expected *errdetails.ErrorInfo, got %T", details[0])
	}
	if info.Reason != "ACCOUNT_SUSPENDED" {
		t.Errorf("expected ACCOUNT_SUSPENDED, got %q", info.Reason)
	}
}

func TestError_Is(t *testing.T) {
	// Sentinel definitions that mirror real usage.
	errUserNotFound := &merr.Error{Code: codes.NotFound, Reason: "USER_NOT_FOUND", Domain: "user"}
	errSessionNotFound := &merr.Error{Code: codes.NotFound, Reason: "SESSION_NOT_FOUND", Domain: "session"}
	errInternal := &merr.Error{Code: codes.Internal, Reason: "INTERNAL", Domain: "user"}

	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "identical sentinels match",
			err:    &merr.Error{Code: codes.NotFound, Reason: "USER_NOT_FOUND", Domain: "user"},
			target: errUserNotFound,
			want:   true,
		},
		{
			name:   "same code different reason must NOT match",
			err:    errUserNotFound,
			target: errSessionNotFound,
			want:   false,
		},
		{
			name:   "same code same reason different domain must NOT match",
			err:    &merr.Error{Code: codes.NotFound, Reason: "USER_NOT_FOUND", Domain: "admin"},
			target: errUserNotFound,
			want:   false,
		},
		{
			name:   "different code must NOT match",
			err:    errInternal,
			target: errUserNotFound,
			want:   false,
		},
		{
			name:   "message difference is ignored",
			err:    &merr.Error{Code: codes.NotFound, Reason: "USER_NOT_FOUND", Domain: "user", Message: "ignored"},
			target: errUserNotFound,
			want:   true,
		},
		{
			name:   "metadata difference is ignored",
			err:    &merr.Error{Code: codes.NotFound, Reason: "USER_NOT_FOUND", Domain: "user", Metadata: map[string]string{"k": "v"}},
			target: errUserNotFound,
			want:   true,
		},
		{
			name:   "non-merr target returns false",
			err:    errUserNotFound,
			target: errors.New("plain error"),
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := errors.Is(tc.err, tc.target)
			if got != tc.want {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tc.err, tc.target, got, tc.want)
			}
		})
	}
}
