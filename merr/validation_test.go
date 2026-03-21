package merr_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dwikynator/minato/merr"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewValidationError_Structure(t *testing.T) {
	err := merr.NewValidationError(
		"VALIDATION_FAILED",
		"request validation failed",
		merr.FieldViolation{Field: "email", Description: "must be a valid email"},
		merr.FieldViolation{Field: "password", Description: "minimum 8 characters"},
	)

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}

	// 1. Check code
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}

	// 2. Check message
	if st.Message() != "request validation failed" {
		t.Errorf("unexpected message: %q", st.Message())
	}

	// 3. Check details
	details := st.Details()
	if len(details) != 2 {
		t.Fatalf("expected 2 details (ErrorInfo + BadRequest), got %d", len(details))
	}

	// ErrorInfo
	info, ok := details[0].(*errdetails.ErrorInfo)
	if !ok {
		t.Fatalf("expected ErrorInfo, got %T", details[0])
	}
	if info.Reason != "VALIDATION_FAILED" {
		t.Errorf("expected reason VALIDATION_FAILED, got %q", info.Reason)
	}

	// BadRequest
	br, ok := details[1].(*errdetails.BadRequest)
	if !ok {
		t.Fatalf("expected BadRequest, got %T", details[1])
	}
	if len(br.FieldViolations) != 2 {
		t.Fatalf("expected 2 field violations, got %d", len(br.FieldViolations))
	}
	if br.FieldViolations[0].Field != "email" {
		t.Errorf("expected field 'email', got %q", br.FieldViolations[0].Field)
	}
	if br.FieldViolations[1].Field != "password" {
		t.Errorf("expected field 'password', got %q", br.FieldViolations[1].Field)
	}
}

func TestGatewayErrorHandler_WithFieldViolations(t *testing.T) {
	validationErr := merr.NewValidationError(
		"VALIDATION_FAILED",
		"request validation failed",
		merr.FieldViolation{Field: "email", Description: "must be a valid email"},
	)

	rec := invokeGatewayError(t, validationErr)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var body struct {
		Error struct {
			Code            string `json:"code"`
			Message         string `json:"message"`
			FieldViolations []struct {
				Field       string `json:"field"`
				Description string `json:"description"`
			} `json:"field_violations"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if body.Error.Code != "VALIDATION_FAILED" {
		t.Errorf("expected code VALIDATION_FAILED, got %q", body.Error.Code)
	}
	if len(body.Error.FieldViolations) != 1 {
		t.Fatalf("expected 1 field violation, got %d", len(body.Error.FieldViolations))
	}
	if body.Error.FieldViolations[0].Field != "email" {
		t.Errorf("expected field 'email', got %q", body.Error.FieldViolations[0].Field)
	}
}
