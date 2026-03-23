package main

import (
	"log/slog"
	"os"
)

type Config struct {
	Port         string
	DSName       string
	JWTSecret    []byte
	ResendAPIKey string
}

func LoadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsName := os.Getenv("DATABASE_URL")
	if dsName == "" {
		slog.Warn("DATABASE_URL not set, using default", "default", "agentmarket.db")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET env var is required but not set")
		os.Exit(1)
	}

	resendApiKey := os.Getenv("RESEND_API_KEY")
	if resendApiKey == "" {
		slog.Error("RESEND_API_KEY env var is required but not set")
		os.Exit(1)
	}

	slog.Info("config loaded", "port", port, "database_url_set", dsName != "")
	return &Config{
		Port:         port,
		DSName:       dsName,
		JWTSecret:    []byte(jwtSecret),
		ResendAPIKey: resendApiKey,
	}
}
