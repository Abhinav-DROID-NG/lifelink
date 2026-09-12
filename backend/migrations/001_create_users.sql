-- 001_create_users.sql
-- Application users (donors and administrators).

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL CHECK (char_length(name) BETWEEN 2 AND 100),
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    phone         TEXT        NOT NULL CHECK (phone <> ''),
    role          TEXT        NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive unique email, without the citext extension.
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE INDEX users_role_idx ON users (role);