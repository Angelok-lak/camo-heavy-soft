-- ============================================================
-- 009_seat_requests.sql
-- F-33: the seat request — born from a real incident (a forgotten
-- request, Q-12). The generation is a stored FACT: the reminder
-- task derives from its absence.
--
-- Q-139 (the real deadline: the 5th of M-2 vs the 25th for M+2)
-- is still OPEN. The deadline day is therefore a PARAMETER with a
-- default of 25 (D-01: no business value in code), to fix the day
-- the field answers.
-- ============================================================

ALTER TABLE school ADD COLUMN seat_request_deadline_day
    integer NOT NULL DEFAULT 25
    CHECK (seat_request_deadline_day BETWEEN 1 AND 28);

CREATE TABLE seat_request (
    id           uuid PRIMARY KEY,
    school_id    uuid NOT NULL REFERENCES school(id),
    -- First day of the month the seats are requested FOR.
    target_month date NOT NULL,
    -- The counted lines, frozen at generation (the trace of what was sent).
    lines        jsonb NOT NULL,
    generated_by uuid NOT NULL REFERENCES app_user(id),
    generated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (school_id, target_month)
);
