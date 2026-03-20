package merr

import "google.golang.org/grpc/codes"

// ---------------------------------------------------------------------------
// HTTP-Semantic Error Constructors
//
// Each constructor maps a familiar HTTP concept to the correct gRPC status
// code. Engineers call these instead of remembering gRPC code enums.
//
//   HTTP Status               gRPC Code               Constructor
//   400 Bad Request           InvalidArgument          BadRequest
//   401 Unauthorized          Unauthenticated          Unauthorized
//   403 Forbidden             PermissionDenied         Forbidden
//   404 Not Found             NotFound                 NotFound
//   409 Conflict              AlreadyExists            Conflict
//   412 Precondition Failed   FailedPrecondition       PreconditionFailed
//   429 Too Many Requests     ResourceExhausted        TooManyRequests
//   500 Internal Server Error Internal                 Internal
//   503 Service Unavailable   Unavailable              Unavailable
// ---------------------------------------------------------------------------

// Opt is a functional option for enriching an Error after construction.
type Opt func(*Error)

// WithMetadata attaches key-value metadata to the error.
// Example: merr.NotFound("USER_NOT_FOUND", "no user found", merr.WithMetadata("user_id", "abc"))
func WithMetadata(key, value string) Opt {
	return func(e *Error) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]string)
		}
		e.Metadata[key] = value
	}
}

// WithDomain overrides the default domain for this error.
func WithDomain(domain string) Opt {
	return func(e *Error) {
		e.Domain = domain
	}
}

// newError is the internal factory used by all constructors.
func newError(code codes.Code, reason, message string, opts []Opt) *Error {
	e := &Error{
		Code:    code,
		Message: message,
		Reason:  reason,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// BadRequest creates a 400-equivalent error (gRPC: InvalidArgument).
// Use for malformed input, type errors, or constraint violations on input.
func BadRequest(reason, message string, opts ...Opt) *Error {
	return newError(codes.InvalidArgument, reason, message, opts)
}

// Unauthorized creates a 401-equivalent error (gRPC: Unauthenticated).
// Use when credentials are missing, invalid, or expired.
func Unauthorized(reason, message string, opts ...Opt) *Error {
	return newError(codes.Unauthenticated, reason, message, opts)
}

// Forbidden creates a 403-equivalent error (gRPC: PermissionDenied).
// Use when the caller is authenticated but lacks permission.
func Forbidden(reason, message string, opts ...Opt) *Error {
	return newError(codes.PermissionDenied, reason, message, opts)
}

// NotFound creates a 404-equivalent error (gRPC: NotFound).
func NotFound(reason, message string, opts ...Opt) *Error {
	return newError(codes.NotFound, reason, message, opts)
}

// Conflict creates a 409-equivalent error (gRPC: AlreadyExists).
// Use for duplicate resources or uniqueness constraint violations.
func Conflict(reason, message string, opts ...Opt) *Error {
	return newError(codes.AlreadyExists, reason, message, opts)
}

// PreconditionFailed creates a 412-equivalent error (gRPC: FailedPrecondition).
// Use when a required precondition is not met (e.g., account not verified).
func PreconditionFailed(reason, message string, opts ...Opt) *Error {
	return newError(codes.FailedPrecondition, reason, message, opts)
}

// TooManyRequests creates a 429-equivalent error (gRPC: ResourceExhausted).
func TooManyRequests(reason, message string, opts ...Opt) *Error {
	return newError(codes.ResourceExhausted, reason, message, opts)
}

// Internal creates a 500-equivalent error (gRPC: Internal).
// Use for unexpected server-side failures. Never expose internals in the message.
func Internal(reason, message string, opts ...Opt) *Error {
	return newError(codes.Internal, reason, message, opts)
}

// Unavailable creates a 503-equivalent error (gRPC: Unavailable).
// Use when the service is temporarily unable to handle the request.
func Unavailable(reason, message string, opts ...Opt) *Error {
	return newError(codes.Unavailable, reason, message, opts)
}
