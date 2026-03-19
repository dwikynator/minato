package minato

import "net/http"

// OK returns a 200 OK response.
func OK[T any](data T) Response[T] {
	return Response[T]{Data: data, Status: http.StatusOK}
}

// Created returns a 201 Created response.
func Created[T any](data T) Response[T] {
	return Response[T]{Data: data, Status: http.StatusCreated}
}

// NoContent returns a 204 No Content response.
func NoContent() Response[struct{}] {
	return Response[struct{}]{Status: http.StatusNoContent}
}

// SetHeader replaces any existing values for the given header key.
func (r Response[T]) SetHeader(key, value string) Response[T] {
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	r.Headers.Set(key, value)
	return r
}

// AddHeader appends a value to the given header key (useful for Set-Cookie)
func (r Response[T]) AddHeader(key, value string) Response[T] {
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	r.Headers.Add(key, value)
	return r
}
