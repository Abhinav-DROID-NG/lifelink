package store

import (
	"context"
	"errors"
	"fmt"

	"lifelink/internal/models"
)

// CreateNotification inserts an in-app notification for a user.
func (s *Store) CreateNotification(ctx context.Context, userID int64, requestID *int64, notifType, message string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, request_id, type, message)
		VALUES ($1, $2, $3, $4)`, userID, requestID, notifType, message)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

// ListNotifications lists a user's notifications, unread first, paginated.
func (s *Store) ListNotifications(ctx context.Context, userID int64, limit int) ([]models.Notification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, request_id, type, message, read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY read ASC, created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	notifs := []models.Notification{}
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.RequestID, &n.Type, &n.Message, &n.Read, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		notifs = append(notifs, n)
	}
	return notifs, rows.Err()
}

// MarkNotificationRead marks one notification read, scoped to the user.
func (s *Store) MarkNotificationRead(ctx context.Context, userID, notifID int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2`, notifID, userID)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("notification not found")
	}
	return nil
}

// MarkAllNotificationsRead marks every notification read for a user.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications SET read = TRUE WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// UnreadNotificationCount returns how many unseen notifications a user has.
func (s *Store) UnreadNotificationCount(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND read = FALSE`, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return n, nil
}
