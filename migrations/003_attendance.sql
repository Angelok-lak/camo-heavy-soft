-- ============================================================
-- 003_attendance.sql
-- Slice 2: F-12 attendance, its correction trace, and the hour
-- counters as ONE view — counters are queries, never columns
-- (C-05, RG-122). Every reader (F-13, F-10, US-32) goes through
-- enrollment_hours: two implementations would eventually disagree.
-- ============================================================

CREATE TYPE attendance_value AS ENUM ('PRESENT', 'EXCUSED', 'UNEXCUSED');

-- One row per recorded assignment. UNRECORDED is the ABSENCE of a row
-- (RG-123): nothing to flip by batch job when a lesson passes (same
-- pattern as exam results, RG-112).
CREATE TABLE attendance (
    id                   uuid PRIMARY KEY,
    school_id            uuid NOT NULL REFERENCES school(id),
    lesson_assignment_id uuid NOT NULL UNIQUE REFERENCES lesson_assignment(id),
    value                attendance_value NOT NULL,
    reason               text,
    recorded_by          uuid NOT NULL REFERENCES app_user(id),
    recorded_at          timestamptz NOT NULL DEFAULT now()
);

-- A correction is a FACT to keep, not an overwrite (RG-20).
CREATE TABLE attendance_correction (
    id             uuid PRIMARY KEY,
    school_id      uuid NOT NULL REFERENCES school(id),
    attendance_id  uuid NOT NULL REFERENCES attendance(id),
    previous_value attendance_value NOT NULL,
    new_value      attendance_value NOT NULL,
    corrected_by   uuid NOT NULL REFERENCES app_user(id),
    corrected_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON attendance_correction (attendance_id);

-- The three counters and the two projections (RG-172, RG-180, RG-119).
-- Each present student counts the FULL lesson duration (RG-33).
-- Projected = consumed + planned lessons before the target date that are
-- not yet recorded. NULL projection when no target date is set: nothing
-- meaningful to compare.
CREATE VIEW enrollment_hours AS
SELECT
    e.id AS enrollment_id,
    e.school_id,
    e.hours_before_offroad,
    e.total_hours,
    e.offroad_target_date,
    e.onroad_target_date,
    COALESCE(rec.attended, 0)  AS attended_hours,
    COALESCE(rec.excused, 0)   AS excused_hours,
    COALESCE(rec.unexcused, 0) AS unexcused_hours,
    COALESCE(rec.attended, 0) + COALESCE(rec.excused, 0)
        + COALESCE(rec.unexcused, 0) AS consumed_hours,
    CASE WHEN e.offroad_target_date IS NOT NULL THEN
        COALESCE(rec.attended, 0) + COALESCE(rec.excused, 0)
            + COALESCE(rec.unexcused, 0)
            + COALESCE(pl.before_offroad, 0)
    END AS projected_offroad_hours,
    CASE WHEN e.onroad_target_date IS NOT NULL THEN
        COALESCE(rec.attended, 0) + COALESCE(rec.excused, 0)
            + COALESCE(rec.unexcused, 0)
            + COALESCE(pl.before_onroad, 0)
    END AS projected_onroad_hours
FROM enrollment e
LEFT JOIN LATERAL (
    SELECT
        sum(EXTRACT(EPOCH FROM l.ends_at - l.starts_at) / 3600.0)
            FILTER (WHERE a.value = 'PRESENT')   AS attended,
        sum(EXTRACT(EPOCH FROM l.ends_at - l.starts_at) / 3600.0)
            FILTER (WHERE a.value = 'EXCUSED')   AS excused,
        sum(EXTRACT(EPOCH FROM l.ends_at - l.starts_at) / 3600.0)
            FILTER (WHERE a.value = 'UNEXCUSED') AS unexcused
    FROM lesson_assignment la
    JOIN attendance a ON a.lesson_assignment_id = la.id
    JOIN lesson l ON l.id = la.lesson_id
    WHERE la.enrollment_id = e.id
) rec ON true
LEFT JOIN LATERAL (
    SELECT
        sum(EXTRACT(EPOCH FROM l.ends_at - l.starts_at) / 3600.0)
            FILTER (WHERE l.starts_at::date <= e.offroad_target_date) AS before_offroad,
        sum(EXTRACT(EPOCH FROM l.ends_at - l.starts_at) / 3600.0)
            FILTER (WHERE l.starts_at::date <= e.onroad_target_date)  AS before_onroad
    FROM lesson_assignment la
    JOIN lesson l ON l.id = la.lesson_id
    WHERE la.enrollment_id = e.id
      AND l.status = 'PLANNED'
      AND NOT EXISTS (SELECT 1 FROM attendance a
                      WHERE a.lesson_assignment_id = la.id)
) pl ON true;
