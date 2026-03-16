package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecovery(t *testing.T) {
	// 1. Create a "poison pill" handler that intentionally panics
	panickyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("database connection suddenly dropped!")
	})

	// 2. Wrap our dangerous handler in the Recovery middleware
	secureHandler := Recovery()(panickyHandler)

	// 3. Create a fake incoming HTTP Request
	// This simulates a user's browser sending a GET request to "/"
	req := httptest.NewRequest("GET", "/", nil)

	// 4. Create a fake HTTP ResponseWriter (the Recorder)
	// Instead of writing bytes to real TCP socket, this Recorder captures
	// everything the handler writes (status code, headers, body) into memory,
	// so the test can inspect it afterwards
	rec := httptest.NewRecorder()

	// 5. Execute the handler
	secureHandler.ServeHTTP(rec, req)

	// 6. Assertions

	// A) Did it return a 500 Internal Server Error?
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, go %d", rec.Code)
	}

	// B) Did it set the correct Content-Type?
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}

	// C) Did it return the generic JSON error payload?
	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["error"] != "Internal Server Error" {
		t.Errorf("Expected generic error message, got %q", response["error"])
	}
}
