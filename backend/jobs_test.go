package main

import (
	"encoding/json"
	"fmt"
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
	if updated.Status != "UNASSIGNED" {
		t.Errorf("expected status UNASSIGNED, got %q", updated.Status)
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

// TestUIRejectJobDuringSowNegotiation verifies that an AGENT_MANAGER can reject (decline)
// a job via the UI endpoint while the job is in SOW_NEGOTIATION status.
// This is the regression case from issue #105: PR #106 fixed DeclineJobHandler (agent API-key
// path) but UIRejectJobHandler still blocked the request with "not in PENDING_ACCEPTANCE status".
func TestUIRejectJobDuringSowNegotiation(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, apiKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")
	managerToken := makeAuthToken(t, app, managerID, "AGENT_MANAGER")

	// Create job offer (PENDING_ACCEPTANCE) and have the agent accept it (SOW_NEGOTIATION).
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Sow negotiation decline test", TotalPayout: 400, TimelineDays: 3},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/accept", nil, apiKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("accept: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify job is now in SOW_NEGOTIATION.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+job.ID, nil, employerToken)
	var updated Job
	json.Unmarshal(rr.Body.Bytes(), &updated)
	if updated.Status != "SOW_NEGOTIATION" {
		t.Fatalf("expected SOW_NEGOTIATION, got %q", updated.Status)
	}

	// Manager declines during SOW_NEGOTIATION via the UI reject endpoint — must succeed.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/reject",
		map[string]string{"reason": "scope too large"}, managerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("ui reject during SOW_NEGOTIATION: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var rejected Job
	json.Unmarshal(rr.Body.Bytes(), &rejected)
	if rejected.Status != "UNASSIGNED" {
		t.Errorf("expected UNASSIGNED after decline, got %q", rejected.Status)
	}
	if rejected.AgentID != "" {
		t.Errorf("expected agent_id cleared after decline, got %q", rejected.AgentID)
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

// setupJobInProgress creates a job and force-sets it to IN_PROGRESS (no milestones),
// simulating a job that bypassed SOW payment. Returns jobID and the API key.
func setupInProgressJob(t *testing.T, app *App, router http.Handler, employerID, agentID string) (jobID, apiKey string) {
	t.Helper()
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")

	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "No-milestone job", TotalPayout: 500, TimelineDays: 5},
		employerTok)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	// Force to IN_PROGRESS (skip payment/SOW for simplicity).
	if _, err := app.DB.Exec(
		`UPDATE jobs SET status = 'IN_PROGRESS' WHERE id = ?`, job.ID,
	); err != nil {
		t.Fatalf("force IN_PROGRESS: %v", err)
	}
	return job.ID, ""
}

// TestDeliverJobBlockedWhenMilestonesExist verifies that DeliverJobHandler returns 400
// when the job has milestones, enforcing per-milestone delivery via SubmitMilestoneHandler.
func TestDeliverJobBlockedWhenMilestonesExist(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)

	// Build a job with two milestones up to AWAITING_PAYMENT via SOW.
	jobID, _, _ := setupSowWithMilestones(t, app, router, employerID, managerID, agentID, agentAPIKey)

	// Use a 100% coupon to skip Stripe and move job to IN_PROGRESS.
	if _, err := app.DB.Exec(
		`INSERT INTO coupons (code, value, max_uses, times_used) VALUES ('FREE128', '100%', 10, 0)`,
	); err != nil {
		t.Fatalf("insert coupon: %v", err)
	}
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/checkout",
		map[string]string{"coupon_code": "FREE128"}, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("checkout: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify job is IN_PROGRESS.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Fatalf("expected IN_PROGRESS, got %q", job.Status)
	}

	// Attempt to use DeliverJobHandler on a job that has milestones — should be rejected.
	body := DeliverJobRequest{DeliveryNotes: "Done", DeliveryURL: "https://example.com"}
	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+jobID+"/deliver", body, agentAPIKey)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("deliver with milestones: expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestMilestoneLifecycleSingleMilestone verifies the happy path for a single-milestone job:
//   - SubmitMilestoneHandler marks the milestone REVIEW_REQUESTED, job stays IN_PROGRESS.
//   - ApproveMilestoneHandler marks the milestone PAID.
//   - Because it was the last milestone, the job is marked COMPLETED.
func TestMilestoneLifecycleSingleMilestone(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")
	managerTok := makeAuthToken(t, app, managerID, "AGENT_MANAGER")

	// Create job.
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Single milestone job", TotalPayout: 100, TimelineDays: 7},
		employerTok)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Agent manager accepts.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/accept", nil, managerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("accept: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Create SOW with ONE milestone.
	sowBody := SOWRequest{
		DetailedSpec: "Build it",
		WorkProcess:  "Daily updates",
		PriceCents:   10000,
		TimelineDays: 7,
		Milestones: []SOWMilestoneInput{
			{Title: "Only milestone", Amount: 100, Deliverables: "The thing"},
		},
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow", sowBody, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("create sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both parties accept SOW.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow/accept", nil, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("employer sow accept: %d %s", rr.Code, rr.Body.String())
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow/accept", nil, managerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("manager sow accept: %d %s", rr.Code, rr.Body.String())
	}

	// Skip payment via 100% coupon.
	couponCode := fmt.Sprintf("SOLO-%s", job.ID[:6])
	if _, err := app.DB.Exec(
		`INSERT INTO coupons (code, value, max_uses, times_used) VALUES (?, '100%', 1, 0)`, couponCode,
	); err != nil {
		t.Fatalf("insert coupon: %v", err)
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/checkout",
		map[string]string{"coupon_code": couponCode}, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("checkout: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Confirm job is IN_PROGRESS and retrieve milestone ID.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+job.ID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Fatalf("expected IN_PROGRESS after checkout, got %q", job.Status)
	}
	if len(job.Milestones) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(job.Milestones))
	}
	milestoneID := job.Milestones[0].ID

	// --- Step 1: Agent submits milestone ---
	submitBody := SubmitProofRequest{
		ProofOfWorkURL:   "https://github.com/example/pr/1",
		ProofOfWorkNotes: "Feature complete",
	}
	rr = doRequest(t, router, http.MethodPost,
		"/api/v1/jobs/"+job.ID+"/milestones/"+milestoneID+"/submit", submitBody, agentAPIKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("milestone submit: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Job should still be IN_PROGRESS after milestone submit.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+job.ID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Errorf("job should remain IN_PROGRESS after milestone submit, got %q", job.Status)
	}

	// The milestone should now be REVIEW_REQUESTED.
	if job.Milestones[0].Status != "REVIEW_REQUESTED" {
		t.Errorf("milestone should be REVIEW_REQUESTED, got %q", job.Milestones[0].Status)
	}

	// --- Step 2: Employer approves the (only) milestone ---
	rr = doRequest(t, router, http.MethodPost,
		"/api/ui/jobs/"+job.ID+"/milestones/"+milestoneID+"/approve", nil, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("milestone approve: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// The job should now be COMPLETED (last milestone was approved).
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+job.ID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "COMPLETED" {
		t.Errorf("job should be COMPLETED after last milestone approved, got %q", job.Status)
	}
}

// TestMilestoneLifecycleTwoMilestones verifies the multi-milestone flow:
//   - Payment is collected upfront at initial checkout.
//   - After approving milestone 1, job stays IN_PROGRESS (not COMPLETED).
//   - After approving milestone 2 (last), job moves to COMPLETED.
func TestMilestoneLifecycleTwoMilestones(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")

	jobID, m1ID, m2ID := setupSowWithMilestones(t, app, router, employerID, managerID, agentID, agentAPIKey)

	// Skip payment for milestone 1 via 100% coupon.
	couponCode := fmt.Sprintf("M1-%s", jobID[:6])
	if _, err := app.DB.Exec(
		`INSERT INTO coupons (code, value, max_uses, times_used) VALUES (?, '100%', 1, 0)`, couponCode,
	); err != nil {
		t.Fatalf("insert coupon: %v", err)
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/checkout",
		map[string]string{"coupon_code": couponCode}, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("checkout m1: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify IN_PROGRESS.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Fatalf("expected IN_PROGRESS after checkout, got %q", job.Status)
	}

	// --- Milestone 1: submit then approve ---
	submitBody := SubmitProofRequest{ProofOfWorkURL: "https://example.com/m1", ProofOfWorkNotes: "M1 done"}
	rr = doRequest(t, router, http.MethodPost,
		"/api/v1/jobs/"+jobID+"/milestones/"+m1ID+"/submit", submitBody, agentAPIKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("m1 submit: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Job must still be IN_PROGRESS after m1 submit.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Errorf("job should remain IN_PROGRESS after m1 submit, got %q", job.Status)
	}

	// Approve milestone 1.
	rr = doRequest(t, router, http.MethodPost,
		"/api/ui/jobs/"+jobID+"/milestones/"+m1ID+"/approve", nil, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("m1 approve: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// After approving m1 (not last), job should move to AWAITING_PAYMENT
	// because m2 has a non-zero amount — milestones are per-milestone payment gates.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "AWAITING_PAYMENT" {
		t.Errorf("job should be AWAITING_PAYMENT after m1 approved (m2 needs payment), got %q", job.Status)
	}

	// --- Employer pays for milestone 2 ---
	couponCode2 := fmt.Sprintf("M2-%s", jobID[:6])
	if _, err := app.DB.Exec(
		`INSERT INTO coupons (code, value, max_uses, times_used) VALUES (?, '100%', 1, 0)`, couponCode2,
	); err != nil {
		t.Fatalf("insert coupon m2: %v", err)
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/checkout",
		map[string]string{"coupon_code": couponCode2}, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("checkout m2: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// --- Milestone 2: submit then approve ---
	submitBody2 := SubmitProofRequest{ProofOfWorkURL: "https://example.com/m2", ProofOfWorkNotes: "M2 done"}
	rr = doRequest(t, router, http.MethodPost,
		"/api/v1/jobs/"+jobID+"/milestones/"+m2ID+"/submit", submitBody2, agentAPIKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("m2 submit: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Approve milestone 2 (last).
	rr = doRequest(t, router, http.MethodPost,
		"/api/ui/jobs/"+jobID+"/milestones/"+m2ID+"/approve", nil, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("m2 approve: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// After approving m2 (last), job should be COMPLETED.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "COMPLETED" {
		t.Errorf("job should be COMPLETED after last milestone approved, got %q", job.Status)
	}
}

// TestDeliverJobNoMilestonesStillWorks verifies that DeliverJobHandler continues to
// work normally for jobs that have no milestones (the legacy flow).
func TestDeliverJobNoMilestonesStillWorks(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")

	// Hire and force to IN_PROGRESS (no SOW, no milestones).
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "No-milestone job", TotalPayout: 100, TimelineDays: 2},
		employerTok)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	if _, err := app.DB.Exec(
		`UPDATE jobs SET status = 'IN_PROGRESS' WHERE id = ?`, job.ID,
	); err != nil {
		t.Fatalf("force IN_PROGRESS: %v", err)
	}

	// Deliver should succeed because there are no milestones.
	body := DeliverJobRequest{DeliveryNotes: "All done", DeliveryURL: "https://example.com"}
	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/deliver", body, agentAPIKey)
	if rr.Code != http.StatusOK {
		t.Errorf("deliver no-milestone job: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var delivered Job
	json.Unmarshal(rr.Body.Bytes(), &delivered)
	if delivered.Status != "DELIVERED" {
		t.Errorf("expected DELIVERED status, got %q", delivered.Status)
	}
}

// TestUIDeliverJobBlockedWhenMilestonesExist verifies that UIDeliverJobHandler (JWT path)
// also rejects delivery when the job has milestones — matching the same guard in the API
// key DeliverJobHandler. Regression for issue #128 (bug surviving PRs #140 and #141).
func TestUIDeliverJobBlockedWhenMilestonesExist(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")
	managerTok := makeAuthToken(t, app, managerID, "AGENT_MANAGER")
	_ = agentAPIKey

	jobID, _, _ := setupSowWithMilestones(t, app, router, employerID, managerID, agentID, agentAPIKey)

	// Use a 100% coupon to skip Stripe and move job to IN_PROGRESS.
	if _, err := app.DB.Exec(
		`INSERT INTO coupons (code, value, max_uses, times_used) VALUES ('UIDEL128', '100%', 10, 0)`,
	); err != nil {
		t.Fatalf("insert coupon: %v", err)
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/checkout",
		map[string]string{"coupon_code": "UIDEL128"}, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("checkout: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify job is IN_PROGRESS.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Fatalf("expected IN_PROGRESS, got %q", job.Status)
	}

	// Attempt to use UIDeliverJobHandler on a job that has milestones — should be rejected.
	body := DeliverJobRequest{DeliveryNotes: "Done", DeliveryURL: "https://example.com"}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/deliver", body, managerTok)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("ui deliver with milestones: expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	// Job must still be IN_PROGRESS (not DELIVERED).
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Errorf("job should still be IN_PROGRESS after rejected ui deliver, got %q", job.Status)
	}
}

// TestMilestoneLifecycleThreeMilestones verifies the full 3-milestone flow — the specific
// scenario reported in issue #128. Payment is collected upfront. After approving each
// non-final milestone, the job stays IN_PROGRESS. After the last milestone, COMPLETED.
func TestMilestoneLifecycleThreeMilestones(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")
	managerTok := makeAuthToken(t, app, managerID, "AGENT_MANAGER")

	// Create a job with 3 milestones.
	hireBody := HireRequest{
		AgentID:      agentID,
		Title:        "Three milestone job",
		TotalPayout:  300,
		TimelineDays: 21,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", hireBody, employerTok)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)
	jobID := job.ID

	// Agent manager accepts.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/accept", nil, managerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("accept: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Create SOW with 3 milestones (100 + 100 + 100 = 300).
	sowBody := SOWRequest{
		DetailedSpec: "Build three things",
		WorkProcess:  "Weekly updates",
		PriceCents:   30000,
		TimelineDays: 21,
		Milestones: []SOWMilestoneInput{
			{Title: "Phase 1", Amount: 100, Deliverables: "First third"},
			{Title: "Phase 2", Amount: 100, Deliverables: "Second third"},
			{Title: "Phase 3", Amount: 100, Deliverables: "Final third"},
		},
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/sow", sowBody, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("create sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both parties accept SOW.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/sow/accept", nil, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("employer sow accept: %d %s", rr.Code, rr.Body.String())
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/sow/accept", nil, managerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("manager sow accept: %d %s", rr.Code, rr.Body.String())
	}

	// Verify AWAITING_PAYMENT and 3 milestones.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "AWAITING_PAYMENT" {
		t.Fatalf("expected AWAITING_PAYMENT, got %q", job.Status)
	}
	if len(job.Milestones) != 3 {
		t.Fatalf("expected 3 milestones, got %d", len(job.Milestones))
	}
	m1ID := job.Milestones[0].ID
	m2ID := job.Milestones[1].ID
	m3ID := job.Milestones[2].ID

	// Payment is collected upfront at initial checkout — one payment covers all milestones.
	couponCode := fmt.Sprintf("MS-%s", jobID[:6])
	if _, err := app.DB.Exec(
		`INSERT INTO coupons (code, value, max_uses, times_used) VALUES (?, '100%', 1, 0)`, couponCode,
	); err != nil {
		t.Fatalf("insert coupon: %v", err)
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/checkout",
		map[string]string{"coupon_code": couponCode}, employerTok)
	if rr.Code != http.StatusOK {
		t.Fatalf("checkout: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify IN_PROGRESS after checkout.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "IN_PROGRESS" {
		t.Fatalf("expected IN_PROGRESS after checkout, got %q", job.Status)
	}

	// Milestones are per-milestone payment gates. Each milestone:
	// employer pays → agent submits → employer approves → next milestone payment.
	submitAndApprove := func(mID string) {
		t.Helper()
		submitBody := SubmitProofRequest{
			ProofOfWorkURL:   "https://example.com/" + mID,
			ProofOfWorkNotes: "Done",
		}
		rr := doRequest(t, router, http.MethodPost,
			"/api/v1/jobs/"+jobID+"/milestones/"+mID+"/submit", submitBody, agentAPIKey)
		if rr.Code != http.StatusOK {
			t.Fatalf("submit %s: expected 200, got %d: %s", mID, rr.Code, rr.Body.String())
		}
		rr = doRequest(t, router, http.MethodPost,
			"/api/ui/jobs/"+jobID+"/milestones/"+mID+"/approve", nil, employerTok)
		if rr.Code != http.StatusOK {
			t.Fatalf("approve %s: expected 200, got %d: %s", mID, rr.Code, rr.Body.String())
		}
	}

	payForMilestone := func(n int) {
		t.Helper()
		code := fmt.Sprintf("MS%d-%s", n, jobID[:6])
		if _, err := app.DB.Exec(
			`INSERT INTO coupons (code, value, max_uses, times_used) VALUES (?, '100%', 1, 0)`, code,
		); err != nil {
			t.Fatalf("insert coupon m%d: %v", n, err)
		}
		rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+jobID+"/checkout",
			map[string]string{"coupon_code": code}, employerTok)
		if rr.Code != http.StatusOK {
			t.Fatalf("checkout m%d: expected 200, got %d: %s", n, rr.Code, rr.Body.String())
		}
	}

	// --- Milestone 1 ---
	submitAndApprove(m1ID)

	// After M1 approved, job must NOT be COMPLETED — moves to AWAITING_PAYMENT for M2.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status == "COMPLETED" {
		t.Errorf("BUG: job marked COMPLETED after only milestone 1 of 3 was approved")
	}
	if job.Status != "AWAITING_PAYMENT" {
		t.Errorf("expected AWAITING_PAYMENT after m1 approved (m2 needs payment), got %q", job.Status)
	}

	// --- Pay for and complete Milestone 2 ---
	payForMilestone(2)
	submitAndApprove(m2ID)

	// After M2 approved, job must NOT be COMPLETED — moves to AWAITING_PAYMENT for M3.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status == "COMPLETED" {
		t.Errorf("BUG: job marked COMPLETED after only milestone 2 of 3 was approved")
	}
	if job.Status != "AWAITING_PAYMENT" {
		t.Errorf("expected AWAITING_PAYMENT after m2 approved (m3 needs payment), got %q", job.Status)
	}

	// --- Pay for and complete Milestone 3 (last) ---
	payForMilestone(3)
	submitAndApprove(m3ID)

	// After M3 (last) approved, job MUST be COMPLETED.
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+jobID, nil, employerTok)
	json.Unmarshal(rr.Body.Bytes(), &job)
	if job.Status != "COMPLETED" {
		t.Errorf("expected COMPLETED after all 3 milestones approved, got %q", job.Status)
	}
}

// TestDeclineJobResetsSowAccepted verifies that when an agent declines a job via the
// agent API key path, both SoW accepted fields are reset to false. This ensures that
// if the employer re-offers the same job, the new offer starts with a clean acceptance
// state (issue #120).
func TestDeclineJobResetsSowAccepted(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")
	managerToken := makeAuthToken(t, app, managerID, "AGENT_MANAGER")

	// Employer creates a job offer.
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Decline SoW reset test", TotalPayout: 300, TimelineDays: 5},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Agent accepts the offer (moves to SOW_NEGOTIATION).
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/accept", nil, managerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("accept: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Employer updates the SoW and both parties accept it.
	sowBody := SOWRequest{
		DetailedSpec: "Build the thing",
		WorkProcess:  "Daily standups",
		PriceCents:   30000,
		TimelineDays: 5,
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow", sowBody, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("create sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow/accept", nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("employer accept sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify employer_accepted is now true before decline.
	var sowBeforeDecline SOW
	rr = doRequest(t, router, http.MethodGet, "/api/ui/jobs/"+job.ID+"/sow", nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("get sow before decline: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	json.Unmarshal(rr.Body.Bytes(), &sowBeforeDecline)
	if !sowBeforeDecline.EmployerAccepted {
		t.Fatal("expected employer_accepted to be true before decline")
	}

	// Agent declines the job (API key path).
	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/decline", nil, agentAPIKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("decline: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var declined Job
	json.Unmarshal(rr.Body.Bytes(), &declined)
	if declined.Status != "UNASSIGNED" {
		t.Fatalf("expected UNASSIGNED after decline, got %q", declined.Status)
	}

	// Verify both SoW accepted fields are reset.
	var agentAccepted, employerAccepted int
	err := app.DB.QueryRow(
		`SELECT agent_accepted, employer_accepted FROM sow WHERE job_id = ?`, job.ID,
	).Scan(&agentAccepted, &employerAccepted)
	if err != nil {
		t.Fatalf("query sow after decline: %v", err)
	}
	if agentAccepted != 0 {
		t.Errorf("expected agent_accepted = 0 after decline, got %d", agentAccepted)
	}
	if employerAccepted != 0 {
		t.Errorf("expected employer_accepted = 0 after decline, got %d", employerAccepted)
	}
}

// TestUIRejectJobResetsSowAccepted verifies that when an AGENT_MANAGER rejects a job
// via the UI endpoint, both SoW accepted fields are reset. This is the UI path
// counterpart to TestDeclineJobResetsSowAccepted (issue #120).
func TestUIRejectJobResetsSowAccepted(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, _ := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")
	managerToken := makeAuthToken(t, app, managerID, "AGENT_MANAGER")

	// Employer creates a job offer.
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "UI reject SoW reset test", TotalPayout: 400, TimelineDays: 7},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Agent accepts the offer (moves to SOW_NEGOTIATION).
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/accept", nil, managerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("accept: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Both parties accept the SoW.
	sowBody := SOWRequest{
		DetailedSpec: "Build it",
		WorkProcess:  "Async",
		PriceCents:   40000,
		TimelineDays: 7,
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow", sowBody, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("create sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow/accept", nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("employer accept sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Manager rejects via the UI endpoint.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/reject",
		map[string]string{"reason": "not a good fit"}, managerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("ui reject: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var rejected Job
	json.Unmarshal(rr.Body.Bytes(), &rejected)
	if rejected.Status != "UNASSIGNED" {
		t.Fatalf("expected UNASSIGNED after reject, got %q", rejected.Status)
	}

	// Both SoW accepted fields must be reset.
	var agentAccepted, employerAccepted int
	err := app.DB.QueryRow(
		`SELECT agent_accepted, employer_accepted FROM sow WHERE job_id = ?`, job.ID,
	).Scan(&agentAccepted, &employerAccepted)
	if err != nil {
		t.Fatalf("query sow after ui reject: %v", err)
	}
	if agentAccepted != 0 {
		t.Errorf("expected agent_accepted = 0 after ui reject, got %d", agentAccepted)
	}
	if employerAccepted != 0 {
		t.Errorf("expected employer_accepted = 0 after ui reject, got %d", employerAccepted)
	}
}

// TestAssignAgentResetsSowAccepted verifies that when an employer re-offers a job to a
// new agent via AssignAgentHandler, the SoW accepted fields from a previous negotiation
// are reset to false (issue #120).
func TestAssignAgentResetsSowAccepted(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")
	managerToken := makeAuthToken(t, app, managerID, "AGENT_MANAGER")

	// Create a second agent to re-offer to.
	managerID2, _ := createTestUser(t, app, "AGENT_MANAGER")
	agentID2, _ := createTestAgent(t, app, managerID2)

	// Employer creates a job with agent 1.
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire",
		HireRequest{AgentID: agentID, Title: "Re-offer SoW reset test", TotalPayout: 500, TimelineDays: 10},
		employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire: expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var job Job
	json.Unmarshal(rr.Body.Bytes(), &job)

	// Agent 1 accepts, SOW is negotiated, employer accepts SoW.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/accept", nil, managerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("accept: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	sowBody := SOWRequest{
		DetailedSpec: "Spec",
		WorkProcess:  "Process",
		PriceCents:   50000,
		TimelineDays: 10,
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow", sowBody, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("create sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/sow/accept", nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("employer accept sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Agent 1 declines (job resets to UNASSIGNED).
	rr = doRequest(t, router, http.MethodPost, "/api/v1/jobs/"+job.ID+"/decline", nil, agentAPIKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("decline: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Employer re-offers to agent 2 via AssignAgentHandler.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/jobs/"+job.ID+"/assign",
		AssignAgentRequest{AgentID: agentID2}, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("assign agent 2: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var assigned Job
	json.Unmarshal(rr.Body.Bytes(), &assigned)
	if assigned.Status != "PENDING_ACCEPTANCE" {
		t.Fatalf("expected PENDING_ACCEPTANCE after re-offer, got %q", assigned.Status)
	}

	// Both SoW accepted fields must be reset after re-offer.
	var agentAccepted, employerAccepted int
	err := app.DB.QueryRow(
		`SELECT agent_accepted, employer_accepted FROM sow WHERE job_id = ?`, job.ID,
	).Scan(&agentAccepted, &employerAccepted)
	if err != nil {
		t.Fatalf("query sow after re-offer: %v", err)
	}
	if agentAccepted != 0 {
		t.Errorf("expected agent_accepted = 0 after re-offer, got %d", agentAccepted)
	}
	if employerAccepted != 0 {
		t.Errorf("expected employer_accepted = 0 after re-offer, got %d", employerAccepted)
	}
}
