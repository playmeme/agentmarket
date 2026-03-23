package main

import (
	"log"
	"net/http"
)

// Version is injected at build time by ldflags
var Version = "development"

func main() {
	cfg := LoadConfig()
	db, err := InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	app := NewApp(cfg, db)
	router := NewRouter(app)

	log.Printf("Server starting on :%s (Version: %s)...", cfg.Port, Version)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
