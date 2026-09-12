package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"lifelink/internal/models"
	"lifelink/internal/store"
)

func (s *Server) handleListDonors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := parsePagination(q.Get)

	var available *bool
	if v := q.Get("available"); v != "" {
		b := v == "true"
		available = &b
	}

	if v := q.Get("blood_group"); v != "" && !isValidBloodGroup(v) {
		writeError(w, errInvalidBloodGroup())
		return
	}
	if v := q.Get("gender"); v != "" && !isValidGender(v) {
		writeError(w, errValidation("Invalid gender"))
		return
	}

	items, total, totalPages, err := s.store.ListDonors(r.Context(), store.DonorFilter{
		BloodGroup: q.Get("blood_group"),
		City:       q.Get("city"),
		Gender:     q.Get("gender"),
		Available:  available,
		Page:       page,
		Limit:      limit,
	})
	if err != nil {
		s.logger.Error("list donors", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, models.Page{Items: items, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func (s *Server) handleGetDonor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}
	donor, err := s.store.GetDonorByUserID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrDonorNotFound)
		return
	}
	if err != nil {
		s.logger.Error("get donor", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, donor)
}

func (s *Server) handleGetMyProfile(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	s.writeProfile(w, r, user.ID)
}

func (s *Server) handleUpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	s.updateProfile(w, r, user.ID)
}

// handleUpdateDonor updates any profile the caller owns (or as admin).
func (s *Server) handleUpdateDonor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}
	if err := s.authorizeProfileWrite(r, id); err != nil {
		writeError(w, err)
		return
	}
	s.updateProfile(w, r, id)
}

func (s *Server) handleUpdateMyAvailability(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	s.updateAvailability(w, r, user.ID)
}

func (s *Server) handleUpdateDonorAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}
	if err := s.authorizeProfileWrite(r, id); err != nil {
		writeError(w, err)
		return
	}
	s.updateAvailability(w, r, id)
}

// authorizeProfileWrite permits the profile owner or an admin.
func (s *Server) authorizeProfileWrite(r *http.Request, targetID int64) error {
	user := userFromContext(r.Context())
	if user.ID == targetID || user.Role == models.RoleAdmin {
		return nil
	}
	return ErrForbidden
}

func (s *Server) writeProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	profile, _, err := s.store.GetDonorProfileByUserID(r.Context(), userID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrDonorNotFound)
		return
	}
	if err != nil {
		s.logger.Error("get my profile", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, profile)
}

type profileUpdateRequest struct {
	BloodGroup       string  `json:"blood_group"`
	DateOfBirth      string  `json:"date_of_birth"`
	Gender           string  `json:"gender"`
	City             string  `json:"city"`
	Address          string  `json:"address"`
	LastDonationDate *string `json:"last_donation_date"`
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	current, _, err := s.store.GetDonorProfileByUserID(r.Context(), userID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrDonorNotFound)
		return
	}
	if err != nil {
		s.logger.Error("load profile for update", "err", err)
		writeError(w, ErrInternal)
		return
	}

	var req profileUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	if msg := validateProfileUpdate(req); msg != "" {
		writeError(w, errValidation(msg))
		return
	}

	if req.BloodGroup != "" {
		current.BloodGroup = req.BloodGroup
	}
	if req.DateOfBirth != "" {
		dob, err := parseDate(req.DateOfBirth)
		if err != nil {
			writeError(w, errValidation("Invalid date of birth"))
			return
		}
		current.DateOfBirth = dob
	}
	if req.Gender != "" {
		current.Gender = req.Gender
	}
	if req.City != "" {
		current.City = req.City
	}
	current.Address = req.Address

	if req.LastDonationDate != nil {
		if *req.LastDonationDate == "" {
			current.LastDonationDate = nil
		} else {
			d, err := parseDate(*req.LastDonationDate)
			if err != nil || d.After(time.Now()) {
				writeError(w, errValidation("Invalid last donation date"))
				return
			}
			current.LastDonationDate = &d
		}
	}

	if err := s.store.UpdateDonorProfile(r.Context(), userID, current); err != nil {
		s.logger.Error("update profile", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, current)
}

func validateProfileUpdate(req profileUpdateRequest) string {
	switch {
	case req.BloodGroup != "" && !isValidBloodGroup(req.BloodGroup):
		return "Invalid blood group"
	case req.Gender != "" && !isValidGender(req.Gender):
		return "Invalid gender"
	case req.DateOfBirth != "":
		dob, err := parseDate(req.DateOfBirth)
		if err != nil || !isAdult(dob) {
			return "Invalid date of birth (donors must be at least 18)"
		}
	case req.City != "" && len(req.City) > 100:
		return "City is too long"
	case len(req.Address) > 500:
		return "Address is too long"
	}
	return ""
}

type availabilityRequest struct {
	Available bool `json:"available"`
}

func (s *Server) updateAvailability(w http.ResponseWriter, r *http.Request, userID int64) {
	var req availabilityRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	if err := s.store.UpdateAvailability(r.Context(), userID, req.Available); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, ErrDonorNotFound)
			return
		}
		s.logger.Error("update availability", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"available": req.Available})
}
