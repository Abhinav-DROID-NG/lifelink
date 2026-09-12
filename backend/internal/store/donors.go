package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"lifelink/internal/models"
)

// DonorFilter drives the donor search endpoint.
type DonorFilter struct {
	BloodGroup string
	City       string
	Gender     string
	Available  *bool
	Page       int
	Limit      int
}

// CreateDonorProfile inserts a donor profile for an existing user.
func (s *Store) CreateDonorProfile(ctx context.Context, p *models.DonorProfile) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO donor_profiles (user_id, blood_group, date_of_birth, gender, city, address, available, last_donation_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		p.UserID, p.BloodGroup, p.DateOfBirth, p.Gender, p.City, p.Address, p.Available, p.LastDonationDate).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create donor profile: %w", err)
	}
	return nil
}

// GetDonorProfileByUserID fetches a profile plus user identity for a user.
func (s *Store) GetDonorProfileByUserID(ctx context.Context, userID int64) (*models.DonorProfile, *models.User, error) {
	p := &models.DonorProfile{}
	u := &models.User{}
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, p.user_id, p.blood_group, p.date_of_birth, p.gender, p.city, p.address,
		       p.available, p.last_donation_date, p.created_at, p.updated_at,
		       u.name, u.email, u.phone, u.role, u.is_active
		FROM donor_profiles p
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1`, userID).
		Scan(&p.ID, &p.UserID, &p.BloodGroup, &p.DateOfBirth, &p.Gender, &p.City, &p.Address,
			&p.Available, &p.LastDonationDate, &p.CreatedAt, &p.UpdatedAt,
			&u.Name, &u.Email, &u.Phone, &u.Role, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get donor profile: %w", err)
	}
	return p, u, nil
}

// UpdateDonorProfile updates mutable donor fields for a profile owner.
func (s *Store) UpdateDonorProfile(ctx context.Context, userID int64, p *models.DonorProfile) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE donor_profiles
		SET blood_group = $2, date_of_birth = $3, gender = $4, city = $5, address = $6,
		    last_donation_date = $7, updated_at = now()
		WHERE user_id = $1`,
		userID, p.BloodGroup, p.DateOfBirth, p.Gender, p.City, p.Address, p.LastDonationDate)
	if err != nil {
		return fmt.Errorf("update donor profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateAvailability flips a donor's availability flag.
func (s *Store) UpdateAvailability(ctx context.Context, userID int64, available bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE donor_profiles SET available = $2, updated_at = now() WHERE user_id = $1`, userID, available)
	if err != nil {
		return fmt.Errorf("update availability: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDonors searches active donors, filtered and paginated.
func (s *Store) ListDonors(ctx context.Context, f DonorFilter) ([]models.Donor, int64, int, error) {
	var sb strings.Builder
	sb.WriteString(`FROM donor_profiles p JOIN users u ON u.id = p.user_id WHERE u.is_active = TRUE`)
	args := []any{}
	if f.BloodGroup != "" {
		args = append(args, f.BloodGroup)
		sb.WriteString(fmt.Sprintf(` AND p.blood_group = $%d`, len(args)))
	}
	if f.City != "" {
		args = append(args, strings.ToLower(f.City))
		sb.WriteString(fmt.Sprintf(` AND lower(p.city) = $%d`, len(args)))
	}
	if f.Gender != "" {
		args = append(args, f.Gender)
		sb.WriteString(fmt.Sprintf(` AND p.gender = $%d`, len(args)))
	}
	if f.Available != nil {
		args = append(args, *f.Available)
		sb.WriteString(fmt.Sprintf(` AND p.available = $%d`, len(args)))
	}

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+sb.String(), args...).Scan(&total); err != nil {
		return nil, 0, 0, fmt.Errorf("count donors: %w", err)
	}

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	sql := `SELECT u.id, u.name, p.blood_group, p.gender, p.city, p.available, p.last_donation_date ` +
		sb.String() +
		fmt.Sprintf(` ORDER BY p.available DESC, u.name LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list donors: %w", err)
	}
	defer rows.Close()

	donors := []models.Donor{}
	for rows.Next() {
		var d models.Donor
		if err := rows.Scan(&d.UserID, &d.Name, &d.BloodGroup, &d.Gender, &d.City, &d.Available, &d.LastDonationDate); err != nil {
			return nil, 0, 0, fmt.Errorf("scan donor: %w", err)
		}
		donors = append(donors, d)
	}
	return donors, total, totalPages(total, f.Limit), rows.Err()
}

// GetDonorByUserID returns the public view of a single donor.
func (s *Store) GetDonorByUserID(ctx context.Context, userID int64) (*models.Donor, error) {
	d := &models.Donor{}
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, p.blood_group, p.gender, p.city, p.available, p.last_donation_date
		FROM donor_profiles p JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1 AND u.is_active = TRUE`, userID).
		Scan(&d.UserID, &d.Name, &d.BloodGroup, &d.Gender, &d.City, &d.Available, &d.LastDonationDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get donor: %w", err)
	}
	return d, nil
}

// ListAllDonors returns all donors with profile info (admin view), paginated.
func (s *Store) ListAllDonors(ctx context.Context, bloodGroup, city string, page, limit int) ([]models.Donor, int64, int, error) {
	var sb strings.Builder
	sb.WriteString(`FROM donor_profiles p JOIN users u ON u.id = p.user_id`)
	args := []any{}
	if bloodGroup != "" {
		args = append(args, bloodGroup)
		sb.WriteString(fmt.Sprintf(` AND p.blood_group = $%d`, len(args)))
	}
	if city != "" {
		args = append(args, strings.ToLower(city))
		sb.WriteString(fmt.Sprintf(` AND lower(p.city) = $%d`, len(args)))
	}

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+sb.String(), args...).Scan(&total); err != nil {
		return nil, 0, 0, fmt.Errorf("count all donors: %w", err)
	}

	args = append(args, limit, (page-1)*limit)
	sql := `SELECT u.id, u.name, p.blood_group, p.gender, p.city, p.available, p.last_donation_date ` +
		sb.String() +
		fmt.Sprintf(` ORDER BY u.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list all donors: %w", err)
	}
	defer rows.Close()

	donors := []models.Donor{}
	for rows.Next() {
		var d models.Donor
		if err := rows.Scan(&d.UserID, &d.Name, &d.BloodGroup, &d.Gender, &d.City, &d.Available, &d.LastDonationDate); err != nil {
			return nil, 0, 0, fmt.Errorf("scan all donors: %w", err)
		}
		donors = append(donors, d)
	}
	return donors, total, totalPages(total, limit), rows.Err()
}

// CountDonorProfiles returns the number of donor profiles with active accounts.
func (s *Store) CountDonorProfiles(ctx context.Context) (int64, error) {
	var n int64
	err := s.Pool().QueryRow(ctx,
		`SELECT count(*) FROM donor_profiles p JOIN users u ON u.id = p.user_id WHERE u.is_active = TRUE`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count donor profiles: %w", err)
	}
	return n, nil
}

func totalPages(total int64, limit int) int {
	if limit <= 0 {
		return 0
	}
	pages := int(total) / limit
	if int(total)%limit != 0 {
		pages++
	}
	return pages
}
