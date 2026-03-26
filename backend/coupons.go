package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Coupon represents a discount code row from the coupons table.
type Coupon struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Value     string `json:"value"` // "10%" or "91.00"
	MaxUses   int    `json:"max_uses"`
	TimesUsed int    `json:"times_used"`
}

// validateCoupon looks up a coupon by code and returns it if valid (not exhausted).
// Returns sql.ErrNoRows when the code does not exist.
func (app *App) validateCoupon(code string) (*Coupon, error) {
	var c Coupon
	err := app.DB.QueryRow(
		`SELECT id, code, value, max_uses, times_used FROM coupons WHERE code = ?`,
		code,
	).Scan(&c.ID, &c.Code, &c.Value, &c.MaxUses, &c.TimesUsed)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// calcDiscount computes the discount in cents given a coupon value string and the
// original amount in cents. It returns the discount amount (clamped to amountCents)
// and any parse error.
func calcDiscount(value string, amountCents int64) (int64, error) {
	if strings.HasSuffix(value, "%") {
		pctStr := strings.TrimSuffix(value, "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil {
			return 0, err
		}
		discount := int64(math.Round(float64(amountCents) * pct / 100.0))
		if discount > amountCents {
			discount = amountCents
		}
		return discount, nil
	}

	// Flat dollar amount, e.g. "91.00"
	dollars, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	discount := int64(math.Round(dollars * 100))
	if discount > amountCents {
		discount = amountCents
	}
	return discount, nil
}

// ValidateCouponHandler validates a coupon code against a given amount.
// POST /api/ui/coupons/validate
// Body: { "code": "M1PAID", "amount_cents": 19100 }
func (app *App) ValidateCouponHandler(w http.ResponseWriter, r *http.Request) {
	log := slog.With("request_id", requestID(r.Context()), "handler", "validate_coupon")

	var req struct {
		Code        string `json:"code"`
		AmountCents int64  `json:"amount_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}
	if req.AmountCents <= 0 {
		writeError(w, http.StatusBadRequest, "amount_cents must be greater than zero")
		return
	}

	coupon, err := app.validateCoupon(req.Code)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusBadRequest, "invalid coupon code")
		return
	}
	if err != nil {
		log.Error("validate_coupon: db error", "code", req.Code, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if coupon.TimesUsed >= coupon.MaxUses {
		writeError(w, http.StatusBadRequest, "coupon has already been used")
		return
	}

	discountCents, err := calcDiscount(coupon.Value, req.AmountCents)
	if err != nil {
		log.Error("validate_coupon: failed to calculate discount", "value", coupon.Value, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to calculate discount")
		return
	}

	finalCents := req.AmountCents - discountCents

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":              true,
		"discount_cents":     discountCents,
		"final_amount_cents": finalCents,
	})
}

// applyCouponUsage increments times_used for the given coupon code.
func (app *App) applyCouponUsage(code string) error {
	_, err := app.DB.Exec(
		`UPDATE coupons SET times_used = times_used + 1 WHERE code = ?`,
		code,
	)
	return err
}
