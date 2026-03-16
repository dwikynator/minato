package main

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/dwikynator/minato"
	"github.com/dwikynator/minato/health"
	"github.com/dwikynator/minato/middleware"
)

func main() {
	// 1. Configure the server with options
	server := minato.New(
		minato.WithAddr(":8080"),
		minato.WithReadHeaderTimeout(5*time.Second),
		minato.WithIdleTimeout(10*time.Second),
		minato.WithShutdownTimeout(5*time.Second),

		minato.WithReadinessCheck("postgres", checkFakeDB),
	)

	// 2. Register Global Middleware (Order matters!)
	//  a. RequestID goes first so everything else can log it
	server.Use(middleware.RequestID())
	// b. Recovery goes second to catch panics in the Logger or Handlers
	server.Use(middleware.Recovery())
	// c. Logger goes third to time the handlers and log the results
	server.Use(middleware.Logger(
		middleware.WithBodyLogging(true), // Enable body loggin for testing
	))
	// d. CORS goes fourth to handle OPTIONS preflights and injects headers.
	server.Use(middleware.CORS(
		middleware.WithAllowedOrigins("https://example.com"),
		middleware.WithAllowedMethods("GET", "POST", "OPTIONS"),
		middleware.WithAllowedHeaders("Content-Type", "Authorization", "X-Request-ID"),
	))

	// 3. Register Routes

	// Health Endpoints
	server.Router().Get("/healthz", health.Liveness())

	// Manually passing our checks map for now
	checks := map[string]health.CheckFunc{
		"postgres": checkFakeDB,
	}
	server.Router().Get("/readyz", health.Readiness(checks))

	// Standard GET route
	server.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	// POST route to demonstrate body logging
	server.Router().Post("/echo", func(w http.ResponseWriter, r *http.Request) {
		// Just copy the request body to the response body
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		// Add something to the payload to prove we touched it
		payload["echoed_at"] = time.Now().Format(time.RFC3339)

		WriteJSON(w, http.StatusOK, payload)
	})
	// "Poison pill" route to test the Recovery middleware
	server.Router().Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("Oops! Something went terribly wrong here.")
	})
	// 4. Start the server (blocks until SIGINT/SIGTERM)
	if err := server.Run(); err != nil {
		panic(err)
	}
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// checkFakeDB simulates a database connection that randomly fails 30% of the time
func checkFakeDB(ctx context.Context) error {
	time.Sleep(20 * time.Millisecond) // Simulate network latency
	if rand.Float32() < 0.3 {
		return errors.New("connection timeout")
	}
	return nil
}
