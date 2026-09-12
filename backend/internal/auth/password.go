// Package auth provides password hashing and JWT signing/verification.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// ErrWeakPassword is returned when a password is shorter than the minimum.
var ErrWeakPassword = errors.New("password must be at least 8 characters")

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(plain string) (string, error) {
	if len(plain) < 8 {
		return "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether plain matches the stored bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}