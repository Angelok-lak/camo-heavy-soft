-- ============================================================
-- 007_exam_bookings.sql
-- F-07.1: booking a candidate onto an exam session, plus the
-- absence reasons reference table (D-01: no business value lives
-- in code).
--
-- committed_units freezes the price applied at booking time
-- (F-01 case 6): a later scale change never rewrites history.
-- Credit counters stay DERIVED (C-05): committed = sum over
-- COMMITTED bookings, remaining = allowance - committed.
-- ============================================================

CREATE TYPE exam_test_kind AS ENUM ('OFFROAD', 'ONROAD');
CREATE TYPE exam_booking_status AS ENUM
    ('COMMITTED', 'PASSED', 'FAILED', 'ABSENT', 'WITHDRAWN');

CREATE TABLE exam_booking (
    id              uuid PRIMARY KEY,
    school_id       uuid NOT NULL REFERENCES school(id),
    exam_session_id uuid NOT NULL REFERENCES exam_session(id),
    enrollment_id   uuid NOT NULL REFERENCES enrollment(id),
    test_kind       exam_test_kind NOT NULL,
    status          exam_booking_status NOT NULL DEFAULT 'COMMITTED',
    committed_units integer NOT NULL,
    -- A booking made over missing exam prerequisites is an override,
    -- traced in clear text (the system alerted, the office decided).
    override_note   text,
    booked_by       uuid NOT NULL REFERENCES app_user(id),
    booked_at       timestamptz NOT NULL DEFAULT now(),
    result_by       uuid REFERENCES app_user(id),
    result_at       timestamptz,
    UNIQUE (exam_session_id, enrollment_id, test_kind)
);
CREATE INDEX ON exam_booking (exam_session_id);
CREATE INDEX ON exam_booking (enrollment_id);

-- Absence reasons: a reference list, deactivation only (RG-205).
CREATE TABLE absence_reason (
    id        uuid PRIMARY KEY,
    school_id uuid NOT NULL REFERENCES school(id),
    label     text NOT NULL,
    active    boolean NOT NULL DEFAULT true
);
