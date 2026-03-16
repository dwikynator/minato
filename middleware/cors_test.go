package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	// 1. Create a dummy handler that sets a custom header so we know it was executed
	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// 2. Wrap it with our CORS middleware, using some custom options to test configuration
	handler := CORS(
		WithAllowedOrigins("https://example.com"),
		WithAllowedMethods("GET", "POST", "OPTIONS"),
	)(nextHandler)

	t.Run("normal request", func(t *testing.T) {
		nextHandlerCalled = false // Reset flag
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// A) Did it call the next handler
		if !nextHandlerCalled {
			t.Error("Expected next handler to be called for GET request")
		}

		// B) Did it set the Origin header on the response?
		origin := rec.Header().Get("Access-Control-Allow-Origin")
		if origin != "https://example.com" {
			t.Errorf("Expected Origin Header to be https://example.com, got %q", origin)
		}
	})

	t.Run("preflight OPTIONS request", func(t *testing.T) {
		nextHandlerCalled = false // Reset flag
		req := httptest.NewRequest("OPTIONS", "/", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// A) Did it STOP before calling the next handler?
		if nextHandlerCalled {
			t.Error("Did not expect next handler to be called for OPTIONS request")
		}

		// B) Did it return 204 No Content?
		if rec.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", rec.Code)
		}

		// C) Did it set the Origin header?
		origin := rec.Header().Get("Access-Control-Allow-Origin")
		if origin != "https://example.com" {
			t.Errorf("Expected Origin header to be https://example.com, got %q", origin)
		}
		// D) Did it set the Methods header?
		methods := rec.Header().Get("Access-Control-Allow-Methods")
		if methods != "GET, POST, OPTIONS" {
			t.Errorf("Expected Methods header to be GET, POST, OPTIONS, got %q", methods)
		}
		// E) Did it set the default Headers header? (since we didn't specify it in config)
		headers := rec.Header().Get("Access-Control-Allow-Headers")
		expectedHeaders := "Content-Type, Authorization, X-Request-ID"
		if headers != expectedHeaders {
			t.Errorf("Expected Headers to be %q, got %q", expectedHeaders, headers)
		}
	})
}
