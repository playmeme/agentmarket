package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// setupJobFixtures creates a verified employer, an agent handler, an agent, and returns them.
func setupJobFixtures(t *testing.T, app *App) (employerID, handlerID, agentID, agentAPIKey string) {
	t.Helper()
	employerID, _ = createVerifiedTestUser(t, app, "EMPLOYER")
	handlerID, _ = createTestUser(t, app, "AGENT_HANDLER")
	agentID, agentAPIKey = createTestAgent(t, app, handlerID)
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
		Milestones: []MilestoneInput{
			{Title: "Design", Amount: 2000, Criteria: []string{"wireframes done"}},
			{Title: "Build", Amount: 3000, Criteria: []string{"code merged", "tests pass"}},
		},
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
	if len(job.Milestones) != 2 {
		t.Errorf("expected 2 milestones, got %d", len(job.Milestones))
	}
	// Verify criteria were persisted.
	if len(job.Milestones[0].Criteria) != 1 {
		t.Errorf("milestone 0: expected 1 criterion, got %d", len(job.Milestones[0].Criteria))
	}
	if len(job.Milestones[1].Criteria) != 2 {
		t.Errorf("milestone 1: expected 2 criteria, got %d", len(job.Milestones[1].Criteria))
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

	handlerID, _ := createTestUser(t, app, "AGENT_HANDLER")
	token := makeAuthToken(t, app, handlerID, "AGENT_HANDLER")
	_, _, agentID, _ := setupJobFixtures(t, app)

	body := HireRequest{AgentID: agentID, Title: "Work", TotalPayout: 100, TimelineDays: 1}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", body, token)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for AGENT_HANDLER role, got %d", rr.Code)
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
	if updated.Status != "IN_PROGRESS" {
		t.Errorf("expected status IN_PROGRESS, got %q", updated.Status)
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
