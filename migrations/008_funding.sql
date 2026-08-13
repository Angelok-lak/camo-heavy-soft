-- ============================================================
-- 008_funding.sql
-- F-05 within D-02's limit: the funding file carries the FUNDER
-- and the COVERAGE STATUS, nothing money-shaped. One file per
-- enrollment; every state change is a traced transition, and a
-- rejection requires its reason (RG-189).
-- ============================================================

CREATE TABLE funder_kind (
    id        uuid PRIMARY KEY,
    school_id uuid NOT NULL REFERENCES school(id),
    label     text NOT NULL,
    -- Self-funding follows its own cycle (RG-15); flagged, not hardcoded.
    self_funded boolean NOT NULL DEFAULT false,
    active    boolean NOT NULL DEFAULT true
);

CREATE TYPE funding_status AS ENUM
    ('DRAFT', 'SUBMITTED', 'APPROVED', 'SETTLED', 'REJECTED');

CREATE TABLE funding (
    id             uuid PRIMARY KEY,
    school_id      uuid NOT NULL REFERENCES school(id),
    enrollment_id  uuid NOT NULL UNIQUE REFERENCES enrollment(id),
    funder_kind_id uuid REFERENCES funder_kind(id),
    status         funding_status NOT NULL DEFAULT 'DRAFT',
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE funding_transition (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    funding_id  uuid NOT NULL REFERENCES funding(id),
    from_status funding_status NOT NULL,
    to_status   funding_status NOT NULL,
    reason      text,
    author_id   uuid NOT NULL REFERENCES app_user(id),
    at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON funding_transition (funding_id);
