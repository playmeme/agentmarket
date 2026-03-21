package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// Version is injected at build time by ldflags
var Version = "development"

// NewRouter sets up the routes so they can be tested independently
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"version": Version,
		})
	})

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	return mux
}

func main() {
	router := NewRouter()
	log.Printf("Server starting on :8080 (Version: %s)...", Version)
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
