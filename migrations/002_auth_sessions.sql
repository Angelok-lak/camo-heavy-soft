-- ============================================================
-- 002_auth_sessions.sql
-- F-17: server-side sessions. The token lives in an HttpOnly
-- cookie; only its SHA-256 lands here. A row is revocable, so a
-- suspension (RG-211) takes effect on the very next request.
-- ============================================================

CREATE TABLE auth_session (
    id            uuid PRIMARY KEY,
    school_id     uuid NOT NULL REFERENCES school(id),
    user_id       uuid NOT NULL REFERENCES app_user(id),
    token_hash    bytea NOT NULL UNIQUE,
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);

CREATE INDEX ON auth_session (user_id) WHERE revoked_at IS NULL;
