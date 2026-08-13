-- ============================================================
-- 017_email_templates.sql
-- Polished default wording for every message the centre sends
-- (D-01: templates are data, edited in Paramétrage). Two system
-- kinds are composed and sent by the app itself; the office_*
-- kinds are STARTING POINTS offered by the free-message form —
-- {{prenom}}/{{nom}} are filled from the file when picked.
-- Dev phase: upserts overwrite — once real users edit templates,
-- later migrations must stop touching bodies.
-- ============================================================

INSERT INTO communication_template (id, school_id, kind, subject, body)
SELECT gen_random_uuid(), s.id, v.kind, v.subject, v.body
FROM school s, (VALUES
  ('exam_convocation',
   'Convocation — épreuve {{epreuve}} du {{date}}',
   E'Bonjour {{prenom}},\n\nVous êtes convoqué(e) à l''épreuve {{epreuve}} :\n\n• Date : {{date}}\n• Lieu : {{lieu}}\n• Heure de l''épreuve : {{heure}}\n• Présentez-vous dès {{presentation}}\n\nPensez à apporter une pièce d''identité en cours de validité.\nEn cas d''empêchement, prévenez le secrétariat au plus tôt : une absence non justifiée fait perdre la place d''examen.\n\nBonne préparation,\nL''équipe CAMO-EDUCASER'),
  ('lesson_assignment',
   'Nouvelle séance le {{date}}',
   E'Bonjour {{prenom}},\n\nUne séance {{type}} vient d''être planifiée pour vous :\n\n• Date : {{date}}\n• Horaires : de {{heure_debut}} à {{heure_fin}}{{avec}}\n\nMerci d''arriver quelques minutes en avance. En cas d''empêchement, prévenez le secrétariat au plus tôt pour libérer le créneau.\n\nÀ bientôt,\nL''équipe CAMO-EDUCASER'),
  ('office_welcome',
   'Bienvenue au centre — votre formation {{objectif}}',
   E'Bonjour {{prenom}},\n\nVotre inscription est bien enregistrée — bienvenue !\n\nPour démarrer dans de bonnes conditions, merci de nous transmettre dès que possible les pièces de votre dossier (pièce d''identité, photo-signature numérique, avis médical le cas échéant). Le secrétariat reste à votre disposition pour toute question.\n\nÀ très vite,\nL''équipe CAMO-EDUCASER'),
  ('office_documents',
   'Votre dossier — pièces manquantes',
   E'Bonjour {{prenom}},\n\nIl manque encore une ou plusieurs pièces à votre dossier pour pouvoir vous présenter à l''examen.\n\nMerci de nous les faire parvenir rapidement — sans dossier complet, aucune place d''examen ne peut être réservée à votre nom.\n\nRépondez directement à ce message ou passez au secrétariat.\n\nL''équipe CAMO-EDUCASER'),
  ('office_hours_gap',
   'Point sur votre progression',
   E'Bonjour {{prenom}},\n\nAu rythme actuel, vos heures de formation risquent de ne pas être au niveau requis pour votre échéance d''examen.\n\nNous vous proposons de faire un point ensemble pour ajouter des séances et rester dans les temps. Contactez le secrétariat pour convenir des créneaux.\n\nL''équipe CAMO-EDUCASER'),
  ('office_no_show',
   'Votre absence à la séance du {{date}}',
   E'Bonjour {{prenom}},\n\nNous avons constaté votre absence à la séance prévue le {{date}}.\n\nLes heures non effectuées retardent votre préparation et les séances réservées mobilisent un véhicule et un formateur. Merci de prévenir le secrétariat dès que possible en cas d''empêchement.\n\nPour reprogrammer la séance, contactez-nous.\n\nL''équipe CAMO-EDUCASER')
) AS v(kind, subject, body)
ON CONFLICT (school_id, kind) DO UPDATE SET subject = EXCLUDED.subject, body = EXCLUDED.body;
