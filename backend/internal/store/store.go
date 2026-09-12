// Package store contains all PostgreSQL data access.
package store

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides typed data access over a shared connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps a connection pool in a Store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool exposes the underlying pool for transactions and seeding.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
