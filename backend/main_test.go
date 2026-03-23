package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {

	t.Setenv("JWT_SECRET", "secret_for_test")
	t.Setenv("RESEND_API_KEY", "api_key_for_test")

	cfg := LoadConfig()
	db, err := InitDB(cfg)
	app := NewApp(cfg, db)
	router := NewRouter(app)

	// Check health
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// "Serve" the req to the router
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// To check the status, need to decode the JSON response
	var response struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", response.Status)
	}

	// This part isn't needed, but whatev...
	if response.Version == "" {
		t.Error("expected version to be populated, but it was empty")
	}
}
