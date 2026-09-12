package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"lifelink/internal/models"
	"lifelink/internal/store"
)

type createRequestPayload struct {
	DonorID       int64  `json:"donor_id"`
	BloodGroup    string `json:"blood_group"`
	PatientName   string `json:"patient_name"`
	HospitalName  string `json:"hospital_name"`
	Location      string `json:"location"`
	RequiredDate  string `json:"required_date"`
	Message       string `json:"message"`
	ContactNumber string `json:"contact_number"`
}

func (s *Server) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	requester := userFromContext(r.Context())

	var payload createRequestPayload
	if err := decodeJSON(w, r, &payload); err != nil {
		return
	}

	if msg := validateRequestPayload(payload); msg != "" {
		writeError(w, errValidation(msg))
		return
	}
	if payload.DonorID == requester.ID {
		writeError(w, errValidation("You cannot request yourself"))
		return
	}

	donor, err := s.store.GetDonorByUserID(r.Context(), payload.DonorID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrDonorNotFound)
		return
	}
	if err != nil {
		s.logger.Error("request lookup donor", "err", err)
		writeError(w, ErrInternal)
		return
	}
	if !donor.Available {
		writeError(w, ErrDonorUnavailable)
		return
	}
	if donor.BloodGroup != payload.BloodGroup {
		writeError(w, errValidation("Blood group does not match the donor's group"))
		return
	}

	requiredDate, _ := parseDate(payload.RequiredDate) // validated above

	req := &models.BloodRequest{
		RequesterID:   requester.ID,
		DonorID:       donor.UserID,
		BloodGroup:    payload.BloodGroup,
		PatientName:   payload.PatientName,
		HospitalName:  payload.HospitalName,
		Location:      payload.Location,
		RequiredDate:  requiredDate,
		Message:       payload.Message,
		ContactNumber: payload.ContactNumber,
		Status:        models.StatusPending,
	}

	err = s.store.CreateRequest(r.Context(), req)
	if errors.Is(err, store.ErrDuplicateActive) {
		writeError(w, ErrDuplicateRequest)
		return
	}
	if err != nil {
		s.logger.Error("create request", "err", err)
		writeError(w, ErrInternal)
		return
	}

	notifMsg := requester.Name + " requested your " + req.BloodGroup + " blood donation."
	if err := s.store.CreateNotification(r.Context(), req.DonorID, &req.ID, models.NotifRequestReceived, notifMsg); err != nil {
		s.logger.Error("notify donor", "err", err)
	}

	req.RequesterName, req.DonorName = requester.Name, donor.Name
	req.ContactNumber = "" // the requester already knows this
	writeSuccess(w, http.StatusCreated, req)
}

func validateRequestPayload(p createRequestPayload) string {
	switch {
	case p.DonorID <= 0:
		return "Donor is required"
	case !isValidBloodGroup(p.BloodGroup):
		return "Invalid blood group"
	case p.PatientName == "":
		return "Patient name is required"
	case p.HospitalName == "":
		return "Hospital name is required"
	case p.Location == "":
		return "Location is required"
	case !isValidPhone(p.ContactNumber):
		return "Enter a valid contact number"
	case p.RequiredDate == "":
		return "Required date is required"
	}

	d, err := parseDate(p.RequiredDate)
	if err != nil {
		return "Invalid required date"
	}
	today := time.Now().Truncate(24 * time.Hour)
	if d.Before(today) {
		return "Required date cannot be in the past"
	}
	return ""
}

func (s *Server) handleListRequests(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	q := r.URL.Query()
	page, limit := parsePagination(q.Get)
	direction := q.Get("type")
	if direction != "" && direction != "sent" && direction != "received" {
		writeError(w, errValidation("Invalid type; use sent or received"))
		return
	}
	status := q.Get("status")
	if status != "" && !isValidRequestStatus(status) {
		writeError(w, errInvalidStatus())
		return
	}

	items, total, totalPages, err := s.store.ListRequestsForUser(r.Context(), user.ID, direction, status, page, limit)
	if err != nil {
		s.logger.Error("list requests", "err", err)
		writeError(w, ErrInternal)
		return
	}
	items = s.redactContact(items, user)
	writeSuccess(w, http.StatusOK, models.Page{Items: items, Page: page, Limit: limit, Total: total, TotalPages: totalPages})
}

func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}
	req, err := s.store.GetRequest(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrRequestNotFound)
		return
	}
	if err != nil {
		s.logger.Error("get request", "err", err)
		writeError(w, ErrInternal)
		return
	}
	if !s.canViewRequest(user, req) {
		writeError(w, ErrForbidden)
		return
	}
	if user.ID != req.DonorID && user.Role != models.RoleAdmin {
		req.ContactNumber = ""
	}
	writeSuccess(w, http.StatusOK, req)
}

// redactContact hides contact numbers unless the user is the donor or admin.
func (s *Server) redactContact(items []models.BloodRequest, user *models.User) []models.BloodRequest {
	for i := range items {
		if user.ID != items[i].DonorID && user.Role != models.RoleAdmin {
			items[i].ContactNumber = ""
		}
	}
	return items
}

func (s *Server) canViewRequest(user *models.User, req *models.BloodRequest) bool {
	return user.Role == models.RoleAdmin || user.ID == req.RequesterID || user.ID == req.DonorID
}

type statusUpdatePayload struct {
	Status string `json:"status"`
}

func (s *Server) handleUpdateRequestStatus(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}

	var payload statusUpdatePayload
	if err := decodeJSON(w, r, &payload); err != nil {
		return
	}
	if !isValidRequestStatus(payload.Status) {
		writeError(w, errInvalidStatus())
		return
	}

	req, err := s.store.GetRequest(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrRequestNotFound)
		return
	}
	if err != nil {
		s.logger.Error("load request for status", "err", err)
		writeError(w, ErrInternal)
		return
	}

	ok, reason := allowedTransition(user, req, payload.Status)
	if !ok {
		writeError(w, &AppError{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: reason})
		return
	}

	updated, err := s.store.UpdateRequestStatus(r.Context(), id, payload.Status)
	if err != nil {
		s.logger.Error("update request status", "err", err)
		writeError(w, ErrInternal)
		return
	}

	switch payload.Status {
	case models.StatusAccepted:
		msg := req.DonorName + " accepted your blood request."
		if err := s.store.CreateNotification(r.Context(), req.RequesterID, &req.ID, models.NotifRequestAccepted, msg); err != nil {
			s.logger.Error("notify accepted", "err", err)
		}
	case models.StatusRejected:
		msg := req.DonorName + " declined your blood request."
		if err := s.store.CreateNotification(r.Context(), req.RequesterID, &req.ID, models.NotifRequestRejected, msg); err != nil {
			s.logger.Error("notify rejected", "err", err)
		}
	}

	if user.ID != updated.DonorID && user.Role != models.RoleAdmin {
		updated.ContactNumber = ""
	}
	writeSuccess(w, http.StatusOK, updated)
}

var requestTransitions = map[string][]string{
	models.StatusPending:   {models.StatusAccepted, models.StatusRejected, models.StatusCancelled},
	models.StatusAccepted:  {models.StatusCompleted, models.StatusCancelled},
	models.StatusRejected:  {},
	models.StatusCompleted: {},
	models.StatusCancelled: {},
}

func isValidRequestStatus(s string) bool {
	_, ok := requestTransitions[s]
	return ok
}

// allowedTransition decides whether user may move req to nextStatus.
func allowedTransition(user *models.User, req *models.BloodRequest, next string) (bool, string) {
	allowedNext := requestTransitions[req.Status]
	if !containsString(allowedNext, next) {
		return false, "Cannot transition from " + req.Status + " to " + next
	}
	if user.Role == models.RoleAdmin {
		return true, ""
	}
	switch {
	case user.ID == req.DonorID:
		switch next {
		case models.StatusAccepted, models.StatusRejected:
			return req.Status == models.StatusPending, "Only pending requests can be accepted or rejected"
		case models.StatusCompleted:
			return req.Status == models.StatusAccepted, "Only accepted requests can be marked completed"
		}
		return false, "Donors cannot perform this action"
	case user.ID == req.RequesterID:
		if next == models.StatusCancelled {
			return true, ""
		}
		return false, "Requesters can only cancel requests"
	default:
		return false, "You do not have permission to update this request"
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (s *Server) handleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, ErrBadRequest)
		return
	}
	req, err := s.store.GetRequest(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, ErrRequestNotFound)
		return
	}
	if err != nil {
		s.logger.Error("load request for delete", "err", err)
		writeError(w, ErrInternal)
		return
	}
	if user.Role != models.RoleAdmin && user.ID != req.RequesterID {
		writeError(w, ErrForbidden)
		return
	}
	if err := s.store.DeleteRequest(r.Context(), id); err != nil {
		s.logger.Error("delete request", "err", err)
		writeError(w, ErrInternal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
