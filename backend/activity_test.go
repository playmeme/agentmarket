package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// TestGetPublicActivity_EmptyFeed verifies that the endpoint returns an empty
// array (not null) when there are no qualifying jobs.
func TestGetPublicActivity_EmptyFeed(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	rr := doRequest(t, router, http.MethodGet, "/api/ui/activity", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var events []ActivityEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if events == nil {
		t.Error("expected non-nil slice, got nil")
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// TestGetPublicActivity_JobOffered verifies that a public UNASSIGNED job
// appears as a "job_offered" event.
func TestGetPublicActivity_JobOffered(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	// Create an employer and a public UNASSIGNED job.
	employerID, _ := createVerifiedTestUser(t, app, "EMPLOYER")
	_, err := app.DB.Exec(
		`INSERT INTO jobs (id, employer_id, status, title, description, total_payout, timeline_days, is_public)
		 VALUES (?, ?, 'UNASSIGNED', 'Write a Go API', 'desc', 100, 7, 1)`,
		"job-offered-1", employerID,
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/ui/activity", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var events []ActivityEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	e := events[0]
	if e.Kind != "job_offered" {
		t.Errorf("expected kind job_offered, got %q", e.Kind)
	}
	if e.JobTitle != "Write a Go API" {
		t.Errorf("expected title 'Write a Go API', got %q", e.JobTitle)
	}
	// Agent info must be absent for privacy.
	if e.AgentID != "" || e.AgentName != "" {
		t.Errorf("job_offered must not expose agent: agent_id=%q agent_name=%q", e.AgentID, e.AgentName)
	}
}

// TestGetPublicActivity_PrivateJobNotShown verifies that a non-public job does
// not appear in the feed regardless of status.
func TestGetPublicActivity_PrivateJobNotShown(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _ := createVerifiedTestUser(t, app, "EMPLOYER")
	// is_public = 0 (default)
	_, err := app.DB.Exec(
		`INSERT INTO jobs (id, employer_id, status, title, description, total_payout, timeline_days)
		 VALUES (?, ?, 'UNASSIGNED', 'Secret Project', 'desc', 100, 7)`,
		"job-private-1", employerID,
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/ui/activity", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var events []ActivityEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for private job, got %d: %+v", len(events), events)
	}
}

// TestGetPublicActivity_AgentHired verifies that a job assigned to an agent
// appears as an "agent_hired" event without exposing the job title.
func TestGetPublicActivity_AgentHired(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _ := createVerifiedTestUser(t, app, "EMPLOYER")
	managerID, _ := createVerifiedTestUser(t, app, "AGENT_MANAGER")
	agentID, _ := createTestAgent(t, app, managerID)

	_, err := app.DB.Exec(
		`INSERT INTO jobs (id, employer_id, agent_id, status, title, description, total_payout, timeline_days)
		 VALUES (?, ?, ?, 'IN_PROGRESS', 'Secret Job', 'desc', 200, 14)`,
		"job-hired-1", employerID, agentID,
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/ui/activity", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var events []ActivityEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	e := events[0]
	if e.Kind != "agent_hired" {
		t.Errorf("expected kind agent_hired, got %q", e.Kind)
	}
	if e.AgentID != agentID {
		t.Errorf("expected agent_id %q, got %q", agentID, e.AgentID)
	}
	if e.AgentName != "Test Agent" {
		t.Errorf("expected agent name 'Test Agent', got %q", e.AgentName)
	}
	// Job info must be absent for privacy.
	if e.JobID != "" || e.JobTitle != "" {
		t.Errorf("agent_hired must not expose job: job_id=%q job_title=%q", e.JobID, e.JobTitle)
	}
}

// TestGetPublicActivity_JobCompleted verifies that a public COMPLETED job
// with an agent appears as a "job_completed" event showing both.
func TestGetPublicActivity_JobCompleted(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _ := createVerifiedTestUser(t, app, "EMPLOYER")
	managerID, _ := createVerifiedTestUser(t, app, "AGENT_MANAGER")
	agentID, _ := createTestAgent(t, app, managerID)

	_, err := app.DB.Exec(
		`INSERT INTO jobs (id, employer_id, agent_id, status, title, description, total_payout, timeline_days, is_public)
		 VALUES (?, ?, ?, 'COMPLETED', 'Build a CLI Tool', 'desc', 500, 30, 1)`,
		"job-completed-1", employerID, agentID,
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/ui/activity", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var events []ActivityEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
	}
	e := events[0]
	if e.Kind != "job_completed" {
		t.Errorf("expected kind job_completed, got %q", e.Kind)
	}
	if e.JobTitle != "Build a CLI Tool" {
		t.Errorf("expected title 'Build a CLI Tool', got %q", e.JobTitle)
	}
	if e.AgentID != agentID {
		t.Errorf("expected agent_id %q, got %q", agentID, e.AgentID)
	}
	if e.AgentName != "Test Agent" {
		t.Errorf("expected agent name 'Test Agent', got %q", e.AgentName)
	}
}

// TestGetPublicActivity_SortOrder verifies that events are returned newest first.
func TestGetPublicActivity_SortOrder(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	employerID, _ := createVerifiedTestUser(t, app, "EMPLOYER")

	// Insert two public jobs with different timestamps.
	_, err := app.DB.Exec(
		`INSERT INTO jobs (id, employer_id, status, title, description, total_payout, timeline_days, is_public, created_at)
		 VALUES (?, ?, 'UNASSIGNED', 'Older Job', 'desc', 100, 7, 1, ?)`,
		"job-older", employerID, time.Now().Add(-2*time.Hour).UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		t.Fatalf("insert older job: %v", err)
	}
	_, err = app.DB.Exec(
		`INSERT INTO jobs (id, employer_id, status, title, description, total_payout, timeline_days, is_public, created_at)
		 VALUES (?, ?, 'UNASSIGNED', 'Newer Job', 'desc', 200, 14, 1, ?)`,
		"job-newer", employerID, time.Now().Add(-1*time.Hour).UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		t.Fatalf("insert newer job: %v", err)
	}

	rr := doRequest(t, router, http.MethodGet, "/api/ui/activity", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var events []ActivityEvent
	if err := json.Unmarshal(rr.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].JobTitle != "Newer Job" {
		t.Errorf("expected first event to be 'Newer Job', got %q", events[0].JobTitle)
	}
	if events[1].JobTitle != "Older Job" {
		t.Errorf("expected second event to be 'Older Job', got %q", events[1].JobTitle)
	}
}

// TestGetPublicActivity_NoAuthRequired verifies the endpoint works without a JWT.
func TestGetPublicActivity_NoAuthRequired(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	// No auth token — should still return 200.
	rr := doRequest(t, router, http.MethodGet, "/api/ui/activity", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 without auth, got %d", rr.Code)
	}
}
