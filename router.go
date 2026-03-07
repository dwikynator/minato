package minato

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Router struct {
	mux *chi.Mux
}

func newRouter() *Router {
	return &Router{
		mux: chi.NewMux(),
	}
}

func (r *Router) Use(middlewares ...func(http.Handler) http.Handler) {
	r.mux.Use(middlewares...)
}

func (r *Router) Get(pattern string, h http.HandlerFunc) {
	r.mux.Get(pattern, h)
}

func (r *Router) Post(pattern string, h http.HandlerFunc) {
	r.mux.Post(pattern, h)
}

func (r *Router) Put(pattern string, h http.HandlerFunc) {
	r.mux.Put(pattern, h)
}

func (r *Router) Delete(pattern string, h http.HandlerFunc) {
	r.mux.Delete(pattern, h)
}

func (r *Router) Patch(pattern string, h http.HandlerFunc) {
	r.mux.Patch(pattern, h)
}

func (r *Router) Group(pattern string, fn func(r *Router)) {
	sub := newRouter()
	fn(sub)
	r.mux.Mount(pattern, sub.mux)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
