package store

import (
	"context"
	"fmt"
)

// DashboardStats is the admin dashboard summary.
type DashboardStats struct {
	TotalDonors      int64             `json:"total_donors"`
	AvailableDonors  int64             `json:"available_donors"`
	TotalRequests    int64             `json:"total_requests"`
	PendingRequests  int64             `json:"pending_requests"`
	TotalUsers       int64             `json:"total_users"`
	BloodGroupCounts []BloodGroupCount `json:"blood_group_counts"`
}

// BloodGroupCount is one row of the blood-group distribution.
type BloodGroupCount struct {
	BloodGroup string `json:"blood_group"`
	Count      int64  `json:"count"`
}

// DashboardStats computes the admin dashboard summary.
func (s *Store) DashboardStats(ctx context.Context) (*DashboardStats, error) {
	st := &DashboardStats{}
	if err := s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM donor_profiles p JOIN users u ON u.id = p.user_id WHERE u.is_active = TRUE),
		       (SELECT count(*) FROM donor_profiles WHERE available = TRUE),
		       (SELECT count(*) FROM blood_requests),
		       (SELECT count(*) FROM blood_requests WHERE status = 'PENDING'),
		       (SELECT count(*) FROM users)`).
		Scan(&st.TotalDonors, &st.AvailableDonors, &st.TotalRequests, &st.PendingRequests, &st.TotalUsers); err != nil {
		return nil, fmt.Errorf("query dashboard stats: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT blood_group, count(*) FROM donor_profiles
		GROUP BY blood_group ORDER BY count(*) DESC, blood_group`)
	if err != nil {
		return nil, fmt.Errorf("query blood group counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c BloodGroupCount
		if err := rows.Scan(&c.BloodGroup, &c.Count); err != nil {
			return nil, fmt.Errorf("scan blood group count: %w", err)
		}
		st.BloodGroupCounts = append(st.BloodGroupCounts, c)
	}
	return st, rows.Err()
}