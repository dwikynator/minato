package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

// CheckFunc is a function that checks a dependency's health
// It should return nil if healthy, or an error if unhealthy
type CheckFunc func(ctx context.Context) error

// Liveness returns an http.HandlerFunc for GET /healthz
// Always returns 200 {"status": "ok"} - just proves the process is alive
func Liveness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// Rediness returns an http.HandlerFunc for GET /readyz
// It runs all registered checks concurrently
// Returns 200 if all pass, 503 if any fail
func Readiness(checks map[string]CheckFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If there are no check registered, we are always ready
		if len(checks) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"checks": map[string]string{},
			})
			return
		}

		// need to run checks concurrently so one slow dependency doesn't delay everything.
		var wg sync.WaitGroup
		var mu sync.Mutex // Protecs the results map from concurrent writes

		results := make(map[string]string)
		isHealthy := true

		wg.Add(len(checks))

		for name, checkFn := range checks {
			go func(checkName string, fn CheckFunc) {
				defer wg.Done()

				err := fn(r.Context())

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					results[checkName] = err.Error()
					isHealthy = false
				} else {
					results[checkName] = "ok"
				}
			}(name, checkFn)
		}
		// wait for all checks to finish
		wg.Wait()

		// Determine the final HTTP status code and overall status string
		statusCode := http.StatusOK
		overallStatus := "ok"
		if !isHealthy {
			statusCode = http.StatusServiceUnavailable
			overallStatus = "unavailable"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]any{
			"status": overallStatus,
			"checks": results,
		})
	}
}
