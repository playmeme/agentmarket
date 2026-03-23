package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestListAgents_Empty(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	userID, _ := createTestUser(t, app, "EMPLOYER")
	token := makeAuthToken(t, app, userID, "EMPLOYER")

	rr := doRequest(t, router, http.MethodGet, "/api/ui/agents/", nil, token)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var agents []Agent
	if err := json.Unmarshal(rr.Body.Bytes(), &agents); err != nil {
		t.Fatalf("failed to decode agents: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestListAgents_Unauthenticated(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	// Anonymous users should be able to list agents without a token.
	rr := doRequest(t, router, http.MethodGet, "/api/ui/agents/", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for unauthenticated request, got %d: %s", rr.Code, rr.Body.String())
	}

	var agents []Agent
	if err := json.Unmarshal(rr.Body.Bytes(), &agents); err != nil {
		t.Fatalf("failed to decode agents: %v", err)
	}
}

func TestCreateAgent(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	handlerID, _ := createTestUser(t, app, "AGENT_HANDLER")
	token := makeAuthToken(t, app, handlerID, "AGENT_HANDLER")

	body := CreateAgentRequest{
		Name:        "My Agent",
		Description: "Does stuff",
		WebhookURL:  "https://hooks.example.com/agent",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/handlers/agents", body, token)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp CreateAgentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Agent.ID == "" {
		t.Error("expected agent ID in response")
	}
	if resp.APIKey == "" {
		t.Error("expected API key in response")
	}
	if resp.Agent.Name != body.Name {
		t.Errorf("expected name %q, got %q", body.Name, resp.Agent.Name)
	}
	if resp.Agent.HandlerID != handlerID {
		t.Errorf("expected handler_id %q, got %q", handlerID, resp.Agent.HandlerID)
	}
}

func TestCreateAgentUnauthenticated(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	body := CreateAgentRequest{Name: "Sneaky Agent"}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/handlers/agents", body, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCreateAgentWrongRole(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	// EMPLOYER role should not be allowed to create agents.
	employerID, _ := createTestUser(t, app, "EMPLOYER")
	token := makeAuthToken(t, app, employerID, "EMPLOYER")

	body := CreateAgentRequest{Name: "Nope Agent"}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/handlers/agents", body, token)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestListAgentsAfterCreate(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	handlerID, _ := createTestUser(t, app, "AGENT_HANDLER")
	handlerToken := makeAuthToken(t, app, handlerID, "AGENT_HANDLER")

	// Create two agents.
	for i := 0; i < 2; i++ {
		rr := doRequest(t, router, http.MethodPost, "/api/ui/handlers/agents",
			CreateAgentRequest{Name: "Agent"}, handlerToken)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create agent %d: expected 201, got %d", i, rr.Code)
		}
	}

	// List agents as an EMPLOYER — should see all active agents.
	employerID, _ := createTestUser(t, app, "EMPLOYER")
	employerToken := makeAuthToken(t, app, employerID, "EMPLOYER")
	rr := doRequest(t, router, http.MethodGet, "/api/ui/agents/", nil, employerToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var agents []Agent
	json.Unmarshal(rr.Body.Bytes(), &agents)
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestGetAgent(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	handlerID, _ := createTestUser(t, app, "AGENT_HANDLER")
	agentID, _ := createTestAgent(t, app, handlerID)

	// Any authenticated user can get an agent by ID.
	userID, _ := createTestUser(t, app, "EMPLOYER")
	token := makeAuthToken(t, app, userID, "EMPLOYER")

	rr := doRequest(t, router, http.MethodGet, "/api/ui/agents/"+agentID, nil, token)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var a Agent
	json.Unmarshal(rr.Body.Bytes(), &a)
	if a.ID != agentID {
		t.Errorf("expected agent ID %q, got %q", agentID, a.ID)
	}
}

func TestGetAgent_Unauthenticated(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	handlerID, _ := createTestUser(t, app, "AGENT_HANDLER")
	agentID, _ := createTestAgent(t, app, handlerID)

	// Anonymous users should also be able to fetch a single agent.
	rr := doRequest(t, router, http.MethodGet, "/api/ui/agents/"+agentID, nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for unauthenticated request, got %d: %s", rr.Code, rr.Body.String())
	}

	var a Agent
	json.Unmarshal(rr.Body.Bytes(), &a)
	if a.ID != agentID {
		t.Errorf("expected agent ID %q, got %q", agentID, a.ID)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	userID, _ := createTestUser(t, app, "EMPLOYER")
	token := makeAuthToken(t, app, userID, "EMPLOYER")

	rr := doRequest(t, router, http.MethodGet, "/api/ui/agents/nonexistent-id", nil, token)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
