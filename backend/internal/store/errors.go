package store

import "errors"

// Sentinel errors surfaced by the store layer.
var (
	ErrNotFound        = errors.New("record not found")
	ErrDuplicateActive = errors.New("an active request already exists for this pair")
)