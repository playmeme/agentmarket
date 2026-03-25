package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func InitDB(cfg *Config) (*sql.DB, error) {
	dsn := cfg.DSName
	if dsn == "" {
		dsn = "./data/agentmarket.db"
	}

	// Embed PRAGMAs in the DSN so they apply to every connection in the pool,
	// not just the first one. Fixes #42: foreign_keys was silently disabled for
	// most requests when set via a standalone db.Exec call.
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn += sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Limit the connection pool so SQLite's write-lock contention stays bounded.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	slog.Info("database initialized", "dsn", dsn)
	return db, nil
}

// migrations is an ordered list of schema changes; index i upgrades the database
// from version i to version i+1. Add new migrations by appending to this slice —
// never modify existing entries.
var migrations = []func(tx *sql.Tx) error{
	// version 0 → 1: consolidated initial schema.
	// Combines all previous ad-hoc migrations (M1 initial tables, M2 jobs expansion,
	// M3 RETRACTED status + sow + notifications) into a single idempotent migration.
	// Fresh databases get the final schema directly; existing databases that already
	// ran the old ad-hoc migrations will have user_version=0 and will re-run this,
	// but all statements use CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS
	// so they are safe to run against a database that already has these tables.
	func(tx *sql.Tx) error {
		stmts := []string{
			// Remove any leftover temp tables from the old ad-hoc rebuild approach.
			`DROP TABLE IF EXISTS jobs_new`,
			`DROP TABLE IF EXISTS jobs_retracted`,

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

			// Final jobs schema: expanded CHECK constraint (all statuses through RETRACTED)
			// and delivery / stripe-checkout columns added in M2/M3.
			`CREATE TABLE IF NOT EXISTS jobs (
				id TEXT PRIMARY KEY,
				employer_id TEXT NOT NULL REFERENCES users(id),
				agent_id TEXT NOT NULL REFERENCES agents(id),
				status TEXT NOT NULL DEFAULT 'PENDING_ACCEPTANCE' CHECK(status IN (
					'PENDING_ACCEPTANCE','IN_PROGRESS','COMPLETED','DISPUTED','CANCELLED',
					'SOW_NEGOTIATION','AWAITING_PAYMENT','DELIVERED','RETRACTED'
				)),
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


			`CREATE TABLE IF NOT EXISTS refresh_tokens (
				token_hash TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				expires_at DATETIME NOT NULL,
				revoked INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id)`,

		}

		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("statement failed: %w\nSQL: %s", err, stmt)
			}
		}
		return nil
	},
}

// complexMigration pins a single connection, disables foreign keys for the duration,
// and passes that connection to fn. Use this for table-rebuild migrations (the
// rename/copy/drop pattern) where active FK constraints would block the DROP.
// fn is responsible for beginning and committing its own transaction on conn.
func complexMigration(db *sql.DB, fn func(conn *sql.Conn) error) error {
	ctx := context.Background()

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("complexMigration: failed to acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("complexMigration: failed to disable foreign keys: %w", err)
	}
	defer func() {
		// Best-effort re-enable; the connection is about to be closed.
		_, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	}()

	return fn(conn)
}

// RunMigrations applies any pending migrations using PRAGMA user_version to track
// which migrations have already been applied. It is idempotent: migrations whose
// index is below the current user_version are skipped. Each migration runs in its
// own transaction; on failure the transaction is rolled back and the error is returned
// with the version numbers for easier debugging.
func RunMigrations(db *sql.DB) error {
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("failed to read user_version: %w", err)
	}

	total := len(migrations)
	if current >= total {
		slog.Info("migrations: database is up to date", "version", current)
		return nil
	}

	slog.Info("migrations: applying pending migrations", "from_version", current, "to_version", total)

	for i := current; i < total; i++ {
		slog.Info("migrations: applying", "version_from", i, "version_to", i+1)

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migration %d→%d: failed to begin transaction: %w", i, i+1, err)
		}

		if err := migrations[i](tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d→%d failed: %w", i, i+1, err)
		}

		// Write the new version inside the same transaction so it is atomic with
		// the schema change. PRAGMA inside a transaction is valid for user_version.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d→%d: failed to set user_version: %w", i, i+1, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d→%d: failed to commit: %w", i, i+1, err)
		}

		slog.Info("migrations: applied", "version", i+1)
	}

	slog.Info("migrations complete", "version", total)
	return nil
}
