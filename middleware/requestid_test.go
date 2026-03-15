package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := RequestIDFromContext(r.Context())
		w.Write([]byte(id))
	})

	handler := RequestID()(nextHandler)

	t.Run("generates new ID if missing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		resID := rec.Header().Get("X-Request-ID")
		if resID == "" {
			t.Errorf("Expected X-Request-ID header to be set in response")
		}

		ctxID := rec.Body.String()
		if ctxID != resID {
			t.Errorf("Expected context ID %q to match response header ID %q", ctxID, resID)
		}
	})

	t.Run("uses existing ID if provided", func(t *testing.T) {
		existingID := "my-custom-request-id"
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Request-ID", existingID)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		resID := rec.Header().Get("X-Request-ID")
		if resID != existingID {
			t.Errorf("Expected response header to be %q, got %q", existingID, resID)
		}

		ctxID := rec.Body.String()
		if ctxID != existingID {
			t.Errorf("Expected context ID to be %q, got %q", existingID, ctxID)
		}
	})
}
