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

// TestMilestoneLevelCapture verifies the per-milestone Stripe capture behaviour
// introduced to fix the multi-milestone payment intent bug (issue #116).
//
// It tests two sub-cases:
//
//  1. When a milestone has a non-empty stripe_payment_intent, ApproveMilestoneHandler
//     attempts to capture it. With no real Stripe key configured the capture call
//     returns an error and the handler returns HTTP 500 — confirming the capture
//     attempt is made and is not silently skipped.
//
//  2. When a milestone has no stripe_payment_intent (e.g. paid via coupon or not
//     yet set by the webhook), ApproveMilestoneHandler succeeds (HTTP 200) and
//     marks the milestone PAID without crashing.
func TestMilestoneLevelCapture(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, managerID, agentID, agentAPIKey := setupJobFixtures(t, app)
	employerTok := makeAuthToken(t, app, employerID, "EMPLOYER")

	// --- Sub-case 1: milestone has a stripe_payment_intent —
	// ApproveMilestoneHandler should attempt capture and return 500 (no Stripe key).
	t.Run("attempts capture when intent present", func(t *testing.T) {
		jobID, m1ID, _ := setupSowWithMilestones(t, app, router, employerID, managerID, agentID, agentAPIKey)

		// Simulate the Stripe webhook: set job IN_PROGRESS and store a fake
		// payment intent on milestone 1.
		fakeIntentID := "pi_test_milestone_capture_123"
		if _, err := app.DB.Exec(
			`UPDATE jobs SET status = 'IN_PROGRESS', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, jobID,
		); err != nil {
			t.Fatalf("set job IN_PROGRESS: %v", err)
		}
		if _, err := app.DB.Exec(
			`UPDATE milestones SET stripe_payment_intent = ? WHERE id = ?`, fakeIntentID, m1ID,
		); err != nil {
			t.Fatalf("set milestone intent: %v", err)
		}

		// Agent submits proof of work to move milestone 1 → REVIEW_REQUESTED.
		submitBody := SubmitProofRequest{ProofOfWorkURL: "https://example.com/proof1", ProofOfWorkNotes: "Done"}
		rr := doRequest(t, router, http.MethodPost,
			"/api/v1/jobs/"+jobID+"/milestones/"+m1ID+"/submit", submitBody, agentAPIKey)
		if rr.Code != http.StatusOK {
			t.Fatalf("submit proof: expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		// Approve milestone 1. Because no real Stripe key is configured, the
		// capture call will fail — we expect a 500 response. This confirms that
		// ApproveMilestoneHandler actually attempted the capture (it did not
		// skip it when the intent was present).
		rr = doRequest(t, router, http.MethodPost,
			"/api/ui/jobs/"+jobID+"/milestones/"+m1ID+"/approve", nil, employerTok)
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 (Stripe capture attempted, no key configured), got %d: %s",
				rr.Code, rr.Body.String())
		}

		// The milestone must NOT have been marked PAID (capture failed, so the
		// DB transaction was never committed).
		var status string
		if err := app.DB.QueryRow(`SELECT status FROM milestones WHERE id = ?`, m1ID).Scan(&status); err != nil {
			t.Fatalf("query milestone status: %v", err)
		}
		if status == "PAID" {
			t.Error("milestone should not be PAID after failed Stripe capture")
		}
	})

	// --- Sub-case 2: milestone has NO stripe_payment_intent —
	// ApproveMilestoneHandler should skip capture and succeed (HTTP 200).
	t.Run("succeeds without intent (no capture needed)", func(t *testing.T) {
		// Build a fresh employer/agent setup so this sub-test is independent.
		emp2ID, mgr2ID, ag2ID, ag2APIKey := setupJobFixtures(t, app)
		emp2Tok := makeAuthToken(t, app, emp2ID, "EMPLOYER")

		jobID, m1ID, _ := setupSowWithMilestones(t, app, router, emp2ID, mgr2ID, ag2ID, ag2APIKey)

		// Simulate the webhook completing without storing an intent on the milestone
		// (e.g. coupon path, or webhook ran before the fix).
		if _, err := app.DB.Exec(
			`UPDATE jobs SET status = 'IN_PROGRESS', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, jobID,
		); err != nil {
			t.Fatalf("set job IN_PROGRESS: %v", err)
		}
		// Explicitly ensure stripe_payment_intent is empty (it is by default, but be explicit).
		if _, err := app.DB.Exec(
			`UPDATE milestones SET stripe_payment_intent = '' WHERE id = ?`, m1ID,
		); err != nil {
			t.Fatalf("clear milestone intent: %v", err)
		}

		// Agent submits proof → REVIEW_REQUESTED.
		submitBody := SubmitProofRequest{ProofOfWorkURL: "https://example.com/proof2", ProofOfWorkNotes: "Done"}
		rr := doRequest(t, router, http.MethodPost,
			"/api/v1/jobs/"+jobID+"/milestones/"+m1ID+"/submit", submitBody, ag2APIKey)
		if rr.Code != http.StatusOK {
			t.Fatalf("submit proof: expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		// Approve milestone 1 — no Stripe intent, so capture is skipped and the
		// handler must return 200.
		rr = doRequest(t, router, http.MethodPost,
			"/api/ui/jobs/"+jobID+"/milestones/"+m1ID+"/approve", nil, emp2Tok)
		if rr.Code != http.StatusOK {
			t.Fatalf("approve milestone (no intent): expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		// Milestone must be PAID.
		var status string
		if err := app.DB.QueryRow(`SELECT status FROM milestones WHERE id = ?`, m1ID).Scan(&status); err != nil {
			t.Fatalf("query milestone status: %v", err)
		}
		if status != "PAID" {
			t.Errorf("expected milestone status PAID, got %q", status)
		}

		// The response body should be the updated milestone.
		var m Milestone
		if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
			t.Fatalf("decode milestone response: %v", err)
		}
		if m.Status != "PAID" {
			t.Errorf("response milestone status: expected PAID, got %q", m.Status)
		}
	})
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
