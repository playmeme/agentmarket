package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
)

// Version is injected at build time by ldflags
var Version = "development"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

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

	slog.Info("server starting", "port", cfg.Port, "version", Version)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
