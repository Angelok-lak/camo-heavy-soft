# CLAUDE.md

Logiciel de gestion pour un centre de formation aux permis du groupe lourd (C, CE, D).
Phase 1 et 2 closes sur le lot 1. Phase 3 en cours : réalisation de la tranche 1.

---

## Lire ceci avant toute chose

| Fichier | Contenu | Autorité |
|---|---|---|
| `FEATURES.md` | Features, statuts, règles `RG-01` à `RG-260`, décisions `D-01` à `D-08` | **Source de vérité de l'avancement** |
| `QUESTIONS.md` | Questions `Q-xx`, arbitrages `A-xx`, points réglementaires `V-xx`, hypothèses `H-xx` | Source de vérité des points ouverts |
| `MODELE_DONNEES.md` | Modèle de données, choix de conception `C-01` à `C-24` | |
| `CONTRATS_API.md` | Un contrat par feature, `C-25` à `C-30` | |
| `GLOSSARY.md` | Domaine français → code anglais | **Aucun terme métier hors de cette table** |

Ne jamais contredire une `RG-xx` sans le dire explicitement. Si une règle paraît fausse, le signaler plutôt que la contourner.

---

## Les six décisions qui gouvernent tout

| ID | Décision |
|---|---|
| **D-01** | Centre unique, ouverture SaaS ensuite. Toute table porte `school_id`, toute requête est filtrée dessus. **Aucune valeur métier en dur** : tout va en paramétrage |
| **D-02** | Financement et facturation hors MVP. Seuls le financeur et le statut de prise en charge sont portés |
| **D-03** | Planning en saisie manuelle avec détection de conflits et suggestion assistée. **Un seul composant de disponibilité**, interrogé dans les deux sens. Pas d'optimisation globale |
| **D-04** | **Le système alerte, il ne bloque jamais.** Sans exception. Trois sévérités, public par type d'alerte, alertes strictement internes, tout forçage tracé |
| **D-05** | Chaque feature émet ses événements et compteurs dès sa livraison |
| **D-06** | Toute action pouvant toucher plusieurs objets est conçue dès le départ en action groupée : liste d'impact, aperçu, application, compte rendu, identifiant de lot |
| **D-07** | Durabilité et export des données. Issue d'un incident réel de perte de base. *Non spécifiée* |
| **D-08** | Chaque feature déclare ses éléments « à faire » pour F-36 |
| **D-09** | **Socle UI** (acté par Angelo en réalisation ; numéroté D-09 car D-08 était pris). React, back-office authentifié, aucun besoin SEO/SSR. Composants : Tailwind + **Mantine** (amendement tranché par Angelo après banc d'essai Mantine / DaisyUI / Ant Design sur l'écran Ressources ; shadcn/ui écarté), listes en TanStack Table. Planning : FullCalendar Premium (`resourceTimeline`) en composant **strictement contrôlé** — il affiche, il ne décide pas ; toute logique de disponibilité et de conflit reste dans le composant unique RG-78 côté Go. F-11 mobile : vue liste dédiée, pas le composant planning. Graphiques (F-20) : Recharts, tranché à l'arrivée de F-20. Licence FullCalendar end-user assumée ; un passage SaaS (D-01) imposera une renégociation OEM |

**D-04 est la plus violée par réflexe.** Un `return error` sur une règle métier est presque toujours un bug de conception. Les permissions, elles, refusent — ce n'est pas une règle métier (RG-214).

---

## Architecture

### Cœur applicatif : un binaire Go conteneurisé, pas du serverless

Le brief initial disait « AWS, orientation serverless ». **Écarté**, pour une raison décisive : la base PostgreSQL tourne 24 h/24 de toute façon, donc l'argument « scale to zero » s'effondre. Lambda imposerait en plus RDS Proxy pour le pool de connexions — une pièce et un coût fixe de plus, pour aucune économie.

- **Backend** : Go, binaire unique, App Runner ou une tâche Fargate
- **Base** : PostgreSQL sur RDS. `tstzrange` + index GiST pour les chevauchements
- **Front** : React en application monopage. Pas de Nuxt : aucun référencement, aucune page publique, le rendu serveur n'apporte rien
- **Serverless conservé où il a du sens** : S3 pour les documents, EventBridge pour les traitements planifiés, SES pour les envois

Si le SaaS multi-centres arrive, le même conteneur passe à plusieurs instances. Rien à réécrire.

### Structure du repo

Découpage **par domaine**, jamais par couche technique — cohérent avec le découpage en features.

```
internal/
  availability/   composant de disponibilité (RG-78) — fait
  planning/       F-09, F-03, F-10
  attendance/     F-12
  enrollment/     F-04, F-13
  exams/          F-06, F-07.1, F-07.2
  people/         F-02, F-05, F-29
  resources/      F-15, F-16
  settings/       F-01
  tasks/          F-36
  auth/           F-17
migrations/
```

---

## Trois invariants techniques

**Aucun compteur n'est stocké** (C-05). Ni heures, ni crédits, ni état de préparation. Tout est recalculé à la lecture. À 300 parcours, c'est instantané. Une colonne de compteur est un bug.

**Les alertes sont des requêtes, pas des lignes** (C-06). Elles se déduisent de l'état et accompagnent l'objet qu'elles concernent. Seule exception : `DisruptionCase`, qui porte des décisions humaines.

**Le brouillon est un journal d'intentions** (C-09). La session d'édition porte ses opérations en attente ; l'état affiché est l'enregistré plus les opérations appliquées en mémoire. Aucune autre requête du système ne connaît son existence. Le verrou est une contrainte `EXCLUDE` en base, pas du code applicatif.

---

## Tranche 1 — en cours

**Ce qu'on montre** : une semaine de planning réelle, camions et formateurs du centre, élèves placés, détection de conflits fonctionnelle. C'est la douleur numéro un exprimée par le centre.

| Feature | Gardé | Coupé |
|---|---|---|
| F-17 | Connexion, trois profils, permissions | Écran de gestion des comptes |
| F-01 | Types de séance, durées, horaires, catégories | Objectifs, prérequis, alertes, motifs, crédits |
| F-15 · F-16 | Créer, lister, déclarer une indisponibilité | Remise en service, traitement d'impact, cumul d'heures |
| F-02 | Nom, prénom, coordonnées | Pièces, NEPH, doublons, fusion |
| F-04 | Élève + objectif + échéance | Seuils, dérogations, marqueurs, clôture |
| F-09 | Poser, déplacer, annuler · verrou · conflits | Traitement d'impact, absence annoncée, remplacement en masse |
| F-03 | Placer, retirer, recherche simple | Placement en masse, inscription par session |

**Hors tranche 1** : F-05, F-06, F-07.1, F-07.2, F-10, F-12, F-13, F-29, F-36.

F-10 sort mécaniquement : elle classe par retard sur les heures projetées, or les compteurs viennent de F-13, qui vient de F-12. Remplacée par une recherche par nom.

**Le verrou d'édition reste dedans** malgré la tentation de le couper : le rétrofiter voudrait dire reprendre toutes les écritures du planning.

### Tranches suivantes

2. F-12 puis F-13 — les présences alimentent les compteurs, qui réveillent F-10 et les alertes d'écart
3. F-06, F-07.1, F-29, F-05 — examens et prérequis, le rebours prend son sens
4. F-07.2, F-36, reste de F-01, F-09 complet

### État de la réalisation

- [x] `migrations/001_slice1_schema.sql` — en anglais, conforme au glossaire
- [x] `internal/availability` — composant + tests
- [x] Application du brouillon sur un `Snapshot` — `internal/planning/draft.go`, pur, testé
- [x] Chargement du `Snapshot` depuis PostgreSQL — `internal/planning/store.go` + `save.go`, tests d'intégration sous `TEST_DATABASE_URL`
- [x] API HTTP de F-09 — `internal/planning/http.go`, `cmd/server`
- [x] Front planning — `web/` (Vite + React), vue semaine, conflits explicités, boucle d'édition complète
- [x] F-17 tranche 1 — `internal/auth` + `migrations/002_auth_sessions.sql` : connexion bcrypt, sessions serveur révocables (cookie HttpOnly, glissement 12 h), profils en union (RG-208), suspension effective à la requête suivante (RG-211), échec de connexion indiscernable (login inconnu = mauvais secret = compte suspendu). Coupé conformément à la tranche : écran de gestion des comptes, réinitialisation de secret (Q-146)
- [x] A-37 — RG-259 resserré sur les seules séances touchées par le brouillon (voir `QUESTIONS.md`)
- [x] F-01 tranche 1 — `internal/settings` : types de séance, durées, catégories, horaires. Désactivation jamais suppression (RG-205). Direction seule (hypothèse tranche 1, à confirmer)
- [x] F-15 · F-16 tranche 1 — `internal/resources` : créer, lister (état courant + séances futures), archiver (accepté avec séances, RG-73), déclarer une indisponibilité avec liste d'impact jointe, sans toucher aux séances (RG-80, D-04)
- [x] F-02 · F-04 tranche 1 — `internal/people` + `internal/enrollment` : dossier réduit, parcours avec seuils copiés (C-07), un seul actif par personne (RG-115, index partiel → 409), circulation jamais avant plateau (RG-24, structurel)
- [x] `internal/events` — socle D-05, chaque écriture émet son événement
- [x] Front : navigation Planning · Ressources · Élèves · Paramétrage, onglets et actions pilotés par les permissions du contexte

**Reste à durcir** : tests d'intégration dédiés sur settings/resources/people/enrollment (validés par smoke test HTTP pour l'instant) · journal_parametre (RG-203) non implémenté, les événements D-05 en tiennent lieu.

### Tranche 2 — livrée

- [x] `migrations/003_attendance.sql` — attendance (UNRECORDED = absence de ligne, RG-123), attendance_correction (RG-20), **vue `enrollment_hours`** : les trois compteurs et les deux projetés sont une requête unique (C-05, RG-172, RG-119), tous les lecteurs passent par elle
- [x] F-12 — `internal/attendance` : séance courante prégarnie PRÉSENT (RG-236), saisie idempotente (RG-126), correction tracée (RG-20), non-renseignées par ancienneté (RG-125, formateur = ses séances), feuille de présence faits seuls (RG-127). 5 tests d'intégration (dont : présent = durée totale RG-33, pas de double compte projeté/consommé)
- [x] F-13 — `internal/enrollment/hours.go` : lecture parcours avec compteurs + alertes d'écart jointes (C-29), liste des écarts triée par échéance (US-32). Sévérité fixe `WARNING` : la bascule paramétrable (RG-114) viendra avec le reste de F-01
- [x] F-10 — `internal/planning/suggest.go` : proposition classée par retard projeté (RG-153), rang expliqué (RG-97), exclusions minimales (RG-154)
- [x] Front : onglet Présences, écarts et suivi d'heures dans Élèves, suggestions classées dans le placement du planning
- [x] Marque produit : **CAMO-EDUCASER**
- [x] Seed cohérent : historique 3 semaines sans aucun conflit involontaire ; les conflits de la semaine courante sont les six cas de démo, un par type

### Retours v0 intégrés + F-06 première coupe

- [x] Refonte UX : Ressources / Élèves / Présences en pleine largeur, filtres + tri par colonne, actions en modales, vues détail (ressource : indispos + séances à venir · élève : dossier + compteurs + historique de présences). Aucune référence de spec (RG/US/D/C) visible à l'écran ; textes de conflit du composant de disponibilité passés en français (copy utilisateur, C-26)
- [x] Pose de séance **au clic sur la grille** en édition (créneau snappé 30 min, durées standard, type, ressources et élèves dans la même modale)
- [x] F-06 première coupe — `migrations/004_exam_sessions.sql` + `internal/exams` : lieux d'examen avec trajet (RG-140, gérés dans Paramétrage), sessions datées avec enveloppe (RG-36), annulation motivée ; À VENIR/PASSÉE dérivés (RG-44) ; chargées dans le Snapshot (immobilisation trajet compris) et affichées sur le planning (blocs hachurés) ; onglet Examens, seule voie d'écriture (RG-152). Reste de F-06/F-07 : affectations, résultats, moteur de crédits

### D-09 appliqué

- [x] Planning sur **FullCalendar timeGrid** strictement contrôlé — **amendement à D-09 tranché par Angelo** : la vue `resourceTimeline` (lignes par ressource) a été essayée puis rejetée, l'organisation jours-en-colonnes / heures-en-lignes est retenue. Conséquence heureuse : timeGrid est standard, **aucune licence Premium nécessaire** pour l'instant (les paquets premium restent installés si une vue « par ressource » optionnelle revient). Séances colorées par sévérité du pire conflit (calculée côté Go), examens non éditables trajet compris ; **chaque geste devient une opération du brouillon** (drag = déplacer, resize = redimensionner, clic créneau = poser préremplie) et le calendrier est systématiquement `revert()` — la vérité revient du serveur
- [x] Tailwind branché (base utilitaire, migration shadcn/ui au fil de l'eau) · listes Ressources et Élèves sur **TanStack Table v8** (composant partagé `DataTable`)
- Versions épinglées : `@fullcalendar/react@6.1.x` (la v7 est incompatible avec les paquets premium 6.x), `@tanstack/react-table@8` (la v9 alpha change l'API)
- [x] **Thèmes d'affichage** : 5 palettes (Sable défaut · Craie · Ardoise · Lagune · Nuit sombre) en jetons CSS `[data-theme=…]`, sélecteur dans la barre de navigation, persisté `localStorage` (`web/src/theme.ts`). Les couleurs en dur de `index.css` ont été converties en variables/`color-mix` ; la sévérité des conflits garde les mêmes codes dans tous les thèmes. Question ouverte : porter le choix au compte utilisateur plutôt qu'au navigateur
- [x] **Mantine adoptée** (arbitrage Angelo, 13/08/2026) : banc d'essai Mantine / DaisyUI / Ant Design monté sur l'écran Ressources (mêmes données et actions, rendu seul variant) puis démonté ; Mantine retenue (~68 ko gzip, comportement complet, code le plus court). `MantineProvider` à la racine (`App.tsx`), synchronisé avec les thèmes maison (mode sombre sur Nuit, couleur primaire approchée par thème). **Ressources est le premier écran migré** ; les autres migrent au fil de l'eau, en veillant au responsive mobile (consigne Angelo : soigner les alignements en vue téléphone)

### Vue élèves maquette + fiche à onglets

- [x] Filtres d'affichage du planning par formateur et par véhicule (pur affichage, aucune logique métier côté client)
- [x] `migrations/005_person_neph.sql` — NEPH porté par la personne, unique par centre (RG-182, index partiel → 409 explicite)
- [x] Liste élèves façon maquette : recherche unique (NEPH, nom, prénom, email, téléphone — côté serveur), filtre par permis, tri, compte de résultats, **cartes** avec badge permis, anneau de progression (consommées/total, via la vue `enrollment_hours`), pastilles de manquants (écart, NEPH, échéances, aucune séance à venir)
- [x] Fiche élève en modale à onglets **Dossier · Parcours · Séances · Historique** — séances passées+à venir (`GET /api/enrollments/{id}/lessons`), historique métier depuis `domain_event` (RG-185, C-10 ; ne se remplit que par les actions passées par l'API, le seed SQL n'émet rien)
- **En attente de leur feature** : onglet/blocs financement (F-05, D-02), documents et prérequis (F-29), payeur (C-12), évaluation/contrat/paiement de la maquette Klaxo (hors périmètre lot 1)
- [x] Fiche élève refondue en **page pleine** : en-tête riche, Dossier en deux colonnes (contacts éditables sur place + cycle de financement grisé « à venir » · check-lists d'état du dossier calculées sur les vraies données — présentées comme état pratique, pas comme la préparation officielle qui attend F-29), Parcours en tuiles + jauges projeté/seuil
- [x] **Sessions zombies réglées** : expiration paresseuse des sessions OPEN inactives > 30 min à l'ouverture (RG-145 ; délai à migrer en `parametre_centre`), le 409 porte l'id du détenteur, bannière « Reprendre ma session / L'abandonner » quand le verrou est à soi, « Libérer de force » pour la direction
### Communications + e-photo (F-35)

- [x] **Communications (arbitrage Angelo : WhatsApp option A + email)** — `migrations/012` + `internal/comms` : chaque message est une ligne tracée (destinataire, canal, contenu, statut). **Email** : envoi réel si `SMTP_HOST/SMTP_FROM` (+`SMTP_USER/PASS`) sont posés, sinon statut `SIMULATED` complet et rejouable ; **WhatsApp** : lien `wa.me` préparé — un destinataire par message (pas de multicast possible en option A), clic = ouvre WhatsApp prérempli + marquage envoyé. Canal préféré par personne ET par payeur. **Convocations d'examen** : un message personnel par candidat engagé (épreuve, date, lieu, heure de présentation trajet déduit), copie au payeur (A-06), modèle `{{variables}}` en base (D-01) éditable via `PUT /api/communication-templates/{kind}`. **Message libre** depuis la fiche (onglet Messages). Reste : panneau Paramétrage des modèles (l'API existe), planning hebdo à l'élève, notification d'absence au payeur, V-09 (CNIL envois groupés) avant tout envoi de masse
- [x] **F-35 e-photo + socle documents** — `migrations/013` + `internal/docs` : portail public mobile **à jeton** (7 jours, un envoi, brûlé après usage), photo smartphone + **code e-photo ANTS optionnel**, document rangé dans le dossier, **prérequis « Photo-signature numérique (e-photo) » validé automatiquement** (commentaire « Reçue via le portail »), événement tracé. Fiche : bloc Documents + « Demander l'e-photo » avec envoi du lien par WhatsApp/email en un clic. Octets en base en dev → S3 en cible. **V-10 toujours ouverte** : une photo smartphone brute n'est pas forcément agréée ANTS — d'où le champ code ; à vérifier avant de promettre l'agrément. **Arbitrage Angelo : le candidat paie** — le portail guide le parcours en deux étapes (app agréée à ses frais → saisie du code chez nous, prérequis validé). Passage au flux intégré (API type ephoto.io, webhook, paiement candidat dans leur tunnel) dès qu'un contrat partenaire existe — question à poser au prestataire : l'encaissement côté candidat

### F-07 résultats + moteur de crédits

- [x] **Saisie des résultats** — `migrations/014` + `internal/exams/results.go` : `POST /api/exam-bookings/{id}/result` (réussi/échoué/absent), gardes structurelles (session commencée, pas sur un retiré, resaisie identique = no-op), **correction tracée** (`exam_booking_correction`, RG-106), événement émis. « Non renseignée » **dérivée** (engagement + session passée, RG-112) — rien ne bascule par batch, et une ligne « À traiter » la rappelle (les crédits restent engagés tant que le résultat manque)
- [x] **Crédits complets** : engagées / consommées (réussi+échoué) / **perdues** (absent + reliquat non engagé une fois la session passée, RG-39/40) / restantes — tout dérivé (C-05), affiché sous la barre d'unités
- [x] **Expiration du plateau dérivée** (RG-25) : `school.offroad_validity_months` (défaut 12, A-16 — changer le paramètre repropage partout), date d'obtention = max des réussites plateau. Affichée dans les candidats proposés (« plateau obtenu le… · expire dans N j » sous 60 j · « Ne présentation », sessions passées seules comptées) et dans la fiche Parcours (bloc réussite + alerte d'expiration D-04, la jauge plateau s'efface une fois l'épreuve obtenue)
- [x] Écran session passée : boutons Réussi/Échoué/Absent par candidat (statut courant surligné), pastille « Non renseigné », lecture seule hors gestionnaires. Seed : session passée à Melun avec réussite, échec et un non-renseigné de démo

### Retours v0, deuxième salve

- [x] **Ré-engagement après retrait réparé** : retirer un candidat garde la ligne comme trace (RG-104), ré-engager **ravive** cette ligne (unités refigées, forçage recalculé, tampon de retrait purgé) au lieu de percuter la contrainte d'unicité. Vérifié : engager → retirer → ré-engager = même ligne, compteurs justes
- [x] **Écran Ressources refondu** : cartes groupées par type (véhicules, formateurs, salles), pastille d'état, motif et dates de l'indispo en cours sur la carte, **actions contextuelles** (en service → déclarer · indisponible → **remettre en service avec motif facultatif**, séances restées assignées en compte rendu RG-66/RG-80), archivage déplacé dans la fiche. `migrations/015` : `restored_note` sur la déclaration
- [x] Cartes élèves : anneau de pourcentage remplacé par heures + barre fine, pastille d'écart adoucie
- [x] **A-39 (Angelo)** : échéance d'envoi dépassée → **génération de la demande de places refusée** (422 + bouton désactivé avec explication). Assumé face à D-04 : l'action est sans objet, le fichier ne peut plus être transmis
- [x] **Fichier de demande en .xlsx au format officiel** (excelize) : titre bleu centré « AUTO ECOLE MOIS ANNÉE », « à envoyer avant le… » en rouge, « Nombre d'examens » sur BE / Isolés / Ensembles bordés, « En Unités » en vert, une ligne par semaine avec dates en rouge, nom de fichier « TABLEAU BE C CE MOIS ANNÉE.xlsx »
- [x] Messages 409 restés en anglais traduits (parcours actif existant, RG-115/184)
- [x] **Écran Présences (bureau) refondu** : séances non renseignées en cartes groupées par jour (heure, formateur, véhicule, élèves, pastille N/M saisis, « Faire l'appel »), rappel de conséquence en tête de liste
- [x] **Message automatique à chaque placement** (Angelo) — `migrations/016` + `comms.NotifyLessonAssignment`, branché sur les **deux** chemins d'écriture (placement direct A-38 et enregistrement du brouillon) : modèle `lesson_assignment` en base (D-01, variables prenom/type/date/heures/avec), canal préféré de la personne, email auto ou simulé, WhatsApp préparé affiché dans le panneau de placement avec bouton d'envoi. Gardes : séance passée = pas de message (correction d'historique), aucune coordonnée = rien, échec de notification ne casse jamais le placement. Migration 016 pose aussi le modèle de convocation par défaut (n'existait qu'en base de dev)
- [x] **Modèles de messages : écran + rédaction** — `migrations/017` : six modèles rédigés (convocation détaillée avec rappel pièce d'identité et perte de place, placement de séance, et quatre courriers bureau : bienvenue, pièces manquantes, retard d'heures, absence non prévenue). Panneau Paramétrage « Modèles de messages » : sélecteur par type (badge « auto » sur ceux envoyés par le système), éditeur sujet+corps, pastilles de variables cliquables, **aperçu en direct** sur le côté (carte email en-tête CAMO-EDUCASER + bulle WhatsApp) rendu avec valeurs d'exemple. Les modèles `office_*` sont proposés en **point de départ dans le message libre** ({{prenom}}/{{nom}}/{{objectif}} préremplis depuis la fiche). Écriture réservée direction (204/403 vérifiés)
- [x] **Message depuis la liste des élèves** : bouton « Message » sur chaque carte (formulaire partagé avec l'onglet Messages de la fiche — canal préréglé selon les coordonnées, WhatsApp préparé/ouvert en un clic)

### F-33 calée sur les fichiers réels

- [x] **Q-139 tranchée, Q-131 obtenue** : les tableaux réels (BE C CE août/septembre) donnent l'échéance (**le 5 de M-2**, migration 011) et la structure — **lignes par semaine, colonnes BE / Isolés C-D-C1-D1 / Ensembles CE-C1E-DE-D1E, en unités**. Suggestion par semaine (1 u. plateau, 2 u. circulation, élèves nommés), saisie libre en grille, fichier généré au format officiel. Le fichier source note aussi « tableau de bord demandes et annulations avec moyenne » → future brique F-20

### Tableau de bord F-20 + retours

- [x] **F-20 première coupe** — `internal/tasks/dashboard.go` (`GET /api/dashboard`, tout dérivé C-05) : tuiles (élèves actifs, séances de la semaine, heures réalisées du mois, sessions à venir + unités engagées), **graphique Recharts** heures/semaine sur 8 semaines (D-09), répartitions permis et financements en barres étiquetées, la liste « À traiter » en dessous
- [x] Paramétrage : la liste latérale remplacée par un **accueil en grille de cartes** groupées avec descriptions, section pleine largeur avec retour
- [x] **Demande de places libre** : modale (Examens + tableau de bord) avec mois cible, échéance, **suggestion expliquée** (le compte ET les élèves derrière chaque nombre), champs entièrement libres, lignes ajoutables ; la génération stocke ce qui a été saisi, pas la suggestion

### F-33, hors-réseau, payeur

- [x] **F-33 demande de places** — `migrations/009` + `internal/exams/seatrequests.go` : besoins **dérivés** des échéances par mois cible (M+1..M+3), génération tracée (`seat_request`, régénérable), fichier CSV téléchargeable, **rappel au tableau de bord** à J-10 de l'échéance (critique à J-2), née de l'incident d'oubli (Q-12). **Défauts appliqués à confirmer** : jour d'envoi paramétré sur `school.seat_request_deadline_day` (défaut 25, cycle M-2 → M) tant que **Q-139 reste ouverte** ; colonnes CSV provisoires tant que **Q-131** (fichier réel) n'est pas fourni
- [x] **Hors-réseau moniteur** : cache local de la séance du jour, émargements en file `localStorage` quand le réseau tombe, **renvoi automatique au retour** (l'idempotence RG-126 rend les rejeux sûrs), badge « Hors réseau » + compteur d'attente. Pas encore une PWA installable (app shell non caché hors ligne)
- [x] **Payeur (C-12)** — `migrations/010` : table `payer` (contact référent RG-187), `enrollment.payer_id` (NULL = l'élève paie), API liste/création/association, bloc dans la fiche Dossier avec création à la volée. Distinct du financeur (F-05)
- [x] Tableau de bord : renommé, **premier onglet et écran d'accueil**, centré avec en-tête explicatif · « Sans objet » → **« Non applicable »** sur les prérequis (conforme RG-193 ; « sans objet » reste réservé aux lignes d'impact MOOT) · placement direct en section repliable · cartes élèves resserrées (2 pastilles max + « +N »)

### Rattrapage maquettes validées (retours v0 d'Angelo)

- [x] **Onglet « À traiter »** (graine de F-36) — `internal/tasks` : liste 100 % dérivée (C-06) — indispos en cours avec séances restantes et prochaine, sans date de retour avec ancienneté, séances sans présences, écarts — chaque ligne avec sa conséquence en clair et son action. **Remise en service** réelle (RG-66) avec liste des séances restées assignées
- [x] **Vue session d'examen maquette** — `migrations/007` + `internal/exams/bookings.go` : engagements plateau (1 u.) / circulation (2 u.), unités **figées à l'engagement**, compteurs dérivés (enveloppe/engagées/restantes + barre), retrait qui libère (RG-104), candidats proposés classés par échéance avec **prérequis manquants nommés sans exclusion** (RG-90 rév.), engagement malgré prérequis manquants = **forçage tracé en clair** sur la ligne. Barème 1/2 en constantes → à migrer en paramétrage. Ouvrable depuis l'onglet Examens **et** depuis le bloc examen du planning (RG-152 : toujours pas éditable depuis le planning)
- [x] **A-38 acté** : placement direct d'un élève sur une séance depuis le planning **sans session d'édition** (F-03 est sa propre verticale), conflits recalculés joints à la réponse. Le brouillon reste seul chemin pour créer/déplacer/annuler
- [x] **Émargement moniteur maquette** — le formateur a sa carte du jour : élèves prégarnis Présent, un tap = absent, **motifs en pastilles** depuis la table `absence_reason` (D-01 : rien en dur), bandeau séances non renseignées. Hors réseau : pas encore (nécessite file locale + idempotence déjà en place côté API)
- [x] **F-05 minimal (D-02)** — `migrations/008` : financeurs (référence, dont autofinancement au cycle propre RG-15), dossier par parcours né avec lui, cycle À monter › Déposé › Accordé › Soldé + Refusé motif obligatoire (RG-189), transitions tracées, **FundingOK branché sur la détection de conflits**. Bloc financement réel dans la fiche élève
- [x] **Pastille santé** dans la liste élèves : couleur + raisons **calculées côté serveur** (C-26), tooltip au survol, infos financement dans la carte
- [x] **Paramétrage reconstruit sur la maquette validée** : navigation latérale groupée (Formations / Planning / Ressources / Examens / Système), un écran par élément avec titre + sous-titre, bandeau bleu « valeurs recopiées », objectifs dépliables avec gros champs d'heures (CRUD ajouté), prérequis, financeurs, motifs. **Reste de F-01** : propagation avec populations divergentes (RG-252/253), journal des modifications (RG-203), fermetures exceptionnelles, alertes et délais, barème crédits éditable
- [x] **F-29 prérequis** (remplace la check-list improvisée de la fiche, signalée par Angelo) — `migrations/006_requirements.sql` + `internal/people/requirements.go` : modèles par objectif (jeu ENTRÉE/EXAMEN, obligatoire, `instructor_may_validate` RG-21/A-14, durée de validité), **copie au parcours dans la transaction de création** (C-07, F-29 cas 3), statuts NON VALIDÉ / VALIDÉ / NON APPLICABLE avec motif (RG-193), EXPIRÉ **dérivé** de `valid_until`, complétude **dérivée** (RG-192), jeux intitulés par finalité (RG-255). Permission vérifiée : formateur 204 sur un prérequis validable, 403 sinon (RG-21/22). Fiche élève branchée (Valider / Dévalider / Sans objet), modèles gérés dans Paramétrage. Reste de F-29 : pièces jointes/justificatifs (documents, S3), propagation des modèles aux parcours existants (F-01, RG-252/253)

Démo locale : `docker run -d --name crit-pg-test -p 5544:5432 -e POSTGRES_PASSWORD=test postgres:16`, appliquer les deux migrations, `go run ./cmd/devseed`, puis `DATABASE_URL=postgres://postgres:test@localhost:5544/postgres LISTEN_ADDR=:8099 go run ./cmd/server` et `cd web && npm run dev`. Comptes de seed : `angelo/secretariat` · `direction/direction` · `alhan/formateur`.

---

## Méthode de travail

Le travail avance par features identifiées, avec un statut à tout moment :

`À CADRER` → `SPÉCIFIÉE` → `CONÇUE` → `EN DÉV` → `LIVRÉE` · `EN ATTENTE`

Une feature ne saute pas d'étape et ne recule que sur décision explicite d'Angelo.

**Une feature est une tranche verticale de valeur métier**, livrable de bout en bout et utilisable seule. Jamais un découpage par couche technique.

**Attentes sur les réponses** : français, tutoiement, ton direct · distinguer ce qui vient des specs, ce qui est déduit, ce qui est proposé · toute affirmation réglementaire incertaine marquée `[À VÉRIFIER]` avec la source, jamais un seuil ou un article inventé · des options avec avantages et inconvénients quand il y a un vrai arbitrage · au maximum trois questions ouvertes en fin de réponse, les plus bloquantes en premier · ne jamais réécrire un livrable entier pour une correction ciblée.

**Signaler plutôt que contourner.** Une contradiction entre deux règles, une spec qui ne tient pas à l'écriture, un cas limite non prévu : le dire, proposer, laisser trancher.

---

## Points ouverts à ne pas oublier

| Réf | Sujet |
|---|---|
| Qualiopi | Analyse écrite dans `docs/QUALIOPI.md` (août 2026) : grille des 32 indicateurs, backlog logiciel priorisé (enquêtes de satisfaction en premier), 3 questions à trancher (NDA/calendrier, CFA/sous-traitance, titres pro) |
| Q-130 | Fréquence chiffrée des modifications de planning — dernière donnée terrain manquante |
| Q-139 | Échéance réelle d'envoi des demandes de places : le 5 de M-2, ou le 25 pour M+2 et M+3 |
| Q-142 · V-13 | Durées de conservation par type de donnée |
| D-07 | Durabilité et export, non spécifiée. Née d'un incident réel |
| 36 `A-xx` · 13 `V-xx` | Défauts appliqués et points réglementaires, à revoir avant mise en production |
