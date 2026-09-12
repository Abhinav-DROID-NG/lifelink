package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"lifelink/internal/models"
)

const requestSelect = `
	SELECT r.id, r.requester_id, r.donor_id, r.blood_group, r.patient_name, r.hospital_name,
	       r.location, r.required_date, r.message, r.contact_number, r.status, r.created_at, r.updated_at,
	       ru.name, du.name
	FROM blood_requests r
	JOIN users ru ON ru.id = r.requester_id
	JOIN users du ON du.id = r.donor_id`

// CreateRequest inserts a blood request. Returns ErrDuplicateActive when the
// requester already has a PENDING request for the same donor.
func (s *Store) CreateRequest(ctx context.Context, r *models.BloodRequest) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO blood_requests (requester_id, donor_id, blood_group, patient_name, hospital_name,
		                            location, required_date, message, contact_number, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`,
		r.RequesterID, r.DonorID, r.BloodGroup, r.PatientName, r.HospitalName,
		r.Location, r.RequiredDate, r.Message, r.ContactNumber, models.StatusPending).
		Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateActive
		}
		return fmt.Errorf("create request: %w", err)
	}
	return nil
}

// GetRequest fetches a request by id, joined with participant names.
func (s *Store) GetRequest(ctx context.Context, id int64) (*models.BloodRequest, error) {
	r := &models.BloodRequest{}
	err := s.pool.QueryRow(ctx, requestSelect+` WHERE r.id = $1`, id).
		Scan(&r.ID, &r.RequesterID, &r.DonorID, &r.BloodGroup, &r.PatientName, &r.HospitalName,
			&r.Location, &r.RequiredDate, &r.Message, &r.ContactNumber, &r.Status, &r.CreatedAt, &r.UpdatedAt,
			&r.RequesterName, &r.DonorName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return r, nil
}

// ListRequestsForUser returns requests where the user is requester ("sent")
// or donor ("received"), paginated.
func (s *Store) ListRequestsForUser(ctx context.Context, userID int64, direction, status string, page, limit int) ([]models.BloodRequest, int64, int, error) {
	cond := ""
	switch direction {
	case "received":
		cond = "r.donor_id = $1"
	case "sent":
		cond = "r.requester_id = $1"
	default:
		cond = "(r.donor_id = $1 OR r.requester_id = $1)"
	}
	args := []any{userID}
	if status != "" {
		args = append(args, status)
		cond = fmt.Sprintf("(%s) AND r.status = $%d", cond, len(args))
	}

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM blood_requests r WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, 0, fmt.Errorf("count user requests: %w", err)
	}

	args = append(args, limit, (page-1)*limit)
	sql := requestSelect +
		` WHERE ` + cond +
		fmt.Sprintf(` ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list user requests: %w", err)
	}
	defer rows.Close()

	reqs, err := scanRequests(rows)
	if err != nil {
		return nil, 0, 0, err
	}
	return reqs, total, totalPages(total, limit), nil
}

// UpdateRequestStatus sets a request's status. A transition into ACCEPTED or
// REJECTED atomically flips the donor's availability check at the handler layer.
func (s *Store) UpdateRequestStatus(ctx context.Context, id int64, status string) (*models.BloodRequest, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE blood_requests SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return nil, fmt.Errorf("update request status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.GetRequest(ctx, id)
}

// DeleteRequest removes a request permanently.
func (s *Store) DeleteRequest(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM blood_requests WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAllRequests returns every request (admin view), optionally filtered by
// status and recipient/donor name search, paginated.
func (s *Store) ListAllRequests(ctx context.Context, status, search string, page, limit int) ([]models.BloodRequest, int64, int, error) {
	var sb strings.Builder
	sb.WriteString(" WHERE 1=1")
	args := []any{}
	if status != "" {
		args = append(args, status)
		sb.WriteString(fmt.Sprintf(` AND r.status = $%d`, len(args)))
	}
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		sb.WriteString(fmt.Sprintf(` AND (lower(ru.name) LIKE $%d OR lower(du.name) LIKE $%d OR lower(r.hospital_name) LIKE $%d)`, len(args), len(args), len(args)))
	}

	var total int64
	if err := s.pool.QueryRow(ctx, strings.Replace(requestSelect, `SELECT r.id, r.requester_id`, `SELECT count(*)`, 1)+sb.String(), args...).Scan(&total); err != nil {
		return nil, 0, 0, fmt.Errorf("count all requests: %w", err)
	}

	args = append(args, limit, (page-1)*limit)
	sql := requestSelect + sb.String() +
		fmt.Sprintf(` ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list all requests: %w", err)
	}
	defer rows.Close()

	reqs, err := scanRequests(rows)
	if err != nil {
		return nil, 0, 0, err
	}
	return reqs, total, totalPages(total, limit), nil
}

func scanRequests(rows pgx.Rows) ([]models.BloodRequest, error) {
	reqs := []models.BloodRequest{}
	for rows.Next() {
		var r models.BloodRequest
		if err := rows.Scan(&r.ID, &r.RequesterID, &r.DonorID, &r.BloodGroup, &r.PatientName, &r.HospitalName,
			&r.Location, &r.RequiredDate, &r.Message, &r.ContactNumber, &r.Status, &r.CreatedAt, &r.UpdatedAt,
			&r.RequesterName, &r.DonorName); err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}
