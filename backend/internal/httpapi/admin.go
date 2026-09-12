package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"lifelink/internal/models"
	"lifelink/internal/store"
)

func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.DashboardStats(r.Context())
	if err != nil {
		s.logger.Error("admin stats", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, stats)
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := parsePagination(q.Get)

	users, total, err := s.store.ListUsers(r.Context(), q.Get("search"), page, limit)
	if err != nil {
		s.logger.Error("admin list users", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, models.Page{Items: users, Page: page, Limit: limit, Total: total, TotalPages: calcTotalPages(total, limit)})
}

func (s *Server) handleAdminDonors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := parsePagination(q.Get)

	if v := q.Get("blood_group"); v != "" && !isValidBloodGroup(v) {
		writeError(w, errInvalidBloodGroup())
		return
	}

	donors, total, totalPages, err := s.store.ListAllDonors(r.Context(), q.Get("blood_group"), q.Get("city"), page, limit)
	if err != nil {
		s.logger.Error("admin list donors", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, models.Page{Items: donors, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func (s *Server) handleAdminRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := parsePagination(q.Get)
	if v := q.Get("status"); v != "" && !isValidRequestStatus(v) {
		writeError(w, errInvalidStatus())
		return
	}

	requests, total, totalPages, err := s.store.ListAllRequests(r.Context(), q.Get("status"), q.Get("search"), page, limit)
	if err != nil {
		s.logger.Error("admin list requests", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, models.Page{Items: requests, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

type userStatusPayload struct {
	IsActive bool `json:"is_active"`
}

func (s *Server) handleAdminUserStatus(w http.ResponseWriter, r *http.Request) {
	admin := userFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}
	if id == admin.ID {
		writeError(w, errValidation("You cannot change your own account status"))
		return
	}

	var payload userStatusPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		return
	}

	if err := s.store.UpdateUserActive(r.Context(), id, payload.IsActive); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, "User")
			return
		}
		s.logger.Error("admin update user status", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"id": id, "is_active": payload.IsActive})
}
