package main

import (
	"log"
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
		slog.Warn("WARN: DATABASE_URL env var is missing from config")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("FATAL: JWT_SECRET env var is missing from config")
	}

	resendApiKey := os.Getenv("RESEND_API_KEY")
	if resendApiKey == "" {
		log.Fatal("FATAL: RESEND_API_KEY env var is missing from config")
	}

	return &Config{
		Port:         port,
		DSName:       dsName,
		JWTSecret:    []byte(jwtSecret),
		ResendAPIKey: resendApiKey,
	}
}
