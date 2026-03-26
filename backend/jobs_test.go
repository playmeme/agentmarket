package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// setupJobFixtures creates a verified employer, an agent handler, an agent, and returns them.
func setupJobFixtures(t *testing.T, app *App) (employerID, managerID, agentID, agentAPIKey string) {
	t.Helper()
	employerID, _ = createVerifiedTestUser(t, app, "EMPLOYER")
	managerID, _ = createTestUser(t, app, "AGENT_MANAGER")
	agentID, agentAPIKey = createTestAgent(t, app, managerID)
	return
}

func TestHireAgent(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, _ := setupJobFixtures(t, app)
	token := makeAuthToken(t, app, employerID, "EMPLOYER")

	body := HireRequest{
		AgentID:      agentID,
		Title:        "Build me a feature",
		Description:  "The details",
		TotalPayout:  5000,
		TimelineDays: 7,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", body, token)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var job Job
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if job.ID == "" {
		t.Error("expected job ID")
	}
	if job.Status != "PENDING_ACCEPTANCE" {
		t.Errorf("expected status PENDING_ACCEPTANCE, got %q", job.Status)
	}
	// Milestones are now linked to sow_id and are set during SOW negotiation,
	// not at job creation time. No milestones should be present at hire time.
	if len(job.Milestones) != 0 {
		t.Errorf("expected 0 milestones at hire time (milestones belong to SOW), got %d", len(job.Milestones))
	}
}

// TestHireAgentNoAgentID verifies that a job created without an agent_id gets
// UNASSIGNED status. No offer has been made yet, so the job is just a draft brief.
func TestHireAgentNoAgentID(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _ := createVerifiedTestUser(t, app, "EMPLOYER")
	token := makeAuthToken(t, app, employerID, "EMPLOYER")

	body := HireRequest{
		// AgentID intentionally omitted (empty string)
		Title:        "Draft job without agent",
		Description:  "No agent assigned yet",
		TotalPayout:  1000,
		TimelineDays: 5,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", body, token)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for job without agent_id, got %d: %s", rr.Code, rr.Body.String())
	}

	var job Job
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if job.ID == "" {
		t.Error("expected job ID to be non-empty")
	}
	if job.AgentID != "" {
		t.Errorf("expected agent_id to be empty, got %q", job.AgentID)
	}
	// A job created with no agent should be UNASSIGNED, not PENDING_ACCEPTANCE.
	// PENDING_ACCEPTANCE is reserved for jobs where an offer has been made.
	if job.Status != "UNASSIGNED" {
		t.Errorf("expected status UNASSIGNED, got %q", job.Status)
	}
}

func TestHireAgentUnverifiedEmail(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	// Unverified employer.
	employerID, _ := createTestUser(t, app, "EMPLOYER")
	_, _, agentID, _ := setupJobFixtures(t, app)
	token := makeAuthToken(t, app, employerID, "EMPLOYER")

	body := HireRequest{
		AgentID: agentID, Title: "Work", TotalPayout: 100, TimelineDays: 1,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", body, token)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unverified employer, got %d", rr.Code)
	}
}

func TestHireAgentWrongRole(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	managerID, _ := createTestUser(t, app, "AGENT_MANAGER")
	token := makeAuthToken(t, app, managerID, "AGENT_MANAGER")
	_, _, agentID, _ := setupJobFixtures(t, app)

	body := HireRequest{AgentID: agentID, Title: "Work", TotalPayout: 100, TimelineDays: 1}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", body, token)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for AGENT_MANAGER role, got %d", rr.Code)
	}
}

func TestListJobs(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, _ := setupJobFixtures(t, app)
	token := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Create a job.
	hireBody := HireRequest{
		AgentID: agentID, Title: "Test Job", TotalPayout: 1000, TimelineDays: 3,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", hireBody, token)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/", nil, token)
	if rr.Code != http.StatusOK {
		t.Fatalf("list jobs: expected 200, got %d", rr.Code)
	}
	var jobs []Job
	json.Unmarshal(rr.Body.Bytes(), &jobs)
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
}

func TestAcceptJob(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, apiKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Create a job.
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Accept me", TotalPayout: 500, TimelineDays: 2},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Accept the job via the agent API key.
	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/accept", nil, apiKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("accept: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var updated Job
	json.Unmarshal(rr.Body.Bytes(), &updated)
	if updated.Status != "SOW_NEGOTIATION" {
		t.Errorf("expected status SOW_NEGOTIATION, got %q", updated.Status)
	}
}

func TestDeclineJob(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, apiKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Decline me", TotalPayout: 500, TimelineDays: 2},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Decline the job via API key.
	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/decline", nil, apiKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("decline: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var updated Job
	json.Unmarshal(rr.Body.Bytes(), &updated)
	if updated.Status != "CANCELLED" {
		t.Errorf("expected status CANCELLED, got %q", updated.Status)
	}
}

func TestAcceptJobAlreadyAccepted(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, apiKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Double-accept", TotalPayout: 100, TimelineDays: 1},
		employerToken)
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/accept", nil, apiKey)
	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/accept", nil, apiKey)
	if rr.Code != http.StatusNotFound {
		t.Errorf("double-accept: expected 404, got %d", rr.Code)
	}
}

func TestGetPendingJobs(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, apiKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Create two jobs.
	for i := 0; i < 2; i++ {
		doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
			HireRequest{AgentID: agentID, Title: "Pending job", TotalPayout: 100, TimelineDays: 1},
			employerToken)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/v1/jobs/pending", nil, apiKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var jobs []Job
	json.Unmarshal(rr.Body.Bytes(), &jobs)
	if len(jobs) != 2 {
		t.Errorf("expected 2 pending jobs, got %d", len(jobs))
	}
}

func TestRetractOffer(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, _ := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Create a job with agent assigned (PENDING_ACCEPTANCE)
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Job to retract", TotalPayout: 500, TimelineDays: 3},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "PENDING_ACCEPTANCE" {
		t.Fatalf("expected PENDING_ACCEPTANCE status, got %q", job.Status)
	}

	// Retract the offer
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/retract", nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("retract: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var retracted Job
	json.Unmarshal(rr.Body.Bytes(), &retracted)
	if retracted.Status != "UNASSIGNED" {
		t.Errorf("expected UNASSIGNED status, got %q", retracted.Status)
	}
	if retracted.AgentID != "" {
		t.Errorf("expected agent_id to be cleared, got %q", retracted.AgentID)
	}

	// Retracting again should fail — status is now UNASSIGNED (not a retractable status)
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/retract", nil, employerToken)
	if rr.Code != http.StatusConflict {
		t.Errorf("double-retract: expected 409, got %d", rr.Code)
	}
}

func TestRetractOfferWrongEmployer(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, _ := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Create a job
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Someone else job", TotalPayout: 100, TimelineDays: 1},
		employerToken)
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Different employer tries to retract
	otherEmployerID, _ := createVerifiedTestUser(t, app, "EMPLOYER")
	otherToken := makeAuthToken(t, app, otherEmployerID, "EMPLOYER")

	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/retract", nil, otherToken)
	if rr.Code != http.StatusNotFound {
		t.Errorf("wrong employer: expected 404, got %d", rr.Code)
	}
}

// TestRetractOfferDuringSowNegotiation verifies that an employer can retract while the
// job is in SOW_NEGOTIATION (agent has accepted but scope is still being negotiated).
func TestRetractOfferDuringSowNegotiation(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, apiKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Create a job and have the agent accept it (moves to SOW_NEGOTIATION)
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Sow negotiation job", TotalPayout: 300, TimelineDays: 5},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/accept", nil, apiKey)

	// Verify job is now in SOW_NEGOTIATION
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+job.ID, nil, employerToken)
	var updated Job
	json.Unmarshal(rr.Body.Bytes(), &updated)
	if updated.Status != "SOW_NEGOTIATION" {
		t.Fatalf("expected SOW_NEGOTIATION, got %q", updated.Status)
	}

	// Employer retracts during negotiation — should succeed
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/retract", nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("retract during SOW_NEGOTIATION: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var retracted Job
	json.Unmarshal(rr.Body.Bytes(), &retracted)
	if retracted.Status != "UNASSIGNED" {
		t.Errorf("expected UNASSIGNED, got %q", retracted.Status)
	}
	if retracted.AgentID != "" {
		t.Errorf("expected agent_id cleared, got %q", retracted.AgentID)
	}
}

// TestRetractOfferFinalContract verifies that retraction is blocked once the contract is
// final (IN_PROGRESS and beyond — i.e. after payment has been captured).
func TestRetractOfferFinalContract(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, _ := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Create a job
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Final contract job", TotalPayout: 200, TimelineDays: 2},
		employerToken)
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Force job directly to IN_PROGRESS (simulating completed payment)
	_, err := app.DB.Exec(
		`UPDATE jobs SET status = 'IN_PROGRESS', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		job.ID,
	)
	if err != nil {
		t.Fatalf("failed to set job to IN_PROGRESS: %v", err)
	}

	// Employer tries to retract a final contract — should be blocked
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/retract", nil, employerToken)
	if rr.Code != http.StatusConflict {
		t.Errorf("final contract retract: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}
