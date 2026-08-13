-- ============================================================
-- 005_person_neph.sql
-- F-02 grows: the NEPH belongs to the PERSON, not the enrollment
-- (RG-182) — that is what lets a returning student keep it.
-- Unique per school when present; most files start without one.
-- ============================================================

ALTER TABLE person ADD COLUMN neph text;

CREATE UNIQUE INDEX person_neph_unique
    ON person (school_id, neph) WHERE neph IS NOT NULL;
