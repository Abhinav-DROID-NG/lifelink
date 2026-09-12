package httpapi

import (
	"errors"
	"net/http"
	"time"

	"lifelink/internal/auth"
	"lifelink/internal/models"
	"lifelink/internal/store"
)

type registerRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Phone       string `json:"phone"`
	BloodGroup  string `json:"blood_group"`
	DateOfBirth string `json:"date_of_birth"`
	Gender      string `json:"gender"`
	City        string `json:"city"`
	Address     string `json:"address"`
	Available   *bool  `json:"available"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}

	if msg := validateRegister(req); msg != "" {
		writeError(w, errValidation(msg))
		return
	}

	dob, err := parseDate(req.DateOfBirth)
	if err != nil {
		writeError(w, errValidation("Invalid date of birth"))
		return
	}
	if !isAdult(dob) {
		writeError(w, errValidation("Donors must be at least 18 years old"))
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, errValidation("Password must be at least 8 characters"))
		return
	}

	available := true
	if req.Available != nil {
		available = *req.Available
	}

	user := &models.User{Name: req.Name, Email: req.Email, PasswordHash: hash, Phone: req.Phone, Role: models.RoleUser}
	profile := &models.DonorProfile{
		UserID:      0,
		BloodGroup:  req.BloodGroup,
		DateOfBirth: dob,
		Gender:      req.Gender,
		City:        req.City,
		Address:     req.Address,
		Available:   available,
	}

	err = s.store.CreateUserWithProfile(r.Context(), user, profile)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(w, ErrEmailTaken)
		return
	}
	if err != nil {
		s.logger.Error("register", "err", err)
		writeError(w, ErrInternal)
		return
	}

	token, err := auth.NewToken(s.jwtSecret, *user)
	if err != nil {
		s.logger.Error("sign token", "err", err)
		writeError(w, ErrInternal)
		return
	}

	writeSuccess(w, http.StatusCreated, map[string]any{
		"token":         token,
		"user":          user,
		"donor_profile": profile,
	})
}

func validateRegister(req registerRequest) string {
	switch {
	case len(req.Name) < 2 || len(req.Name) > 100:
		return "Name must be between 2 and 100 characters"
	case !isValidEmail(req.Email):
		return "Enter a valid email address"
	case len(req.Password) < 8:
		return "Password must be at least 8 characters"
	case !isValidPhone(req.Phone):
		return "Enter a valid phone number (at least 10 digits)"
	case !isValidBloodGroup(req.BloodGroup):
		return "Invalid blood group"
	case !isValidGender(req.Gender):
		return "Invalid gender"
	case req.City == "":
		return "City is required"
	}
	return ""
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, ErrInvalidCredentials)
		return
	}

	user, err := s.store.GetUserByEmail(r.Context(), req.Email)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrInvalidCredentials)
		return
	}
	if err != nil {
		s.logger.Error("login lookup", "err", err)
		writeError(w, ErrInternal)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, ErrInvalidCredentials)
		return
	}
	if !user.IsActive {
		writeError(w, ErrAccountDisabled)
		return
	}

	token, err := auth.NewToken(s.jwtSecret, *user)
	if err != nil {
		s.logger.Error("sign token", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

// handleLogout is stateless (JWT): the client simply discards the token.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	result := map[string]any{"user": user, "has_donor_profile": false}

	profile, _, err := s.store.GetDonorProfileByUserID(r.Context(), user.ID)
	if err == nil {
		result["has_donor_profile"] = true
		result["donor_profile"] = profile
	} else if !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("me profile", "err", err)
		writeError(w, ErrInternal)
		return
	}
	writeSuccess(w, http.StatusOK, result)
}

func parseDate(s string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func isAdult(dob time.Time) bool {
	today := time.Now()
	cutoff := time.Date(today.Year()-18, today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return !dob.After(cutoff)
}
