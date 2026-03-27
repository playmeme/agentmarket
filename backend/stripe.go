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
// Optional body: { "coupon_code": "M1PAID" }
// If the coupon covers the full amount, Stripe is skipped and the job is moved to
// IN_PROGRESS directly. If it is a partial discount, Stripe is called with the
// reduced amount.
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

	// Parse optional body for coupon_code (body may be empty for no-coupon flow).
	var reqBody struct {
		CouponCode string `json:"coupon_code"`
	}
	// Ignore decode errors — body is optional; an empty/missing body is fine.
	_ = json.NewDecoder(r.Body).Decode(&reqBody)

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

	// Determine the charge amount: use the first PENDING milestone if milestones exist,
	// otherwise fall back to the full SoW price.
	var currentMilestoneID string
	var currentMilestoneNumber int
	var baseAmountCents int64

	var firstMilestoneID string
	var firstMilestoneAmount int64
	var firstMilestoneOrderIndex int
	milestoneErr := app.DB.QueryRow(
		`SELECT id, amount, order_index FROM milestones
		 WHERE sow_id = ? AND status = 'PENDING'
		 ORDER BY order_index ASC LIMIT 1`,
		sow.ID,
	).Scan(&firstMilestoneID, &firstMilestoneAmount, &firstMilestoneOrderIndex)

	if milestoneErr == nil {
		// Milestones exist — charge the first PENDING milestone amount (stored in dollars)
		currentMilestoneID = firstMilestoneID
		currentMilestoneNumber = firstMilestoneOrderIndex + 1
		baseAmountCents = firstMilestoneAmount * 100
	} else if milestoneErr == sql.ErrNoRows {
		// No milestones — charge the full SoW amount
		baseAmountCents = int64(sow.PriceCents)
	} else {
		log.Error("checkout: db error fetching first milestone", "job_id", jobID, "sow_id", sow.ID, "error", milestoneErr)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if baseAmountCents <= 0 {
		writeError(w, http.StatusBadRequest, "charge amount must be greater than zero")
		return
	}

	// --- Coupon handling ---
	chargeAmountCents := baseAmountCents
	var appliedCoupon *Coupon

	if reqBody.CouponCode != "" {
		coupon, couponErr := app.validateCoupon(reqBody.CouponCode)
		if couponErr == sql.ErrNoRows {
			writeError(w, http.StatusBadRequest, "invalid coupon code")
			return
		}
		if couponErr != nil {
			log.Error("checkout: coupon db error", "code", reqBody.CouponCode, "error", couponErr)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if coupon.TimesUsed >= coupon.MaxUses {
			writeError(w, http.StatusBadRequest, "coupon has already been used")
			return
		}

		discountCents, calcErr := calcDiscount(coupon.Value, chargeAmountCents)
		if calcErr != nil {
			log.Error("checkout: coupon discount calculation failed", "value", coupon.Value, "error", calcErr)
			writeError(w, http.StatusInternalServerError, "failed to calculate coupon discount")
			return
		}

		chargeAmountCents -= discountCents
		appliedCoupon = coupon
		log.Info("checkout: coupon applied", "code", coupon.Code, "discount_cents", discountCents, "final_cents", chargeAmountCents)
	}

	// If the coupon covers the full amount, skip Stripe entirely.
	if chargeAmountCents <= 0 {
		// If paying for a specific milestone, mark it PAID first.
		if currentMilestoneID != "" {
			if _, dbErr := app.DB.Exec(
				`UPDATE milestones SET status = 'PAID', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
				currentMilestoneID,
			); dbErr != nil {
				log.Error("checkout: failed to mark milestone PAID after full coupon", "milestone_id", currentMilestoneID, "error", dbErr)
				writeError(w, http.StatusInternalServerError, "database error")
				return
			}
		}
		// Mark job as IN_PROGRESS directly.
		_, dbErr := app.DB.Exec(
			`UPDATE jobs SET status = 'IN_PROGRESS', current_milestone_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			nullableString(currentMilestoneID), jobID,
		)
		if dbErr != nil {
			log.Error("checkout: failed to update job to IN_PROGRESS after full coupon", "job_id", jobID, "error", dbErr)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		// Increment coupon usage.
		if err := app.applyCouponUsage(appliedCoupon.Code); err != nil {
			log.Error("checkout: failed to increment coupon usage", "code", appliedCoupon.Code, "error", err)
			// Non-fatal: job is already IN_PROGRESS, log and continue.
		}
		log.Info("checkout: full coupon applied, job moved to IN_PROGRESS", "job_id", jobID, "coupon", appliedCoupon.Code, "employer_id", employerID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"paid":    true,
			"message": "Coupon covered full amount",
		})
		return
	}

	// --- Stripe checkout for full or partial (after discount) payment ---
	stripe.Key = app.Config.StripeSecretKey

	var productName string
	if currentMilestoneID != "" {
		productName = fmt.Sprintf("Job %s — Milestone %d", jobID, currentMilestoneNumber)
	} else {
		productName = fmt.Sprintf("Job Payment: %s", jobID)
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(productName),
					},
					UnitAmount: stripe.Int64(chargeAmountCents),
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

	// Save checkout session ID and current milestone (if any) to job
	_, err = app.DB.Exec(
		`UPDATE jobs SET stripe_checkout_session_id = ?, current_milestone_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		cs.ID, nullableString(currentMilestoneID), jobID,
	)
	if err != nil {
		log.Error("checkout: failed to save session id", "job_id", jobID, "session_id", cs.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// If a partial coupon was applied, record the usage now that the Stripe
	// session exists (the session becoming "completed" will move the job).
	if appliedCoupon != nil {
		if err := app.applyCouponUsage(appliedCoupon.Code); err != nil {
			log.Error("checkout: failed to increment partial coupon usage", "code", appliedCoupon.Code, "error", err)
			// Non-fatal.
		}
	}

	log.Info("checkout session created", "job_id", jobID, "session_id", cs.ID, "employer_id", employerID, "charge_cents", chargeAmountCents)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":        cs.URL,
		"session_id": cs.ID,
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

		// If this checkout session was for a specific milestone, store the payment
		// intent on that milestone row so ApproveMilestoneHandler can capture it
		// later. We do this before updating the job so the intent is persisted even
		// if the job update races with another request.
		if paymentIntentID != "" {
			if _, milestoneErr := app.DB.Exec(
				`UPDATE milestones SET stripe_payment_intent = ?, stripe_checkout_session_id = ?, updated_at = CURRENT_TIMESTAMP
				 WHERE id = (SELECT current_milestone_id FROM jobs WHERE stripe_checkout_session_id = ?)
				   AND stripe_payment_intent = ''`,
				paymentIntentID, cs.ID, cs.ID,
			); milestoneErr != nil {
				log.Error("webhook: failed to store payment intent on milestone", "session_id", cs.ID, "error", milestoneErr)
				// Non-fatal: log and continue — the job update below still moves the job to IN_PROGRESS.
			}
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
