package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Models ---

type SOW struct {
	ID               string `json:"id"`
	JobID            string `json:"job_id"`
	DetailedSpec     string `json:"detailed_spec"` // detailed specification of work
	WorkProcess      string `json:"work_process"`  // communication/process description
	PriceCents       int    `json:"price_cents"`
	TimelineDays     int    `json:"timeline_days"`
	AgentAccepted    bool   `json:"agent_accepted"`
	EmployerAccepted bool   `json:"employer_accepted"`
	LastEditedBy     string `json:"last_edited_by,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// --- Request types ---

type SOWMilestoneInput struct {
	Title        string   `json:"title"`
	Amount       int64    `json:"amount"`
	Deliverables string   `json:"deliverables"`
	Criteria     []string `json:"criteria"`
}

type SOWRequest struct {
	DetailedSpec string              `json:"detailed_spec"`
	WorkProcess  string              `json:"work_process"`
	PriceCents   int                 `json:"price_cents"`
	TimelineDays int                 `json:"timeline_days"`
	Milestones   []SOWMilestoneInput `json:"milestones"`
}

// --- Helpers ---

func (app *App) getSOWByJobID(jobID string) (SOW, error) {
	var s SOW
	var agentAccepted, employerAccepted int
	var lastEditedBy sql.NullString
	err := app.DB.QueryRow(
		`SELECT id, job_id, detailed_spec, work_process, price_cents, timeline_days, agent_accepted, employer_accepted, last_edited_by, created_at, updated_at
		 FROM sow WHERE job_id = ?`,
		jobID,
	).Scan(&s.ID, &s.JobID, &s.DetailedSpec, &s.WorkProcess, &s.PriceCents, &s.TimelineDays,
		&agentAccepted, &employerAccepted, &lastEditedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, err
	}
	s.AgentAccepted = agentAccepted == 1
	s.EmployerAccepted = employerAccepted == 1
	if lastEditedBy.Valid {
		s.LastEditedBy = lastEditedBy.String
	}
	return s, nil
}

// isJobParticipant returns true if userID is either the employer or the handler
// of the agent assigned to this job.
func (app *App) isJobParticipant(jobID, userID string) (bool, error) {
	var count int
	err := app.DB.QueryRow(
		`SELECT COUNT(*) FROM jobs j
		 LEFT JOIN agents a ON j.agent_id = a.id
		 WHERE j.id = ? AND (j.employer_id = ? OR a.handler_id = ?)`,
		jobID, userID, userID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// isJobEmployer returns true if userID is the employer who owns the job.
func (app *App) isJobEmployer(jobID, userID string) (bool, error) {
	var count int
	err := app.DB.QueryRow(
		`SELECT COUNT(*) FROM jobs WHERE id = ? AND employer_id = ?`,
		jobID, userID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// --- Handlers ---

func (app *App) CreateOrUpdateSOW(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "create_or_update_sow")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")

	// Verify job exists and is in an editable status
	var jobStatus string
	err := app.DB.QueryRow("SELECT status FROM jobs WHERE id = ?", jobID).Scan(&jobStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("sow upsert: db error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// SOW_NEGOTIATION: both employer and agent can edit.
	// UNASSIGNED / PENDING_ACCEPTANCE: employer-only pre-fill before agent is assigned.
	employerOnlyStatuses := jobStatus == "UNASSIGNED" || jobStatus == "PENDING_ACCEPTANCE"
	if jobStatus != "SOW_NEGOTIATION" && !employerOnlyStatuses {
		writeError(w, http.StatusBadRequest, "job must be in SOW_NEGOTIATION, UNASSIGNED, or PENDING_ACCEPTANCE status to edit SOW")
		return
	}

	// Verify caller is authorized
	if employerOnlyStatuses {
		// Only employer may pre-fill SoW before an agent is assigned
		ok, err := app.isJobEmployer(jobID, userID)
		if err != nil {
			log.Error("sow upsert: db error checking employer", "job_id", jobID, "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "not authorized to edit this SOW")
			return
		}
	} else {
		ok, err := app.isJobParticipant(jobID, userID)
		if err != nil {
			log.Error("sow upsert: db error checking participant", "job_id", jobID, "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "not authorized to edit this SOW")
			return
		}
	}

	var req SOWRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if SOW exists
	var existingID string
	err = app.DB.QueryRow("SELECT id FROM sow WHERE job_id = ?", jobID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Create new SOW
		sowID := uuid.New().String()
		_, err = app.DB.Exec(
			`INSERT INTO sow (id, job_id, detailed_spec, work_process, price_cents, timeline_days, agent_accepted, employer_accepted, last_edited_by)
			 VALUES (?, ?, ?, ?, ?, ?, 0, 0, ?)`,
			sowID, jobID, req.DetailedSpec, req.WorkProcess, req.PriceCents, req.TimelineDays, userID,
		)
		if err != nil {
			log.Error("sow create: insert error", "job_id", jobID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to create SOW")
			return
		}
		log.Info("SOW created", "job_id", jobID, "sow_id", sowID, "user_id", userID)
	} else if err != nil {
		log.Error("sow upsert: db error checking existing sow", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	} else {
		// Update existing SOW — reset both acceptance flags
		_, err = app.DB.Exec(
			`UPDATE sow SET detailed_spec = ?, work_process = ?, price_cents = ?, timeline_days = ?,
			 agent_accepted = 0, employer_accepted = 0, last_edited_by = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE job_id = ?`,
			req.DetailedSpec, req.WorkProcess, req.PriceCents, req.TimelineDays, userID, jobID,
		)
		if err != nil {
			log.Error("sow update: exec error", "job_id", jobID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update SOW")
			return
		}
		log.Info("SOW updated", "job_id", jobID, "sow_id", existingID, "user_id", userID)
	}

	// Update milestones if provided in the request.
	// Milestones now reference sow_id directly; look up the SOW id for this job.
	if req.Milestones != nil {
		var sowID string
		if err = app.DB.QueryRow("SELECT id FROM sow WHERE job_id = ?", jobID).Scan(&sowID); err != nil {
			log.Error("sow upsert: failed to resolve sow_id for milestones", "job_id", jobID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to resolve SOW")
			return
		}
		// Delete existing milestones and criteria for this SOW, then re-insert
		if _, err = app.DB.Exec(`DELETE FROM criteria WHERE milestone_id IN (SELECT id FROM milestones WHERE sow_id = ?)`, sowID); err != nil {
			log.Error("sow upsert: delete criteria error", "sow_id", sowID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update milestones")
			return
		}
		if _, err = app.DB.Exec(`DELETE FROM milestones WHERE sow_id = ?`, sowID); err != nil {
			log.Error("sow upsert: delete milestones error", "sow_id", sowID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update milestones")
			return
		}
		for i, ms := range req.Milestones {
			msID := uuid.New().String()
			if _, err = app.DB.Exec(
				`INSERT INTO milestones (id, sow_id, title, amount, order_index, deliverables) VALUES (?, ?, ?, ?, ?, ?)`,
				msID, sowID, ms.Title, ms.Amount, i, ms.Deliverables,
			); err != nil {
				log.Error("sow upsert: milestone insert error", "sow_id", sowID, "milestone_index", i, "error", err)
				writeError(w, http.StatusInternalServerError, "failed to create milestone")
				return
			}
			for _, criteriaDesc := range ms.Criteria {
				cID := uuid.New().String()
				if _, err = app.DB.Exec(
					`INSERT INTO criteria (id, milestone_id, description) VALUES (?, ?, ?)`,
					cID, msID, criteriaDesc,
				); err != nil {
					log.Error("sow upsert: criteria insert error", "milestone_id", msID, "error", err)
					writeError(w, http.StatusInternalServerError, "failed to create criteria")
					return
				}
			}
		}
		log.Info("SOW milestones updated", "sow_id", sowID, "count", len(req.Milestones))
	}

	sow, err := app.getSOWByJobID(jobID)
	if err != nil {
		log.Error("sow upsert: failed to retrieve after save", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve SOW")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(sow)
}

func (app *App) GetSOW(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "get_sow")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")

	// Verify caller is a participant (employer or agent handler).
	// isJobParticipant uses LEFT JOIN so employer can access even without an agent.
	ok, err := app.isJobParticipant(jobID, userID)
	if err != nil {
		log.Error("get sow: db error checking participant", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not authorized to view this SOW")
		return
	}

	sow, err := app.getSOWByJobID(jobID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "SOW not found for this job")
		return
	}
	if err != nil {
		log.Error("get sow: db error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sow)
}

func (app *App) AcceptSOW(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "accept_sow")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	role, _ := r.Context().Value(contextKeyUserRole).(string)
	jobID := chi.URLParam(r, "job_id")

	// Verify job exists and is in SOW_NEGOTIATION
	var jobStatus string
	err := app.DB.QueryRow("SELECT status FROM jobs WHERE id = ?", jobID).Scan(&jobStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("accept sow: db error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobStatus != "SOW_NEGOTIATION" {
		writeError(w, http.StatusBadRequest, "job must be in SOW_NEGOTIATION status to accept SOW")
		return
	}

	// Verify caller is a participant
	ok, err := app.isJobParticipant(jobID, userID)
	if err != nil {
		log.Error("accept sow: db error checking participant", "job_id", jobID, "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not authorized to accept this SOW")
		return
	}

	// Set acceptance flag based on role
	var updateQuery string
	if role == "EMPLOYER" {
		updateQuery = `UPDATE sow SET employer_accepted = 1, updated_at = CURRENT_TIMESTAMP WHERE job_id = ?`
	} else {
		// AGENT_HANDLER
		updateQuery = `UPDATE sow SET agent_accepted = 1, updated_at = CURRENT_TIMESTAMP WHERE job_id = ?`
	}

	result, err := app.DB.Exec(updateQuery, jobID)
	if err != nil {
		log.Error("accept sow: update error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "SOW not found for this job")
		return
	}

	log.Info("SOW accepted", "job_id", jobID, "user_id", userID, "role", role)

	// Check if both parties have now accepted
	var agentAccepted, employerAccepted int
	err = app.DB.QueryRow(
		"SELECT agent_accepted, employer_accepted FROM sow WHERE job_id = ?", jobID,
	).Scan(&agentAccepted, &employerAccepted)
	if err != nil {
		log.Error("accept sow: db error checking acceptance", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if agentAccepted == 1 && employerAccepted == 1 {
		// Both accepted — move job to AWAITING_PAYMENT
		_, err = app.DB.Exec(
			`UPDATE jobs SET status = 'AWAITING_PAYMENT', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			jobID,
		)
		if err != nil {
			log.Error("accept sow: failed to advance job status", "job_id", jobID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		log.Info("both parties accepted SOW, job moved to AWAITING_PAYMENT", "job_id", jobID)
	}

	sow, err := app.getSOWByJobID(jobID)
	if err != nil {
		log.Error("accept sow: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve SOW")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sow)
}
