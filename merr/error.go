// Package merr provides ergonomic gRPC-aware err types for Minato services.
//
// Instead of manually constructing google.golang.org/grpc/status objects with
// details, handlers return a *merr.Error which implements the GRPCStatus()
// interface. The gRPC server runtime automatically picks up the rich status.

package merr

import (
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error is a structured gRPC-aware error. Handlers return this instead of
// manually composing google.golang.org/grpc/status objects
//
// It implements:
// - error 			- standar Go error interface
// - GRPCStatus() 	- automatically used by grpc-go to build the wire status
type Error struct {
	// Code is the canonical gRPC status code (e.g. codes.NotFound)
	Code codes.Code

	// Message is the human-readable error string. NOT for programmatic use by clients.
	Message string

	// Reason is the stable, machine-readable error identifier clients should switch on.
	// Example: "USER_NOT_FOUND", "INVALID_CREDENTIALS".
	Reason string

	// Domain identifes the service that produced the error (e.g. "core-auth")
	Domain string

	// Metadata carries optional key-value context (e.g. {"retry_after": "30"})
	Metadata map[string]string
}

// Error implements the standard error interface.
func (e *Error) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("%s: %s [%s]", e.Code, e.Message, e.Reason)
	}

	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// GRPCStatus converts this Error into a full gRPC Status with ErrorInfo details.
// The grpc-go server calls this automatically - no manual conversion needed.
func (e *Error) GRPCStatus() *status.Status {
	st := status.New(e.Code, e.Message)

	detail := &errdetails.ErrorInfo{
		Reason:   e.Reason,
		Domain:   e.Domain,
		Metadata: e.Metadata,
	}

	// WithDetails can only fail if the detail cannot be marshalled.
	// ErrorInfo is a well-known proto message, so this is safe.
	stWithDetails, err := st.WithDetails(detail)
	if err != nil {
		// Fallback: return the status without details rather than panic
		return st
	}
	return stWithDetails
}

// Is allows errors.Is matching by code, reason, and domain:
//
//	errors.Is(err, merr.ErrUserNotFound)
//
// All three fields must match so that errors sharing the same gRPC code but
// different reasons (e.g. USER_NOT_FOUND vs SESSION_NOT_FOUND) are correctly
// treated as distinct sentinels.
// Message and Metadata are intentionally excluded: Message is human-readable
// and may vary per call-site; Metadata carries runtime context.
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.Code == t.Code && e.Reason == t.Reason && e.Domain == t.Domain
	}
	return false
}
