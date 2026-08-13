-- ============================================================
-- 015_restore_note.sql
-- F-16: the return to service can carry an optional note
-- (what was fixed, who decided), kept on the declaration it
-- closes — the trace stays in one place.
-- ============================================================

ALTER TABLE unavailability ADD COLUMN restored_note text;
