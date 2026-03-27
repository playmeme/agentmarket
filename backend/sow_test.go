package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// setupSOWNegotiationFixture creates an employer + manager + agent + job in SOW_NEGOTIATION,
// with an existing SoW record, and returns the relevant IDs and tokens.
func setupSOWNegotiationFixture(t *testing.T, app *App) (jobID, employerID, managerID string, employerToken, managerToken string) {
	t.Helper()
	router := NewRouter(app)

	employerID, _ = createVerifiedTestUser(t, app, "EMPLOYER")
	managerID, _ = createTestUser(t, app, "AGENT_MANAGER")
	agentID, agentAPIKey := createTestAgent(t, app, managerID)
	employerToken = makeAuthToken(t, app, employerID, "EMPLOYER")
	managerToken = makeAuthToken(t, app, managerID, "AGENT_MANAGER")

	// Hire the agent
	hireBody := HireRequest{
		AgentID:      agentID,
		Title:        "Test Job",
		Description:  "Test Description",
		TotalPayout:  10000,
		TimelineDays: 7,
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/jobs/hire", hireBody, employerToken)
	if rr.Code != http.StatusCreated {
		t.Fatalf("hire failed: %d %s", rr.Code, rr.Body.String())
	}
	var job Job
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	jobID = job.ID

	// Agent accepts to move to SOW_NEGOTIATION
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/v1/jobs/%s/accept", jobID), nil, agentAPIKey)
	if rr.Code != http.StatusOK {
		t.Fatalf("agent accept failed: %d %s", rr.Code, rr.Body.String())
	}

	// Create a SoW so the lock endpoints have something to operate on
	sowBody := SOWRequest{
		DetailedSpec: "Do the thing",
		WorkProcess:  "Weekly updates",
		PriceCents:   10000,
		TimelineDays: 7,
	}
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow", jobID), sowBody, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("create sow failed: %d %s", rr.Code, rr.Body.String())
	}

	return jobID, employerID, managerID, employerToken, managerToken
}

// TestSOWLock_BasicAcquireAndRelease verifies that an employer can lock, heartbeat,
// and unlock a SoW without errors.
func TestSOWLock_BasicAcquireAndRelease(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	jobID, _, _, employerToken, _ := setupSOWNegotiationFixture(t, app)

	// Lock
	rr := doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("lock: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var lockResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &lockResp); err != nil {
		t.Fatalf("decode lock response: %v", err)
	}
	if lockResp["allowed"] != true {
		t.Errorf("expected allowed=true, got %v", lockResp["allowed"])
	}

	// Heartbeat
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/heartbeat", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("heartbeat: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Unlock
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/unlock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("unlock: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestSOWLock_ConflictWhenOtherUserHoldsLock verifies that attempting to lock a SoW
// while another user holds a fresh lock returns 409.
func TestSOWLock_ConflictWhenOtherUserHoldsLock(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	jobID, _, _, employerToken, managerToken := setupSOWNegotiationFixture(t, app)

	// Employer acquires lock
	rr := doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("employer lock: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Manager tries to acquire lock → should get 409
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, managerToken)
	if rr.Code != http.StatusConflict {
		t.Fatalf("manager lock: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var conflictResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &conflictResp); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if conflictResp["allowed"] != false {
		t.Errorf("expected allowed=false, got %v", conflictResp["allowed"])
	}
	if conflictResp["editing_by"] == "" || conflictResp["editing_by"] == nil {
		t.Errorf("expected editing_by to be populated, got %v", conflictResp["editing_by"])
	}
}

// TestSOWLock_ReentrantLock verifies that the same user can acquire the lock twice
// (idempotent re-entry — no conflict).
func TestSOWLock_ReentrantLock(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	jobID, _, _, employerToken, _ := setupSOWNegotiationFixture(t, app)

	// First lock
	rr := doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("first lock: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Second lock from same user — should be allowed
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("reentrant lock: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var lockResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &lockResp); err != nil {
		t.Fatalf("decode lock response: %v", err)
	}
	if lockResp["allowed"] != true {
		t.Errorf("expected allowed=true for reentrant lock, got %v", lockResp["allowed"])
	}
}

// TestSOWLock_HeartbeatFailsWithoutLock verifies that heartbeat returns 409
// when the caller does not hold the lock.
func TestSOWLock_HeartbeatFailsWithoutLock(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	jobID, _, _, employerToken, managerToken := setupSOWNegotiationFixture(t, app)

	// Employer acquires the lock
	rr := doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("lock: expected 200, got %d", rr.Code)
	}

	// Manager tries to heartbeat — they don't hold the lock → should 409
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/heartbeat", jobID), nil, managerToken)
	if rr.Code != http.StatusConflict {
		t.Fatalf("manager heartbeat: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestSOWLock_UnlockOnlyClearsOwnLock verifies that unlock does NOT clear another
// user's lock (i.e. the conditional WHERE editing_id = ? is enforced).
func TestSOWLock_UnlockOnlyClearsOwnLock(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	jobID, _, _, employerToken, managerToken := setupSOWNegotiationFixture(t, app)

	// Employer acquires lock
	rr := doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("lock: expected 200, got %d", rr.Code)
	}

	// Manager tries to unlock employer's lock → should silently succeed (no error) but NOT clear the lock
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/unlock", jobID), nil, managerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("manager unlock: expected 200 (silent no-op), got %d: %s", rr.Code, rr.Body.String())
	}

	// Employer's lock should still be active — manager should still get 409
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, managerToken)
	if rr.Code != http.StatusConflict {
		t.Fatalf("manager lock after failed unlock: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestSOWSave_ClearsLock verifies that saving a SoW (CreateOrUpdateSOW) clears
// the edit lock so another user can immediately take over.
func TestSOWSave_ClearsLock(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	jobID, _, _, employerToken, managerToken := setupSOWNegotiationFixture(t, app)

	// Employer acquires lock
	rr := doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("lock: expected 200, got %d", rr.Code)
	}

	// Employer saves the SoW (should clear the lock)
	sowBody := SOWRequest{
		DetailedSpec: "Updated spec",
		WorkProcess:  "Updated process",
		PriceCents:   15000,
		TimelineDays: 10,
	}
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow", jobID), sowBody, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("save sow: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Manager should now be able to acquire the lock (employer's lock was cleared on save)
	rr = doRequest(t, router, http.MethodPost, fmt.Sprintf("/api/ui/jobs/%s/sow/lock", jobID), nil, managerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("manager lock after employer save: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var lockResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &lockResp); err != nil {
		t.Fatalf("decode lock response: %v", err)
	}
	if lockResp["allowed"] != true {
		t.Errorf("expected allowed=true, got %v", lockResp["allowed"])
	}
}
