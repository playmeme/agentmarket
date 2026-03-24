package main

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

func InitDB(cfg *Config) (*sql.DB, error) {
	dsn := cfg.DSName
	if dsn == "" {
		dsn = "./data/agentmarket.db"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	slog.Info("database initialized", "dsn", dsn)
	return db, nil
}

func RunMigrations(db *sql.DB) error {
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
		// M2: new job statuses — SQLite CHECK constraints can't be altered easily,
		// so we recreate the jobs table with expanded statuses via a migration
		// that uses a temp rename approach.
		// Instead, since SQLite doesn't support ALTER COLUMN, we handle new statuses
		// by dropping the CHECK constraint via recreation.
		`CREATE TABLE IF NOT EXISTS jobs_new (
			id TEXT PRIMARY KEY,
			employer_id TEXT NOT NULL REFERENCES users(id),
			agent_id TEXT NOT NULL REFERENCES agents(id),
			status TEXT NOT NULL DEFAULT 'PENDING_ACCEPTANCE' CHECK(status IN ('PENDING_ACCEPTANCE','IN_PROGRESS','COMPLETED','DISPUTED','CANCELLED','SOW_NEGOTIATION','AWAITING_PAYMENT','DELIVERED')),
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			total_payout INTEGER NOT NULL,
			timeline_days INTEGER NOT NULL,
			stripe_payment_intent TEXT,
			stripe_checkout_session_id TEXT,
			delivered_at DATETIME,
			delivery_notes TEXT,
			delivery_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT OR IGNORE INTO jobs_new (id, employer_id, agent_id, status, title, description, total_payout, timeline_days, stripe_payment_intent, created_at, updated_at)
		 SELECT id, employer_id, agent_id, status, title, description, total_payout, timeline_days, stripe_payment_intent, created_at, updated_at FROM jobs`,
		`DROP TABLE IF EXISTS jobs`,
		`ALTER TABLE jobs_new RENAME TO jobs`,
		// M2: sow table
		`CREATE TABLE IF NOT EXISTS sow (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL REFERENCES jobs(id),
			scope TEXT NOT NULL DEFAULT '',
			deliverables TEXT NOT NULL DEFAULT '',
			price_cents INTEGER NOT NULL DEFAULT 0,
			timeline_days INTEGER NOT NULL DEFAULT 0,
			agent_accepted INTEGER NOT NULL DEFAULT 0,
			employer_accepted INTEGER NOT NULL DEFAULT 0,
			last_edited_by TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// M3: notifications table
		`CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			job_id TEXT REFERENCES jobs(id),
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			read INTEGER NOT NULL DEFAULT 0,
			dismissed INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, read)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	slog.Info("migrations complete")
	return nil
}
