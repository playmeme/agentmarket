package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// spaHandler implements the http.Handler interface, so we can use it with chi.
type spaHandler struct {
	staticPath string
	indexPath  string
}

// If index.html is missing at the current route, then serve index.html
// This avoids 404 when the user presses refresh while at a route url
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// If the route starts with "/api", then the request was for a bad API url
	if strings.HasPrefix(r.URL.Path, "/api") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "API route not found"}`))
		return
	}

	// Get the absolute path of static files
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path = filepath.Join(h.staticPath, path)

	// Check for file at that exact path
	_, err = os.Stat(path)
	if os.IsNotExist(err) {	// Serve index.html instead
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil { // Other errors (e.g. permissions)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}


func NewRouter(app *App) *chi.Mux {
	r := chi.NewRouter()

	// Unlogged routes
	r.Get("/health", healthHandler)

	r.Use(RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Static files
	spa := spaHandler{
		staticPath: "./static", 
		indexPath:  "index.html",
	}
	r.Handle("/*", spa)

	// Public routes
	r.Route("/api/ui/auth", func(r chi.Router) {
		r.Post("/signup", app.SignupHandler)
		r.Post("/login", app.LoginHandler)
		r.Post("/verify-email", app.VerifyEmailHandler)
		r.Post("/forgot-password", app.ForgotPasswordHandler)
		r.Post("/reset-password", app.ResetPasswordHandler)
	})

	// Public agent browsing routes (no auth required)
	r.Route("/api/ui/agents", func(r chi.Router) {
		r.Get("/", app.ListAgentsHandler)
		r.Get("/{id}", app.GetAgentHandler)
	})

	// JWT-protected UI routes
	r.Route("/api/ui", func(r chi.Router) {
		r.Use(app.JWTAuth)

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
			r.Post("/{job_id}/sow", app.CreateOrUpdateSOW)
			r.Get("/{job_id}/sow", app.GetSOW)
			r.Post("/{job_id}/sow/accept", app.AcceptSOW)
			r.Post("/{job_id}/checkout", app.CreateCheckoutHandler)
			r.Post("/{job_id}/approve-delivery", app.ApproveDeliveryHandler)
			r.Post("/{job_id}/request-revision", app.RequestRevisionHandler)
		})

		r.Get("/transactions", app.GetTransactionsHandler)
	})

	// Public webhook routes (no auth)
	r.Post("/api/webhooks/stripe", app.HandleStripeWebhook)

	// API key protected agent routes
	r.Route("/api/v1/jobs", func(r chi.Router) {
		r.Use(app.APIKeyAuth)
		r.Get("/pending", app.GetPendingJobsHandler)
		r.Post("/{job_id}/accept", app.AcceptJobHandler)
		r.Post("/{job_id}/decline", app.DeclineJobHandler)
		r.Post("/{job_id}/milestones/{milestone_id}/submit", app.SubmitMilestoneHandler)
		r.Post("/{job_id}/deliver", app.DeliverJobHandler)
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
