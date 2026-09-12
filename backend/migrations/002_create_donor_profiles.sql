-- 002_create_donor_profiles.sql
-- Extended donor information, one row per user who chooses to donate.

CREATE TABLE donor_profiles (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT      NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    blood_group        TEXT        NOT NULL CHECK (blood_group IN ('A+', 'A-', 'B+', 'B-', 'AB+', 'AB-', 'O+', 'O-')),
    date_of_birth      DATE        NOT NULL,
    gender             TEXT        NOT NULL CHECK (gender IN ('male', 'female', 'other')),
    city               TEXT        NOT NULL CHECK (city <> ''),
    address            TEXT        NOT NULL DEFAULT '',
    available          BOOLEAN     NOT NULL DEFAULT TRUE,
    last_donation_date DATE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A donor must be a legal donor age (18 or older).
    CONSTRAINT chk_donor_age CHECK (date_of_birth <= (now() AT TIME ZONE 'UTC')::date - INTERVAL '18 years')
);

-- Search is the primary read path: blood group, city and availability.
CREATE INDEX donor_profiles_blood_group_idx ON donor_profiles (blood_group);
CREATE INDEX donor_profiles_city_idx       ON donor_profiles (city);
CREATE INDEX donor_profiles_available_idx  ON donor_profiles (available) WHERE available;