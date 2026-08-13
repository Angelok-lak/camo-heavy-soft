-- ============================================================
-- 011_seat_deadline_day.sql
-- Q-139 settled by the REAL files: "à envoyer avant le 5 juin"
-- for August, "le 5 juillet" for September — the 5th of M-2.
-- The interview said the 25th; the artifact wins, the parameter
-- stays adjustable if the field corrects it.
-- ============================================================

ALTER TABLE school ALTER COLUMN seat_request_deadline_day SET DEFAULT 5;
UPDATE school SET seat_request_deadline_day = 5 WHERE seat_request_deadline_day = 25;
