-- ============================================================
-- 014_exam_results.sql
-- F-07: results and the credit engine's missing half.
-- A corrected result is a FACT to keep (RG-106), never an
-- overwrite. The off-road pass validity (RG-25, A-16: one year,
-- adjustable) is a school parameter; the EXPIRY itself is always
-- derived — changing the parameter must propagate (C-05).
-- ============================================================

CREATE TABLE exam_booking_correction (
    id              uuid PRIMARY KEY,
    school_id       uuid NOT NULL REFERENCES school(id),
    exam_booking_id uuid NOT NULL REFERENCES exam_booking(id),
    previous_status exam_booking_status NOT NULL,
    new_status      exam_booking_status NOT NULL,
    corrected_by    uuid NOT NULL REFERENCES app_user(id),
    corrected_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON exam_booking_correction (exam_booking_id);

ALTER TABLE school ADD COLUMN offroad_validity_months
    integer NOT NULL DEFAULT 12 CHECK (offroad_validity_months > 0);
