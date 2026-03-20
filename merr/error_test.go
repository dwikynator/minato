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
	err1 := &merr.Error{Code: codes.NotFound, Message: "a"}
	err2 := &merr.Error{Code: codes.NotFound, Message: "b"}
	err3 := &merr.Error{Code: codes.Internal, Message: "c"}

	if !errors.Is(err1, err2) {
		t.Error("expected err1 Is err2 (same code)")
	}
	if errors.Is(err1, err3) {
		t.Error("expected err1 NOT Is err3 (different code)")
	}
}
