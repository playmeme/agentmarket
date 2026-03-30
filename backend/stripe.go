package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	stripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/account"
	"github.com/stripe/stripe-go/v84/accountlink"
	"github.com/stripe/stripe-go/v84/checkout/session"
	"github.com/stripe/stripe-go/v84/customer"
	"github.com/stripe/stripe-go/v84/webhook"
)

// CreateCheckoutHandler creates a Stripe Checkout Session for a job (employer only).
// POST /api/ui/jobs/{job_id}/checkout
// Optional body: { "coupon_code": "M1PAID", "tip_amount": 5.00 }
// If the coupon covers the full amount (and no tip), Stripe is skipped and the job
// is moved to IN_PROGRESS directly. If it is a partial discount, Stripe is called
// with the reduced amount. An optional tip (fixed dollar amount) is added on top of
// the post-coupon amount and stored separately in the job record.
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

	// Parse optional body for coupon_code and tip_amount (body may be empty).
	var reqBody struct {
		CouponCode string  `json:"coupon_code"`
		TipAmount  float64 `json:"tip_amount"` // dollars, e.g. 5.00; defaults to 0
	}
	// Ignore decode errors — body is optional; an empty/missing body is fine.
	_ = json.NewDecoder(r.Body).Decode(&reqBody)

	// Convert tip to cents; reject negative values.
	tipCents := int64(reqBody.TipAmount * 100)
	if tipCents < 0 {
		writeError(w, http.StatusBadRequest, "tip_amount must be zero or positive")
		return
	}

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

	// If the milestone amount is $0, skip Stripe entirely and advance the job.
	// Do NOT mark the milestone PAID — it stays PENDING so the agent can still
	// submit deliverables and the manager can approve them. Only payment is
	// skipped, not the work/review cycle. This matches the coupon-covers-full
	// path below which also leaves milestones PENDING.
	if baseAmountCents == 0 && currentMilestoneID != "" {
		_, dbErr := app.DB.Exec(
			`UPDATE jobs SET status = 'IN_PROGRESS', current_milestone_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			nullableString(currentMilestoneID), jobID,
		)
		if dbErr != nil {
			log.Error("checkout: failed to update job to IN_PROGRESS for $0 milestone", "job_id", jobID, "error", dbErr)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		log.Info("checkout: $0 milestone bypassed Stripe, job moved to IN_PROGRESS", "job_id", jobID, "milestone_id", currentMilestoneID, "employer_id", employerID)

		_ = app.CreateNotification(employerID, jobID, NotifPaymentDue,
			"Milestone started",
			fmt.Sprintf("Milestone %d is $0 — no payment required, job started", currentMilestoneNumber))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"paid":    true,
			"message": fmt.Sprintf("Milestone %d is $0 — no payment required", currentMilestoneNumber),
		})
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

	// --- Tip handling ---
	// The tip is applied after the coupon discount. A tip never reduces the charge amount.
	// If the coupon covers the full base amount but a tip is present, we still go through
	// Stripe (we cannot skip it when money is owed).
	totalChargeCents := chargeAmountCents + tipCents
	if tipCents > 0 {
		log.Info("checkout: tip added", "tip_cents", tipCents, "base_after_coupon", chargeAmountCents, "total", totalChargeCents)
	}

	// If the coupon covers the full amount AND there is no tip, skip Stripe entirely.
	if chargeAmountCents <= 0 && tipCents == 0 {
		// Do NOT mark the milestone PAID here. Payment being captured only means
		// the employer has funded this milestone phase; the milestone stays PENDING
		// so the agent can submit deliverables via SubmitMilestoneHandler, which
		// moves it to REVIEW_REQUESTED. The PAID status is set only after the
		// employer approves the delivered work via ApproveMilestoneHandler.
		//
		// Mark job as IN_PROGRESS directly.
		couponCents := baseAmountCents // full coupon = discount equals the base amount
		var transactionCents int64     // $0 charged
		_, dbErr := app.DB.Exec(
			`UPDATE jobs SET status = 'IN_PROGRESS', current_milestone_id = ?, tip_cents = 0, coupon_cents = ?, transaction_cents = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			nullableString(currentMilestoneID), couponCents, transactionCents, jobID,
		)
		// Also record on the milestone row if applicable.
		if dbErr == nil && currentMilestoneID != "" {
			_, _ = app.DB.Exec(
				`UPDATE milestones SET coupon_cents = ?, tip_cents = 0, transaction_cents = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
				couponCents, currentMilestoneID,
			)
		}
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

	// ----------------------------------------------------
	// Agent must have a Stripe Connect account, else error out
	var agentStripeAccountID sql.NullString
	agentAcctErr := app.DB.QueryRow(
		`SELECT u.stripe_account_id FROM users u
		 JOIN jobs j ON j.agent_id = u.id
		 WHERE j.id = ?`, jobID,
	).Scan(&agentStripeAccountID)
	if agentAcctErr != nil && agentAcctErr != sql.ErrNoRows {
		log.Error("stripe checkout: failed to look up agent stripe account", "job_id", jobID, "error", agentAcctErr)
		return  // If there's no destination, then there's no use in doing this payment.
	}


	// --- Stripe checkout for full or partial (after discount) payment ---
	stripe.Key = app.Config.StripeSecretKey


	// Get or setup this employer's Stripe customer ID
	var employerStripeCustomerID sql.NullString
	var employerEmail, employerName string
	emplAcctErr := app.DB.QueryRow(
		"SELECT stripe_customer_id, email, name FROM users WHERE id = ?", employerID,
	).Scan(&employerStripeCustomerID, &employerEmail, &employerName)

	if emplAcctErr != nil {
		slog.Warn("stripe checkout: db read error", "employer user_id", employerID, "error", err)
	}

	// If employer doesn't have a Stripe customer ID, create a new one in Stripe and save it
	if employerStripeCustomerID.String == "" {
		customerParams := &stripe.CustomerParams{
			Email: stripe.String(employerEmail), // must have email for Stripe Link and receipts
			Name:  stripe.String(employerName),
			// Storing internal DB ID in Stripe's metadata makes debugging easier when cross-referencing records
			Metadata: map[string]string{
				"internal_user_id": employerID,
			},
		}
		newCustomer, err := customer.New(customerParams)
		if err != nil {
			slog.Warn("stripe checkout: failed to create customer profile", "employer_id", employerID, "error", err)
		}
		employerStripeCustomerID.String = newCustomer.ID

		if _, err := app.DB.Exec("UPDATE users SET stripe_customer_id = ? WHERE id = ?", employerStripeCustomerID, employerID); err != nil {
			slog.Warn("update stripe fields: failed to set new customer_id", "user_id", employerID, "error", err)
		}
		log.Info("created new stripe customer", "employer_id", employerID, "stripe_customer_id", employerStripeCustomerID)
	}


	var productName string
	if currentMilestoneID != "" {
		productName = fmt.Sprintf("Job %s — Milestone %d", jobID, currentMilestoneNumber)
	} else {
		productName = fmt.Sprintf("Job Payment: %s", jobID)
	}

	lineItems := []*stripe.CheckoutSessionLineItemParams{
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
	}

	// Add tip as a separate line item so it is visible on the Stripe receipt.
	if tipCents > 0 {
		lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String("usd"),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String("Tip"),
				},
				UnitAmount: stripe.Int64(tipCents),
			},
			Quantity: stripe.Int64(1),
		})
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems:          lineItems,
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
		Customer:           stripe.String(employerStripeCustomerID.String),
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			CaptureMethod:  stripe.String("manual"),
			TransferData:  &stripe.CheckoutSessionPaymentIntentDataTransferDataParams{
				Destination: stripe.String(agentStripeAccountID.String),
			},
		},
		SuccessURL: stripe.String(fmt.Sprintf("%s/jobs/%s?payment=success", app.Config.BaseURL, jobID)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/jobs/%s?payment=cancelled", app.Config.BaseURL, jobID)),
	}
	log.Info("checkout: transfer_data set for agent connected account", "job_id", jobID, "destination", agentStripeAccountID.String)

	cs, err := session.New(params)
	if err != nil {
		log.Error("checkout: stripe session creation failed", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create checkout session")
		return
	}

	// Save checkout session ID, current milestone (if any), tip, coupon, and
	// transaction total to job.
	couponCents := baseAmountCents - chargeAmountCents // discount in cents
	_, err = app.DB.Exec(
		`UPDATE jobs SET stripe_checkout_session_id = ?, current_milestone_id = ?, tip_cents = ?, coupon_cents = ?, transaction_cents = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		cs.ID, nullableString(currentMilestoneID), tipCents, couponCents, totalChargeCents, jobID,
	)
	// Also record on the milestone row if applicable.
	if err == nil && currentMilestoneID != "" {
		_, _ = app.DB.Exec(
			`UPDATE milestones SET coupon_cents = ?, tip_cents = ?, transaction_cents = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			couponCents, tipCents, totalChargeCents, currentMilestoneID,
		)
	}
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

	log.Info("checkout session created", "job_id", jobID, "session_id", cs.ID, "employer_id", employerID, "charge_cents", chargeAmountCents, "tip_cents", tipCents, "total_cents", totalChargeCents)

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
	event, err := webhook.ConstructEventWithOptions(payload, sigHeader, app.Config.StripeWebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
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

	case "checkout.session.expired":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			log.Error("webhook: failed to parse checkout.session.expired", "error", err)
			writeError(w, http.StatusBadRequest, "failed to parse event data")
			return
		}

		result, err := app.DB.Exec(
			`UPDATE jobs SET stripe_checkout_session_id = '', updated_at = CURRENT_TIMESTAMP
			 WHERE stripe_checkout_session_id = ? AND status = 'AWAITING_PAYMENT'`,
			cs.ID,
		)
		if err != nil {
			log.Error("webhook: failed to clear stale session id after checkout.session.expired", "session_id", cs.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		affected, _ := result.RowsAffected()
		log.Info("webhook: checkout.session.expired processed, stale session ID cleared", "session_id", cs.ID, "rows_affected", affected)

	default:
		log.Info("webhook: unhandled event type", "type", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}

// ConnectOnboardHandler creates a Stripe Connect Express account for the
// authenticated user and returns an onboarding link. If the user already has a
// connected account, it generates a fresh onboarding link for that account.
// POST /api/ui/stripe/connect/onboard
func (app *App) ConnectOnboardHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "connect_onboard")

	userID, _ := r.Context().Value(contextKeyUserID).(string)
	stripe.Key = app.Config.StripeSecretKey

	// Check if user already has a connected account.
	var existingID sql.NullString
	if err := app.DB.QueryRow("SELECT stripe_account_id FROM users WHERE id = ?", userID).Scan(&existingID); err != nil {
		log.Error("connect onboard: db error", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	accountID := ""
	if existingID.Valid && existingID.String != "" {
		accountID = existingID.String
	} else {
		// Create a new Express account.
		acctParams := &stripe.AccountParams{
			Type: stripe.String(string(stripe.AccountTypeExpress)),
			Capabilities: &stripe.AccountCapabilitiesParams{
				Transfers: &stripe.AccountCapabilitiesTransfersParams{
					Requested: stripe.Bool(true),
				},
			},
		}
		acct, err := account.New(acctParams)
		if err != nil {
			log.Error("connect onboard: failed to create express account", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to create Stripe Connect account")
			return
		}
		accountID = acct.ID

		if _, err := app.DB.Exec("UPDATE users SET stripe_account_id = ? WHERE id = ?", accountID, userID); err != nil {
			log.Error("connect onboard: failed to store account id", "user_id", userID, "account_id", accountID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		log.Info("connect onboard: express account created", "user_id", userID, "account_id", accountID)
	}

	// Generate an onboarding link.
	linkParams := &stripe.AccountLinkParams{
		Account:    stripe.String(accountID),
		Type:       stripe.String(string(stripe.AccountLinkTypeAccountOnboarding)),
		RefreshURL: stripe.String(fmt.Sprintf("%s/connect/refresh", app.Config.BaseURL)),
		ReturnURL:  stripe.String(fmt.Sprintf("%s/connect/complete", app.Config.BaseURL)),
	}
	link, err := accountlink.New(linkParams)
	if err != nil {
		log.Error("connect onboard: failed to create account link", "user_id", userID, "account_id", accountID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create onboarding link")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"account_id":     accountID,
		"onboarding_url": link.URL,
	})
}

// ConnectStatusHandler returns the Stripe Connect status for the authenticated user.
// GET /api/ui/stripe/connect/status
func (app *App) ConnectStatusHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "connect_status")

	userID, _ := r.Context().Value(contextKeyUserID).(string)

	var acctID sql.NullString
	if err := app.DB.QueryRow("SELECT stripe_account_id FROM users WHERE id = ?", userID).Scan(&acctID); err != nil {
		log.Error("connect status: db error", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if !acctID.Valid || acctID.String == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"connected": false,
		})
		return
	}

	stripe.Key = app.Config.StripeSecretKey
	acct, err := account.GetByID(acctID.String, nil)
	if err != nil {
		log.Error("connect status: stripe api error", "user_id", userID, "account_id", acctID.String, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch Stripe account")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"connected":         true,
		"account_id":        acctID.String,
		"charges_enabled":   acct.ChargesEnabled,
		"payouts_enabled":   acct.PayoutsEnabled,
		"details_submitted": acct.DetailsSubmitted,
	})
}

// UpdateStripeFieldsHandler allows a user to set their own stripe_account_id
// and/or stripe_customer_id directly. This is an escape hatch for cases where
// the account was created outside of the onboarding flow.
// PUT /api/ui/stripe/connect/account
func (app *App) UpdateStripeFieldsHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "update_stripe_fields")

	userID, _ := r.Context().Value(contextKeyUserID).(string)

	var req struct {
		StripeAccountID  string `json:"stripe_account_id"`
		StripeCustomerID string `json:"stripe_customer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.StripeAccountID != "" {
		if _, err := app.DB.Exec("UPDATE users SET stripe_account_id = ? WHERE id = ?", req.StripeAccountID, userID); err != nil {
			log.Error("update stripe fields: failed to set account_id", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		log.Info("stripe_account_id updated", "user_id", userID, "account_id", req.StripeAccountID)
	}
	if req.StripeCustomerID != "" {
		if _, err := app.DB.Exec("UPDATE users SET stripe_customer_id = ? WHERE id = ?", req.StripeCustomerID, userID); err != nil {
			log.Error("update stripe fields: failed to set customer_id", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		log.Info("stripe_customer_id updated", "user_id", userID, "customer_id", req.StripeCustomerID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
