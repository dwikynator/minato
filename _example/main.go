package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dwikynator/minato"
)

func main() {
	server := minato.New(
		minato.WithAddr(":8080"),
		minato.WithReadHeaderTimeout(5*time.Second),
		minato.WithIdleTimeout(10*time.Second),
		minato.WithShutdownTimeout(5*time.Second),
	)

	server.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if err := server.Run(); err != nil {
		panic(err)
	}
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
