package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "agentmarket.db"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	DB = db
	log.Println("Database initialized")
	return nil
}

func RunMigrations() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			role TEXT NOT NULL CHECK(role IN ('EMPLOYER','AGENT_HANDLER')),
			name TEXT NOT NULL,
			handle TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			email_verified_at DATETIME,
			stripe_customer_id TEXT,
			stripe_account_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			handler_id TEXT NOT NULL REFERENCES users(id),
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			api_key_hash TEXT NOT NULL,
			webhook_url TEXT DEFAULT '',
			is_active INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			employer_id TEXT NOT NULL REFERENCES users(id),
			agent_id TEXT NOT NULL REFERENCES agents(id),
			status TEXT NOT NULL DEFAULT 'PENDING_ACCEPTANCE' CHECK(status IN ('PENDING_ACCEPTANCE','IN_PROGRESS','COMPLETED','DISPUTED','CANCELLED')),
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			total_payout INTEGER NOT NULL,
			timeline_days INTEGER NOT NULL,
			stripe_payment_intent TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS milestones (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL REFERENCES jobs(id),
			title TEXT NOT NULL,
			amount INTEGER NOT NULL,
			order_index INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','REVIEW_REQUESTED','APPROVED','PAID')),
			proof_of_work_url TEXT DEFAULT '',
			proof_of_work_notes TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS criteria (
			id TEXT PRIMARY KEY,
			milestone_id TEXT NOT NULL REFERENCES milestones(id),
			description TEXT NOT NULL,
			is_verified INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, migration := range migrations {
		if _, err := DB.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	log.Println("Migrations complete")
	return nil
}
