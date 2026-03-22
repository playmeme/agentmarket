package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func NewRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Static files
	r.Handle("/*", http.FileServer(http.Dir("./static")))

	// Public routes
	r.Get("/health", healthHandler)

	r.Route("/api/ui/auth", func(r chi.Router) {
		r.Post("/signup", SignupHandler)
		r.Post("/login", LoginHandler)
		r.Post("/verify-email", VerifyEmailHandler)
		r.Post("/forgot-password", ForgotPasswordHandler)
		r.Post("/reset-password", ResetPasswordHandler)
	})

	// JWT-protected UI routes
	r.Route("/api/ui", func(r chi.Router) {
		r.Use(JWTAuth)

		r.Route("/agents", func(r chi.Router) {
			r.Get("/", ListAgentsHandler)
			r.Get("/{id}", GetAgentHandler)
		})

		r.Route("/handlers", func(r chi.Router) {
			r.Post("/agents", CreateAgentHandler)
			r.Get("/agents", ListHandlerAgentsHandler)
			r.Get("/jobs", ListJobsHandler)
		})

		// Singular aliases for frontend compatibility
		r.Route("/handler", func(r chi.Router) {
			r.Get("/agents", ListHandlerAgentsHandler)
			r.Get("/jobs", ListJobsHandler)
		})

		r.Route("/jobs", func(r chi.Router) {
			r.Post("/hire", HireAgentHandler)
			r.Get("/", ListJobsHandler)
			r.Get("/{id}", GetJobHandler)
			r.Post("/{job_id}/milestones/{milestone_id}/approve", ApproveMilestoneHandler)
		})
	})

	// API key protected agent routes
	r.Route("/api/v1/jobs", func(r chi.Router) {
		r.Use(APIKeyAuth)
		r.Get("/pending", GetPendingJobsHandler)
		r.Post("/{job_id}/accept", AcceptJobHandler)
		r.Post("/{job_id}/decline", DeclineJobHandler)
		r.Post("/{job_id}/milestones/{milestone_id}/submit", SubmitMilestoneHandler)
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
