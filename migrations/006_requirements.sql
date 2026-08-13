-- ============================================================
-- 006_requirements.sql
-- F-29: requirements, two sets — entry into training and exam
-- presentation (RG-02). Templates hang off the objective; each
-- enrollment gets its own COPY at creation (C-07): a template
-- change never rewrites an existing file unless propagated.
--
-- EXPIRED is never stored: it derives from valid_until (RG-193's
-- pattern). Completeness is derived too (RG-192).
-- ============================================================

CREATE TYPE requirement_set AS ENUM ('ENTRY', 'EXAM');
CREATE TYPE requirement_status AS ENUM ('NOT_VALIDATED', 'VALIDATED', 'NOT_APPLICABLE');

-- prerequis_modele: what the objective demands.
CREATE TABLE requirement_template (
    id                       uuid PRIMARY KEY,
    school_id                uuid NOT NULL REFERENCES school(id),
    objective_id             uuid NOT NULL REFERENCES objective(id),
    label                    text NOT NULL,
    req_set                  requirement_set NOT NULL,
    mandatory                boolean NOT NULL DEFAULT true,
    -- RG-21 / A-14: the template says whether an instructor may validate
    instructor_may_validate  boolean NOT NULL DEFAULT false,
    validity_months          integer CHECK (validity_months > 0),
    expected_document        text,
    active                   boolean NOT NULL DEFAULT true
);
CREATE INDEX ON requirement_template (objective_id) WHERE active;

-- prerequis_parcours: the enrollment's own copy. Copied columns carry no
-- FK dependency for their VALUES — the template link is kept only to
-- know the origin (propagation, F-01 later).
CREATE TABLE enrollment_requirement (
    id                       uuid PRIMARY KEY,
    school_id                uuid NOT NULL REFERENCES school(id),
    enrollment_id            uuid NOT NULL REFERENCES enrollment(id),
    template_id              uuid REFERENCES requirement_template(id),
    label                    text NOT NULL,
    req_set                  requirement_set NOT NULL,
    mandatory                boolean NOT NULL,
    instructor_may_validate  boolean NOT NULL,
    validity_months          integer,
    status                   requirement_status NOT NULL DEFAULT 'NOT_VALIDATED',
    validated_by             uuid REFERENCES app_user(id),
    validated_at             timestamptz,
    comment                  text,
    -- Set at validation from validity_months; EXPIRED derives from it.
    valid_until              date,
    na_reason                text
);
CREATE INDEX ON enrollment_requirement (enrollment_id);
