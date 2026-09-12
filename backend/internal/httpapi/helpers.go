// Package httpapi exposes the REST API: handlers, middleware and routing.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"lifelink/internal/models"
	"lifelink/internal/store"
)

// AppError is a REST-level error with an HTTP status and machine code.
type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e *AppError) Error() string { return e.Message }

// Application error codes and HTTP statuses.
var (
	ErrInvalidCredentials = &AppError{Status: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "Invalid email or password"}
	ErrEmailTaken         = &AppError{Status: http.StatusConflict, Code: "EMAIL_ALREADY_EXISTS", Message: "An account with this email already exists"}
	ErrDonorNotFound      = &AppError{Status: http.StatusNotFound, Code: "DONOR_NOT_FOUND", Message: "Donor not found"}
	ErrRequestNotFound    = &AppError{Status: http.StatusNotFound, Code: "REQUEST_NOT_FOUND", Message: "Request not found"}
	ErrUserNotFound       = &AppError{Status: http.StatusNotFound, Code: "USER_NOT_FOUND", Message: "User not found"}
	ErrUnauthorized       = &AppError{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: "Authentication required"}
	ErrForbidden          = &AppError{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: "You do not have permission to perform this action"}
	ErrAccountDisabled    = &AppError{Status: http.StatusForbidden, Code: "ACCOUNT_DISABLED", Message: "Your account has been disabled"}
	ErrDonorUnavailable   = &AppError{Status: http.StatusConflict, Code: "DONOR_UNAVAILABLE", Message: "This donor is currently unavailable"}
	ErrDuplicateRequest   = &AppError{Status: http.StatusConflict, Code: "DUPLICATE_REQUEST", Message: "You already have a pending request for this donor"}
	ErrBadRequest         = &AppError{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "Invalid request"}
	ErrInvalidJSON        = &AppError{Status: http.StatusBadRequest, Code: "INVALID_JSON", Message: "Request body is not valid JSON"}
	ErrBodyTooLarge       = &AppError{Status: http.StatusRequestEntityTooLarge, Code: "BODY_TOO_LARGE", Message: "Request body too large"}
	ErrInternal           = &AppError{Status: http.StatusInternalServerError, Code: "INTERNAL_ERROR", Message: "Something went wrong"}
)

func errInvalidBloodGroup() *AppError {
	return &AppError{Status: http.StatusUnprocessableEntity, Code: "INVALID_BLOOD_GROUP", Message: "Invalid blood group"}
}

func errValidation(msg string) *AppError {
	return &AppError{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: msg}
}

func errInvalidStatus() *AppError {
	return &AppError{Status: http.StatusUnprocessableEntity, Code: "INVALID_REQUEST_STATUS", Message: "Invalid request status or transition"}
}

type response struct {
	Success bool       `json:"success"`
	Data    any        `json:"data,omitempty"`
	Error   *errorBody `json:"error,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"success":false,"error":{"code":"INTERNAL_ERROR","message":"encode failed"}}`, http.StatusInternalServerError)
	}
}

func writeSuccess(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, response{Success: true, Data: data})
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.Status, response{Success: false, Error: &errorBody{Code: appErr.Code, Message: appErr.Message}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, response{Success: false, Error: &errorBody{Code: ErrInternal.Code, Message: ErrInternal.Message}})
}

func writeNotFound(w http.ResponseWriter, resource string) {
	writeJSON(w, http.StatusNotFound, response{Success: false, Error: &errorBody{Code: "NOT_FOUND", Message: resource + " not found"}})
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

var validBloodGroups = map[string]bool{
	"A+": true, "A-": true, "B+": true, "B-": true, "AB+": true, "AB-": true, "O+": true, "O-": true,
}

var validGenders = map[string]bool{"male": true, "female": true, "other": true}

func isValidBloodGroup(g string) bool { return validBloodGroups[g] }
func isValidGender(g string) bool     { return validGenders[g] }

func isValidEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	dot := strings.LastIndex(email[at:], ".")
	return dot > 1
}

func isValidPhone(phone string) bool {
	digits := 0
	for _, r := range phone {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == '-' || r == ' ' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= 10
}

// parsePagination reads page (>=1) and limit (1..100, default 20).
func parsePagination(values func(string) string) (page, limit int) {
	page, _ = strconv.Atoi(values("page"))
	if page < 1 {
		page = 1
	}
	limit, _ = strconv.Atoi(values("limit"))
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

// ---------------------------------------------------------------------------
// Context helpers
// ---------------------------------------------------------------------------

type ctxKey int

const (
	ctxUserKey ctxKey = iota
)

// userFromContext returns the authenticated user set by authMiddleware.
func userFromContext(ctx context.Context) *models.User {
	if u, ok := ctx.Value(ctxUserKey).(*models.User); ok {
		return u
	}
	return nil
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeNotFound(w, "Resource")
	case errors.Is(err, store.ErrDuplicateActive):
		writeError(w, ErrDuplicateRequest)
	case errors.Is(err, store.ErrEmailTaken):
		writeError(w, ErrEmailTaken)
	default:
		writeError(w, err)
	}
}
