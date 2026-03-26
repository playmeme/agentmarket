package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v82"
	stripesession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/paymentintent"
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
	SowID            string      `json:"sow_id"`
	Title            string      `json:"title"`
	Amount           int64       `json:"amount"`
	OrderIndex       int         `json:"order_index"`
	Deliverables     string      `json:"deliverables"`
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
	AgentName           string      `json:"agent_name,omitempty"`
	Status              string      `json:"status"`
	Title               string      `json:"title"`
	Description         string      `json:"description"`
	TotalPayout         int64       `json:"total_payout"`
	TimelineDays        int         `json:"timeline_days"`
	SowLink             string      `json:"sow_link,omitempty"`
	StripePaymentIntent string      `json:"stripe_payment_intent,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
	Milestones          []Milestone `json:"milestones,omitempty"`
	SOW                 *SOW        `json:"sow"`
}

// --- Request types ---

type MilestoneInput struct {
	Title        string   `json:"title"`
	Amount       int64    `json:"amount"`
	Deliverables string   `json:"deliverables"`
	Criteria     []string `json:"criteria"`
}

type HireRequest struct {
	AgentID      string           `json:"agent_id"`
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	TotalPayout  int64            `json:"total_payout"`
	TimelineDays int              `json:"timeline_days"`
	SowLink      string           `json:"sow_link"`
	Milestones   []MilestoneInput `json:"milestones"`
}

type AssignAgentRequest struct {
	AgentID string `json:"agent_id"`
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
			slog.Error("load milestone criteria: scan error", "milestone_id", milestoneID, "error", err)
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
		`SELECT m.id, m.sow_id, m.title, m.amount, m.order_index, m.deliverables, m.status, m.proof_of_work_url, m.proof_of_work_notes, m.created_at, m.updated_at
		 FROM milestones m
		 JOIN sow s ON m.sow_id = s.id
		 WHERE s.job_id = ? ORDER BY m.order_index`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var milestones []Milestone
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.SowID, &m.Title, &m.Amount, &m.OrderIndex, &m.Deliverables, &m.Status,
			&m.ProofOfWorkURL, &m.ProofOfWorkNotes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			slog.Error("load milestones: scan error", "job_id", jobID, "error", err)
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
	var agentID, sowLink, stripe sql.NullString
	err := row.Scan(&j.ID, &j.EmployerID, &agentID, &j.Status, &j.Title, &j.Description,
		&j.TotalPayout, &j.TimelineDays, &sowLink, &stripe, &j.CreatedAt, &j.UpdatedAt)
	if agentID.Valid {
		j.AgentID = agentID.String
	}
	if sowLink.Valid {
		j.SowLink = sowLink.String
	}
	if stripe.Valid {
		j.StripePaymentIntent = stripe.String
	}
	if sowLink.Valid {
		j.SowLink = sowLink.String
	}
	return j, err
}

// scanJobWithName scans a job row that includes an extra agent_name column at the end.
func (app *App) scanJobWithName(row interface{ Scan(...interface{}) error }) (Job, error) {
	var j Job
	var agentID, sowLink, stripe sql.NullString
	err := row.Scan(&j.ID, &j.EmployerID, &agentID, &j.Status, &j.Title, &j.Description,
		&j.TotalPayout, &j.TimelineDays, &sowLink, &stripe, &j.CreatedAt, &j.UpdatedAt, &j.AgentName)
	if agentID.Valid {
		j.AgentID = agentID.String
	}
	if sowLink.Valid {
		j.SowLink = sowLink.String
	}
	if stripe.Valid {
		j.StripePaymentIntent = stripe.String
	}
	if sowLink.Valid {
		j.SowLink = sowLink.String
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

	// Coerce empty agent_id to nil so SQLite stores NULL instead of an empty
	// string. An empty string fails the FK constraint against the agents table
	// because no agent has id="". This happens when /jobs/new submits without
	// a pre-selected agent. See: https://github.com/playmeme/agentmarket/issues/56
	var agentIDVal interface{}
	if req.AgentID != "" {
		agentIDVal = req.AgentID
	}

	// Set initial status based on whether an agent is being assigned up-front.
	// UNASSIGNED: no agent selected yet — job is a draft brief with no offer made.
	// PENDING_ACCEPTANCE: an agent has been selected and the offer is outstanding.
	initialStatus := "UNASSIGNED"
	if req.AgentID != "" {
		initialStatus = "PENDING_ACCEPTANCE"
	}

	jobID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO jobs (id, employer_id, agent_id, status, title, description, total_payout, timeline_days, sow_link)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		jobID, employerID, agentIDVal, initialStatus, req.Title, req.Description, req.TotalPayout, req.TimelineDays, req.SowLink,
	)
	if err != nil {
		log.Error("job creation failed: insert error", "employer_id", employerID, "agent_id", req.AgentID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	// Milestones are now linked to sow_id (not job_id) and are managed during
	// SOW negotiation via CreateOrUpdateSOW. Any milestones in the hire request
	// are intentionally ignored here — they will be set once the agent accepts
	// and a SOW is created.

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
	)

	// Notify the agent's handler when a job offer is created with an agent assigned
	if req.AgentID != "" {
		var handlerID string
		if err := app.DB.QueryRow("SELECT handler_id FROM agents WHERE id = ?", req.AgentID).Scan(&handlerID); err == nil {
			_ = app.CreateNotification(handlerID, jobID, NotifJobOffer,
				"New job offer: "+req.Title,
				"You have received a new job offer. Review it on your dashboard.")
		}
	}

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

// UpdateJobHandler updates a job brief (title, description, payout, timeline, milestones).
// PUT /api/ui/jobs/{id}
// Only the owning employer may update, and only while no agent is assigned.
func (app *App) UpdateJobHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "update_job")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: update job requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can update jobs")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "id")

	// Load existing job to verify ownership and status
	var existingAgentID sql.NullString
	var existingEmployerID string
	err := app.DB.QueryRow(
		`SELECT employer_id, agent_id FROM jobs WHERE id = ?`, jobID,
	).Scan(&existingEmployerID, &existingAgentID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("update job: database error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if existingEmployerID != employerID {
		writeError(w, http.StatusForbidden, "you do not own this job")
		return
	}
	if existingAgentID.Valid && existingAgentID.String != "" {
		writeError(w, http.StatusConflict, "cannot edit a job that already has an agent assigned")
		return
	}

	var req HireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" || req.TotalPayout == 0 || req.TimelineDays == 0 {
		writeError(w, http.StatusBadRequest, "title, total_payout, and timeline_days are required")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		log.Error("update job: begin transaction error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE jobs SET title = ?, description = ?, total_payout = ?, timeline_days = ?, sow_link = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		req.Title, req.Description, req.TotalPayout, req.TimelineDays, req.SowLink, jobID,
	)
	if err != nil {
		log.Error("update job: update error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update job")
		return
	}

	// Milestones are now linked to sow_id (not job_id) and are managed during
	// SOW negotiation via CreateOrUpdateSOW. Job updates only touch the job brief;
	// milestone changes are handled through the SOW endpoint.

	if err := tx.Commit(); err != nil {
		log.Error("update job: commit error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	log.Info("job updated", "job_id", jobID, "employer_id", employerID, "title", req.Title)

	job, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("update job: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// AssignAgentHandler assigns an agent to an existing unassigned job.
// Requires email verification (because this initiates the agent relationship).
func (app *App) AssignAgentHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "assign_agent")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: assign agent requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can assign agents")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "id")

	var req AssignAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}

	// Email verification required to assign an agent
	var emailVerifiedAt sql.NullTime
	err := app.DB.QueryRow("SELECT email_verified_at FROM users WHERE id = ?", employerID).Scan(&emailVerifiedAt)
	if err != nil {
		log.Error("assign agent: database error checking email verification", "employer_id", employerID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !emailVerifiedAt.Valid {
		log.Warn("assign agent blocked: employer email not verified", "employer_id", employerID)
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

	// Only allow assigning to UNASSIGNED jobs owned by this employer.
	// UNASSIGNED means the job brief exists but no offer has been made yet.
	result, err := app.DB.Exec(
		`UPDATE jobs SET agent_id = ?, status = 'PENDING_ACCEPTANCE', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND employer_id = ? AND status = 'UNASSIGNED'`,
		req.AgentID, jobID, employerID,
	)
	if err != nil {
		log.Error("assign agent: database error", "job_id", jobID, "employer_id", employerID, "agent_id", req.AgentID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to assign agent")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "job not found, not owned by you, or already has an agent assigned")
		return
	}

	log.Info("agent assigned to job", "job_id", jobID, "employer_id", employerID, "agent_id", req.AgentID)

	// Notify agent's handler of job offer
	var handlerID string
	if err := app.DB.QueryRow("SELECT handler_id FROM agents WHERE id = ?", req.AgentID).Scan(&handlerID); err == nil {
		var jobTitle string
		_ = app.DB.QueryRow("SELECT title FROM jobs WHERE id = ?", jobID).Scan(&jobTitle)
		_ = app.CreateNotification(handlerID, jobID, NotifJobOffer,
			"New job offer: "+jobTitle,
			"You have received a new job offer. Review it on your dashboard.")
	}

	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("assign agent: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

func (app *App) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(contextKeyUserID).(string)
	role, _ := r.Context().Value(contextKeyUserRole).(string)

	var rows *sql.Rows
	var err error

	if role == "EMPLOYER" {
		// JOIN agents so we can return the agent name alongside agent_id
		rows, err = app.DB.Query(
			`SELECT j.id, j.employer_id, j.agent_id, j.status, j.title, j.description, j.total_payout, j.timeline_days, j.sow_link, j.stripe_payment_intent, j.created_at, j.updated_at, COALESCE(a.name, '')
			 FROM jobs j
			 LEFT JOIN agents a ON j.agent_id = a.id
			 WHERE j.employer_id = ?
			 ORDER BY j.created_at DESC`,
			userID,
		)
	} else {
		// AGENT_HANDLER: list jobs for any of their agents
		rows, err = app.DB.Query(
			`SELECT j.id, j.employer_id, j.agent_id, j.status, j.title, j.description, j.total_payout, j.timeline_days, j.sow_link, j.stripe_payment_intent, j.created_at, j.updated_at, COALESCE(a.name, '')
			 FROM jobs j
			 JOIN agents a ON j.agent_id = a.id
			 WHERE a.handler_id = ?
			 ORDER BY j.created_at DESC`,
			userID,
		)
	}

	if err != nil {
		slog.Error("list jobs: unknown database error", "user_id", userID, "role", role, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		j, err := app.scanJobWithName(rows)
		if err != nil {
			slog.Error("list jobs: scan error", "user_id", userID, "role", role, "error", err)
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
		`SELECT j.id, j.employer_id, j.agent_id, j.status, j.title, j.description, j.total_payout, j.timeline_days, j.sow_link, j.stripe_payment_intent, j.created_at, j.updated_at, COALESCE(a.name, '')
		 FROM jobs j
		 LEFT JOIN agents a ON j.agent_id = a.id
		 WHERE j.id = ?`,
		jobID,
	)
	j, err := app.scanJobWithName(row)
	if err != nil {
		return j, err
	}

	milestones, err := app.loadMilestonesForJob(jobID)
	if err != nil {
		return j, err
	}
	j.Milestones = milestones

	sow, err := app.getSOWByJobID(jobID)
	if err == nil {
		j.SOW = &sow
	} else if err != sql.ErrNoRows {
		slog.Warn("failed to fetch SOW for job", "job_id", jobID, "error", err)
	}

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
		slog.Error("get job: database error", "job_id", jobID, "error", err)
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
		 WHERE id = ? AND sow_id = (SELECT id FROM sow WHERE job_id = ?) AND status = 'REVIEW_REQUESTED'`,
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
		`SELECT id, sow_id, title, amount, order_index, deliverables, status, proof_of_work_url, proof_of_work_notes, created_at, updated_at
		 FROM milestones WHERE id = ?`,
		milestoneID,
	)
	if err := row.Scan(&m.ID, &m.SowID, &m.Title, &m.Amount, &m.OrderIndex, &m.Deliverables, &m.Status,
		&m.ProofOfWorkURL, &m.ProofOfWorkNotes, &m.CreatedAt, &m.UpdatedAt); err != nil {
		log.Error("milestone approval: failed to retrieve after update", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve milestone")
		return
	}

	// Notify both parties: employer (confirmation) and agent's handler (milestone completed + payment incoming)
	_ = app.CreateNotification(employerID, jobID, NotifMilestoneConfirmed,
		"Milestone approved: "+m.Title,
		"You approved a milestone. Payment will be processed.")

	var agentHandlerID string
	if err := app.DB.QueryRow(
		`SELECT a.handler_id FROM jobs j JOIN agents a ON j.agent_id = a.id WHERE j.id = ?`, jobID,
	).Scan(&agentHandlerID); err == nil {
		_ = app.CreateNotification(agentHandlerID, jobID, NotifMilestoneCompleted,
			"Milestone approved: "+m.Title,
			"The employer has approved your milestone. Payment is on its way.")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

// --- Agent API (API key auth) ---

func (app *App) GetPendingJobsHandler(w http.ResponseWriter, r *http.Request) {
	agentID, _ := r.Context().Value(contextKeyAgentID).(string)

	rows, err := app.DB.Query(
		`SELECT id, employer_id, agent_id, status, title, description, total_payout, timeline_days, sow_link, stripe_payment_intent, created_at, updated_at
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

	// Load job to get payout and timeline for default SOW
	var totalPayout int64
	var timelineDays int
	err := app.DB.QueryRow(
		"SELECT total_payout, timeline_days FROM jobs WHERE id = ? AND agent_id = ? AND status = 'PENDING_ACCEPTANCE'",
		jobID, agentID,
	).Scan(&totalPayout, &timelineDays)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found or not in PENDING_ACCEPTANCE status")
		return
	}
	if err != nil {
		log.Error("job accept: db error loading job", "job_id", jobID, "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		log.Error("job accept: begin transaction error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer tx.Rollback()

	// Move job to SOW_NEGOTIATION
	result, err := tx.Exec(
		`UPDATE jobs SET status = 'SOW_NEGOTIATION', updated_at = CURRENT_TIMESTAMP
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

	// Create default SOW from job's existing payout and timeline
	sowID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO sow (id, job_id, detailed_spec, work_process, price_cents, timeline_days, agent_accepted, employer_accepted)
		 VALUES (?, ?, '', '', ?, ?, 0, 0)`,
		sowID, jobID, totalPayout, timelineDays,
	)
	if err != nil {
		log.Error("job accept: failed to create default SOW", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create default SOW")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("job accept: commit error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	log.Info("job accepted, moved to SOW_NEGOTIATION", "job_id", jobID, "agent_id", agentID, "sow_id", sowID)

	// Notify employer that job offer was accepted
	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("job accept: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	_ = app.CreateNotification(j.EmployerID, jobID, NotifJobOfferAccepted,
		"Job offer accepted: "+j.Title,
		"Your job offer has been accepted. SOW negotiation has begun.")

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

	// Notify employer that job offer was rejected
	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("job decline: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	_ = app.CreateNotification(j.EmployerID, jobID, NotifJobOfferRejected,
		"Job offer declined: "+j.Title,
		"Your job offer was declined by the agent.")

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
		 WHERE id = ? AND sow_id = (SELECT id FROM sow WHERE job_id = ?) AND status = 'PENDING'`,
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
		`SELECT id, sow_id, title, amount, order_index, deliverables, status, proof_of_work_url, proof_of_work_notes, created_at, updated_at
		 FROM milestones WHERE id = ?`,
		milestoneID,
	)
	if err := row.Scan(&m.ID, &m.SowID, &m.Title, &m.Amount, &m.OrderIndex, &m.Deliverables, &m.Status,
		&m.ProofOfWorkURL, &m.ProofOfWorkNotes, &m.CreatedAt, &m.UpdatedAt); err != nil {
		log.Error("milestone submit: failed to retrieve after update", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve milestone")
		return
	}

	// Notify employer that a milestone was delivered
	var employerID string
	if err := app.DB.QueryRow("SELECT employer_id FROM jobs WHERE id = ?", jobID).Scan(&employerID); err == nil {
		_ = app.CreateNotification(employerID, jobID, NotifMilestoneDelivered,
			"Milestone submitted: "+m.Title,
			"An agent has submitted a milestone for your review.")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

// --- Delivery and Approval Handlers ---

type DeliverJobRequest struct {
	DeliveryNotes string `json:"delivery_notes"`
	DeliveryURL   string `json:"delivery_url"`
}

// DeliverJobHandler marks a job as DELIVERED (agent API key auth).
// POST /api/v1/jobs/{job_id}/deliver
func (app *App) DeliverJobHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "deliver_job")

	agentID, _ := r.Context().Value(contextKeyAgentID).(string)
	jobID := chi.URLParam(r, "job_id")

	var req DeliverJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := app.DB.Exec(
		`UPDATE jobs SET status = 'DELIVERED', delivery_notes = ?, delivery_url = ?, delivered_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND agent_id = ? AND status = 'IN_PROGRESS'`,
		req.DeliveryNotes, req.DeliveryURL, jobID, agentID,
	)
	if err != nil {
		log.Error("job deliver failed: database error", "job_id", jobID, "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "job not found or not in IN_PROGRESS status")
		return
	}

	log.Info("job delivered", "job_id", jobID, "agent_id", agentID, "delivery_url", req.DeliveryURL)

	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("job deliver: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	// Notify employer that job was delivered
	_ = app.CreateNotification(j.EmployerID, jobID, NotifMilestoneDelivered,
		"Job delivered: "+j.Title,
		"The agent has delivered the job. Please review and approve or request a revision.")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

// ApproveDeliveryHandler captures the Stripe payment and completes the job (employer JWT auth).
// POST /api/ui/jobs/{job_id}/approve-delivery
func (app *App) ApproveDeliveryHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "approve_delivery")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: approve delivery requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can approve delivery")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")

	// Load job — must be DELIVERED and belong to employer
	var jobStatus string
	var stripePaymentIntent sql.NullString
	err := app.DB.QueryRow(
		"SELECT status, stripe_payment_intent FROM jobs WHERE id = ? AND employer_id = ?",
		jobID, employerID,
	).Scan(&jobStatus, &stripePaymentIntent)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("approve delivery: db error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobStatus != "DELIVERED" {
		writeError(w, http.StatusBadRequest, "job must be in DELIVERED status to approve")
		return
	}

	// Capture the Stripe payment intent if present
	if stripePaymentIntent.Valid && stripePaymentIntent.String != "" {
		stripe.Key = app.Config.StripeSecretKey
		_, err = paymentintent.Capture(stripePaymentIntent.String, nil)
		if err != nil {
			log.Error("approve delivery: stripe capture failed", "job_id", jobID, "payment_intent", stripePaymentIntent.String, "error", err)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to capture payment: %v", err))
			return
		}
		log.Info("payment captured", "job_id", jobID, "payment_intent", stripePaymentIntent.String)
	}

	_, err = app.DB.Exec(
		`UPDATE jobs SET status = 'COMPLETED', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		jobID,
	)
	if err != nil {
		log.Error("approve delivery: failed to mark job completed", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	log.Info("job delivery approved, job completed", "job_id", jobID, "employer_id", employerID)

	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("approve delivery: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	// Notify agent's handler of payment received and job completion
	var agentHandlerID string
	if err := app.DB.QueryRow(
		`SELECT a.handler_id FROM jobs j JOIN agents a ON j.agent_id = a.id WHERE j.id = ?`, jobID,
	).Scan(&agentHandlerID); err == nil {
		_ = app.CreateNotification(agentHandlerID, jobID, NotifPaymentReceived,
			"Payment received: "+j.Title,
			"The employer has approved delivery and payment has been captured. Job is complete.")
		_ = app.CreateNotification(agentHandlerID, jobID, NotifMilestoneConfirmed,
			"Job complete: "+j.Title,
			"The job has been marked as completed.")
	}

	// Notify employer of completion confirmation
	_ = app.CreateNotification(employerID, jobID, NotifMilestoneConfirmed,
		"Job complete: "+j.Title,
		"The job has been completed and payment captured.")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

// RequestRevisionHandler moves a DELIVERED job back to IN_PROGRESS (employer JWT auth, one revision).
// POST /api/ui/jobs/{job_id}/request-revision
func (app *App) RequestRevisionHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "request_revision")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: request revision requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can request revision")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")

	var jobStatus string
	var deliveryNotes sql.NullString
	err := app.DB.QueryRow(
		"SELECT status, delivery_notes FROM jobs WHERE id = ? AND employer_id = ?",
		jobID, employerID,
	).Scan(&jobStatus, &deliveryNotes)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("request revision: db error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobStatus != "DELIVERED" {
		writeError(w, http.StatusBadRequest, "job must be in DELIVERED status to request revision")
		return
	}

	// Check if already revised — tracked by [REVISED] marker in delivery_notes
	notes := ""
	if deliveryNotes.Valid {
		notes = deliveryNotes.String
	}
	alreadyRevised := false
	const revisionMarker = "[REVISED]"
	for i := 0; i <= len(notes)-len(revisionMarker); i++ {
		if notes[i:i+len(revisionMarker)] == revisionMarker {
			alreadyRevised = true
			break
		}
	}
	if alreadyRevised {
		writeError(w, http.StatusBadRequest, "only one revision is allowed per job")
		return
	}

	newNotes := notes + " [REVISED]"
	_, err = app.DB.Exec(
		`UPDATE jobs SET status = 'IN_PROGRESS', delivery_notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		newNotes, jobID,
	)
	if err != nil {
		log.Error("request revision: failed to update job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	log.Info("revision requested, job moved to IN_PROGRESS", "job_id", jobID, "employer_id", employerID)

	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("request revision: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

// RetractOfferHandler allows an employer to retract a job offer before both parties have
// committed via payment. Retraction is permitted while the job is in any of:
//   - PENDING_ACCEPTANCE  — agent has not yet accepted
//   - SOW_NEGOTIATION     — scope is being negotiated
//   - AWAITING_PAYMENT    — both parties agreed but employer has not paid
//
// Once payment succeeds (status moves to IN_PROGRESS) the contract is final and cannot
// be retracted.
// POST /api/ui/jobs/{id}/retract
func (app *App) RetractOfferHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "retract_offer")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: retract offer requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can retract offers")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "id")

	// Fetch current job status and any outstanding Stripe checkout session ID.
	var currentStatus string
	var stripeCheckoutSessionID sql.NullString
	err := app.DB.QueryRow(
		`SELECT status, stripe_checkout_session_id FROM jobs WHERE id = ? AND employer_id = ?`,
		jobID, employerID,
	).Scan(&currentStatus, &stripeCheckoutSessionID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found or not owned by you")
		return
	}
	if err != nil {
		log.Error("retract offer: db error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Only allow retraction before the contract is final (i.e. before payment).
	retractableStatuses := map[string]bool{
		"PENDING_ACCEPTANCE": true,
		"SOW_NEGOTIATION":    true,
		"AWAITING_PAYMENT":   true,
	}
	if !retractableStatuses[currentStatus] {
		writeError(w, http.StatusConflict, "cannot retract an offer once the contract is final (job is "+currentStatus+")")
		return
	}

	// If the job reached AWAITING_PAYMENT, an open Stripe Checkout Session may exist.
	// Attempt to expire it so the employer is not charged later. We treat a Stripe
	// failure as a warning — the DB update still proceeds.
	if stripeCheckoutSessionID.Valid && stripeCheckoutSessionID.String != "" {
		stripe.Key = app.Config.StripeSecretKey
		_, stripeErr := stripesession.Expire(stripeCheckoutSessionID.String, &stripe.CheckoutSessionExpireParams{})
		if stripeErr != nil {
			log.Warn("retract offer: could not expire stripe checkout session",
				"job_id", jobID,
				"session_id", stripeCheckoutSessionID.String,
				"error", stripeErr,
			)
		} else {
			log.Info("retract offer: stripe checkout session expired",
				"job_id", jobID,
				"session_id", stripeCheckoutSessionID.String,
			)
		}
	}

	result, err := app.DB.Exec(
		`UPDATE jobs
		    SET status = 'RETRACTED',
		        agent_id = NULL,
		        stripe_checkout_session_id = NULL,
		        updated_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND employer_id = ?
		    AND status IN ('PENDING_ACCEPTANCE', 'SOW_NEGOTIATION', 'AWAITING_PAYMENT')`,
		jobID, employerID,
	)
	if err != nil {
		log.Error("retract offer failed: database error", "job_id", jobID, "employer_id", employerID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusConflict, "job could not be retracted — it may have been updated concurrently")
		return
	}

	log.Info("offer retracted", "job_id", jobID, "employer_id", employerID, "previous_status", currentStatus)

	j, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("retract offer: failed to retrieve after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(j)
}

// TransactionSummary is a lightweight view of a job for transaction listing.
type TransactionSummary struct {
	JobID               string `json:"job_id"`
	Title               string `json:"title"`
	Status              string `json:"status"`
	TotalPayout         int64  `json:"total_payout"`
	StripePaymentIntent string `json:"stripe_payment_intent,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

// GetTransactionsHandler lists all jobs for the current user with payment status.
// GET /api/ui/transactions
func (app *App) GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(contextKeyUserID).(string)
	role, _ := r.Context().Value(contextKeyUserRole).(string)

	var query string
	if role == "EMPLOYER" {
		query = `SELECT id, title, status, total_payout, stripe_payment_intent, created_at, updated_at
		          FROM jobs WHERE employer_id = ? ORDER BY created_at DESC`
	} else {
		query = `SELECT j.id, j.title, j.status, j.total_payout, j.stripe_payment_intent, j.created_at, j.updated_at
		          FROM jobs j JOIN agents a ON j.agent_id = a.id
		          WHERE a.handler_id = ? ORDER BY j.created_at DESC`
	}

	rows, err := app.DB.Query(query, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	transactions := []TransactionSummary{}
	for rows.Next() {
		var t TransactionSummary
		var spi sql.NullString
		if err := rows.Scan(&t.JobID, &t.Title, &t.Status, &t.TotalPayout, &spi, &t.CreatedAt, &t.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		if spi.Valid {
			t.StripePaymentIntent = spi.String
		}
		transactions = append(transactions, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

// --- Milestone management during SOW_NEGOTIATION ---

type MilestoneUpdateRequest struct {
	Title    string   `json:"title"`
	Amount   int64    `json:"amount"`
	Criteria []string `json:"criteria"`
}

// resetSOWAcceptance clears both agent_accepted and employer_accepted on the SOW for a job.
// Called whenever milestones change during negotiation so both parties must re-accept.
func (app *App) resetSOWAcceptance(jobID string) error {
	_, err := app.DB.Exec(
		`UPDATE sow SET agent_accepted = 0, employer_accepted = 0, updated_at = CURRENT_TIMESTAMP WHERE job_id = ?`,
		jobID,
	)
	return err
}

// AddMilestoneHandler adds a milestone to a job during SOW_NEGOTIATION.
// POST /api/ui/jobs/{job_id}/milestones
func (app *App) AddMilestoneHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "add_milestone")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")

	var jobStatus string
	err := app.DB.QueryRow("SELECT status FROM jobs WHERE id = ?", jobID).Scan(&jobStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("add milestone: db error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobStatus != "SOW_NEGOTIATION" {
		writeError(w, http.StatusBadRequest, "milestones can only be edited during SOW_NEGOTIATION")
		return
	}

	ok, err := app.isJobParticipant(jobID, userID)
	if err != nil {
		log.Error("add milestone: db error checking participant", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not authorized to edit milestones for this job")
		return
	}

	var req MilestoneUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	// Determine the next order_index
	var maxOrder int
	err = app.DB.QueryRow(`SELECT COALESCE(MAX(order_index), -1) FROM milestones WHERE job_id = ?`, jobID).Scan(&maxOrder)
	if err != nil {
		log.Error("add milestone: db error getting max order_index", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		log.Error("add milestone: begin tx error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback()

	msID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO milestones (id, job_id, title, amount, order_index) VALUES (?, ?, ?, ?, ?)`,
		msID, jobID, req.Title, req.Amount, maxOrder+1,
	)
	if err != nil {
		log.Error("add milestone: insert error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to add milestone")
		return
	}

	for _, criteriaDesc := range req.Criteria {
		cID := uuid.New().String()
		_, err = tx.Exec(`INSERT INTO criteria (id, milestone_id, description) VALUES (?, ?, ?)`, cID, msID, criteriaDesc)
		if err != nil {
			log.Error("add milestone: criteria insert error", "milestone_id", msID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to add criteria")
			return
		}
	}

	// Reset SOW acceptance so both parties must re-accept after milestone change
	_, err = tx.Exec(
		`UPDATE sow SET agent_accepted = 0, employer_accepted = 0, updated_at = CURRENT_TIMESTAMP WHERE job_id = ?`,
		jobID,
	)
	if err != nil {
		log.Error("add milestone: failed to reset SOW acceptance", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to reset SOW acceptance")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("add milestone: commit error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	log.Info("milestone added", "job_id", jobID, "milestone_id", msID, "user_id", userID)

	job, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("add milestone: failed to retrieve job after insert", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// EditMilestoneHandler edits an existing milestone during SOW_NEGOTIATION.
// PUT /api/ui/jobs/{job_id}/milestones/{milestone_id}
func (app *App) EditMilestoneHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "edit_milestone")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")
	milestoneID := chi.URLParam(r, "milestone_id")

	var jobStatus string
	err := app.DB.QueryRow("SELECT status FROM jobs WHERE id = ?", jobID).Scan(&jobStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("edit milestone: db error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobStatus != "SOW_NEGOTIATION" {
		writeError(w, http.StatusBadRequest, "milestones can only be edited during SOW_NEGOTIATION")
		return
	}

	ok, err := app.isJobParticipant(jobID, userID)
	if err != nil {
		log.Error("edit milestone: db error checking participant", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not authorized to edit milestones for this job")
		return
	}

	// Verify milestone belongs to this job
	var milestoneJobID string
	err = app.DB.QueryRow(`SELECT job_id FROM milestones WHERE id = ?`, milestoneID).Scan(&milestoneJobID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "milestone not found")
		return
	}
	if err != nil {
		log.Error("edit milestone: db error fetching milestone", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if milestoneJobID != jobID {
		writeError(w, http.StatusForbidden, "milestone does not belong to this job")
		return
	}

	var req MilestoneUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		log.Error("edit milestone: begin tx error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE milestones SET title = ?, amount = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		req.Title, req.Amount, milestoneID,
	)
	if err != nil {
		log.Error("edit milestone: update error", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update milestone")
		return
	}

	// Replace criteria
	if _, err = tx.Exec(`DELETE FROM criteria WHERE milestone_id = ?`, milestoneID); err != nil {
		log.Error("edit milestone: delete criteria error", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update criteria")
		return
	}
	for _, criteriaDesc := range req.Criteria {
		cID := uuid.New().String()
		_, err = tx.Exec(`INSERT INTO criteria (id, milestone_id, description) VALUES (?, ?, ?)`, cID, milestoneID, criteriaDesc)
		if err != nil {
			log.Error("edit milestone: criteria insert error", "milestone_id", milestoneID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update criteria")
			return
		}
	}

	// Reset SOW acceptance so both parties must re-accept after milestone change
	_, err = tx.Exec(
		`UPDATE sow SET agent_accepted = 0, employer_accepted = 0, updated_at = CURRENT_TIMESTAMP WHERE job_id = ?`,
		jobID,
	)
	if err != nil {
		log.Error("edit milestone: failed to reset SOW acceptance", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to reset SOW acceptance")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("edit milestone: commit error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	log.Info("milestone updated", "job_id", jobID, "milestone_id", milestoneID, "user_id", userID)

	job, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("edit milestone: failed to retrieve job after update", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// DeleteMilestoneHandler removes a milestone during SOW_NEGOTIATION.
// DELETE /api/ui/jobs/{job_id}/milestones/{milestone_id}
func (app *App) DeleteMilestoneHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "delete_milestone")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")
	milestoneID := chi.URLParam(r, "milestone_id")

	var jobStatus string
	err := app.DB.QueryRow("SELECT status FROM jobs WHERE id = ?", jobID).Scan(&jobStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("delete milestone: db error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobStatus != "SOW_NEGOTIATION" {
		writeError(w, http.StatusBadRequest, "milestones can only be edited during SOW_NEGOTIATION")
		return
	}

	ok, err := app.isJobParticipant(jobID, userID)
	if err != nil {
		log.Error("delete milestone: db error checking participant", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not authorized to edit milestones for this job")
		return
	}

	// Verify milestone belongs to this job
	var milestoneJobID string
	err = app.DB.QueryRow(`SELECT job_id FROM milestones WHERE id = ?`, milestoneID).Scan(&milestoneJobID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "milestone not found")
		return
	}
	if err != nil {
		log.Error("delete milestone: db error fetching milestone", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if milestoneJobID != jobID {
		writeError(w, http.StatusForbidden, "milestone does not belong to this job")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		log.Error("delete milestone: begin tx error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`DELETE FROM criteria WHERE milestone_id = ?`, milestoneID); err != nil {
		log.Error("delete milestone: delete criteria error", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete criteria")
		return
	}
	result, err := tx.Exec(`DELETE FROM milestones WHERE id = ?`, milestoneID)
	if err != nil {
		log.Error("delete milestone: delete error", "milestone_id", milestoneID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete milestone")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		_ = tx.Rollback()
		writeError(w, http.StatusNotFound, "milestone not found")
		return
	}

	// Reset SOW acceptance so both parties must re-accept after milestone change
	_, err = tx.Exec(
		`UPDATE sow SET agent_accepted = 0, employer_accepted = 0, updated_at = CURRENT_TIMESTAMP WHERE job_id = ?`,
		jobID,
	)
	if err != nil {
		log.Error("delete milestone: failed to reset SOW acceptance", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to reset SOW acceptance")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("delete milestone: commit error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	log.Info("milestone deleted", "job_id", jobID, "milestone_id", milestoneID, "user_id", userID)

	job, err := app.getJobDetail(jobID)
	if err != nil {
		log.Error("delete milestone: failed to retrieve job after delete", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// DeleteJobHandler permanently deletes a job brief and all its child records.
// Deletion is only permitted when no agent has been assigned (agent_id IS NULL).
// DELETE /api/ui/jobs/{id}
func (app *App) DeleteJobHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "delete_job")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: delete job requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can delete jobs")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "id")

	// Verify ownership and that no agent is assigned.
	var exists int
	err := app.DB.QueryRow(
		`SELECT 1 FROM jobs WHERE id = ? AND employer_id = ? AND agent_id IS NULL`,
		jobID, employerID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found, not owned by you, or an agent is already assigned")
		return
	}
	if err != nil {
		log.Error("delete job: db error checking ownership", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	tx, err := app.DB.Begin()
	if err != nil {
		log.Error("delete job: begin tx error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback()

	// Delete child rows in dependency order.
	if _, err = tx.Exec(
		`DELETE FROM criteria WHERE milestone_id IN (SELECT id FROM milestones WHERE job_id = ?)`,
		jobID,
	); err != nil {
		log.Error("delete job: failed to delete criteria", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete job")
		return
	}

	if _, err = tx.Exec(`DELETE FROM milestones WHERE job_id = ?`, jobID); err != nil {
		log.Error("delete job: failed to delete milestones", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete job")
		return
	}

	if _, err = tx.Exec(`DELETE FROM sow WHERE job_id = ?`, jobID); err != nil {
		log.Error("delete job: failed to delete sow", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete job")
		return
	}

	if _, err = tx.Exec(`DELETE FROM notifications WHERE job_id = ?`, jobID); err != nil {
		log.Error("delete job: failed to delete notifications", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete job")
		return
	}

	result, err := tx.Exec(
		`DELETE FROM jobs WHERE id = ? AND employer_id = ? AND agent_id IS NULL`,
		jobID, employerID,
	)
	if err != nil {
		log.Error("delete job: failed to delete job row", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete job")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		_ = tx.Rollback()
		writeError(w, http.StatusConflict, "job could not be deleted — it may have been updated concurrently")
		return
	}

	if err := tx.Commit(); err != nil {
		log.Error("delete job: commit error", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	log.Info("job deleted", "job_id", jobID, "employer_id", employerID)

	w.WriteHeader(http.StatusNoContent)
}
