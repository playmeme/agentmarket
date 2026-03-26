package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Notification types
const (
	// Agent notifications
	NotifJobOffer               = "JOB_OFFER"
	NotifMilestoneCompleted     = "MILESTONE_COMPLETED"
	NotifPaymentReceived        = "PAYMENT_RECEIVED"
	NotifMilestoneDeadline      = "MILESTONE_DEADLINE"

	// Employer notifications
	NotifJobOfferAccepted       = "JOB_OFFER_ACCEPTED"
	NotifJobOfferRejected       = "JOB_OFFER_REJECTED"
	NotifMilestoneDelivered     = "MILESTONE_DELIVERED"
	NotifPaymentDue             = "PAYMENT_DUE"
	NotifPaymentOverdue         = "PAYMENT_OVERDUE"

	// Both parties
	NotifMilestoneConfirmed     = "MILESTONE_CONFIRMED"
)

// --- Model ---

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	JobID     string    `json:"job_id,omitempty"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	Dismissed bool      `json:"dismissed"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Service ---

// CreateNotification inserts a notification record and sends an email to the user.
func (app *App) CreateNotification(userID, jobID, notifType, title, message string) error {
	id := uuid.New().String()

	_, err := app.DB.Exec(
		`INSERT INTO notifications (id, user_id, job_id, type, title, message) VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, jobID, notifType, title, message,
	)
	if err != nil {
		slog.Error("failed to insert notification", "user_id", userID, "type", notifType, "error", err)
		return err
	}

	slog.Info("notification created", "id", id, "user_id", userID, "type", notifType)

	// Send email — best effort, do not fail if email fails
	var email, role string
	if err := app.DB.QueryRow("SELECT email, role FROM users WHERE id = ?", userID).Scan(&email, &role); err != nil {
		slog.Warn("notification: could not fetch user email", "user_id", userID, "error", err)
		return nil
	}

	dashboardPath := "/dashboard/employer"
	if role == "AGENT_HANDLER" {
		dashboardPath = "/dashboard/handler"
	}

	htmlBody := "<h2>" + title + "</h2><p>" + message + "</p>" +
		"<p><a href=\"" + app.Config.BaseURL + dashboardPath + "\">View on AgentMarket</a></p>"

	if err := SendEmail(app.Config.ResendAPIKey, email, title, htmlBody); err != nil {
		slog.Warn("notification email failed", "user_id", userID, "type", notifType, "error", err)
	}

	return nil
}

// GetNotifications returns non-dismissed notifications for a user.
func (app *App) GetNotifications(userID string) ([]Notification, error) {
	rows, err := app.DB.Query(
		`SELECT id, user_id, job_id, type, title, message, read, dismissed, created_at
		 FROM notifications
		 WHERE user_id = ? AND dismissed = 0
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		var read, dismissed int
		var jobID sql.NullString
		if err := rows.Scan(&n.ID, &n.UserID, &jobID, &n.Type, &n.Title, &n.Message,
			&read, &dismissed, sqliteTime{&n.CreatedAt}); err != nil {
			return nil, err
		}
		if jobID.Valid {
			n.JobID = jobID.String
		}
		n.Read = read == 1
		n.Dismissed = dismissed == 1
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []Notification{}
	}
	return notifications, nil
}

// GetUnreadCount returns the count of unread, non-dismissed notifications for a user.
func (app *App) GetUnreadCount(userID string) (int, error) {
	var count int
	err := app.DB.QueryRow(
		`SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read = 0 AND dismissed = 0`,
		userID,
	).Scan(&count)
	return count, err
}

// DismissNotification marks a notification as dismissed (and read) for the owning user.
func (app *App) DismissNotification(notifID, userID string) error {
	result, err := app.DB.Exec(
		`UPDATE notifications SET dismissed = 1, read = 1, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND user_id = ?`,
		notifID, userID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// markNotificationsRead marks all non-dismissed notifications for a user as read.
func (app *App) markNotificationsRead(userID string) error {
	_, err := app.DB.Exec(
		`UPDATE notifications SET read = 1, updated_at = CURRENT_TIMESTAMP
		 WHERE user_id = ? AND read = 0 AND dismissed = 0`,
		userID,
	)
	return err
}

// --- Handlers ---

// GetNotificationsHandler returns all non-dismissed notifications for the logged-in user.
// GET /api/ui/notifications
func (app *App) GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "get_notifications")

	userID, _ := r.Context().Value(contextKeyUserID).(string)

	notifications, err := app.GetNotifications(userID)
	if err != nil {
		log.Error("get notifications: database error", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Mark all as read when the user fetches them
	if err := app.markNotificationsRead(userID); err != nil {
		log.Warn("get notifications: failed to mark as read", "user_id", userID, "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifications)
}

// GetNotificationCountHandler returns the unread notification count for the badge.
// GET /api/ui/notifications/count
func (app *App) GetNotificationCountHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "get_notification_count")

	userID, _ := r.Context().Value(contextKeyUserID).(string)

	count, err := app.GetUnreadCount(userID)
	if err != nil {
		log.Error("get notification count: database error", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

// DismissNotificationHandler marks a notification as dismissed.
// POST /api/ui/notifications/{id}/dismiss
func (app *App) DismissNotificationHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "dismiss_notification")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	notifID := chi.URLParam(r, "id")

	if err := app.DismissNotification(notifID, userID); err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	} else if err != nil {
		log.Error("dismiss notification: database error", "notif_id", notifID, "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	log.Info("notification dismissed", "notif_id", notifID, "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "dismissed"})
}

// GetAgentNotificationsHandler returns notifications for an agent (API key auth).
// GET /api/v1/notifications
func (app *App) GetAgentNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "get_agent_notifications")

	agentID, _ := r.Context().Value(contextKeyAgentID).(string)

	// Look up the handler user ID for this agent
	var handlerID string
	if err := app.DB.QueryRow("SELECT handler_id FROM agents WHERE id = ?", agentID).Scan(&handlerID); err != nil {
		log.Error("get agent notifications: db error fetching handler", "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	notifications, err := app.GetNotifications(handlerID)
	if err != nil {
		log.Error("get agent notifications: database error", "handler_id", handlerID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifications)
}
