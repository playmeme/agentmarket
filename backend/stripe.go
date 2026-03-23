package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
)

// CreateCheckoutHandler creates a Stripe Checkout Session for a job (employer only).
// POST /api/ui/jobs/{job_id}/checkout
func (app *App) CreateCheckoutHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "create_checkout")

	role, _ := r.Context().Value(contextKeyUserRole).(string)
	if role != "EMPLOYER" {
		log.Warn("authz failure: checkout requires EMPLOYER role", "role", role)
		writeError(w, http.StatusForbidden, "only EMPLOYER role can initiate payment")
		return
	}

	employerID, _ := r.Context().Value(contextKeyUserID).(string)
	jobID := chi.URLParam(r, "job_id")

	// Verify job belongs to employer and is in AWAITING_PAYMENT
	var jobStatus string
	err := app.DB.QueryRow(
		"SELECT status FROM jobs WHERE id = ? AND employer_id = ?",
		jobID, employerID,
	).Scan(&jobStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		log.Error("checkout: db error fetching job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if jobStatus != "AWAITING_PAYMENT" {
		writeError(w, http.StatusBadRequest, "job must be in AWAITING_PAYMENT status to initiate checkout")
		return
	}

	// Load SOW for the price
	sow, err := app.getSOWByJobID(jobID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusBadRequest, "no SOW found for this job")
		return
	}
	if err != nil {
		log.Error("checkout: db error fetching sow", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if sow.PriceCents <= 0 {
		writeError(w, http.StatusBadRequest, "SOW price must be greater than zero")
		return
	}

	stripe.Key = app.Config.StripeSecretKey

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Job Payment: %s", jobID)),
					},
					UnitAmount: stripe.Int64(int64(sow.PriceCents)),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			CaptureMethod: stripe.String("manual"),
		},
		SuccessURL: stripe.String(fmt.Sprintf("%s/jobs/%s?payment=success", app.Config.BaseURL, jobID)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/jobs/%s?payment=cancelled", app.Config.BaseURL, jobID)),
	}

	cs, err := session.New(params)
	if err != nil {
		log.Error("checkout: stripe session creation failed", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create checkout session")
		return
	}

	// Save checkout session ID to job
	_, err = app.DB.Exec(
		`UPDATE jobs SET stripe_checkout_session_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		cs.ID, jobID,
	)
	if err != nil {
		log.Error("checkout: failed to save session id", "job_id", jobID, "session_id", cs.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	log.Info("checkout session created", "job_id", jobID, "session_id", cs.ID, "employer_id", employerID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"checkout_url": cs.URL,
		"session_id":   cs.ID,
	})
}

// HandleStripeWebhook handles incoming Stripe webhook events.
// POST /api/webhooks/stripe
func (app *App) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "stripe_webhook")

	const maxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error("webhook: failed to read body", "error", err)
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, sigHeader, app.Config.StripeWebhookSecret)
	if err != nil {
		log.Warn("webhook: signature verification failed", "error", err)
		writeError(w, http.StatusBadRequest, "invalid webhook signature")
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			log.Error("webhook: failed to parse checkout.session.completed", "error", err)
			writeError(w, http.StatusBadRequest, "failed to parse event data")
			return
		}

		paymentIntentID := ""
		if cs.PaymentIntent != nil {
			paymentIntentID = cs.PaymentIntent.ID
		}

		result, err := app.DB.Exec(
			`UPDATE jobs SET status = 'IN_PROGRESS', stripe_payment_intent = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE stripe_checkout_session_id = ? AND status = 'AWAITING_PAYMENT'`,
			paymentIntentID, cs.ID,
		)
		if err != nil {
			log.Error("webhook: failed to update job after checkout.session.completed", "session_id", cs.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		affected, _ := result.RowsAffected()
		log.Info("webhook: checkout.session.completed processed", "session_id", cs.ID, "payment_intent", paymentIntentID, "rows_affected", affected)

	default:
		log.Info("webhook: unhandled event type", "type", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}
