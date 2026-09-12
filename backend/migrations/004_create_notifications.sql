-- 004_create_notifications.sql
-- In-app notifications shown to donors and requesters.

CREATE TABLE notifications (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    request_id BIGINT      REFERENCES blood_requests (id) ON DELETE CASCADE,
    type       TEXT        NOT NULL CHECK (type IN ('REQUEST_RECEIVED', 'REQUEST_ACCEPTED', 'REQUEST_REJECTED')),
    message    TEXT        NOT NULL CHECK (message <> ''),
    read       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The notifications screen always lists a single user's unread-first.
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);