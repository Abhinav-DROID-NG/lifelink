-- 003_create_blood_requests.sql
-- Blood donation requests sent by a user to a donor.

CREATE TYPE request_status AS ENUM (
    'PENDING',
    'ACCEPTED',
    'REJECTED',
    'COMPLETED',
    'CANCELLED'
);

CREATE TABLE blood_requests (
    id              BIGSERIAL PRIMARY KEY,
    requester_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    donor_id        BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    blood_group     TEXT        NOT NULL CHECK (blood_group IN ('A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-')),
    patient_name    TEXT        NOT NULL CHECK (patient_name <> ''),
    hospital_name   TEXT        NOT NULL CHECK (hospital_name <> ''),
    location        TEXT        NOT NULL CHECK (location <> ''),
    required_date   DATE        NOT NULL,
    message         TEXT        NOT NULL DEFAULT '',
    contact_number  TEXT        NOT NULL CHECK (contact_number <> ''),
    status          request_status NOT NULL DEFAULT 'PENDING',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A user cannot request themselves.
    CONSTRAINT chk_requester_not_donor CHECK (requester_id <> donor_id)
);

-- Only one active (PENDING) request between the same pair of users.
CREATE UNIQUE INDEX blood_requests_active_pair_idx
    ON blood_requests (requester_id, donor_id)
    WHERE status = 'PENDING';

CREATE INDEX blood_requests_donor_idx       ON blood_requests (donor_id);
CREATE INDEX blood_requests_requester_idx   ON blood_requests (requester_id);
CREATE INDEX blood_requests_status_idx      ON blood_requests (status);