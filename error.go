package minato

import (
	"context"
	"encoding/json"
	"net/http"
)

// ValidationError is a framework-owned wrapper applied by the adapter around
// any error returned by a pluggable Validator.
//
// Why wrap? Minato's Validator interface is pluggable — the framework does not
// know which validation library (go-playground/validator, ozzo-validation, etc.)
// the developer will use, so it cannot type-assert against a library-specific
// error type directly inside defaultErrorMapper.
//
// Instead, the adapter always wraps validation failures in *ValidationError
// before calling handleError. That way the mapper only ever needs to check
// for this one, stable, framework-owned type.
type ValidationError struct {
	Err error // the original error from the Validator implementation
}

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// handleError runs the mapper and serializes the error response.
func handleError(ctx context.Context, w http.ResponseWriter, err error, mapper ErrorMapper) {
	res := mapper(ctx, err)

	// Write standard error headers
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(res.Status)

	_ = json.NewEncoder(w).Encode(res.Body)
}

// defaultErrorMapper is the built-in ErrorMapper used by every route unless
// overridden with WithErrorMapper. It implements the three-tier contract:
//   - Binding errors   → 400 with structured field details
//   - Validation errors → 400 with the validation message
//   - Everything else  → 500 with a safe generic message (no internal leak)
func defaultErrorMapper(_ context.Context, err error) ErrorResponse {
	// 1. Binding / type-coercion error: produced by bindRequest.
	if bindErr, ok := err.(*BindError); ok {
		return ErrorResponse{
			Status: http.StatusBadRequest,
			Body: map[string]string{
				"error":  "invalid parameter",
				"field":  bindErr.Field,
				"source": bindErr.Source,
			},
		}
	}

	// 2. Validation error: produced by the pluggable Validator, wrapped by the adapter.
	// We check for *ValidationError (our own type), NOT for a library-specific type,
	// keeping the mapper decoupled from any particular validation library.
	if valErr, ok := err.(*ValidationError); ok {
		return ErrorResponse{
			Status: http.StatusBadRequest,
			Body: map[string]string{
				"error": valErr.Error(),
			},
		}
	}

	// 3. Unknown / unhandled error: return a safe generic 500.
	// Never expose internal error details to the caller.
	return ErrorResponse{
		Status: http.StatusInternalServerError,
		Body:   map[string]string{"error": "internal server error"},
	}
}
