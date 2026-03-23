package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Models ---

type Criterion struct {
	ID          string    `json:"id"`
	MilestoneID string    `json:"milestone_id"`
	Description string    `json:"description"`
	IsVerified  bool      `json:"is_verified"`
	CreatedAt   time.Time `json:"created_at"`
}

type Milestone struct {
	ID               string      `json:"id"`
	JobID            string      `json:"job_id"`
	Title            string      `json:"title"`
	Amount           int64       `json:"amount"`
	OrderIndex       int         `json:"order_index"`
	Status           string      `json:"status"`
	ProofOfWorkURL   string      `json:"proof_of_work_url"`
	ProofOfWorkNotes string      `json:"proof_of_work_notes"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	Criteria         []Criterion `json:"criteria,omitempty"`
}

type Job struct {
	ID                  string      `json:"id"`
	EmployerID          string      `json:"employer_id"`
	AgentID             string      `json:"agent_id"`
	Status              string      `json:"status"`
	Title               string      `json:"title"`
	Description         string      `json:"description"`
	TotalPayout         int64       `json:"total_payout"`
	TimelineDays        int         `json:"timeline_days"`
	StripePaymentIntent string      `json:"stripe_payment_intent,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
	Milestones          []Milestone `json:"milestones,omitempty"`
}

// --- Request types ---

type MilestoneInput struct {
	Title    string   `json:"title"`
	Amount   int64    `json:"amount"`
	Criteria []string `json:"criteria"`
}

type HireRequest struct {
	AgentID      string           `json:"agent_id"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	TotalPayout  int64            `json:"total_payout"`
	TimelineDays int              `json:"timeline_days"`
	Milestones   []MilestoneInput `json:"milestones"`
}

type SubmitProofRequest struct {
	ProofOfWorkURL   string `json:"proof_of_work_url"`
	ProofOfWorkNotes string `json:"proof_of_work_notes"`
}

// --- Helpers ---

func (app *App) loadCriteriaForMilestone(milestoneID string) ([]Criterion, error) {
	rows, err := app.DB.Query(
		`SELECT id, milestone_id, description, is_verified, created_at FROM criteria WHERE milestone_id = ? ORDER BY rowid`,
		milestoneID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var criteria []Criterion
	for rows.Next() {
		var c Criterion
		var isVerified int
		if err := rows.Scan(&c.ID, &c.MilestoneID, &c.Description, &isVerified, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.IsVerified = isVerified == 1
		criteria = append(criteria, c)
	}
	if criteria == nil {
		criteria = []Criterion{}
	}
	return criteria, nil
}

func (app *App) loadMilestonesForJob(jobID string) ([]Milestone, error) {
	rows, err := app.DB.Query(
		`SELECT id, job_id, title, amount, order_index, status, proof_of_work_url, proof_of_work_notes, created_at, updated_at
		 FROM milestones WHERE job_id = ? ORDER BY order_index`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var milestones []Milestone
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.JobID, &m.Title, &m.Amount, &m.OrderIndex, &m.Status,
			&m.ProofOfWorkURL, &m.ProofOfWorkNotes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		criteria, err := app.loadCriteriaForMilestone(m.ID)
		if err != nil {
			return nil, err
		}
		m.Criteria = criteria
		milestones = append(milestones, m)
	}
	if milestones == nil {
		milestones = []Milestone{}
	}
	return milestones, nil
}

func (app *App) scanJob(row interface{ Scan(...interface{}) error }) (Job, error) {
	var j Job
	var stripe sql.NullString
	err := row.Scan(&j.ID, &j.EmployerID, &j.AgentID, &j.Status, &j.Title, &j.Description,
		&j.TotalPayout, &j.TimelineDays, &stripe, &j.CreatedAt, &j.UpdatedAt)
	if stripe.Valid {
		j.StripePaymentIntent = stripe.String
	}
	return j, err
}

// --- UI Handlers ---

func (app *App) HireAgentHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "hire_agent")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: hire agent requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can hire agents")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)

	var req HireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title == "" || req.TotalPayout == 0 || req.TimelineDays == 0 {
		writeError(w, http.StatusBadRequest, "title, total_payout, and timeline_days are required")
		return
	}

	// Email verification is only required when assigning an agent to the job.
	// Employers may create and save job listings without verifying their email first.
	if req.AgentID != "" {
		var emailVerifiedAt sql.NullTime
		err := app.DB.QueryRow("SELECT email_verified_at FROM users WHERE id = ?", employerID).Scan(&emailVerifiedAt)
		if err != nil {
			log.Error("hire agent: database error checking email verification", "employer_id", employerID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if !emailVerifiedAt.Valid {
			log.Warn("hire agent blocked: employer email not verified", "employer_id", employerID)
			writeError(w, http.StatusForbidden, "Please verify your email before assigning an agent")
			return
		}

		// Verify agent exists and is active
		var agentExists int
		err = app.DB.QueryRow("SELECT COUNT(*) FROM agents WHERE id = ? AND is_active = 1", req.AgentID).Scan(&agentExists)
		if err != nil || agentExists == 0 {
			writeError(w, http.StatusNotFound, "agent not found or inactive")
			return
		}
	}

	tx, err := app.DB.Begin()
	if err != nil {
		log.Error("job creation failed: begin transaction error", "employer_id", employerID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback()

	jobID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO jobs (id, employer_id, agent_id, title, description, total_payout, timeline_days)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		jobID, employerID, req.AgentID, req.Title, req.Description, req.TotalPayout, req.TimelineDays,
	)
	if err != nil {
		log.Error("job creation failed: insert error", "employer_id", employerID, "agent_id", req.AgentID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	for i, ms := range req.Milestones {
		msID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO milestones (id, job_id, title, amount, order_index) VALUES (?, ?, ?, ?, ?)`,
			msID, jobID, ms.Title, ms.Amount, i,
		)
		if err != nil {
			log.Error("job creation failed: milestone insert error", "job_id", jobID, "milestone_index", i, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to create milestone")
			return
		}

		for _, criteriaDesc := range ms.Criteria {
			cID := uuid.New().String()
			_, err = tx.Exec(
				`INSERT INTO criteria (id, milestone_id, description) VALUES (?, ?, ?)`,
				cID, msID, criteriaDesc,
			)
			if err != nil {
				log.Error("job creation failed: criteria insert error", "milestone_id", msID, "error", err)
				writeError(w, http.StatusInternalServerError, "failed to create criteria")
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error("job creation failed: commit error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	log.Info("job created",
		"job_id", jobID,
		"employer_id", employerID,
		"agent_id", req.AgentID,
		"title", req.Title,
		"total_payout", req.TotalPayout,
		"milestones", len(req.Milestones),
	)

	job, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("job creation: failed to retrieve after insert", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (app *App) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(contextKeyUserID).(string)
	role, _ := r.Context().Value(contextKeyUserRole).(string)

	var rows *sql.Rows
	var err error

	if role == "EMPLOYER" {
		rows, err = app.DB.Query(
			`SELECT id, employer_id, agent_id, status, title, description, total_payout, timeline_days, stripe_payment_intent, created_at, updated_at
			 FROM jobs WHERE employer_id = ? ORDER BY created_at DESC`,
			userID,
		)
	} else {
		// AGENT_HANDLER: list jobs for any of their agents
		rows, err = app.DB.Query(
			`SELECT j.id, j.employer_id, j.agent_id, j.status, j.title, j.description, j.total_payout, j.timeline_days, j.stripe_payment_intent, j.created_at, j.updated_at
			 FROM jobs j
			 JOIN agents a ON j.agent_id = a.id
			 WHERE a.handler_id = ?
			 ORDER BY j.created_at DESC`,
			userID,
		)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		j, err := app.scanJob(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		jobs = append(jobs, j)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (app *App) getJobDetail(jobID string) (Job, error) {
	row := app.DB.QueryRow(
		`SELECT id, employer_id, agent_id, status, title, description, total_payout, timeline_days, stripe_payment_intent, created_at, updated_at
		 FROM jobs WHERE id = ?`,
		jobID,
	)
	j, err := app.scanJob(row)
	if err != nil {
		return j, err
	}

	milestones, err := app.loadMilestonesForJob(jobID)
	if err != nil {
		return j, err
	}
	j.Milestones = milestones
	return j, nil
}

func (app *App) GetJobHandler(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	j, err := app.getJobDetail(jobID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

func (app *App) ApproveMilestoneHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "approve_milestone")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: approve milestone requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER can approve milestones")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")
	milestoneID := chi.URLParam(r, "milestone_id")

	// Verify the job belongs to this employer
	var count int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM jobs WHERE id = ? AND employer_id = ?", jobID, employerID).Scan(&count)
	if err != nil || count == 0 {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	result, err := app.DB.Exec(
		`UPDATE milestones SET status = 'APPROVED', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND job_id = ? AND status = 'REVIEW_REQUESTED'`,
		milestoneID, jobID,
	)
	if err != nil {
		log.Error("milestone approval failed: database error", "job_id", jobID, "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusBadRequest, "milestone not found or not in REVIEW_REQUESTED status")
		return
	}

	log.Info("milestone approved", "job_id", jobID, "milestone_id", milestoneID, "employer_id", employerID)

	var m Milestone
	row := app.DB.QueryRow(
		`SELECT id, job_id, title, amount, order_index, status, proof_of_work_url, proof_of_work_notes, created_at, updated_at
		 FROM milestones WHERE id = ?`,
		milestoneID,
	)
	if err := row.Scan(&m.ID, &m.JobID, &m.Title, &m.Amount, &m.OrderIndex, &m.Status,
		&m.ProofOfWorkURL, &m.ProofOfWorkNotes, &m.CreatedAt, &m.UpdatedAt); err != nil {
		log.Error("milestone approval: failed to retrieve after update", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve milestone")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

// --- Agent API (API key auth) ---

func (app *App) GetPendingJobsHandler(w http.ResponseWriter, r *http.Request) {
	agentID, _ := r.Context().Value(contextKeyAgentID).(string)

	rows, err := app.DB.Query(
		`SELECT id, employer_id, agent_id, status, title, description, total_payout, timeline_days, stripe_payment_intent, created_at, updated_at
		 FROM jobs WHERE agent_id = ? AND status = 'PENDING_ACCEPTANCE' ORDER BY created_at DESC`,
		agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		j, err := app.scanJob(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		jobs = append(jobs, j)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func (app *App) AcceptJobHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "accept_job")

	agentID, _ := r.Context().Value(contextKeyAgentID).(string)
	jobID := chi.URLParam(r, "job_id")

	result, err := app.DB.Exec(
		`UPDATE jobs SET status = 'IN_PROGRESS', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND agent_id = ? AND status = 'PENDING_ACCEPTANCE'`,
		jobID, agentID,
	)
	if err != nil {
		log.Error("job accept failed: database error", "job_id", jobID, "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "job not found or not in PENDING_ACCEPTANCE status")
		return
	}

	log.Info("job accepted", "job_id", jobID, "agent_id", agentID)

	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("job accept: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

func (app *App) DeclineJobHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "decline_job")

	agentID, _ := r.Context().Value(contextKeyAgentID).(string)
	jobID := chi.URLParam(r, "job_id")

	result, err := app.DB.Exec(
		`UPDATE jobs SET status = 'CANCELLED', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND agent_id = ? AND status = 'PENDING_ACCEPTANCE'`,
		jobID, agentID,
	)
	if err != nil {
		log.Error("job decline failed: database error", "job_id", jobID, "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "job not found or not in PENDING_ACCEPTANCE status")
		return
	}

	log.Info("job declined", "job_id", jobID, "agent_id", agentID)

	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("job decline: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

func (app *App) SubmitMilestoneHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "submit_milestone")

	agentID, _ := r.Context().Value(contextKeyAgentID).(string)
	jobID := chi.URLParam(r, "job_id")
	milestoneID := chi.URLParam(r, "milestone_id")

	// Verify job belongs to this agent and is IN_PROGRESS
	var count int
	err := app.DB.QueryRow(
		"SELECT COUNT(*) FROM jobs WHERE id = ? AND agent_id = ? AND status = 'IN_PROGRESS'",
		jobID, agentID,
	).Scan(&count)
	if err != nil || count == 0 {
		writeError(w, http.StatusNotFound, "job not found or not in progress")
		return
	}

	var req SubmitProofRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := app.DB.Exec(
		`UPDATE milestones SET status = 'REVIEW_REQUESTED', proof_of_work_url = ?, proof_of_work_notes = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND job_id = ? AND status = 'PENDING'`,
		req.ProofOfWorkURL, req.ProofOfWorkNotes, milestoneID, jobID,
	)
	if err != nil {
		log.Error("milestone submit failed: database error", "job_id", jobID, "milestone_id", milestoneID, "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusBadRequest, "milestone not found or not in PENDING status")
		return
	}

	log.Info("milestone submitted for review",
		"job_id", jobID,
		"milestone_id", milestoneID,
		"agent_id", agentID,
		"proof_url", req.ProofOfWorkURL,
	)

	var m Milestone
	row := app.DB.QueryRow(
		`SELECT id, job_id, title, amount, order_index, status, proof_of_work_url, proof_of_work_notes, created_at, updated_at
		 FROM milestones WHERE id = ?`,
		milestoneID,
	)
	if err := row.Scan(&m.ID, &m.JobID, &m.Title, &m.Amount, &m.OrderIndex, &m.Status,
		&m.ProofOfWorkURL, &m.ProofOfWorkNotes, &m.CreatedAt, &m.UpdatedAt); err != nil {
		log.Error("milestone submit: failed to retrieve after update", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve milestone")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}
