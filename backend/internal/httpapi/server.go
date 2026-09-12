package httpapi

import (
	"log/slog"
	"net/http"

	"lifelink/internal/store"
)

// Server wires the store, config and logger into HTTP handlers.
type Server struct {
	store          *store.Store
	jwtSecret      string
	logger         *slog.Logger
	allowedOrigins []string
}

// New creates a Server.
func New(st *store.Store, jwtSecret string, logger *slog.Logger, allowedOrigins []string) *Server {
	return &Server{store: st, jwtSecret: jwtSecret, logger: logger, allowedOrigins: allowedOrigins}
}

// handleHealth is a liveness probe used by Docker healthchecks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Handler returns the fully wired HTTP handler (router + middleware).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/health", s.handleHealth)

	// Auth
	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))

	// Donors (read is public; writes require auth)
	mux.HandleFunc("GET /api/donors", s.handleListDonors)
	mux.HandleFunc("GET /api/donors/me", s.requireAuth(s.handleGetMyProfile))
	mux.HandleFunc("PUT /api/donors/me", s.requireAuth(s.handleUpdateMyProfile))
	mux.HandleFunc("PATCH /api/donors/me/availability", s.requireAuth(s.handleUpdateMyAvailability))
	mux.HandleFunc("GET /api/donors/{id}", s.handleGetDonor)
	mux.HandleFunc("PUT /api/donors/{id}", s.requireAuth(s.handleUpdateDonor))
	mux.HandleFunc("PATCH /api/donors/{id}/availability", s.requireAuth(s.handleUpdateDonorAvailability))

	// Requests
	mux.HandleFunc("POST /api/requests", s.requireAuth(s.handleCreateRequest))
	mux.HandleFunc("GET /api/requests", s.requireAuth(s.handleListRequests))
	mux.HandleFunc("GET /api/requests/{id}", s.requireAuth(s.handleGetRequest))
	mux.HandleFunc("PATCH /api/requests/{id}/status", s.requireAuth(s.handleUpdateRequestStatus))
	mux.HandleFunc("DELETE /api/requests/{id}", s.requireAuth(s.handleDeleteRequest))

	// Notifications
	mux.HandleFunc("GET /api/notifications", s.requireAuth(s.handleListNotifications))
	mux.HandleFunc("PATCH /api/notifications/read-all", s.requireAuth(s.handleReadAllNotifications))
	mux.HandleFunc("PATCH /api/notifications/{id}/read", s.requireAuth(s.handleMarkNotificationRead))

	// Admin
	mux.HandleFunc("GET /api/admin/stats", s.requireAdmin(s.handleAdminStats))
	mux.HandleFunc("GET /api/admin/users", s.requireAdmin(s.handleAdminUsers))
	mux.HandleFunc("GET /api/admin/donors", s.requireAdmin(s.handleAdminDonors))
	mux.HandleFunc("GET /api/admin/requests", s.requireAdmin(s.handleAdminRequests))
	mux.HandleFunc("PATCH /api/admin/users/{id}/status", s.requireAdmin(s.handleAdminUserStatus))

	return s.withMiddleware(mux)
}
