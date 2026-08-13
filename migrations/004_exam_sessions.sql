-- ============================================================
-- 004_exam_sessions.sql
-- F-06, first cut: dated exam sessions. A session ties up its
-- resources for its duration PLUS the round trip to the exam
-- place (RG-140) — the travel time lives on the place.
--
-- Status holds CANCELLED only: "upcoming" and "past" are derived
-- from the date (RG-44), nothing to flip by batch job.
-- ============================================================

CREATE TABLE exam_place (
    id             uuid PRIMARY KEY,
    school_id      uuid NOT NULL REFERENCES school(id),
    label          text NOT NULL,
    travel_minutes integer NOT NULL DEFAULT 0 CHECK (travel_minutes >= 0),
    active         boolean NOT NULL DEFAULT true
);

CREATE TYPE exam_session_status AS ENUM ('SCHEDULED', 'CANCELLED');

CREATE TABLE exam_session (
    id               uuid PRIMARY KEY,
    school_id        uuid NOT NULL REFERENCES school(id),
    exam_place_id    uuid NOT NULL REFERENCES exam_place(id),
    starts_at        timestamptz NOT NULL,
    ends_at          timestamptz NOT NULL,
    -- The ONLY entered limit (RG-36); the credit engine reads it later.
    credit_allowance integer,
    status           exam_session_status NOT NULL DEFAULT 'SCHEDULED',
    cancel_reason    text,
    cancelled_by     uuid REFERENCES app_user(id),
    cancelled_at     timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    slot             tstzrange GENERATED ALWAYS AS (
                         tstzrange(starts_at, ends_at, '[)')
                     ) STORED,
    CHECK (ends_at > starts_at)
);
CREATE INDEX ON exam_session USING gist (school_id, slot) WHERE status = 'SCHEDULED';

CREATE TABLE exam_session_resource (
    exam_session_id uuid NOT NULL REFERENCES exam_session(id) ON DELETE CASCADE,
    resource_id     uuid NOT NULL REFERENCES resource(id),
    PRIMARY KEY (exam_session_id, resource_id)
);
CREATE INDEX ON exam_session_resource (resource_id);
