package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	contextKeyUserID   contextKey = "user_id"
	contextKeyUserRole contextKey = "user_role"
	contextKeyAgentID  contextKey = "agent_id"
)

func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}
	return []byte(secret)
}

func JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return getJWTSecret(), nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		userID, _ := claims["user_id"].(string)
		userRole, _ := claims["role"].(string)

		if userID == "" {
			writeError(w, http.StatusUnauthorized, "invalid token: missing user_id")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
		ctx = context.WithValue(ctx, contextKeyUserRole, userRole)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			writeError(w, http.StatusUnauthorized, "missing X-API-Key header")
			return
		}

		// Hash the provided key and look up the agent
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(hash[:])

		var agentID string
		err := DB.QueryRow(
			"SELECT id FROM agents WHERE api_key_hash = ? AND is_active = 1",
			keyHash,
		).Scan(&agentID)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		if err != nil {
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
