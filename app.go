package main

import (
	"database/sql"
)

// App holds all the global dependencies for your application.
type App struct {
	Config *Config
	DB     *sql.DB
}

// NewApp initializes the central application state
func NewApp(cfg *Config, db *sql.DB) *App {
	return &App{
		Config: cfg,
		DB:     db,
	}
}
