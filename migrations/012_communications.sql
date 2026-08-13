-- ============================================================
-- 012_communications.sql
-- Communications: every message the centre sends is a ROW first
-- — recipient, channel, content, status — so the file's history
-- is complete whatever the channel does.
--
-- WhatsApp works by prepared link (one recipient per message,
-- the office clicks to send); email sends itself when SMTP is
-- configured, and is stored as SIMULATED until then.
-- Templates are data (D-01), never code.
-- ============================================================

CREATE TYPE communication_channel AS ENUM ('EMAIL', 'WHATSAPP');
CREATE TYPE communication_status AS ENUM ('PREPARED', 'SENT', 'SIMULATED', 'FAILED');

CREATE TABLE communication (
    id                uuid PRIMARY KEY,
    school_id         uuid NOT NULL REFERENCES school(id),
    person_id         uuid REFERENCES person(id),
    payer_id          uuid REFERENCES payer(id),
    channel           communication_channel NOT NULL,
    kind              text NOT NULL, -- 'exam_convocation', 'free', …
    recipient_label   text NOT NULL,
    recipient_address text NOT NULL, -- email or phone
    subject           text,
    body              text NOT NULL,
    status            communication_status NOT NULL DEFAULT 'PREPARED',
    -- What the message is about (an exam session, a lesson…).
    about_kind        text,
    about_id          uuid,
    created_by        uuid NOT NULL REFERENCES app_user(id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    sent_at           timestamptz,
    CHECK (person_id IS NOT NULL OR payer_id IS NOT NULL)
);
CREATE INDEX ON communication (person_id);
CREATE INDEX ON communication (about_kind, about_id);

-- The channel each person prefers; the payer has one too (A-06: the
-- payer receives planning, convocations and absences — never results).
ALTER TABLE person ADD COLUMN preferred_channel communication_channel NOT NULL DEFAULT 'EMAIL';
ALTER TABLE payer ADD COLUMN preferred_channel communication_channel NOT NULL DEFAULT 'EMAIL';

CREATE TABLE communication_template (
    id        uuid PRIMARY KEY,
    school_id uuid NOT NULL REFERENCES school(id),
    kind      text NOT NULL,
    subject   text NOT NULL,
    body      text NOT NULL,
    UNIQUE (school_id, kind)
);
