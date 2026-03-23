package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	contextKeyUserID    contextKey = "user_id"
	contextKeyUserRole  contextKey = "user_role"
	contextKeyAgentID   contextKey = "agent_id"
	contextKeyRequestID contextKey = "request_id"
)

// RequestID generates a unique ID for each request and stores it in the context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			// fallback: empty string still works, just less useful
			next.ServeHTTP(w, r)
			return
		}
		id := hex.EncodeToString(b)
		ctx := context.WithValue(r.Context(), contextKeyRequestID, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestID(ctx context.Context) string {
	id, _ := ctx.Value(contextKeyRequestID).(string)
	return id
}

func (app *App) JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			slog.Warn("jwt auth failed: missing or invalid header",
				"request_id", requestID(r.Context()),
				"path", r.URL.Path,
			)
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return app.Config.JWTSecret, nil
		})

		if err != nil || !token.Valid {
			slog.Warn("jwt auth failed: invalid or expired token",
				"request_id", requestID(r.Context()),
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			slog.Warn("jwt auth failed: invalid claims",
				"request_id", requestID(r.Context()),
				"path", r.URL.Path,
			)
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		userID, _ := claims["user_id"].(string)
		userRole, _ := claims["role"].(string)

		if userID == "" {
			slog.Warn("jwt auth failed: missing user_id in claims",
				"request_id", requestID(r.Context()),
				"path", r.URL.Path,
			)
			writeError(w, http.StatusUnauthorized, "invalid token: missing user_id")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
		ctx = context.WithValue(ctx, contextKeyUserRole, userRole)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *App) APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			slog.Warn("api key auth failed: missing or invalid header",
				"request_id", requestID(r.Context()),
				"path", r.URL.Path,
			)
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		apiKey = strings.TrimSpace(apiKey)

		// Hash the provided key and look up the agent
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(hash[:])

		var agentID string
		err := app.DB.QueryRow(
			"SELECT id FROM agents WHERE api_key_hash = ? AND is_active = 1",
			keyHash,
		).Scan(&agentID)

		if err == sql.ErrNoRows {
			slog.Warn("api key auth failed: key not found or inactive",
				"request_id", requestID(r.Context()),
				"path", r.URL.Path,
			)
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		if err != nil {
			slog.Error("api key auth: database error",
				"request_id", requestID(r.Context()),
				"path", r.URL.Path,
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyAgentID, agentID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + escapeJSON(message) + `"}`))
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
