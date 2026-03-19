package minato

import (
	"encoding/json"
	"net/http"
)

// RouteOptions holds all per-route configuration for a generic handler adapter.
type RouteOptions struct {
	Validator    Validator
	Mapper       ErrorMapper
	MaxBodyBytes int64 // 0 = unlimited
	StrictJSON   bool  // reject unknown JSON fields
}

type RouteOption func(*RouteOptions)

// WithValidator attaches a struct validator to this route
func WithValidator(v Validator) RouteOption {
	return func(o *RouteOptions) {
		o.Validator = v
	}
}

// WithErrorMapper replaces the default error-to-HTTP translation function for this route.
func WithErrorMapper(m ErrorMapper) RouteOption {
	return func(o *RouteOptions) {
		o.Mapper = m
	}
}

// WithMaxBodyBytes limits the JSON request body size. Prevents oversized payload attacks.
// Example: minato.WithMaxBodyBytes(1 << 20) caps the body at 1 MiB
func WithMaxBodyBytes(n int64) RouteOption {
	return func(o *RouteOptions) {
		o.MaxBodyBytes = n
	}
}

// WithStrictJSON enables json.Decoder.DisallowUnknownFields()
// Requests that contain JSON keys not present in Req are rejected with 400.
func WithStrictJSON(strict bool) RouteOption {
	return func(o *RouteOptions) {
		o.StrictJSON = strict
	}
}

// wrap is the core adapter. It compiles the binder plan ONCE at registration time
// and returns a standard http.HandlerFunc closeure for per-request execution
func wrap[Req any, Res any](h GenericHandlerFunc[Req, Res], opts ...RouteOption) http.HandlerFunc {
	// Apply all RouteOptions at registration time
	cfg := &RouteOptions{
		Mapper: defaultErrorMapper,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// compileBinder runs ONCE; panics on misconfiguration (non-Struct Req).
	plan, err := compileBinder[Req]()
	if err != nil {
		panic(err) // Fail fast at startup; not a runtime error.
	}

	// Snapshot bind config from route options for the closure.
	bCfg := bindConfig{MaxBodyBytes: cfg.MaxBodyBytes, StrictJSON: cfg.StrictJSON}

	// The returned closure is the actual http.HandlerFunc net/http calls per request.
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// A. Bind: populate Req from all HTTP sources.
		req, err := bindRequest[Req](r, plan, bCfg)
		if err != nil {
			handleError(ctx, w, err, cfg.Mapper)
			return
		}

		// B. Validate (optional): runs after binding if a Validator was registered.
		if cfg.Validator != nil {
			if err := cfg.Validator.Validate(req); err != nil {
				handleError(ctx, w, &ValidationError{Err: err}, cfg.Mapper)
				return
			}
		}

		// C. Execute pure business logic function.
		res, err := h(ctx, req)
		if err != nil {
			handleError(ctx, w, err, cfg.Mapper)
			return
		}

		// D. Copy response headers (supports multi-value, e.g. Set-Cookie)
		for k, vals := range res.Headers {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}

		// E. 204 No Content: write status only, no body or Content-Type.
		if res.Status == http.StatusNoContent {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// F. AUto Content-Type + JSON body write.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		status := res.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res.Data)
	}
}

// Package-level route helpers. These are package-level (not methods) because Go does not
// support generic type parameters on methods of non-generic receiver types.
func Get[Req any, Res any](r *Router, pattern string, h GenericHandlerFunc[Req, Res], opts ...RouteOption) {
	r.Get(pattern, wrap(h, opts...))
}

func Post[Req any, Res any](r *Router, pattern string, h GenericHandlerFunc[Req, Res], opts ...RouteOption) {
	r.Post(pattern, wrap(h, opts...))
}

func Put[Req any, Res any](r *Router, pattern string, h GenericHandlerFunc[Req, Res], opts ...RouteOption) {
	r.Put(pattern, wrap(h, opts...))
}

func Delete[Req any, Res any](r *Router, pattern string, h GenericHandlerFunc[Req, Res], opts ...RouteOption) {
	r.Delete(pattern, wrap(h, opts...))
}

func Patch[Req any, Res any](r *Router, pattern string, h GenericHandlerFunc[Req, Res], opts ...RouteOption) {
	r.Patch(pattern, wrap(h, opts...))
}
