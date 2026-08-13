-- ============================================================
-- 010_payers.sql
-- The payer (C-12): the student themselves, a company or a body,
-- with a reference contact (RG-187). A table from day one — free
-- text columns would cost a migration when invoicing arrives.
-- An enrollment with payer_id NULL means the student pays.
-- ============================================================

CREATE TABLE payer (
    id            uuid PRIMARY KEY,
    school_id     uuid NOT NULL REFERENCES school(id),
    label         text NOT NULL,
    contact_name  text,
    contact_email text,
    contact_phone text,
    active        boolean NOT NULL DEFAULT true
);

ALTER TABLE enrollment ADD COLUMN payer_id uuid REFERENCES payer(id);
