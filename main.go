package main

import (
	"log"
	"net/http"
	"os"
)

// Version is injected at build time by ldflags
var Version = "development"

func main() {
	if err := InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer DB.Close()

	if err := RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := NewRouter()
	log.Printf("Server starting on :%s (Version: %s)...", port, Version)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
