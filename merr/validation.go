package merr

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FieldViolation describes a single invalid field.
type FieldViolation struct {
	Field       string // e.g. "email", "password"
	Description string // e.g. "must be a valid email address"
}

// ValidationError creates a 400-equivalent error (gRPC: InvalidArgument)
// with structured field violations attached as google.rpc.BadRequest details.
//
// Usage in a handler:
//
//	return nil, merr.ValidationError("VALIDATION_FAILED", "request validation failed",
//	    merr.FieldViolation{Field: "email", Description: "must be a valid email"},
//	    merr.FieldViolation{Field: "password", Description: "minimum 8 characters"},
//	)
//
// Gateway response:
//
//	{
//	    "error": {
//	        "code": "VALIDATION_FAILED",
//	        "message": "request validation failed",
//	        "field_violations": [
//	            {"field": "email", "description": "must be a valid email"},
//	            {"field": "password", "description": "minimum 8 characters"}
//	        ]
//	    }
//	}
func NewValidationError(reason, message string, violations ...FieldViolation) error {
	st := status.New(codes.InvalidArgument, message)

	// Attach ErrorInfo for the reason code
	info := &errdetails.ErrorInfo{Reason: reason}

	// Attach BadRequest with field violations
	br := &errdetails.BadRequest{}
	for _, v := range violations {
		br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       v.Field,
			Description: v.Description,
		})
	}

	stWithDetails, err := st.WithDetails(info, br)
	if err != nil {
		return st.Err()
	}

	return stWithDetails.Err()
}
