package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type SignupRequest struct {
	Name     string `json:"name"`
	Handle   string `json:"handle"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	ID     string `json:"id"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
	Email  string `json:"email"`
}

func (app *App) generateJWT(userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(app.Config.JWTSecret)
}

func (app *App) SignupHandler(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Handle == "" || req.Email == "" || req.Password == "" || req.Role == "" {
		writeError(w, http.StatusBadRequest, "name, handle, email, password, and role are required")
		return
	}

	if req.Role != "EMPLOYER" && req.Role != "AGENT_HANDLER" {
		writeError(w, http.StatusBadRequest, "role must be EMPLOYER or AGENT_HANDLER")
		return
	}

	// Check for duplicate email before attempting insert, to give a clear error message.
	var existingID string
	emailErr := app.DB.QueryRow("SELECT id FROM users WHERE email = ?", req.Email).Scan(&existingID)
	if emailErr == nil {
		writeError(w, http.StatusConflict, "An account with this email already exists")
		return
	} else if emailErr != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	id := uuid.New().String()
	_, err = app.DB.Exec(
		`INSERT INTO users (id, role, name, handle, email, password_hash) VALUES (?, ?, ?, ?, ?, ?)`,
		id, req.Role, req.Name, req.Handle, req.Email, string(hash),
	)
	if err != nil {
		writeError(w, http.StatusConflict, "handle already exists")
		return
	}

	token, err := app.generateJWT(id, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Send verification email (best-effort; do not fail signup if email fails)
	verifyLink := "https://agentictemp.com/auth/verify-email?token=" + token
	htmlBody := "<p>Welcome to AgentMarket! Please verify your email by clicking the link below:</p>" +
		"<p><a href=\"" + verifyLink + "\">Verify Email</a></p>"
	_ = SendEmail(app.Config.ResendAPIKey, req.Email, "Verify your AgentMarket email", htmlBody)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(AuthResponse{
		Token:  token,
		UserID: id,
		ID:     id,
		Role:   req.Role,
		Name:   req.Name,
		Handle: req.Handle,
		Email:  req.Email,
	})
}

func (app *App) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var id, role, passwordHash, name, handle, email string
	err := app.DB.QueryRow(
		"SELECT id, role, password_hash, name, handle, email FROM users WHERE email = ?",
		req.Email,
	).Scan(&id, &role, &passwordHash, &name, &handle, &email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := app.generateJWT(id, role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token:  token,
		UserID: id,
		ID:     id,
		Role:   role,
		Name:   name,
		Handle: handle,
		Email:  email,
	})
}

func (app *App) VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	// Extract JWT from Authorization header to identify the user
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	// Parse the token to get user ID
	token, err := jwt.Parse(req.Token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return app.Config.JWTSecret, nil
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
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "invalid token: missing user_id")
		return
	}

	_, err = app.DB.Exec(
		"UPDATE users SET email_verified_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		userID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify email")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "email verified"})
}

func (app *App) generatePasswordResetJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"purpose": "password_reset",
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(app.Config.JWTSecret)
}

func (app *App) ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	var userID string
	err := app.DB.QueryRow("SELECT id FROM users WHERE email = ?", req.Email).Scan(&userID)
	// Always return success to avoid leaking which emails are registered
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "if that email exists, a reset link has been sent"})
		return
	}

	resetToken, err := app.generatePasswordResetJWT(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate reset token")
		return
	}

	resetLink := "https://agentictemp.com/auth/reset-password?token=" + resetToken
	htmlBody := "<p>You requested a password reset for your AgentMarket account.</p>" +
		"<p><a href=\"" + resetLink + "\">Reset Password</a></p>" +
		"<p>This link expires in 1 hour. If you did not request this, you can ignore this email.</p>"
	_ = SendEmail(app.Config.ResendAPIKey, req.Email, "Reset your AgentMarket password", htmlBody)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "if that email exists, a reset link has been sent"})
}

func (app *App) ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "token and new_password are required")
		return
	}

	token, err := jwt.Parse(req.Token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return app.Config.JWTSecret, nil
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

	purpose, _ := claims["purpose"].(string)
	if purpose != "password_reset" {
		writeError(w, http.StatusUnauthorized, "invalid token purpose")
		return
	}

	userID, _ := claims["user_id"].(string)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "invalid token: missing user_id")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	_, err = app.DB.Exec(
		"UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		string(hash), userID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "password updated"})
}
