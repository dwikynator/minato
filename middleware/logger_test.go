package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	// 1. Create a buffer to capture the log output
	var buf bytes.Buffer

	// 2. Set up a new slog logger that writes to our buffer isntead of os.Stdout
	handler := slog.NewTextHandler(&buf, nil)
	logger := slog.New(handler)

	// 3. Override the default global slog logger just for the test
	slog.SetDefault(logger)

	// 4. Create a dummy next handler that returns 201 Created
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// 5. Wrap it with new logger middleware
	middleware := Logger()(next)

	// 6. Create a fake request (with a fake request_id in context)
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	ctx := context.WithValue(req.Context(), requestIDKey, "test-id-123")
	req = req.WithContext(ctx)

	// 7. Create a fake response recorder
	rec := httptest.NewRecorder()

	// 8. Run the middleware
	middleware.ServeHTTP(rec, req)

	// 9. Inspect what got written to our buffer
	logOutput := buf.String()

	// 10. Assertions
	if !strings.Contains(logOutput, "method=GET") {
		t.Errorf("Expected log to contain method=GET, got: %s", logOutput)
	}

	if !strings.Contains(logOutput, "path=/api/v1/test") {
		t.Errorf("Expected log to contain path=/api/v1/test, got: %s", logOutput)
	}

	if !strings.Contains(logOutput, "status=201") {
		t.Errorf("Expected log to contain status=201, got: %s", logOutput)
	}

	if !strings.Contains(logOutput, "duration") {
		t.Errorf("Expected log to contain duration, got: %s", logOutput)
	}

	if !strings.Contains(logOutput, "request_id=test-id-123") {
		t.Errorf("Expected log to contain request_id=test-id-123, got: %s", logOutput)
	}
}

func TestLogger_WithBodyLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response":"success"}`))
	})

	middleware := Logger(WithBodyLogging(true))(next)

	req := httptest.NewRequest("POST", "/api/v1/data", bytes.NewBufferString(`{"request":"data"}`))
	ctx := context.WithValue(req.Context(), requestIDKey, "test-id-456")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	logOutput := buf.String()

	if !strings.Contains(logOutput, `req_body="{\"request\":\"data\"}"`) {
		t.Errorf("Expected log to contain req_body, got: %s", logOutput)
	}

	if !strings.Contains(logOutput, `res_body="{\"response\":\"success\"}"`) {
		t.Errorf("Expected log to contain res_body, got: %s", logOutput)
	}
}
