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

// sqliteTime is a sql.Scanner that accepts the string values that
// modernc.org/sqlite returns for DATETIME/TIMESTAMP columns and converts
// them into a *time.Time. SQLite stores timestamps as text (RFC 3339 or
// the "2006-01-02 15:04:05" format produced by CURRENT_TIMESTAMP), and
// the pure-Go modernc.org/sqlite driver does not automatically coerce
// those strings to time.Time — so scanning directly into a *time.Time
// field fails with "unsupported Scan … type string into type *time.Time".
//
// Usage: scan into sqliteTime{&myStruct.CreatedAt} instead of &myStruct.CreatedAt.
type sqliteTime struct{ t *time.Time }

// sqliteTimeFormats lists the timestamp layouts that SQLite / CURRENT_TIMESTAMP
// can produce, in the order we try them.
var sqliteTimeFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999999Z07:00",
}

func (s sqliteTime) Scan(src interface{}) error {
	if src == nil {
		*s.t = time.Time{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		*s.t = v
		return nil
	case string:
		// Treat empty string as zero/NULL time — existing rows may have
		// updated_at = "" (empty string, not NULL) rather than a real timestamp.
		if v == "" {
			*s.t = time.Time{}
			return nil
		}
		for _, layout := range sqliteTimeFormats {
			if t, err := time.Parse(layout, v); err == nil {
				*s.t = t
				return nil
			}
		}
		return fmt.Errorf("sqliteTime: cannot parse %q as a timestamp", v)
	case []byte:
		return sqliteTime{s.t}.Scan(string(v))
	default:
		return fmt.Errorf("sqliteTime: unsupported type %T", src)
	}
}

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
				role TEXT NOT NULL CHECK(role IN ('EMPLOYER','AGENT_MANAGER')),
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
				manager_id TEXT NOT NULL REFERENCES users(id),
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
			// agent_id is nullable: cleared to NULL when a job offer is retracted.
			`CREATE TABLE IF NOT EXISTS jobs (
				id TEXT PRIMARY KEY,
				employer_id TEXT NOT NULL REFERENCES users(id),
				agent_id TEXT REFERENCES agents(id),
				status TEXT NOT NULL DEFAULT 'UNASSIGNED' CHECK(status IN (
					'UNASSIGNED','PENDING_ACCEPTANCE','IN_PROGRESS','COMPLETED','DISPUTED','CANCELLED',
					'SOW_NEGOTIATION','AWAITING_PAYMENT','DELIVERED'
				)),
				title TEXT NOT NULL,
				description TEXT DEFAULT '',
				total_payout INTEGER NOT NULL,
				timeline_days INTEGER NOT NULL,
				sow_link TEXT DEFAULT '',
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
				sow_id TEXT NOT NULL REFERENCES sow(id),
				title TEXT NOT NULL,
				amount INTEGER NOT NULL,
				order_index INTEGER NOT NULL,
				deliverables TEXT NOT NULL DEFAULT '',
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
				detailed_spec TEXT NOT NULL DEFAULT '',
				work_process TEXT NOT NULL DEFAULT '',
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

	// version 1 → 2: add employer_provides column to sow table.
	// Fresh databases created after migration 0 already have this column
	// (it was added to the CREATE TABLE IF NOT EXISTS statement), so this
	// migration only needs to run on existing databases that were created
	// before employer_provides was introduced.
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`ALTER TABLE sow ADD COLUMN employer_provides TEXT NOT NULL DEFAULT ''`)
		if err != nil {
			// Ignore "duplicate column name" error — column already exists on databases
			// that were created fresh with migration 0 after the schema was updated.
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("add employer_provides to sow: %w", err)
			}
		}
		return nil
	},

	// version 2 → 3: placeholder — this migration is handled by rawMigrations[2]
	// below because it requires PRAGMA foreign_keys = OFF at the connection level
	// (not inside a transaction). With _pragma=foreign_keys(1) in the DSN the
	// connection-level setting overrides any in-transaction PRAGMA, so the
	// PRAGMA inside a sql.Tx is silently ignored and DROP TABLE jobs fails with
	// FK constraint errors. complexMigration() pins a raw *sql.Conn and disables
	// FK enforcement before beginning the transaction.
	func(tx *sql.Tx) error { return nil },

	// version 3 → 4: Issue #44 SoW redesign — replace scope/deliverables/employer_provides
	// with detailed_spec and work_process in the sow table.
	// Fresh databases created after migration 0 was updated already have detailed_spec
	// and work_process (and no scope/deliverables columns), so we only copy data when
	// the old columns exist.
	func(tx *sql.Tx) error {
		// Add detailed_spec (replaces scope)
		if _, err := tx.Exec(`ALTER TABLE sow ADD COLUMN detailed_spec TEXT NOT NULL DEFAULT ''`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("add detailed_spec to sow: %w", err)
			}
		}
		// Add work_process (replaces deliverables + employer_provides)
		if _, err := tx.Exec(`ALTER TABLE sow ADD COLUMN work_process TEXT NOT NULL DEFAULT ''`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("add work_process to sow: %w", err)
			}
		}
		// Migrate legacy data: copy scope → detailed_spec, deliverables → work_process.
		// On fresh databases 'scope' does not exist; the UPDATE will return an error
		// which we detect by checking for "no such column". On existing databases with
		// legacy data the UPDATE copies content into the new columns.
		_, updateErr := tx.Exec(`UPDATE sow SET detailed_spec = scope, work_process = deliverables WHERE detailed_spec = ''`)
		if updateErr != nil && !strings.Contains(updateErr.Error(), "no such column") {
			return fmt.Errorf("migrate sow data: %w", updateErr)
		}
		return nil
	},

	// version 4 → 5: Issue #44 — add sow_link column to jobs table (optional URL).
	func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN sow_link TEXT DEFAULT ''`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("add sow_link to jobs: %w", err)
			}
		}
		return nil
	},

	// version 5 → 6: Issue #44 — add deliverables column to milestones table.
	func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE milestones ADD COLUMN deliverables TEXT NOT NULL DEFAULT ''`); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("add deliverables to milestones: %w", err)
			}
		}
		return nil
	},

	// version 6 → 7: Issue #65 — add UNASSIGNED status to jobs table.
	// UNASSIGNED is the initial status for newly created jobs that have no agent
	// assigned yet. PENDING_ACCEPTANCE is reserved for jobs where an offer has
	// been made to a specific agent. Because SQLite does not support ALTER TABLE
	// ... MODIFY COLUMN, we must rebuild the table. This is handled by
	// rawMigrations[6] below (requires PRAGMA foreign_keys = OFF at connection level).
	func(tx *sql.Tx) error { return nil },

	// version 7 → 8: Issue #66 — replace milestones.job_id with milestones.sow_id.
	// Milestones now link directly to their Statement of Work rather than the job.
	// The job can still be reached by traversing sow.job_id. Because SQLite does
	// not support DROP COLUMN we rebuild the milestones table. This requires
	// PRAGMA foreign_keys = OFF at the connection level and is handled by
	// rawMigrations[7] below.
	func(tx *sql.Tx) error { return nil },

	// version 8 → 9: Issue #88 — rename AGENT_HANDLER → AGENT_MANAGER.
	// Renames handler_id → manager_id in the agents table and updates the role
	// CHECK constraint in the users table from AGENT_HANDLER to AGENT_MANAGER.
	// Both changes require table rebuilds (SQLite does not support ALTER COLUMN),
	// so this is handled by rawMigrations[8] below (requires PRAGMA foreign_keys = OFF).
	func(tx *sql.Tx) error { return nil },

	// version 9 → 10: Issue #96 — add coupons table for payment discount codes.
	// code is unique; value is either a percentage (e.g. "10%") or a flat dollar
	// amount (e.g. "91.00"). max_uses limits total redemptions; times_used tracks
	// how many times the coupon has been successfully applied.
	func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS coupons (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			max_uses INTEGER NOT NULL DEFAULT 1,
			times_used INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
		if err != nil {
			return fmt.Errorf("create coupons table: %w", err)
		}
		return nil
	},

	// version 10 → 11: Issue #95 — add current_milestone_id to jobs for milestone-based payments.
	// Tracks which milestone is currently being paid for during AWAITING_PAYMENT state.
	func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE jobs ADD COLUMN current_milestone_id TEXT REFERENCES milestones(id)`); err != nil {
			return fmt.Errorf("migration 10→11: add current_milestone_id to jobs: %w", err)
		}
		return nil
	},
}

// rawMigrations holds migrations that need a raw *sql.DB (and therefore a raw
// *sql.Conn via complexMigration) instead of a *sql.Tx. RunMigrations checks
// this map first; if an entry exists it is used instead of migrations[i].
var rawMigrations = map[int]func(db *sql.DB) error{
	// version 6 → 7: Issue #65 — rebuild jobs table to add UNASSIGNED to the
	// CHECK constraint and change the default status from PENDING_ACCEPTANCE to
	// UNASSIGNED. Fresh databases already have UNASSIGNED in the schema from
	// migration 0 (once that migration is updated), so we check whether UNASSIGNED
	// is already in the constraint before doing the rebuild.
	6: func(db *sql.DB) error {
		ctx := context.Background()

		// Idempotency check: read the current CREATE TABLE statement. If it
		// already contains 'UNASSIGNED' the rebuild is not needed.
		var createSQL string
		err := db.QueryRowContext(ctx,
			`SELECT sql FROM sqlite_master WHERE type='table' AND name='jobs'`,
		).Scan(&createSQL)
		if err != nil {
			return fmt.Errorf("migration 6→7: read jobs schema: %w", err)
		}
		if strings.Contains(createSQL, "UNASSIGNED") {
			return nil // Already up to date — fresh database.
		}

		return complexMigration(db, func(conn *sql.Conn) error {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("migration 6→7: begin tx: %w", err)
			}

			stmts := []string{
				`CREATE TABLE jobs_new (
					id TEXT PRIMARY KEY,
					employer_id TEXT NOT NULL REFERENCES users(id),
					agent_id TEXT REFERENCES agents(id),
					status TEXT NOT NULL DEFAULT 'UNASSIGNED' CHECK(status IN (
						'UNASSIGNED','PENDING_ACCEPTANCE','IN_PROGRESS','COMPLETED','DISPUTED','CANCELLED',
						'SOW_NEGOTIATION','AWAITING_PAYMENT','DELIVERED'
					)),
					title TEXT NOT NULL,
					description TEXT DEFAULT '',
					total_payout INTEGER NOT NULL,
					timeline_days INTEGER NOT NULL,
					sow_link TEXT DEFAULT '',
					stripe_payment_intent TEXT,
					stripe_checkout_session_id TEXT,
					delivered_at DATETIME,
					delivery_notes TEXT,
					delivery_url TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)`,
				`INSERT INTO jobs_new SELECT * FROM jobs`,
				`DROP TABLE jobs`,
				`ALTER TABLE jobs_new RENAME TO jobs`,
			}
			for _, stmt := range stmts {
				if _, execErr := tx.Exec(stmt); execErr != nil {
					_ = tx.Rollback()
					return fmt.Errorf("migration 6→7 rebuild jobs: %w\nSQL: %s", execErr, stmt)
				}
			}

			if _, err := tx.Exec(`PRAGMA user_version = 7`); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration 6→7: set user_version: %w", err)
			}

			return tx.Commit()
		})
	},

	// version 2 → 3: make jobs.agent_id nullable so retracted offers can clear it to NULL.
	// Existing databases have agent_id NOT NULL; we rebuild via rename/copy/drop.
	// Fresh databases already have the nullable schema from migration 0.
	// We check table_info to skip the rebuild when agent_id is already nullable.
	//
	// This MUST run outside a transaction (via complexMigration) because SQLite
	// only respects PRAGMA foreign_keys = OFF when set on the connection, not
	// inside a transaction. With _pragma=foreign_keys(1) in the DSN, an in-
	// transaction PRAGMA is silently ignored, causing DROP TABLE jobs to fail
	// with FK constraint errors from milestones/sow/notifications referencing jobs.
	2: func(db *sql.DB) error {
		ctx := context.Background()

		// Idempotency check: if agent_id is already nullable, nothing to do.
		// Run this outside complexMigration to avoid acquiring an extra connection.
		agentIDNotNull := false
		rows, err := db.QueryContext(ctx, `PRAGMA table_info(jobs)`)
		if err != nil {
			return fmt.Errorf("migration 2→3: pragma table_info: %w", err)
		}
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull int
			var dfltValue interface{}
			var pk int
			if scanErr := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); scanErr != nil {
				rows.Close()
				return fmt.Errorf("migration 2→3: scan table_info: %w", scanErr)
			}
			if name == "agent_id" && notNull == 1 {
				agentIDNotNull = true
			}
		}
		rows.Close()
		if !agentIDNotNull {
			return nil // Already nullable — fresh database, nothing to do.
		}

		// Use complexMigration to pin a single connection and disable FK
		// enforcement at the connection level before touching the table.
		return complexMigration(db, func(conn *sql.Conn) error {
			// Begin a transaction on the pinned connection for atomicity.
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("migration 2→3: begin tx: %w", err)
			}

			stmts := []string{
				`CREATE TABLE jobs_new (
					id TEXT PRIMARY KEY,
					employer_id TEXT NOT NULL REFERENCES users(id),
					agent_id TEXT REFERENCES agents(id),
					status TEXT NOT NULL DEFAULT 'PENDING_ACCEPTANCE' CHECK(status IN (
						'PENDING_ACCEPTANCE','IN_PROGRESS','COMPLETED','DISPUTED','CANCELLED',
						'SOW_NEGOTIATION','AWAITING_PAYMENT','DELIVERED'
					)),
					title TEXT NOT NULL,
					description TEXT DEFAULT '',
					total_payout INTEGER NOT NULL,
					timeline_days INTEGER NOT NULL,
					sow_link TEXT DEFAULT '',
					stripe_payment_intent TEXT,
					stripe_checkout_session_id TEXT,
					delivered_at DATETIME,
					delivery_notes TEXT,
					delivery_url TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)`,
				`INSERT INTO jobs_new SELECT id, employer_id, NULLIF(agent_id,''), status, title, description,
				 total_payout, timeline_days, '' AS sow_link, stripe_payment_intent, stripe_checkout_session_id,
				 delivered_at, delivery_notes, delivery_url, created_at, updated_at FROM jobs`,
				`DROP TABLE jobs`,
				`ALTER TABLE jobs_new RENAME TO jobs`,
			}
			for _, stmt := range stmts {
				if _, execErr := tx.Exec(stmt); execErr != nil {
					_ = tx.Rollback()
					return fmt.Errorf("migration 2→3 rebuild jobs: %w\nSQL: %s", execErr, stmt)
				}
			}

			// Bump user_version inside the same transaction for atomicity.
			if _, err := tx.Exec(`PRAGMA user_version = 3`); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration 2→3: set user_version: %w", err)
			}

			return tx.Commit()
		})
	},

	// version 7 → 8: Issue #66 — rebuild milestones to swap job_id for sow_id.
	// Backfills sow_id from the job→sow relationship before dropping job_id.
	// Fresh databases created after migration 0 is updated already have the new
	// schema; we detect this by checking whether sow_id already exists on the table.
	7: func(db *sql.DB) error {
		ctx := context.Background()

		// Idempotency check: if sow_id already exists, nothing to do.
		rows, err := db.QueryContext(ctx, `PRAGMA table_info(milestones)`)
		if err != nil {
			return fmt.Errorf("migration 7→8: pragma table_info(milestones): %w", err)
		}
		hasSowID := false
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull int
			var dfltValue interface{}
			var pk int
			if scanErr := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); scanErr != nil {
				rows.Close()
				return fmt.Errorf("migration 7→8: scan table_info: %w", scanErr)
			}
			if name == "sow_id" {
				hasSowID = true
			}
		}
		rows.Close()
		if hasSowID {
			return nil // Already up to date — fresh database.
		}

		return complexMigration(db, func(conn *sql.Conn) error {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("migration 7→8: begin tx: %w", err)
			}

			stmts := []string{
				`CREATE TABLE milestones_new (
					id TEXT PRIMARY KEY,
					sow_id TEXT NOT NULL REFERENCES sow(id),
					title TEXT NOT NULL,
					amount INTEGER NOT NULL,
					order_index INTEGER NOT NULL,
					deliverables TEXT NOT NULL DEFAULT '',
					status TEXT NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','REVIEW_REQUESTED','APPROVED','PAID')),
					proof_of_work_url TEXT DEFAULT '',
					proof_of_work_notes TEXT DEFAULT '',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)`,
				// Backfill: join milestones → sow on sow.job_id = milestones.job_id.
				// Milestones with no matching sow row are dropped (they were orphaned).
				`INSERT INTO milestones_new
				 SELECT m.id, s.id, m.title, m.amount, m.order_index, m.deliverables,
				        m.status, m.proof_of_work_url, m.proof_of_work_notes,
				        m.created_at, m.updated_at
				 FROM milestones m
				 JOIN sow s ON s.job_id = m.job_id`,
				`DROP TABLE milestones`,
				`ALTER TABLE milestones_new RENAME TO milestones`,
			}
			for _, stmt := range stmts {
				if _, execErr := tx.Exec(stmt); execErr != nil {
					_ = tx.Rollback()
					return fmt.Errorf("migration 7→8 rebuild milestones: %w\nSQL: %s", execErr, stmt)
				}
			}

			if _, err := tx.Exec(`PRAGMA user_version = 8`); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration 7→8: set user_version: %w", err)
			}

			return tx.Commit()
		})
	},

	// version 8 → 9: Issue #88 — rename AGENT_HANDLER → AGENT_MANAGER.
	// Rebuilds the users table (to update the role CHECK constraint) and the
	// agents table (to rename handler_id → manager_id). Existing AGENT_HANDLER
	// role values are updated to AGENT_MANAGER during the migration.
	// Requires PRAGMA foreign_keys = OFF at the connection level.
	8: func(db *sql.DB) error {
		ctx := context.Background()

		// Idempotency check: if the agents table already has manager_id, nothing to do.
		rows, err := db.QueryContext(ctx, `PRAGMA table_info(agents)`)
		if err != nil {
			return fmt.Errorf("migration 8→9: pragma table_info(agents): %w", err)
		}
		hasManagerID := false
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull int
			var dfltValue interface{}
			var pk int
			if scanErr := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); scanErr != nil {
				rows.Close()
				return fmt.Errorf("migration 8→9: scan table_info: %w", scanErr)
			}
			if name == "manager_id" {
				hasManagerID = true
			}
		}
		rows.Close()
		if hasManagerID {
			return nil // Already up to date — fresh database.
		}

		return complexMigration(db, func(conn *sql.Conn) error {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("migration 8→9: begin tx: %w", err)
			}

			// Tell SQLite to hold off on enforcing FKs until tx.Commit()
			if _, err := tx.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration 8→9: defer foreign keys: %w", err)
			}

			stmts := []string{
				// Step 1: rebuild users table with updated CHECK constraint.
				`CREATE TABLE users_new (
					id TEXT PRIMARY KEY,
					role TEXT NOT NULL CHECK(role IN ('EMPLOYER','AGENT_MANAGER')),
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

				// Step 2: copy and transform (HANDLER to MANAGER) the data
				`INSERT INTO users_new (
					id, role, name, handle, email, password_hash, 
					email_verified_at, stripe_customer_id, stripe_account_id, 
					created_at, updated_at
				) 
				SELECT 
					id, 
					CASE WHEN role = 'AGENT_HANDLER' THEN 'AGENT_MANAGER' ELSE role END, 
					name, handle, email, password_hash, 
					email_verified_at, stripe_customer_id, stripe_account_id, 
					created_at, updated_at 
				FROM users`,

				`DROP TABLE users`,
				`ALTER TABLE users_new RENAME TO users`,


				// Step 3: rebuild agents table renaming handler_id → manager_id.
				`CREATE TABLE agents_new (
					id TEXT PRIMARY KEY,
					manager_id TEXT NOT NULL REFERENCES users(id),
					name TEXT NOT NULL,
					description TEXT DEFAULT '',
					api_key_hash TEXT NOT NULL,
					webhook_url TEXT DEFAULT '',
					is_active INTEGER DEFAULT 1,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)`,
				`INSERT INTO agents_new SELECT id, handler_id, name, description, api_key_hash, webhook_url, is_active, created_at, updated_at FROM agents`,
				`DROP TABLE agents`,
				`ALTER TABLE agents_new RENAME TO agents`,
			}
			for _, stmt := range stmts {
				if _, execErr := tx.Exec(stmt); execErr != nil {
					_ = tx.Rollback()
					return fmt.Errorf("migration 8→9: %w\nSQL: %s", execErr, stmt)
				}
			}

			if _, err := tx.Exec(`PRAGMA user_version = 9`); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration 8→9: set user_version: %w", err)
			}

			return tx.Commit()
		})
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

		// rawMigrations entries manage their own connection, transaction, and
		// user_version bump (required for migrations that need PRAGMA foreign_keys
		// = OFF at the connection level, which is ignored inside a sql.Tx when
		// _pragma=foreign_keys(1) is set in the DSN).
		if rawFn, ok := rawMigrations[i]; ok {
			if err := rawFn(db); err != nil {
				return fmt.Errorf("migration %d→%d failed: %w", i, i+1, err)
			}
			slog.Info("migrations: applied", "version", i+1)
			continue
		}

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

