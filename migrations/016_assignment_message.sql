-- ============================================================
-- 016_assignment_message.sql
-- Automatic message on every student placement (Angelo, août
-- 2026). The template is DATA (D-01) — this migration seeds the
-- default wording per school; Paramétrage edits it. The variable
-- {{avec}} arrives pre-composed (« avec Alhan » or empty).
-- Also seeds the exam convocation default that only existed as a
-- hand-inserted row in the dev base.
-- ============================================================

INSERT INTO communication_template (id, school_id, kind, subject, body)
SELECT gen_random_uuid(), s.id, 'lesson_assignment',
       'Nouvelle séance le {{date}}',
       E'Bonjour {{prenom}},\n\nUne séance {{type}} est planifiée pour vous le {{date}} de {{heure_debut}} à {{heure_fin}}{{avec}}.\n\nEn cas d''empêchement, merci de prévenir le secrétariat au plus tôt.'
FROM school s
ON CONFLICT (school_id, kind) DO NOTHING;

INSERT INTO communication_template (id, school_id, kind, subject, body)
SELECT gen_random_uuid(), s.id, 'exam_convocation',
       'Convocation examen {{epreuve}} — {{date}}',
       E'Bonjour {{prenom}},\n\nVous êtes convoqué(e) à l''épreuve {{epreuve}} le {{date}} à {{lieu}}.\nHeure de l''épreuve : {{heure}}. Merci de vous présenter à {{presentation}}.\n\nEn cas d''empêchement, prévenez le secrétariat au plus tôt.'
FROM school s
ON CONFLICT (school_id, kind) DO NOTHING;
