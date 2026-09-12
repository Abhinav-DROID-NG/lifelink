// Package models defines the core domain types shared across the store and handlers.
package models

import "time"

// User roles.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User is an application account. Credentials are never serialized to JSON.
type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Phone        string    `json:"phone"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DonorProfile holds extended donor information, one per user.
type DonorProfile struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	BloodGroup      string     `json:"blood_group"`
	DateOfBirth     time.Time  `json:"-"`
	Gender          string     `json:"gender"`
	City            string     `json:"city"`
	Address         string     `json:"-"`
	Available       bool       `json:"available"`
	LastDonationDate *time.Time `json:"last_donation_date"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Donor is the public-facing donor view used by search results and profiles.
// It deliberately excludes email, phone, address and date of birth.
type Donor struct {
	UserID           int64      `json:"user_id"`
	Name             string     `json:"name"`
	BloodGroup       string     `json:"blood_group"`
	Gender           string     `json:"gender"`
	City             string     `json:"city"`
	Available        bool       `json:"available"`
	LastDonationDate *time.Time `json:"last_donation_date"`
}

// Request statuses.
const (
	StatusPending   = "PENDING"
	StatusAccepted  = "ACCEPTED"
	StatusRejected  = "REJECTED"
	StatusCompleted = "COMPLETED"
	StatusCancelled = "CANCELLED"
)

// BloodRequest is a donation request sent from a requester to a donor.
type BloodRequest struct {
	ID            int64     `json:"id"`
	RequesterID   int64     `json:"requester_id"`
	DonorID       int64     `json:"donor_id"`
	BloodGroup    string    `json:"blood_group"`
	PatientName   string    `json:"patient_name"`
	HospitalName  string    `json:"hospital_name"`
	Location      string    `json:"location"`
	RequiredDate  time.Time `json:"required_date"`
	Message       string    `json:"message"`
	ContactNumber string    `json:"contact_number"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Joined rows for convenient display.
	RequesterName string `json:"requester_name"`
	DonorName     string `json:"donor_name"`
}

// Notification types.
const (
	NotifRequestReceived = "REQUEST_RECEIVED"
	NotifRequestAccepted = "REQUEST_ACCEPTED"
	NotifRequestRejected = "REQUEST_REJECTED"
)

// Notification is an in-app alert for a user.
type Notification struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	RequestID *int64    `json:"request_id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// Page is a paginated result set.
type Page struct {
	Items      any   `json:"items"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}