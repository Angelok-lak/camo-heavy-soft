-- ============================================================
-- 001_slice1_schema.sql
-- Slice 1 foundation: planning, resources, students, editing
-- PostgreSQL 15+
--
-- References: MODELE_DONNEES.md (C-01..C-30), FEATURES.md (RG-xx)
-- Naming follows GLOSSARY.md. Specs are in French, code is in English.
-- IDs are UUID v7 generated application-side (C-02).
-- Every business table carries school_id, filtered at data access (C-03, RG-209).
-- ============================================================

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ============================================================
-- 1. Organisation
-- ============================================================

CREATE TABLE school (
    id          uuid PRIMARY KEY,
    label       text NOT NULL,
    timezone    text NOT NULL DEFAULT 'Europe/Paris',
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Live parameter (C-07). Bounds suggestions and yields an overridable
-- conflict, never a refusal (RG-138, D-04).
CREATE TABLE opening_hours (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    weekday     smallint NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    starts_at   time NOT NULL,
    ends_at     time NOT NULL,
    CHECK (ends_at > starts_at)
);
CREATE INDEX ON opening_hours (school_id, weekday);

-- Public holidays and school closures (RG-178)
CREATE TABLE closure (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    period      daterange NOT NULL,
    reason      text NOT NULL,
    EXCLUDE USING gist (school_id WITH =, period WITH &&)
);

-- ============================================================
-- 2. Accounts and profiles (F-17)
-- ============================================================

CREATE TYPE account_status AS ENUM ('ACTIVE', 'SUSPENDED', 'ARCHIVED');
CREATE TYPE profile AS ENUM ('MANAGEMENT', 'OFFICE', 'INSTRUCTOR');

-- An account is never deleted: it authors thousands of traces that must
-- stay readable (RG-211).
CREATE TABLE app_user (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    login       text NOT NULL,
    display_name text NOT NULL,
    secret_hash text NOT NULL,
    status      account_status NOT NULL DEFAULT 'ACTIVE',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (school_id, login)
);

-- Historised: yesterday's rights stay readable (RG-213).
-- Effective rights = union of profiles whose period is open (RG-208).
CREATE TABLE user_profile (
    id          uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES app_user(id),
    profile     profile NOT NULL,
    starts_at   timestamptz NOT NULL DEFAULT now(),
    ends_at     timestamptz,
    granted_by  uuid REFERENCES app_user(id)
);
CREATE INDEX ON user_profile (user_id) WHERE ends_at IS NULL;

-- ============================================================
-- 3. Resources (F-15, F-16)
-- One identity table, two satellites (C-08)
-- ============================================================

CREATE TYPE resource_kind AS ENUM ('VEHICLE', 'ROOM', 'TRAINING_PAD', 'INSTRUCTOR');
CREATE TYPE resource_status AS ENUM ('ACTIVE', 'ARCHIVED');

CREATE TABLE resource (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    kind        resource_kind NOT NULL,
    label       text NOT NULL,
    status      resource_status NOT NULL DEFAULT 'ACTIVE',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON resource (school_id, kind, status);

CREATE TABLE licence_category (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    code        text NOT NULL,
    active      boolean NOT NULL DEFAULT true,
    UNIQUE (school_id, code)
);

CREATE TABLE resource_vehicle (
    resource_id uuid PRIMARY KEY REFERENCES resource(id),
    -- INDICATIVE only: never constrains the student count (RG-135)
    indicative_capacity smallint
);

CREATE TABLE resource_vehicle_category (
    resource_id uuid NOT NULL REFERENCES resource_vehicle(resource_id),
    category_id uuid NOT NULL REFERENCES licence_category(id),
    PRIMARY KEY (resource_id, category_id)
);

CREATE TABLE resource_instructor (
    resource_id uuid PRIMARY KEY REFERENCES resource(id),
    -- An instructor may exist with no account (RG-76, RG-210)
    user_id     uuid UNIQUE REFERENCES app_user(id)
);

-- INFORMATIONAL only, produce no conflict whatsoever (RG-75, RG-139)
CREATE TABLE instructor_hours (
    id          uuid PRIMARY KEY,
    resource_id uuid NOT NULL REFERENCES resource_instructor(resource_id),
    weekday     smallint NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    starts_at   time NOT NULL,
    ends_at     time NOT NULL
);

CREATE TYPE unavailability_status AS ENUM ('ONGOING', 'ENDED');

-- One table for every resource kind: that is the payoff of C-08.
-- ends_at NULL = return date unknown (RG-65); surfaces as a warning past
-- N days (RG-67). Overlaps are allowed, no automatic merge (RG-79).
CREATE TABLE unavailability (
    id            uuid PRIMARY KEY,
    school_id     uuid NOT NULL REFERENCES school(id),
    resource_id   uuid NOT NULL REFERENCES resource(id),
    reason        text NOT NULL,
    starts_at     timestamptz NOT NULL,
    ends_at       timestamptz,
    status        unavailability_status NOT NULL DEFAULT 'ONGOING',
    declared_by   uuid NOT NULL REFERENCES app_user(id),
    declared_at   timestamptz NOT NULL DEFAULT now(),
    restored_by   uuid REFERENCES app_user(id),
    restored_at   timestamptz,
    period        tstzrange GENERATED ALWAYS AS (
                      tstzrange(starts_at, ends_at, '[)')
                  ) STORED
);
CREATE INDEX ON unavailability USING gist (resource_id, period)
    WHERE status = 'ONGOING';

-- ============================================================
-- 4. People and enrollments (F-02, F-04 - reduced scope)
-- ============================================================

CREATE TYPE person_status AS ENUM ('ACTIVE', 'ARCHIVED');

-- The person OUTLIVES their enrollments: NEPH and documents belong to
-- them, which is what lets a former student return with no re-entry (RG-182).
CREATE TABLE person (
    id            uuid PRIMARY KEY,
    school_id     uuid NOT NULL REFERENCES school(id),
    last_name     text NOT NULL,
    first_names   text NOT NULL,
    date_of_birth date,
    phone         text,
    email         text,
    status        person_status NOT NULL DEFAULT 'ACTIVE',
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON person (school_id, last_name, first_names);

CREATE TABLE objective (
    id                  uuid PRIMARY KEY,
    school_id           uuid NOT NULL REFERENCES school(id),
    label               text NOT NULL,
    hours_before_offroad numeric(5,2) NOT NULL,
    total_hours         numeric(5,2) NOT NULL,
    active              boolean NOT NULL DEFAULT true,
    CHECK (total_hours >= hours_before_offroad)
);

CREATE TYPE enrollment_life_status AS ENUM ('ACTIVE', 'CLOSED');

-- Thresholds are COPIED from the objective, with no live link (C-07, RG-42).
-- The ABSENCE of a foreign key is what states they are copied.
CREATE TABLE enrollment (
    id                   uuid PRIMARY KEY,
    school_id            uuid NOT NULL REFERENCES school(id),
    person_id            uuid NOT NULL REFERENCES person(id),
    objective_id         uuid NOT NULL REFERENCES objective(id),
    hours_before_offroad numeric(5,2) NOT NULL,
    total_hours          numeric(5,2) NOT NULL,
    offroad_target_date  date,
    onroad_target_date   date,
    life_status          enrollment_life_status NOT NULL DEFAULT 'ACTIVE',
    close_reason         text,
    closed_by            uuid REFERENCES app_user(id),
    closed_at            timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    -- The off-road test always precedes the on-road one (RG-24)
    CHECK (onroad_target_date IS NULL
           OR offroad_target_date IS NULL
           OR onroad_target_date >= offroad_target_date)
);

-- One ACTIVE enrollment per person (RG-115)
CREATE UNIQUE INDEX ON enrollment (person_id) WHERE life_status = 'ACTIVE';

-- ============================================================
-- 5. Planning (F-09, F-03)
-- ============================================================

CREATE TABLE lesson_kind (
    id               uuid PRIMARY KEY,
    school_id        uuid NOT NULL REFERENCES school(id),
    label            text NOT NULL,
    -- Warning when forgotten, never a block (RG-149)
    requires_vehicle boolean NOT NULL DEFAULT false,
    active           boolean NOT NULL DEFAULT true
);

CREATE TABLE standard_lesson_duration (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    minutes     integer NOT NULL CHECK (minutes > 0),
    active      boolean NOT NULL DEFAULT true
);

-- Plain grouping, carries NO business rule of its own (RG-155)
CREATE TABLE course (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    label       text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE lesson_status AS ENUM ('PLANNED', 'CANCELLED');

-- Deliberately reduced to two states: completion is read from attendance,
-- and a reschedule is a move, not a state (phase 1 decision).
CREATE TABLE lesson (
    id              uuid PRIMARY KEY,
    school_id       uuid NOT NULL REFERENCES school(id),
    course_id       uuid REFERENCES course(id),
    starts_at       timestamptz NOT NULL,
    ends_at         timestamptz NOT NULL,
    max_students    smallint,
    status          lesson_status NOT NULL DEFAULT 'PLANNED',
    cancel_reason   text,
    cancelled_by    uuid REFERENCES app_user(id),
    cancelled_at    timestamptz,
    version         integer NOT NULL DEFAULT 1,
    created_at      timestamptz NOT NULL DEFAULT now(),
    slot            tstzrange GENERATED ALWAYS AS (
                        tstzrange(starts_at, ends_at, '[)')
                    ) STORED,
    CHECK (ends_at > starts_at)
);
CREATE INDEX ON lesson USING gist (school_id, slot) WHERE status = 'PLANNED';

-- Zero, one or many kinds. The kind is INFORMATIONAL (RG-134)
CREATE TABLE lesson_lesson_kind (
    lesson_id       uuid NOT NULL REFERENCES lesson(id) ON DELETE CASCADE,
    lesson_kind_id  uuid NOT NULL REFERENCES lesson_kind(id),
    PRIMARY KEY (lesson_id, lesson_kind_id)
);

CREATE TABLE lesson_resource (
    lesson_id   uuid NOT NULL REFERENCES lesson(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES resource(id),
    PRIMARY KEY (lesson_id, resource_id)
);
CREATE INDEX ON lesson_resource (resource_id);

-- Stateless: it exists or it does not (phase 2)
CREATE TABLE lesson_assignment (
    id            uuid PRIMARY KEY,
    lesson_id     uuid NOT NULL REFERENCES lesson(id),
    enrollment_id uuid NOT NULL REFERENCES enrollment(id),
    assigned_by   uuid NOT NULL REFERENCES app_user(id),
    assigned_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (lesson_id, enrollment_id)
);
CREATE INDEX ON lesson_assignment (enrollment_id);

-- ============================================================
-- 6. Edit session - the draft (C-09)
-- ============================================================

CREATE TYPE edit_session_status AS ENUM ('OPEN', 'SAVED', 'DISCARDED');

-- The draft is a JOURNAL OF INTENTIONS, not a copy of the planner.
-- Displayed state = saved state + operations replayed in memory.
-- This row's id IS the batch id of RG-70.
CREATE TABLE edit_session (
    id                  uuid PRIMARY KEY,
    school_id           uuid NOT NULL REFERENCES school(id),
    user_id             uuid NOT NULL REFERENCES app_user(id),
    scope_starts_at     timestamptz NOT NULL,
    scope_ends_at       timestamptz NOT NULL,
    status              edit_session_status NOT NULL DEFAULT 'OPEN',
    -- Persistence option B: the draft survives a disconnect (C-14)
    operations          jsonb NOT NULL DEFAULT '[]'::jsonb,
    -- Fingerprint of the scope at opening, to detect it moved (RG-259)
    state_at_open       text,
    opened_at           timestamptz NOT NULL DEFAULT now(),
    last_activity_at    timestamptz NOT NULL DEFAULT now(),
    closed_at           timestamptz,
    force_released_by   uuid REFERENCES app_user(id),
    scope               tstzrange GENERATED ALWAYS AS (
                            tstzrange(scope_starts_at, scope_ends_at, '[)')
                        ) STORED
);

-- THE LOCK IS THIS CONSTRAINT (RG-144).
-- Guaranteed by the database, not by application code: two open edit
-- sessions cannot overlap on the same school, whatever the caller does.
ALTER TABLE edit_session
    ADD CONSTRAINT exclusive_edit_lock
    EXCLUDE USING gist (school_id WITH =, scope WITH &&)
    WHERE (status = 'OPEN');

-- A conflict detected then overridden is a FACT, so it is stored (A-08)
CREATE TABLE overridden_conflict (
    id              uuid PRIMARY KEY,
    school_id       uuid NOT NULL REFERENCES school(id),
    edit_session_id uuid REFERENCES edit_session(id),
    conflict_kind   text NOT NULL,
    parties         jsonb NOT NULL,
    overridden_by   uuid NOT NULL REFERENCES app_user(id),
    overridden_at   timestamptz NOT NULL DEFAULT now()
);

-- ============================================================
-- 7. Business traceability (C-10)
-- ============================================================

-- Foundation of D-05: dashboard counters are computed from here, so no
-- feature has to plan its own indicator storage.
CREATE TABLE domain_event (
    id          uuid PRIMARY KEY,
    school_id   uuid NOT NULL REFERENCES school(id),
    kind        text NOT NULL,
    subject_kind text NOT NULL,
    subject_id  uuid NOT NULL,
    payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
    batch_id    uuid,
    author_id   uuid REFERENCES app_user(id),
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON domain_event (school_id, subject_kind, subject_id, occurred_at DESC);
CREATE INDEX ON domain_event (school_id, kind, occurred_at DESC);
CREATE INDEX ON domain_event (batch_id) WHERE batch_id IS NOT NULL;
