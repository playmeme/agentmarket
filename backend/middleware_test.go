package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// sentinelHandler writes 200 "OK" so we can confirm middleware passed through.
var sentinelHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

func TestJWTAuthValid(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	userID, _ := createTestUser(t, app, "EMPLOYER")
	token := makeAuthToken(t, app, userID, "EMPLOYER")

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: token,
	})
	rr := httptest.NewRecorder()

	app.JWTAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("valid JWT: expected 200, got %d", rr.Code)
	}
}

func TestJWTAuthMissingHeader(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.JWTAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("missing header: expected 401, got %d", rr.Code)
	}
}

func TestJWTAuthInvalidToken(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: "not.a.valid.jwt",
	})
	rr := httptest.NewRecorder()
	app.JWTAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("invalid token: expected 401, got %d", rr.Code)
	}
}

func TestJWTAuthExpiredToken(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	claims := jwt.MapClaims{
		"user_id": "some-user",
		"role":    "EMPLOYER",
		"exp":     time.Now().Add(-1 * time.Hour).Unix(), // already expired
		"iat":     time.Now().Add(-2 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(app.Config.JWTSecret)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: signed,
	})
	rr := httptest.NewRecorder()
	app.JWTAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expired token: expected 401, got %d", rr.Code)
	}
}

func TestJWTAuthWrongSigningKey(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	claims := jwt.MapClaims{
		"user_id": "some-user",
		"role":    "EMPLOYER",
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString([]byte("wrong-secret"))

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  "jwt",
		Value: signed,
	})
	rr := httptest.NewRecorder()
	app.JWTAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("wrong key: expected 401, got %d", rr.Code)
	}
}

func TestAPIKeyAuthValid(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	managerID, _ := createTestUser(t, app, "AGENT_MANAGER")
	_, plainKey := createTestAgent(t, app, managerID)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+plainKey)
	rr := httptest.NewRecorder()

	app.APIKeyAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("valid API key: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAPIKeyAuthInvalid(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalidkeyvalue")
	rr := httptest.NewRecorder()
	app.APIKeyAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("invalid API key: expected 401, got %d", rr.Code)
	}
}

func TestAPIKeyAuthMissingHeader(t *testing.T) {
	t.Parallel()
	app := setupTestApp(t)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	app.APIKeyAuth(sentinelHandler).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("missing header: expected 401, got %d", rr.Code)
	}
}
