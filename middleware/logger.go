package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// LogPrinter defines the logging methods required by middlewares.
type LogPrinter interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type defaultLogPrinter struct{}

func (l *defaultLogPrinter) Info(msg string, args ...any)  { slog.Info(msg, args...) }
func (l *defaultLogPrinter) Error(msg string, args ...any) { slog.Error(msg, args...) }

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

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

			logArgs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration", duration.String(),
				"request_id", RequestIDFromContext(r.Context()),
			}

			if cfg.bodyLogging {
				logArgs = append(logArgs,
					"req_body", string(reqBody),
					"res_body", rw.body.String(),
				)
			}

			cfg.printer.Info("request", logArgs...)
		})
	}
}
