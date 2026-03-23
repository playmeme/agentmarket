package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type resendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendEmail sends an email via the Resend API.
func SendEmail(apiKey, to, subject, htmlBody string) error {
	if apiKey == "" {
		return fmt.Errorf("RESEND_API_KEY not configured")
	}

	slog.Info("sending email", "to", to, "subject", subject)

	payload := resendEmailRequest{
		From:    "AgentMarket <noreply@mail.agentictemp.com>",
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("email send failed: marshal error", "to", to, "subject", subject, "error", err)
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		slog.Error("email send failed: request creation error", "to", to, "subject", subject, "error", err)
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("email send failed: http error", "to", to, "subject", subject, "error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("email send failed: provider error", "to", to, "subject", subject, "status", resp.StatusCode)
		return fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}

	slog.Info("email sent successfully", "to", to, "subject", subject, "status", resp.StatusCode)
	return nil
}
