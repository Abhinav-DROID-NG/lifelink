// Package seed populates the database with demo users, donors and requests.
package seed

import (
	"context"
	"log/slog"
	"time"

	"lifelink/internal/auth"
	"lifelink/internal/models"
	"lifelink/internal/store"
)

type seedUser struct {
	name     string
	email    string
	password string
	phone    string
	role     string
	blood    string
	dob      string
	gender   string
	city     string
	address  string
	avail    bool
	last     string
}

var demoUsers = []seedUser{
	{name: "Admin", email: "admin@lifelink.app", password: "Admin@123", phone: "+919800000001", role: "admin"},
	{name: "Rahul Kumar", email: "rahul@example.com", password: "Pass@123", phone: "+919800000002", blood: "O+", dob: "1995-04-12", gender: "male", city: "Mangaluru", address: "22 Cadaba Street, Mangaluru", avail: true, last: "2026-06-20"},
	{name: "Anita Shetty", email: "anita@example.com", password: "Pass@123", phone: "+919800000003", blood: "A+", dob: "1998-08-03", gender: "female", city: "Mangaluru", address: "8 Bejai Main Road, Mangaluru", avail: true},
	{name: "Vikram Hegde", email: "vikram@example.com", password: "Pass@123", phone: "+919800000004", blood: "B+", dob: "1992-01-25", gender: "male", city: "Mangaluru", address: "14 Falnir Road, Mangaluru", avail: false},
	{name: "Sneha Rao", email: "sneha@example.com", password: "Pass@123", phone: "+919800000005", blood: "AB+", dob: "1996-11-14", gender: "female", city: "Bengaluru", address: "210 Residency Road, Bengaluru", avail: true, last: "2026-05-02"},
	{name: "Arjun Nair", email: "arjun@example.com", password: "Pass@123", phone: "+919800000006", blood: "O-", dob: "1993-07-09", gender: "male", city: "Kochi", address: "5 MG Road, Kochi", avail: true, last: "2026-04-10"},
	{name: "Priya Patil", email: "priya@example.com", password: "Pass@123", phone: "+919800000007", blood: "A-", dob: "1999-02-28", gender: "female", city: "Pune", address: "33 FC Road, Pune", avail: false},
	{name: "Mohammed Faisal", email: "faisal@example.com", password: "Pass@123", phone: "+919800000008", blood: "B-", dob: "1991-12-05", gender: "male", city: "Hyderabad", address: "77 Banjara Hills, Hyderabad", avail: true},
	{name: "Kavya Menon", email: "kavya@example.com", password: "Pass@123", phone: "+919800000009", blood: "AB-", dob: "1997-06-30", gender: "female", city: "Bengaluru", address: "4 Indiranagar 100ft Road, Bengaluru", avail: true, last: "2026-03-15"},
	{name: "Rohit Sharma", email: "rohit@example.com", password: "Pass@123", phone: "+919800000010", blood: "O+", dob: "1994-09-19", gender: "male", city: "Delhi", address: "12 Lajpat Nagar, Delhi", avail: true},
	{name: "Divya Krishnan", email: "divya@example.com", password: "Pass@123", phone: "+919800000011", blood: "B+", dob: "1995-03-22", gender: "female", city: "Chennai", address: "61 T Nagar, Chennai", avail: true, last: "2026-02-08"},
	{name: "Suresh Gowda", email: "suresh@example.com", password: "Pass@123", phone: "+919800000012", blood: "A+", dob: "1990-05-17", gender: "male", city: "Mangaluru", address: "3 Bendoor, Mangaluru", avail: false},
}

// Run seeds the database when it is empty. Safe to call repeatedly.
func Run(ctx context.Context, st *store.Store, logger *slog.Logger) error {
	var count int64
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		logger.Info("seed skipped, users already present")
		return nil
	}

	ids := map[string]int64{}
	for _, du := range demoUsers {
		hash, err := auth.HashPassword(du.password)
		if err != nil {
			return err
		}

		user := &models.User{Name: du.name, Email: du.email, PasswordHash: hash, Phone: du.phone, Role: models.RoleUser}
		if du.role == "admin" {
			user.Role = models.RoleAdmin
		}
		if du.role == "admin" {
			if err := st.CreateUser(ctx, user); err != nil {
				return err
			}
			ids[du.email] = user.ID
			continue
		}

		dob, _ := time.Parse("2006-01-02", du.dob)
		profile := &models.DonorProfile{
			BloodGroup: du.blood, DateOfBirth: dob, Gender: du.gender,
			City: du.city, Address: du.address, Available: du.avail,
		}
		if du.last != "" {
			d, _ := time.Parse("2006-01-02", du.last)
			profile.LastDonationDate = &d
		}
		if err := st.CreateUserWithProfile(ctx, user, profile); err != nil {
			return err
		}
		ids[du.email] = user.ID
	}

	if err := seedRequests(ctx, st, ids); err != nil {
		return err
	}
	logger.Info("seed complete", "users", len(demoUsers))
	return nil
}

func seedRequests(ctx context.Context, st *store.Store, ids map[string]int64) error {
	type seedReq struct {
		requester, donor         string
		blood, patient, hospital string
		location, date, contact  string
		message, status          string
		notify, notifyMsg        string
	}
	reqs := []seedReq{
		{requester: "anita@example.com", donor: "rahul@example.com", blood: "O+", patient: "Devraj Shetty", hospital: "KD Hospital", location: "Mangaluru", date: "2026-09-20", message: "Urgent need for surgery on the 20th.", contact: "+919812345671", status: "PENDING"},
		{requester: "rahul@example.com", donor: "sneha@example.com", blood: "AB+", patient: "Meera Kumar", hospital: "Aster CMI", location: "Bengaluru", date: "2026-09-14", message: "Thank you for helping us.", contact: "+919812345672", status: "ACCEPTED", notify: "rahul@example.com", notifyMsg: "Sneha Rao accepted your blood request."},
		{requester: "priya@example.com", donor: "arjun@example.com", blood: "O-", patient: "Nikhil Patil", hospital: "Ruby Hall", location: "Pune", date: "2026-08-30", message: "Family emergency.", contact: "+919812345673", status: "COMPLETED"},
		{requester: "divya@example.com", donor: "rohit@example.com", blood: "O+", patient: "Akash Krishnan", hospital: "Apollo", location: "Chennai", date: "2026-08-05", message: "Scheduled transfusion.", contact: "+919812345674", status: "CANCELLED"},
	}

	for _, r := range reqs {
		date, _ := time.Parse("2006-01-02", r.date)
		req := &models.BloodRequest{
			RequesterID: ids[r.requester], DonorID: ids[r.donor], BloodGroup: r.blood,
			PatientName: r.patient, HospitalName: r.hospital, Location: r.location,
			RequiredDate: date, Message: r.message, ContactNumber: r.contact, Status: r.status,
		}
		if err := st.CreateRequest(ctx, req); err != nil {
			return err
		}
		if r.status != "PENDING" {
			if _, err := st.UpdateRequestStatus(ctx, req.ID, r.status); err != nil {
				return err
			}
		}
		if r.notify != "" {
			if err := st.CreateNotification(ctx, ids[r.notify], &req.ID, models.NotifRequestAccepted, r.notifyMsg); err != nil {
				return err
			}
		}
	}

	// Notification for Anita's pending request to Rahul.
	msg := "Anita Shetty requested your O+ blood donation."
	if err := st.CreateNotification(ctx, ids["rahul@example.com"], nil, models.NotifRequestReceived, msg); err != nil {
		return err
	}
	return nil
}
