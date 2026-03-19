package minato

import (
	"context"
	"net/http"
)

// Response is the envelope returned by a GenericHandlerFunc
type Response[T any] struct {
	Data    T           `json:"-"`
	Status  int         `json:"-"`
	Headers http.Header `json:"-"`
}

// GenericHandlerFunc is the pure Go function signature for all business logic.
type GenericHandlerFunc[Req any, Res any] func(ctx context.Context, req Req) (Response[Res], error)

// ErrorResponse dictates how an error is translated into an HTTP response.
type ErrorResponse struct {
	Status  int
	Body    any
	Headers http.Header
}

// ErrorMapper translates a Go error into an HTTP-safe ErrorResponse
type ErrorMapper func(ctx context.Context, err error) ErrorResponse

// Validator is a pluggable interface for struct validation
type Validator interface {
	Validate(v any) error
}
