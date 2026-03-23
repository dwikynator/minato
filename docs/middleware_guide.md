# Minato Middleware Placement Guide

This guide answers one question: **for a given cross-cutting concern, where do I
register it?**

Minato supports three server architectures, each shaped differently:

| Mode | How to run | What serves requests |
|---|---|---|
| **HTTP only** | `WithAddr` only | chi router → HTTP handlers |
| **gRPC only** | `WithGRPCAddr` only | gRPC server → gRPC handlers |
| **Gateway (dual)** | `WithAddr` + `WithGRPCAddr` | HTTP: chi → grpc-gateway → gRPC loopback |

The registration APIs and their scopes:

| API | Scope |
|---|---|
| `srv.Use(mw)` | All HTTP requests (pure REST + gateway ingress) |
| `srv.UseGRPC(interceptor)` | All gRPC requests (direct gRPC clients + gateway loopback) |
| `srv.UsePlugin(plugin)` | Both simultaneously (`Use` + `UseGRPC`) |

---

## Gateway request flow (dual mode)

This is critical to understand before deciding where to place a plugin:

```
                                             ← same process, loopback TCP ──→
HTTP Client ──► chi router ──► grpc-gateway ────────────────────────────────► gRPC server ──► handler
                   │                                                                │
               srv.Use()                                                      srv.UseGRPC()
             (HTTP middleware)                                               (gRPC interceptor)
```

When `UsePlugin()` is used, **a single request is intercepted TWICE** — once at
the HTTP perimeter and once at the gRPC perimeter.

---

## Decision Reference

### ✅ Use `UsePlugin()` — Both transports

Apply when the concern must cover **all** traffic regardless of transport,
and running it twice is either harmless or beneficial:

| Middleware | Why Plugin makes sense |
|---|---|
| **Panic Recovery** | An unrecoverable panic can occur at either perimeter. The HTTP net catches gateway or router crashes. The gRPC net catches service logic crashes. They never double-fire (the gRPC form returns an `error`; the panic never propagates up to the HTTP form). |
| **Auth (mixed workload)** | When your server hosts both pure REST routes **and** gRPC-gateway routes, `AuthPlugin` ensures no surface is left unprotected. For pure-gateway servers, prefer `UseGRPC` instead (see below). |
| **Custom context injection** | Injecting a tenant ID, feature flags, or environment context into every request's context — harmless to run at both boundaries. |

---

### ✅ Use `srv.Use()` — HTTP only

Apply when the concern is specific to the **HTTP perimeter** and has no gRPC
equivalent:

| Middleware | Why HTTP-only |
|---|---|
| **CORS** | A pure HTTP/browser mechanism. gRPC clients never send `Origin` or `OPTIONS` preflight requests. Registering it on gRPC would be meaningless. |
| **Rate Limiting (IP-based)** | IP address is only visible at the HTTP perimeter. The gRPC loopback call always originates from `127.0.0.1`. |
| **Request Size Limiting** | HTTP body size checks must happen before gateway translation reads the body. |
| **Security Headers** | `Strict-Transport-Security`, `X-Frame-Options`, etc. are HTTP response headers; irrelevant to gRPC wire format. |

---

### ✅ Use `srv.UseGRPC()` — gRPC only

Apply when the concern targets **business logic execution**, or when you want to
avoid double-execution on gateway routes:

| Middleware | Why gRPC-only |
|---|---|
| **Auth (pure gateway server)** | The gateway automatically forwards the HTTP `Authorization` header as gRPC `authorization` metadata. Validating at the gRPC perimeter covers all traffic exactly once. |
| **Validation** | Field validation errors are most ergonomically expressed as `gRPC Status` details (e.g. `BadRequest.FieldViolation`). Validation belongs next to the service handler, not at the HTTP edge. |
| **Business Metrics** | Recording which RPC methods are called, latency of service execution, error rates — these metrics are gRPC-semantic. Don't conflate them with gateway translation overhead. |
| **Distributed Tracing (spans)** | Trace spans for service execution should start at the gRPC boundary, after translation overhead. |

---

## The Double-Execution Table

For middleware registered via `UsePlugin()` in **gateway (dual) mode**, this is
the exact behavior for each built-in plugin:

| Plugin | HTTP fire | gRPC fire | Net result | Correct? |
|---|---|---|---|---|
| **RecoveryPlugin** | Catches gateway/router panics | Catches service panics | Two independent safety nets | ✅ Intentional |
| **LoggerPlugin** | Logs HTTP edge (method, path, status, total duration) | Logs gRPC execution (method, status, exec duration) | Two logs, same `request_id`, different durations → reveals gateway overhead | ✅ Intentional |
| **RequestIDPlugin** | Generates/reads UUID, injects into context AND `Grpc-Metadata-X-Request-Id` header | Reads UUID forwarded via metadata | Same UUID in both logs and both contexts | ✅ Correct (requires header propagation) |
| **AuthPlugin** | Validates token once at HTTP edge | Validates token again at gRPC perimeter | Token validated twice per request | ⚠️ Use `UseGRPC(AuthInterceptor(...))` for pure-gateway servers |

---

## Concrete `main.go` Templates

### Template A: HTTP only

```go
srv := minato.New(minato.WithAddr(":8080"))

srv.UsePlugin(
    middleware.RequestIDPlugin(),
    middleware.LoggerPlugin(),
    middleware.RecoveryPlugin(),
)
srv.Use(middleware.CORS())
```

### Template B: gRPC only (no gateway)

```go
srv := minato.New(minato.WithGRPCAddr(":50051"))

srv.UseGRPC(
    middleware.RequestIDInterceptor(),
    middleware.LoggerInterceptor(),
    middleware.RecoveryInterceptor(),
    middleware.AuthInterceptor(
        middleware.WithAuthSkipPaths("/auth.v1.AuthService/Login"),
        middleware.WithAuthValidator(tokenValidator.Validate),
    ),
)
```

### Template C: Gateway mode — pure gRPC service behind HTTP (recommended)

All REST endpoints are generated by grpc-gateway. No pure HTTP routes.

```go
srv := minato.New(
    minato.WithAddr(":8080"),
    minato.WithGRPCAddr(":50051"),
)

// HTTP perimeter: concerns specific to the outer HTTP edge
srv.Use(middleware.CORS())            // Browser preflights, HTTP-only
srv.Use(middleware.RequestID())       // Generate UUID + propagate to gRPC via Grpc-Metadata header
srv.Use(middleware.Logger())          // Log edge HTTP requests (total round-trip duration)

// gRPC perimeter: concerns specific to service execution
srv.UseGRPC(middleware.RecoveryInterceptor())   // Catch service panics
srv.UseGRPC(middleware.LoggerInterceptor())     // Log execution duration
srv.UseGRPC(middleware.AuthInterceptor(        // Validate once — gateway forwards the header
    middleware.WithAuthSkipPaths("/auth.v1.AuthService/Login"),
    middleware.WithAuthValidator(tokenValidator.Validate),
))
```

### Template D: Gateway mode — mixed REST + gRPC routes

Some routes are pure REST (`srv.Router().Get(...)`) and some are gRPC-gateway generated.

```go
srv := minato.New(
    minato.WithAddr(":8080"),
    minato.WithGRPCAddr(":50051"),
)

// Plugins cover both — safe because pure REST routes only hit HTTP,
// and gateway routes are covered at gRPC level too.
srv.UsePlugin(
    middleware.RequestIDPlugin(),
    middleware.LoggerPlugin(),
    middleware.RecoveryPlugin(),
)
srv.Use(middleware.CORS())

// ---------------------------------------------------------
// Auth: Split Registration for EXACTLY 1x Execution
// ---------------------------------------------------------

// 1. Protect ALL gRPC AND Gateway traffic 
// (Runs exactly 1x for both because Gateway forwards the HTTP header)
srv.UseGRPC(middleware.AuthInterceptor(
    middleware.WithAuthSkipPaths("/auth.v1.AuthService/Login"),
    middleware.WithAuthValidator(tokenValidator.Validate),
))

// 2. Protect Pure REST traffic by creating a dedicated sub-router
srv.Router().Group("/api/v1", func(r *minato.Router) {
    r.Use(middleware.AuthHTTP(
        // You only need to list HTTP skip paths here
        middleware.WithAuthSkipPaths("/api/v1/auth/login"),
        middleware.WithAuthValidator(tokenValidator.Validate),
    ))
    
    // Register pure HTTP endpoints ONLY inside this protected group
    r.Get("/custom", nil) 
})
```

---

## Quick Reference Cheatsheet

```
Have a new cross-cutting concern?

Is it HTTP-specific?        → srv.Use()
(CORS, IP rate limit,
 security headers)

Is it gRPC-specific?        → srv.UseGRPC()
(validation, auth on
 pure-gateway server,
 business metrics)

Does it need to cover       → srv.UsePlugin()
both surfaces?              (recovery, logging,
(pure REST + gRPC mixed     custom context, auth
 workloads)                  on mixed servers)
```
