package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// dbCounter gives each test a unique in-memory SQLite database name so
// parallel tests never share state.
var dbCounter atomic.Int64

// setupTestApp creates an in-memory SQLite DB, runs migrations, and returns a
// configured App. Safe to call from parallel tests — no t.Setenv.
func setupTestApp(t *testing.T) *App {
	t.Helper()

	// Each test gets a unique named in-memory DB (cache=shared within that name)
	// so the connection pool sees a single consistent in-memory database,
	// while parallel tests each have their own isolated instance.
	n := dbCounter.Add(1)
	dsn := fmt.Sprintf("file:testmem%d?mode=memory&cache=shared", n)

	cfg := &Config{
		Port:         "8080",
		DSName:       dsn,
		JWTSecret:    []byte("test-secret-key-for-testing"),
		ResendAPIKey: "test-resend-key",
	}

	db, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("failed to init test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := RunMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return NewApp(cfg, db)
}

// newTestServer wraps the app in a real httptest.Server.
func newTestServer(t *testing.T, app *App) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewRouter(app))
	t.Cleanup(srv.Close)
	return srv
}

// makeAuthToken generates a valid JWT for the given userID and role using the app's secret.
func makeAuthToken(t *testing.T, app *App, userID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(app.Config.JWTSecret)
	if err != nil {
		t.Fatalf("failed to sign test JWT: %v", err)
	}
	return signed
}

// createTestUser inserts a user directly into the DB and returns (id, plainPassword).
func createTestUser(t *testing.T, app *App, role string) (id, plainPassword string) {
	t.Helper()
	id = uuid.New().String()
	plainPassword = "TestPass123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt failed: %v", err)
	}
	handle := "user-" + id[:8]
	email := handle + "@test.example"
	_, err = app.DB.Exec(
		`INSERT INTO users (id, role, name, handle, email, password_hash) VALUES (?, ?, ?, ?, ?, ?)`,
		id, role, "Test User", handle, email, string(hash),
	)
	if err != nil {
		t.Fatalf("createTestUser insert failed: %v", err)
	}
	return id, plainPassword
}

// createVerifiedTestUser is like createTestUser but also sets email_verified_at.
func createVerifiedTestUser(t *testing.T, app *App, role string) (id, plainPassword string) {
	t.Helper()
	id, plainPassword = createTestUser(t, app, role)
	_, err := app.DB.Exec(
		`UPDATE users SET email_verified_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	)
	if err != nil {
		t.Fatalf("createVerifiedTestUser update failed: %v", err)
	}
	return id, plainPassword
}

// createTestAgent inserts an agent directly into the DB and returns (agentID, plainAPIKey).
func createTestAgent(t *testing.T, app *App, handlerID string) (agentID, plainAPIKey string) {
	t.Helper()
	plainKey, keyHash, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey failed: %v", err)
	}
	agentID = uuid.New().String()
	_, err = app.DB.Exec(
		`INSERT INTO agents (id, handler_id, name, description, api_key_hash, webhook_url) VALUES (?, ?, ?, ?, ?, ?)`,
		agentID, handlerID, "Test Agent", "A test agent", keyHash, "",
	)
	if err != nil {
		t.Fatalf("createTestAgent insert failed: %v", err)
	}
	return agentID, plainKey
}

// hashAPIKey returns the sha256 hex of a plaintext API key (mirrors middleware logic).
func hashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// doRequest is a convenience helper: builds, optionally authenticates, and sends a request
// to the handler, returning the recorder.
func doRequest(t *testing.T, router http.Handler, method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
	}
	req, err := http.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}
