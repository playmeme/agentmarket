package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Agent struct {
	ID          string    `json:"id"`
	HandlerID   string    `json:"handler_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	WebhookURL  string    `json:"webhook_url"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateAgentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	WebhookURL  string `json:"webhook_url"`
}

type CreateAgentResponse struct {
	Agent  Agent  `json:"agent"`
	APIKey string `json:"api_key"`
}

func generateAPIKey() (plaintext, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	plaintext = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(sum[:])
	return
}

func scanAgent(row interface {
	Scan(...interface{}) error
}) (Agent, error) {
	var a Agent
	var isActive int
	err := row.Scan(&a.ID, &a.HandlerID, &a.Name, &a.Description, &a.WebhookURL, &isActive, &a.CreatedAt, &a.UpdatedAt)
	a.IsActive = isActive == 1
	return a, err
}

func (app *App) ListAgentsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := app.DB.Query(
		`SELECT id, handler_id, name, description, webhook_url, is_active, created_at, updated_at
		 FROM agents WHERE is_active = 1 ORDER BY created_at DESC`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	agents := []Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		agents = append(agents, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (app *App) GetAgentHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	row := app.DB.QueryRow(
		`SELECT id, handler_id, name, description, webhook_url, is_active, created_at, updated_at
		 FROM agents WHERE id = ?`,
		id,
	)

	a, err := scanAgent(row)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

func (app *App) ListHandlerAgentsHandler(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "AGENT_HANDLER" {
		slog.Warn("authz failure: list handler agents requires AGENT_HANDLER role",
			"request_id", requestID(r.Context()),
			"role", role,
		)
		writeError(w, http.StatusForbidden, "only AGENT_HANDLER role can list handler agents")
		return
	}

	handlerID, _ := r.Context().Value(contextKeyUserID).(string)

	rows, err := app.DB.Query(
		`SELECT id, handler_id, name, description, webhook_url, is_active, created_at, updated_at
		 FROM agents WHERE handler_id = ? ORDER BY created_at DESC`,
		handlerID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	agents := []Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		agents = append(agents, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (app *App) CreateAgentHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "create_agent")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "AGENT_HANDLER" {
		log.Warn("authz failure: create agent requires AGENT_HANDLER role", "role", role)
		writeError(w, http.StatusForbidden, "only AGENT_HANDLER role can create agents")
		return
	}

	handlerID, _ := r.Context().Value(contextKeyUserID).(string)

	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	plainKey, keyHash, err := generateAPIKey()
	if err != nil {
		log.Error("agent creation failed: api key generation error", "handler_id", handlerID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate API key")
		return
	}

	id := uuid.New().String()
	_, err = app.DB.Exec(
		`INSERT INTO agents (id, handler_id, name, description, api_key_hash, webhook_url)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, handlerID, req.Name, req.Description, keyHash, req.WebhookURL,
	)
	if err != nil {
		log.Error("agent creation failed: database error", "handler_id", handlerID, "name", req.Name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	log.Info("agent created, api key issued", "agent_id", id, "handler_id", handlerID, "name", req.Name)

	row := app.DB.QueryRow(
		`SELECT id, handler_id, name, description, webhook_url, is_active, created_at, updated_at
		 FROM agents WHERE id = ?`,
		id,
	)
	a, err := scanAgent(row)
	if err != nil {
		log.Error("agent creation: failed to retrieve after insert", "agent_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve agent")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateAgentResponse{Agent: a, APIKey: plainKey})
}

type UpdateAgentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	WebhookURL  string `json:"webhook_url"`
}

func (app *App) UpdateAgentHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "update_agent")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "AGENT_HANDLER" {
		log.Warn("authz failure: update agent requires AGENT_HANDLER role", "role", role)
		writeError(w, http.StatusForbidden, "only AGENT_HANDLER role can update agents")
		return
	}

	handlerID, _ := r.Context().Value(contextKeyUserID).(string)
	agentID := chi.URLParam(r, "id")

	var req UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := app.DB.Exec(
		`UPDATE agents SET name = COALESCE(NULLIF(?, ''), name),
		 description = ?, webhook_url = COALESCE(NULLIF(?, ''), webhook_url),
		 updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND handler_id = ?`,
		req.Name, req.Description, req.WebhookURL, agentID, handlerID,
	)
	if err != nil {
		log.Error("agent update failed: database error", "agent_id", agentID, "handler_id", handlerID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update agent")
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		writeError(w, http.StatusNotFound, "agent not found or not owned by this handler")
		return
	}

	row := app.DB.QueryRow(
		`SELECT id, handler_id, name, description, webhook_url, is_active, created_at, updated_at
		 FROM agents WHERE id = ?`,
		agentID,
	)
	a, err := scanAgent(row)
	if err != nil {
		log.Error("agent update: failed to retrieve after update", "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve agent")
		return
	}

	log.Info("agent updated", "agent_id", agentID, "handler_id", handlerID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}
