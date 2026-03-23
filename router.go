package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func NewRouter(app *App) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Static files
	r.Handle("/*", http.FileServer(http.Dir("./static")))

	// Public routes
	r.Get("/health", healthHandler)

	r.Route("/api/ui/auth", func(r chi.Router) {
		r.Post("/signup", app.SignupHandler)
		r.Post("/login", app.LoginHandler)
		r.Post("/verify-email", app.VerifyEmailHandler)
		r.Post("/forgot-password", app.ForgotPasswordHandler)
		r.Post("/reset-password", app.ResetPasswordHandler)
	})

	// JWT-protected UI routes
	r.Route("/api/ui", func(r chi.Router) {
		r.Use(app.JWTAuth)

		r.Route("/agents", func(r chi.Router) {
			r.Get("/", app.ListAgentsHandler)
			r.Get("/{id}", app.GetAgentHandler)
		})

		r.Route("/handlers", func(r chi.Router) {
			r.Post("/agents", app.CreateAgentHandler)
			r.Get("/agents", app.ListHandlerAgentsHandler)
			r.Get("/jobs", app.ListJobsHandler)
		})

		// Singular aliases for frontend compatibility
		r.Route("/handler", func(r chi.Router) {
			r.Get("/agents", app.ListHandlerAgentsHandler)
			r.Get("/jobs", app.ListJobsHandler)
		})

		r.Route("/jobs", func(r chi.Router) {
			r.Post("/hire", app.HireAgentHandler)
			r.Get("/", app.ListJobsHandler)
			r.Get("/{id}", app.GetJobHandler)
			r.Post("/{job_id}/milestones/{milestone_id}/approve", app.ApproveMilestoneHandler)
		})
	})

	// API key protected agent routes
	r.Route("/api/v1/jobs", func(r chi.Router) {
		r.Use(app.APIKeyAuth)
		r.Get("/pending", app.GetPendingJobsHandler)
		r.Post("/{job_id}/accept", app.AcceptJobHandler)
		r.Post("/{job_id}/decline", app.DeclineJobHandler)
		r.Post("/{job_id}/milestones/{milestone_id}/submit", app.SubmitMilestoneHandler)
	})

	return r
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"version": Version,
	})
}
