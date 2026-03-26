package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSignup(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	body := SignupRequest{
		Name:     "Alice",
		Handle:   "alice",
		Email:    "alice@example.com",
		Password: "Password1!",
		Role:     "EMPLOYER",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/signup", body, "")

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Email != body.Email {
		t.Errorf("expected email %q, got %q", body.Email, resp.Email)
	}
	if resp.Role != body.Role {
		t.Errorf("expected role %q, got %q", body.Role, resp.Role)
	}

	// Check for jwtCookie
	cookies := rr.Result().Cookies()
	var jwtCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "jwt" {
			jwtCookie = c
			break
		}
	}
	if jwtCookie == nil {
		t.Fatal("expected 'jwt' cookie to be set in response")
	}
	if jwtCookie.Value == "" {
		t.Error("expected 'jwt' cookie to have a value")
	}
	if !jwtCookie.HttpOnly {
		t.Error("expected 'jwt' cookie to be HttpOnly to prevent XSS")
	}

	// Confirm user exists in DB
	var count int
	err := app.DB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", body.Email).Scan(&count)
	if err != nil || count != 1 {
		t.Errorf("expected 1 user in DB, got %d (err=%v)", count, err)
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	body := SignupRequest{
		Name: "Bob", Handle: "bob", Email: "bob@example.com",
		Password: "Password1!", Role: "EMPLOYER",
	}
	// First signup should succeed.
	rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/signup", body, "")
	if rr.Code != http.StatusCreated {
		t.Fatalf("first signup: expected 201, got %d", rr.Code)
	}

	// Second signup with same email must fail.
	body.Handle = "bob2" // different handle to isolate email conflict
	rr = doRequest(t, router, http.MethodPost, "/api/ui/auth/signup", body, "")
	if rr.Code != http.StatusConflict {
		t.Errorf("duplicate email: expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSignupMissingFields(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	tests := []struct {
		name string
		body SignupRequest
	}{
		{"missing email", SignupRequest{Name: "X", Handle: "x", Password: "pass", Role: "EMPLOYER"}},
		{"missing password", SignupRequest{Name: "X", Handle: "x", Email: "x@x.com", Role: "EMPLOYER"}},
		{"invalid role", SignupRequest{Name: "X", Handle: "x", Email: "x@x.com", Password: "pass", Role: "ADMIN"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/signup", tc.body, "")
			if rr.Code == http.StatusCreated {
				t.Errorf("%s: expected error status, got 201", tc.name)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	// Create a user via the signup endpoint to get a real bcrypt hash.
	signupBody := SignupRequest{
		Name: "Carol", Handle: "carol", Email: "carol@example.com",
		Password: "Password1!", Role: "AGENT_MANAGER",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/signup", signupBody, "")
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d", rr.Code)
	}

	loginBody := LoginRequest{Email: "carol@example.com", Password: "Password1!"}
	rr = doRequest(t, router, http.MethodPost, "/api/ui/auth/login", loginBody, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	// Check for jwtCookie
	cookies := rr.Result().Cookies()
	var jwtCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "jwt" {
			jwtCookie = c
			break
		}
	}
	if jwtCookie == nil {
		t.Fatal("expected 'jwt' cookie to be set in response")
	}
	if jwtCookie.Value == "" {
		t.Error("expected 'jwt' cookie to have a value")
	}
	if !jwtCookie.HttpOnly {
		t.Error("expected 'jwt' cookie to be HttpOnly to prevent XSS")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	signupBody := SignupRequest{
		Name: "Dave", Handle: "dave", Email: "dave@example.com",
		Password: "Password1!", Role: "EMPLOYER",
	}
	doRequest(t, router, http.MethodPost, "/api/ui/auth/signup", signupBody, "")

	rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/login",
		LoginRequest{Email: "dave@example.com", Password: "WrongPass!"}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: expected 401, got %d", rr.Code)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/login",
		LoginRequest{Email: "nobody@example.com", Password: "pass"}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("unknown email: expected 401, got %d", rr.Code)
	}
}

func TestVerifyEmail(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	// Signup to get a user + token.
	signupBody := SignupRequest{
		Name: "Eve", Handle: "eve", Email: "eve@example.com",
		Password: "Password1!", Role: "EMPLOYER",
	}
	rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/signup", signupBody, "")
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup: expected 201, got %d", rr.Code)
	}
	var signupResp AuthResponse
	json.Unmarshal(rr.Body.Bytes(), &signupResp)


	// Extract the token from the Set-Cookie header instead of the JSON
	var tokenStr string
	for _, c := range rr.Result().Cookies() {
		if c.Name == "jwt" {
			tokenStr = c.Value
			break
		}
	}
	if tokenStr == "" {
		t.Fatal("expected jwt cookie from signup to use for verification")
	}

	// Before verification, email_verified_at should be NULL.
	var evBefore *string
	app.DB.QueryRow("SELECT email_verified_at FROM users WHERE id = ?", signupResp.ID).Scan(&evBefore)
	if evBefore != nil {
		t.Error("email should not be verified before calling verify-email")
	}

	// Call verify-email with the JWT.
	rr = doRequest(t, router, http.MethodPost, "/api/ui/auth/verify-email",
		VerifyEmailRequest{Token: tokenStr}, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("verify-email: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Now email_verified_at should be set.
	var evAfter *string
	app.DB.QueryRow("SELECT email_verified_at FROM users WHERE id = ?", signupResp.ID).Scan(&evAfter)
	if evAfter == nil {
		t.Error("email should be verified after calling verify-email")
	}
}

func TestVerifyEmailInvalidToken(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)
	router := NewRouter(app)

	rr := doRequest(t, router, http.MethodPost, "/api/ui/auth/verify-email",
		VerifyEmailRequest{Token: "this.is.not.valid"}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("invalid token: expected 401, got %d", rr.Code)
	}
}
