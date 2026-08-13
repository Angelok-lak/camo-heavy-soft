-- ============================================================
-- 013_documents_ephoto.sql
-- F-35 first cut + the documents foundation (F-29 §document).
-- The candidate reaches a TOKENIZED public portal from their
-- phone, uploads their photo and/or their ANTS e-photo code —
-- it lands in the file's documents and unlocks the requirement.
--
-- V-10 stays OPEN: whether a raw smartphone photo satisfies the
-- ANTS agrément is a regulatory question; the portal therefore
-- also collects the 22-char e-photo code from approved apps.
--
-- Dev storage is in-database; the target architecture moves the
-- bytes to S3 with the same table as index.
-- ============================================================

CREATE TABLE document (
    id           uuid PRIMARY KEY,
    school_id    uuid NOT NULL REFERENCES school(id),
    person_id    uuid NOT NULL REFERENCES person(id),
    kind         text NOT NULL, -- 'EPHOTO', 'OTHER'
    filename     text NOT NULL,
    content_type text NOT NULL,
    bytes        bytea NOT NULL,
    ants_code    text,
    via          text NOT NULL DEFAULT 'OFFICE', -- 'PORTAL' / 'OFFICE'
    uploaded_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON document (person_id);

CREATE TABLE upload_token (
    token      text PRIMARY KEY,
    school_id  uuid NOT NULL REFERENCES school(id),
    person_id  uuid NOT NULL REFERENCES person(id),
    purpose    text NOT NULL DEFAULT 'EPHOTO',
    created_by uuid NOT NULL REFERENCES app_user(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);
