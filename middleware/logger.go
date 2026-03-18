package middleware

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/dwikynator/minato"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LogPrinter defines the logging methods required by middlewares.
type LogPrinter interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type defaultLogPrinter struct{}

// Info logs an info message using slog.
func (l *defaultLogPrinter) Info(msg string, args ...any) { slog.Info(msg, args...) }

// Error logs an error message using slog.
func (l *defaultLogPrinter) Error(msg string, args ...any) { slog.Error(msg, args...) }

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// WriteHeader captures the status code before writing it to the underlying ResponseWriter.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response body before writing it to the underlying ResponseWriter.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.body != nil {
		rw.body.Write(b)
	}
	return rw.ResponseWriter.Write(b)
}

type loggerConfig struct {
	bodyLogging bool
	printer     LogPrinter
}

type LoggerOption func(*loggerConfig)

// WithBodyLogging enables logging of request and response bodies.
func WithBodyLogging(enabled bool) LoggerOption {
	return func(c *loggerConfig) {
		c.bodyLogging = enabled
	}
}

// WithLogPrinter allows injecting a custom logger into the middleware.
func WithLogPrinter(p LogPrinter) LoggerOption {
	return func(c *loggerConfig) {
		c.printer = p
	}
}

// Logger returns a middleware that logs requests.
func Logger(opts ...LoggerOption) func(http.Handler) http.Handler {
	cfg := &loggerConfig{
		bodyLogging: false,
		printer:     &defaultLogPrinter{},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			var reqBody []byte
			var resBody *bytes.Buffer
			if cfg.bodyLogging {
				if r.Body != nil {
					reqBody, _ = io.ReadAll(r.Body)

					r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
				}

				resBody = &bytes.Buffer{}
			}
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           resBody,
			}
			next.ServeHTTP(rw, r)
			duration := time.Since(start)

			extra := []any{}
			if cfg.bodyLogging {
				extra = append(extra,
					"req_body", string(reqBody),
					"res_body", rw.body.String(),
				)
			}

			logEntry(
				cfg.printer,
				r.Method,
				r.URL.Path,
				rw.statusCode,
				duration,
				RequestIDFromContext(r.Context()),
				extra...,
			)
		})
	}
}

// logEntry is the shared core logic called by both the HTTP and gRPC forms.
func logEntry(p LogPrinter, method, path string, statusCode int, duration time.Duration, requestID string, extra ...any) {
	args := []any{
		"method", method,
		"path", path,
		"status", statusCode,
		"duration", duration.String(),
		"request_id", requestID,
	}
	args = append(args, extra...)
	p.Info("request", args...)
}

// LoggerInterceptor is the gRPC unary interceptor form of Logger.
func LoggerInterceptor(opts ...LoggerOption) grpc.UnaryServerInterceptor {
	cfg := &loggerConfig{printer: &defaultLogPrinter{}}
	for _, o := range opts {
		o(cfg)
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := int(status.Code(err)) // grpc status code as int
		logEntry(cfg.printer, "gRPC", info.FullMethod, code, time.Since(start), RequestIDFromContext(ctx))
		return resp, err
	}
}

// LoggerPlugin bundles Logger (HTTP) and LoggerInterceptor (gRPC).
func LoggerPlugin(opts ...LoggerOption) minato.Plugin {
	return minato.Plugin{
		HTTP: Logger(opts...),
		GRPC: LoggerInterceptor(opts...),
	}
}
