package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"lifelink/internal/models"
)

// ErrEmailTaken indicates a duplicate email address.
var ErrEmailTaken = errors.New("email already exists")

// CreateUser inserts a new user. It returns ErrEmailTaken on duplicates.
func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash, phone, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		u.Name, u.Email, u.PasswordHash, u.Phone, u.Role,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrEmailTaken
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUserByEmail finds a user by case-insensitive email.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, email, password_hash, phone, role, is_active, created_at, updated_at
		FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Phone, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetUserByID finds a user by primary key.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	u := &models.User{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, email, password_hash, phone, role, is_active, created_at, updated_at
		FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Phone, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// UpdateUserActive enables or disables an account.
func (s *Store) UpdateUserActive(ctx context.Context, id int64, active bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	if err != nil {
		return fmt.Errorf("update user active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListUsers returns a paginated, searchable list of users (admin view).
func (s *Store) ListUsers(ctx context.Context, search string, page, limit int) ([]models.User, int64, error) {
	var sb strings.Builder
	sb.WriteString(`FROM users WHERE 1=1`)
	args := []any{}
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		sb.WriteString(fmt.Sprintf(` AND (lower(name) LIKE $%d OR lower(email) LIKE $%d)`, len(args), len(args)))
	}

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+sb.String(), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	args = append(args, limit, (page-1)*limit)
	sql := `SELECT id, name, email, password_hash, phone, role, is_active, created_at, updated_at ` +
		sb.String() +
		fmt.Sprintf(` ORDER BY id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Phone, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}