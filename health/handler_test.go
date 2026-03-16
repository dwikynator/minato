package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLiveness(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	handler := Liveness()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status ok, got %q", response["status"])
	}
}

func TestReadiness(t *testing.T) {

	passingCheck := func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	failingCheck := func(ctx context.Context) error {
		time.Sleep(5 * time.Millisecond)
		return errors.New("database connection refused")
	}

	checks := map[string]CheckFunc{
		"postgres": passingCheck,
		"redis":    failingCheck,
	}

	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()

	handler := Readiness(checks)
	handler.ServeHTTP(rec, req)

	// Since Redis failed, we expect a 503 overall
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rec.Code)
	}
	// Read the JSON response
	var response struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}
	if response.Status != "unavailable" {
		t.Errorf("Expected status 'unavailable', got %q", response.Status)
	}
	if response.Checks["postgres"] != "ok" {
		t.Errorf("Expected postgres to be 'ok', got %q", response.Checks["postgres"])
	}
	if response.Checks["redis"] != "database connection refused" {
		t.Errorf("Expected redis to fail with specific message, got %q", response.Checks["redis"])
	}
}

func TestReadiness_NoChecks(t *testing.T) {
	// If the user didn't register anything, it should just return 200
	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()
	handler := Readiness(nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}
