package minato

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Router is a wrapper around chi.Mux to prevent exposing third-party
// router dependencies directly to library consumers.
type Router struct {
	mux *chi.Mux
}

func newRouter() *Router {
	return &Router{
		mux: chi.NewMux(),
	}
}

// Use appends one or more middlewares onto the Router stack.
func (r *Router) Use(middlewares ...func(http.Handler) http.Handler) {
	r.mux.Use(middlewares...)
}

// Get adds the route `pattern` that matches a GET HTTP method to route handler `h`
func (r *Router) Get(pattern string, h http.HandlerFunc) {
	r.mux.Get(pattern, h)
}

// Post adds the route `pattern` that matches a POST HTTP method to route handler `h`
func (r *Router) Post(pattern string, h http.HandlerFunc) {
	r.mux.Post(pattern, h)
}

// Put adds the route `pattern` that matches a PUT HTTP method to route handler `h`
func (r *Router) Put(pattern string, h http.HandlerFunc) {
	r.mux.Put(pattern, h)
}

// Delete adds the route `pattern` that matches a DELETE HTTP method to route handler `h`
func (r *Router) Delete(pattern string, h http.HandlerFunc) {
	r.mux.Delete(pattern, h)
}

// Patch adds the route `pattern` that matches a PATCH HTTP method to route handler `h`
func (r *Router) Patch(pattern string, h http.HandlerFunc) {
	r.mux.Patch(pattern, h)
}

// Group creates a new inline-router with a fresh middleware stack.
func (r *Router) Group(pattern string, fn func(r *Router)) {
	sub := newRouter()
	fn(sub)
	r.mux.Mount(pattern, sub.mux)
}

// ServeHTTP implements the standard net/http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
