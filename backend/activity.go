package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// ActivityEvent is a single item in the public homepage activity feed.
// The Kind field controls which fields are populated:
//
//   - "job_offered"   — a public job brief was posted (JobID + JobTitle only; no Agent)
//   - "agent_hired"   — an agent started work on a job (AgentID + AgentName only; no Job)
//   - "job_completed" — a job was completed (JobID + JobTitle + AgentID + AgentName)
type ActivityEvent struct {
	Kind      string    `json:"kind"`
	JobID     string    `json:"job_id,omitempty"`
	JobTitle  string    `json:"job_title,omitempty"`
	AgentID   string    `json:"agent_id,omitempty"`
	AgentName string    `json:"agent_name,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// GetPublicActivityHandler returns the most recent public activity events for
// the homepage feed. No authentication is required. Results are ordered newest
// first and capped at 20 items.
//
// Privacy rules (mirrors issue #124):
//   - job_offered:   only public jobs; exposes title but NOT the full SoW or agent
//   - agent_hired:   exposes agent name but NOT the job title
//   - job_completed: exposes both agent name and job title
func (app *App) GetPublicActivityHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "public_activity")

	// Collect up to 20 events across three categories, then merge-sort by time.
	// Doing three small queries is simpler and fast enough for this feed.

	var events []ActivityEvent

	// 1. Job offered — public jobs in UNASSIGNED status (brief posted, no agent yet).
	offeredRows, err := app.DB.Query(`
		SELECT id, title, created_at
		FROM jobs
		WHERE is_public = 1 AND status = 'UNASSIGNED'
		ORDER BY created_at DESC
		LIMIT 20
	`)
	if err != nil {
		log.Error("public activity: query job_offered", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer offeredRows.Close()
	for offeredRows.Next() {
		var e ActivityEvent
		e.Kind = "job_offered"
		if err := offeredRows.Scan(&e.JobID, &e.JobTitle, sqliteTime{&e.OccurredAt}); err != nil {
			log.Error("public activity: scan job_offered", "error", err)
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		events = append(events, e)
	}
	if err := offeredRows.Err(); err != nil {
		log.Error("public activity: rows error job_offered", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// 2. Agent hired — jobs that are actively in progress (agent assigned but not yet done).
	//    We expose the agent name but deliberately omit the job title.
	//    COMPLETED and DELIVERED jobs are handled separately by the job_completed category.
	hiredRows, err := app.DB.Query(`
		SELECT j.updated_at, a.id, a.name
		FROM jobs j
		JOIN agents a ON a.id = j.agent_id
		WHERE j.status IN ('IN_PROGRESS','SOW_NEGOTIATION','AWAITING_PAYMENT','PENDING_ACCEPTANCE')
		ORDER BY j.updated_at DESC
		LIMIT 20
	`)
	if err != nil {
		log.Error("public activity: query agent_hired", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer hiredRows.Close()
	for hiredRows.Next() {
		var e ActivityEvent
		e.Kind = "agent_hired"
		if err := hiredRows.Scan(sqliteTime{&e.OccurredAt}, &e.AgentID, &e.AgentName); err != nil {
			log.Error("public activity: scan agent_hired", "error", err)
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		events = append(events, e)
	}
	if err := hiredRows.Err(); err != nil {
		log.Error("public activity: rows error agent_hired", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// 3. Job completed — COMPLETED or DELIVERED jobs where is_public = 1.
	//    Both agent and job are shown.
	completedRows, err := app.DB.Query(`
		SELECT j.id, j.title, j.updated_at, a.id, a.name
		FROM jobs j
		JOIN agents a ON a.id = j.agent_id
		WHERE j.status IN ('COMPLETED','DELIVERED') AND j.is_public = 1
		ORDER BY j.updated_at DESC
		LIMIT 20
	`)
	if err != nil {
		log.Error("public activity: query job_completed", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer completedRows.Close()
	for completedRows.Next() {
		var e ActivityEvent
		e.Kind = "job_completed"
		if err := completedRows.Scan(&e.JobID, &e.JobTitle, sqliteTime{&e.OccurredAt}, &e.AgentID, &e.AgentName); err != nil {
			log.Error("public activity: scan job_completed", "error", err)
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		events = append(events, e)
	}
	if err := completedRows.Err(); err != nil {
		log.Error("public activity: rows error job_completed", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Sort all events newest-first, then cap at 20.
	sortActivityEvents(events)
	if len(events) > 20 {
		events = events[:20]
	}

	// Always return an array (never null).
	if events == nil {
		events = []ActivityEvent{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// sortActivityEvents sorts events in-place, newest OccurredAt first.
func sortActivityEvents(events []ActivityEvent) {
	// Simple insertion sort — the slice is tiny (≤60 items before trimming).
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].OccurredAt.After(events[j-1].OccurredAt); j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
}
