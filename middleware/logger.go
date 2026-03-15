package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"
)

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
}

type LoggerOption func(*loggerConfig)

func WithBodyLogging(enabled bool) LoggerOption {
	return func(c *loggerConfig) {
		c.bodyLogging = enabled
	}
}

func Logger(opts ...LoggerOption) func(http.Handler) http.Handler {
	cfg := &loggerConfig{
		bodyLogging: false,
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
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.statusCode),
				slog.String("duration", duration.String()),
				slog.String("request_id", RequestIDFromContext(r.Context())),
			}

			if cfg.bodyLogging {
				logArgs = append(logArgs, slog.String("req_body", string(reqBody)),
					slog.String("res_body", rw.body.String()),
				)
			}

			slog.Info("request", logArgs...)
		})
	}
}
