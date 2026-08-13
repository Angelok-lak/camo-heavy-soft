# QUESTIONS.md — Registre des questions produit

Projet : logiciel de gestion pour centre de formation aux permis du groupe lourd.
Document jumeau de `FEATURES.md`.

---

## Conventions

| Préfixe | Objet |
|---|---|
| `Q-xx` | Question produit |
| `A-xx` | Arbitrage avec défaut appliqué |
| `V-xx` | Point réglementaire à vérifier |
| `H-xx` | Hypothèse de cadrage |

**Statuts** : `OUVERTE` · `EN ATTENTE TERRAIN` · `DÉFAUT APPLIQUÉ` · `TRANCHÉE`

---

## Synthèse

| | Nombre |
|---|---|
| Questions `Q` créées | 137 |
| Tranchées | 112 |
| Ouvertes ou bloquantes | 7 |
| En attente terrain | 4 |
| Défauts appliqués `A` | 36 |
| Points réglementaires `V` | 12 |

---

## Section 1 — Ouvertes et bloquantes

| ID | Question | Statut | Bloque |
|---|---|---|---|
| Q-139 | **Échéance réelle d'envoi des demandes de places** | **TRANCHÉE par les fichiers réels** (TABLEAU BE C CE août/septembre 2026) : « à envoyer avant le 5 juin » pour août, « le 5 juillet » pour septembre → **le 5 de M-2**. Le jour reste paramétrable (`school.seat_request_deadline_day`). L'écart avec l'entretien (le 25) à re-vérifier à l'occasion | F-33, RG-177 |
| Q-130 | **Fréquence chiffrée des modifications de planning** | EN ATTENTE TERRAIN | dimensionnement F-09 |
| Q-132 | Périmètre exact du suivi des versements en autofinancement | OUVERTE | F-34, hors lot 1 |
| Q-43 | Existe-t-il une formation où une absence invalide le stage malgré rattrapage ? | OUVERTE | RG-04 |

---

## Section 2 — En attente terrain

| ID | Question | Statut |
|---|---|---|
| Q-130 | **Fréquence chiffrée des modifications de planning sur une semaine réelle** | EN ATTENTE TERRAIN — seule donnée terrain encore manquante |
| Q-12 | Les incidents passés | PARTIELLEMENT TRANCHÉE — 2 recueillis : oubli de demande de places (→ F-33), perte de base de données (→ D-07). Un troisième reste à recueillir |
| Q-15 | Motifs de modification du planning | TRANCHÉE — indisponibilité de ressource, force majeure (climat, accident, maladie), déplacement de date par l'administration reçu par mail |
| Q-131 | Fichier Excel de demande de places | **OBTENU** — structure : lignes par semaine (Sem N + plage de dates), colonnes BE / Isolés C-D-C1-D1 / Ensemble véhicules CE-C1E-DE-D1E, comptes en unités. F-33 calée dessus. Note du centre dans le fichier : « faire mois par mois et tableau de bord demande et annulations avec moyenne » → piste F-20 |

---

## Section 3 — Défauts appliqués (`A-xx`)

Chacun est révocable. Ce sont les points à revérifier en priorité avant la phase 2.

| ID | Sujet | Défaut appliqué |
|---|---|---|
| A-01 | Absence et heures | **TRANCHÉ** — consomme toujours les heures, justifiée ou non, sans paramétrage (RG-128) |
| A-02 | Délai de préavis d'annulation | Existe, paramétrable, valeur à définir |
| A-03 | *(retiré)* | Le formateur ne peut pas annuler un créneau (RG-171) |
| A-04 | Positionnement bloquant à l'inscription | Sans objet — F-27 supprimée |
| A-05 | Grille de compétences | Grille paramétrable, modèle court par défaut |
| A-06 | Le payeur reçoit-il les résultats d'examen ? | Non. Planning, convocations, absences : oui |
| A-07 | Plusieurs payeurs par parcours | Non au MVP |
| A-08 | Passage outre une alerte critique | Tracé : auteur et horodatage, sans justification obligatoire |
| A-09 | Lieu et temps de trajet | Trajet porté par les lieux d'examen uniquement (RG-140, RG-141) |
| A-10 | Compteur d'heures | Réalisé + projeté par échéance |
| A-11 | Heures de référence | Initialisées au standard, modifiables avec trace |
| A-12 | Prérequis d'entrée non validés | Alerte forçable, pas de blocage |
| A-13 | Extension de D-02 sur l'encaissement | Saisie manuelle `ACCORDÉ` → `SOLDÉ` (date + auteur) |
| A-14 | Périmètre de validation du formateur | Indicateur par prérequis, décoché sur le financement |
| A-15 | Type d'épreuve sur une session | Champ paramétrable |
| A-16 | Durée de validité du plateau | 1 an, en paramétrage |
| A-17 | Barème des crédits | Plateau 1, circulation 2, paramétrable |
| A-18 | Délai d'alerte avant expiration du plateau | Paramétrable, valeur à définir |
| A-19 | Rattachement des crédits | Enveloppe portée par la session, non transférable |
| A-20 | Seuils horaires | Deux seuils par objectif |
| A-22 | Dérogation aux seuils | Motif choisi dans une liste courte, pas de saisie libre obligatoire |
| A-23 | Visibilité du détail horaire | Profils habilités uniquement, absent des documents externes |
| A-24 | Intégrité du compteur réalisé | Non négociable. La souplesse passe par les seuils, tracés et motivés |
| A-25 | Visibilité par profil | Formateur : progression sans seuils ni dérogations |
| A-26 | Remise en service | Lève les alertes, ne replace rien automatiquement |
| A-27 | Portée des alertes | Strictement internes |
| A-28 | Indisponibilité non bornée | Liste triée par ancienneté, sévérité `attention` au-delà de N jours |
| A-29 | Public des alertes | Défini par type d'alerte |
| A-30 | Forme des alertes de masse | Une alerte agrégée unique. Même objet que la liste d'impact de D-06 |
| A-31 | Portée du non-proposé | Le moteur ne propose pas, l'affectation manuelle reste possible |
| A-32 | Recalcul du moteur | Automatique à l'ouverture, sans alerte de changement |
| A-33 | Crédits sans résultat après la date | Restent engagés en `NON RENSEIGNÉE` |
| A-34 | Dépassement d'enveloppe | Sévérité croissante, résolution humaine |
| A-35 | Échelle de sévérité | Trois niveaux : critique, attention, info |
| A-36 | Sévérité croissante | Mécanisme unique et paramétrable (RG-114) |
| A-38 | Placement direct hors brouillon | Tranché par Angelo (retour centre) : la multi-édition est rare, F-03 « placer / retirer » s'exécute directement depuis le planning avec alertes jointes. Le brouillon reste le seul chemin pour créer, déplacer, annuler des séances (C-25 intact) |
| A-37 | Portée du jeton de version (RG-259) | Tranché par Angelo (réalisation tranche 1) : le refus ne vaut que si une séance **touchée par le brouillon** a été modifiée à l'extérieur. Une modification extérieure sur une séance non touchée passe ; ses conséquences ressortent dans les conflits recalculés à l'enregistrement, jamais en refus |
| A-39 | Demande de places après échéance | Tranché par Angelo (août 2026) : échéance d'envoi dépassée pour le mois demandé → la génération du fichier est refusée (422 + bouton désactivé avec explication). Assumé face à D-04 : l'action est devenue **sans objet** — le fichier ne peut plus être transmis à l'autorité —, ce n'est pas une règle métier adoucie en alerte |

---

## Section 4 — Points réglementaires à vérifier (`V-xx`)

Aucun n'a d'effet sur la conception : toutes les valeurs concernées sont en paramétrage (D-01).

| ID | Point | Source à consulter |
|---|---|---|
| V-01 | Durée de validité de l'avis médical groupe lourd | service-public.fr · arrêté relatif au contrôle médical de l'aptitude |
| V-02 | Durée de validité de l'ETG groupe lourd | service-public.fr, rubrique permis groupe lourd |
| V-03 | Modalités de l'évaluation préalable pour le groupe lourd | code de la route · arrêté contrat type de formation |
| V-04 | Existence d'un livret d'apprentissage réglementé | idem V-03 |
| V-05 | Volumes horaires réglementaires minimaux par formation | à préciser formation par formation |
| V-06 | Organisation de l'examen pratique groupe lourd | *résolu par le terrain : deux dates, plateau puis circulation* |
| V-07 | Durée de validité de l'épreuve hors circulation (1 an annoncé) | arrêtés relatifs aux épreuves du permis groupe lourd |
| V-08 | Les seuils 49 h / 21 h sont-ils réglementaires ou propres au centre ? | textes relatifs à la formation groupe lourd |
| V-09 | Régime applicable aux envois groupés vers des candidats inscrits | recommandations CNIL sur la prospection par courriel |
| V-10 | Une application smartphone peut-elle produire une photo-signature numérique agréée ANTS ? | documentation ANTS — **conditionne l'existence de F-35** |
| V-11 | Obligations d'habilitation des formateurs groupe lourd | à identifier |
| V-12 | Obligations de contrôle technique des véhicules poids lourd | à identifier |

---

## Section 5 — Hypothèses de cadrage (`H-xx`)

| ID | Hypothèse | Statut |
|---|---|---|
| H-01 | Centre mono-site, un seul agrément | Non confirmée |
| H-02 | Places attribuées au centre en volume | **Confirmée** — système de crédits |
| H-03 | Une formation vendue en collectif ou en individuel | **Dépassée** — parcours individuel assemblé de séances collectives |
| H-04 | Un créneau = 1 véhicule, 1 à 3 élèves en rotation | **Confirmée** |
| H-05 | Outil comptable existant à alimenter | Non confirmée |
| H-06 | Données existantes à reprendre | **Infirmée** — aucune reprise |
| H-07 | Habilitations formateurs différenciées | **Infirmée** — aucune |
| H-08 | Centre certifié Qualiopi | **Confirmée** |
| H-09 | Périmètre formations du lot 1 | Ouverte — C, D, CE connus. Le reste non tranché |
| H-10 | CRM inclus | **Suspendue** — aucun besoin exprimé |
| H-11 | Données existantes | **Infirmée** |
| H-12 | Financements publics sans Qualiopi | **Levée** — le centre est certifié |

---

## Section 6 — Archive des questions tranchées

### Cadrage général

| ID | Question | Réponse |
|---|---|---|
| Q-01 | Utilisateurs | Direction (admin), secrétariat, formateur (vue mobile). Élève non connecté |
| Q-03 | Volumes | Dimensionner pour 300 élèves simultanés |
| Q-04 | Qualiopi | Centre certifié. Pas de module dédié |
| Q-07 | Payeurs et destinataires | Entreprise, organisme ou élève. Deux contacts : élève + payeur |
| Q-09 | Existant | Remplace logiciel, tableur, agenda. Aucune reprise de données |
| Q-10 | Mode de planification | À rebours depuis la session d'examen |
| Q-11 | Douleur réelle | Planning, planification, suivi élève, notification/mail |
| Q-70 | Forfait ou décompte horaire | **Forfait.** Le compteur est un outil de pilotage interne |
| Q-71 | Qui ne doit pas voir le détail horaire | Élève, entreprise, financeur |

### Parcours et formation

| ID | Question | Réponse |
|---|---|---|
| Q-16 | Ce que l'élève achète | Un objectif. Parcours = élève + objectif + heures + échéances + payeur |
| Q-17 | Objectifs simultanés | Non, un seul actif à la fois |
| Q-18 | Contenu variable par élève | Non, identique |
| Q-19 | Durée d'une session | Flexible, définie au niveau des séances |
| Q-20 | Type de session | Existe, ne contraint ni ressources ni élèves |
| Q-21 | Éligibilité session / objectif | Aucune. Placement libre |
| Q-22 | Capacité d'une session | Définie à la création |
| Q-23 | Session pleine | On force, on alerte |
| Q-25 | Définition de « terminé » | Heures atteintes + prérequis d'examen validés |
| Q-26 | Construction du parcours | Au fil de l'eau |
| Q-30 | Niveaux session / séance / présence | Confirmés. Le planning manipule la séance |
| Q-31 | Toutes les séances ou certaines | L'unité est la séance. La session est un raccourci |
| Q-32 | Heures planifiées ou réalisées | Présence constatée |
| Q-33 | Rotation | Chaque élève présent compte la durée totale |
| Q-35 | Objectifs du lot 1 | Le modèle volume + prérequis suffit |
| Q-54 | Répartition du volume | C 49/70 · D 49/70 · CE 70/90 |
| Q-61 | Répartition du CE | 70 h avant plateau, 20 h avant circulation |
| Q-63 | Heures au-delà du seuil plateau | Seul le total compte pour la circulation |
| Q-66 | Mécanisme de dérogation | Deux formes : seuil abaissé, ou déclaré prêt pour l'épreuve |
| Q-72 | Clôture d'un parcours | Action explicite. Statut actif / clôturé |
| Q-73 | Rachat d'heures après échec | Nombre libre, sans effet sur les heures de référence |

### Prérequis et financement

| ID | Question | Réponse |
|---|---|---|
| Q-14 | Ce qui bloque le démarrage et l'examen | Entrée : financement validé ou fonds disponibles. Examen : dossier complet → NEPH, et ETG. Plus le solde à l'examen |
| Q-34 | Prérequis manuels | Deux jeux distincts : entrée et examen |
| Q-37 | Financement bloquant ou alertant | Pas de blocage. `ACCORDÉ` ouvre l'entrée |
| Q-38 | Qui valide un prérequis | Secrétariat et formateur |
| Q-39 | Cas où l'ETG n'est pas requis | Prérequis manuel avec document, paramétrable par objectif |
| Q-40 | Qui saisit la présence | Formateur et secrétariat, corrections tracées |
| Q-42 | Encaissement | Passage manuel à `SOLDÉ` |
| Q-44 | Formateur et financement | Ne valide pas, ne voit pas le détail. Indicateur simple |
| Q-105 | Justificatif d'épreuve déjà obtenue | Facultatif, aucun contrôle |

### Examens et crédits

| ID | Question | Réponse |
|---|---|---|
| Q-13 | Arrivée des places | Demande par le centre, quota indexé au nombre de formateurs, unités fixes, cycle bimestriel (~5 semaines de préavis), transmission par fichier Excel |
| Q-27 | Fongibilité | Plusieurs formations sur une même session |
| Q-28 | Types d'examens | Plateau et circulation. Pas d'ETG |
| Q-29 | Ordre de construction | Session d'examen d'abord, planning à rebours |
| Q-41 | Une ou deux dates | Deux dates séparées, plateau puis circulation |
| Q-45 | Validité d'un résultat partiel | Plateau valable 1 an, obligatoire avant la circulation |
| Q-46 | Obtention des crédits | Saisie manuelle à la création de la session |
| Q-47 | Péremption des crédits | Sans objet — enveloppe rattachée à la session |
| Q-48 | Crédit perdu ou restitué | Perdu dans tous les cas, sauf retrait avant la date |
| Q-49 | Stock global ou séparé | Séparé par session, non fongible |
| Q-50 | Qui fixe la session | Direction ou secrétariat |
| Q-51 | Limite d'une session | Une seule saisie : l'enveloppe en crédits |
| Q-52 | Session mixte | Oui, dans la limite de l'enveloppe |
| Q-56 | Délai plateau → circulation | Aucun minimum |
| Q-57 | Règles du moteur | Détaillées en Q-88 à Q-99 |
| Q-58 | Rattachement des crédits | Enveloppe portée par la session |
| Q-59 | Limite dure ? | Non. D-04 sans exception |
| Q-60 | Crédits non consommés | Perdus, sans report |
| Q-62 | Session en prévisionnel | Aucun statut, elle existe dès sa saisie |
| Q-65 | Ressources mobilisées | Oui, selon les véhicules assignés |
| Q-74 | Compatibilité véhicule / catégorie | Alerte seule |
| Q-75 | Limite physique de candidats | Hors périmètre V1 |
| Q-76 | Session annulée par l'autorité | Crédits non perdus |
| Q-78 | Session mixte plateau / circulation | Oui |
| Q-101 | Désaffectation avant la date | Crédits libérés |
| Q-102 | Plateau échoué, circulation engagée | Alerte seule |
| Q-103 | Formateur et résultat d'examen | Non |

### Moteur d'affectation

| ID | Question | Réponse |
|---|---|---|
| Q-88 | État actuel ou projeté | Projeté sur la capacité à atteindre le seuil, hypothèse non bloquante |
| Q-89 | Candidats non conformes | Non proposés par le moteur, affectables manuellement |
| Q-90 | Double proposition | Oui |
| Q-91 | Critères de priorité | Expiration plateau, ancienneté, représentation, avance/retard |
| Q-92 | Ordre strict ou pondéré | Score pondéré |
| Q-93 | Remplissage de l'enveloppe | Aucun. Arbitrage humain |
| Q-94 | Préférence à égalité | Le plateau prime |
| Q-95 | Compatibilité véhicule | Proposé avec signalement |
| Q-96 | Délais minimums | Aucun |
| Q-97 | Forme du résultat | Liste classée, explication pour les proposés |
| Q-98 | Portée | Session par session |
| Q-99 | Recalcul | Automatique à l'ouverture, sans alerte |
| Q-100 | Rythme hypothétique | Valeur paramétrée au centre |

### Ressources

| ID | Question | Réponse |
|---|---|---|
| Q-79 | Habilitations formateurs | Aucune |
| Q-80 | Actions groupées en V1 | Annulation de session, indisponibilité, absence formateur, libération de séances |
| Q-81 | Annulation d'un lot | Non en V1 |
| Q-82 | Horaires individuels | Oui, informatifs et non bloquants |
| Q-83 | Formateur et ses indisponibilités | Consultation seule |
| Q-84 | Salles et plateaux | Oui, même objet, sans catégories |
| Q-85 | Horaires individuels | Idem Q-82 |

### Séances, planning, aléas

| ID | Question | Réponse |
|---|---|---|
| Q-24 | Ordre entre sessions | Aucun |
| Q-106 | Absence et heures | **Contesté — voir Q-137** |
| Q-107 | Signature de l'élève | Non en V1 |
| Q-108 | Motifs d'absence justifiée | Maladie, convocation, employeur, autre |
| Q-109 | Durée d'une séance | Standard ou libre |
| Q-110 | Type et ressources | Types multiples informatifs + indicateur « requiert un véhicule » |
| Q-111 | Nombre max d'élèves | Saisi librement, déduction indicative possible |
| Q-112 | Rebours par élève ou ressource | Par ressource. Vue élève en complément |
| Q-113 | Séance vide ou remplie | Créée vide puis remplie |
| Q-114 | Ordre entre types de séances | Aucun |
| Q-115 | Liste des conflits | Arrêtée (RG-138). Horaires individuels retirés |
| Q-116 | Temps de trajet | Lieux d'examen uniquement |
| Q-117 | Vue par défaut | Résumé de la semaine |
| Q-118 | Mode d'édition | Glisser-déposer et formulaire |
| Q-119 | Mode brouillon | Session d'édition avec verrou exclusif |
| Q-120 | Périmètre du brouillon | Vues planning |
| Q-121 | Édition concurrente | Verrou exclusif |
| Q-122 | Recentrage de F-10 | Confirmé. Critères : échéance, ancienneté sans séance, retard |
| Q-123 | Type requérant un véhicule | Oui, alerte à l'oubli |
| Q-124 | Création d'une session de formation | Un lot de séances liées, chacune éditable seule |
| Q-125 | Délai de libération du verrou | 30 min, paramétrable |
| Q-126 | Formateur et séances des collègues | Oui, en lecture |
| Q-127 | Formateur et annulation | Non |
| Q-128 | Absence annoncée | Oui, traitée comme tout incident |
| Q-129 | Traitements d'impact clos | Conservés et consultables |

### Communications

| ID | Question | Réponse |
|---|---|---|
| Q-86 | Envoi groupé : cible | Soit les élèves, soit les payeurs |
| Q-87 | Modèles de messages | Convocation, rappel de séance, information de session |
| Q-104 | Répartition des sévérités | Validée, sauf crédits non engagés : info puis attention le dernier jour |

---

## Corrections de numérotation

Deux chaînes de travail parallèles ont divergé. Réattributions appliquées ici :

| ID d'origine | Nouvel ID | Objet |
|---|---|---|
| Q-125 *(chaîne terrain)* | **Q-130** | Fréquence chiffrée des modifications de planning |
| Q-126 *(chaîne terrain)* | **Q-131** | Format du fichier Excel de demande de places |
| Q-127 *(chaîne terrain)* | **Q-132** | Réouverture partielle de D-02 sur les versements |
| Q-128 *(chaîne terrain)* | **Q-133** | Formule du quota de crédits |
| Q-124 *(ex-Q-103, chaîne terrain)* | **Q-134** | Fermetures du centre dans le calcul des semaines |
| Q-85 *(chaîne ressources)* | **Q-135** | Alertes d'échéance en sous-features |
| Q-86 *(chaîne ressources)* | **Q-136** | Cumul de rôles formateur + secrétariat |
| V-07 / V-08 / V-09 *(chaîne ressources)* | **V-11 / V-12** | Habilitations formateurs, contrôle technique |
| *(nouveau)* | **Q-137** | Contradiction sur les absences |

Les identifiants `Q-124` à `Q-129` et `V-07` à `V-09` conservent le sens qu'ils ont dans ce document.
`RG-33`, `RG-35` et `RG-45` sont supprimées. `RG-148` n'a jamais été attribuée et reste réservée.

---

# Ajouts et mouvements des phases 2 et 3

## Tranchées depuis la clôture de la phase 1

| ID | Question | Réponse |
|---|---|---|
| Q-137 | Une absence consomme-t-elle les heures ? | **Oui, toujours, justifiée ou non.** Le terrain fait foi. Impose deux compteurs distincts : consommées et effectuées (RG-128, RG-172) |
| Q-131 | Format du fichier Excel de demande de places | Structure établie depuis deux fichiers réels : un fichier par mois, maille hebdomadaire, trois familles (BE · isolés C/D/C1/D1 · ensembles CE/C1E/DE/D1E) |
| Q-138 | Les quantités du formulaire : examens ou unités ? | **Unités**, cohérentes avec l'enveloppe des sessions |
| Q-133 | Formule du quota de crédits | Non applicable. Total affiché en indication seule (RG-195) |
| Q-134 | Fermetures du centre dans le calcul | Jours fériés et fermetures paramétrés, retirés des semaines proposées (RG-178) |
| Q-140 | Correspondance objectifs → familles du formulaire | Paramétrable (RG-175) |
| Q-141 | Avis médical : document ou validité seule | Le document est stocké, avec durée courte et distincte (RG-199, RG-256) |
| Q-142 | Durées de conservation | Principe acté : une durée par type, paramétrable, avec événement de référence. Valeurs à confirmer (V-13) |
| Q-143 | Fusion de doublons en V1 | Oui |
| Q-144 | Qui déclenche la purge | Liste + action explicite de la direction (RG-201) |
| Q-145 | Politique de mot de passe et durée de session | Standard paramétrable. Pas d'authentification forte en V1 |
| Q-146 | Réinitialisation de mot de passe | Par la direction. Pas de circuit autonome en V1 |
| Q-147 | Formateur cumulant le secrétariat | Voit le financement. Conséquence assumée de l'union des droits (RG-208) |
| Q-136 | Cumul de rôles sur un compte unique | Oui, droits en union (RG-208) |
| Q-135 | Alertes d'échéance en sous-features | Sans objet — F-15 et F-16 sont livrables telles quelles |
| — | Financement requis à l'examen | `ACCORDÉ` en mode organisme, `SOLDÉ` en autofinancement (RG-11, RG-15) |
| — | Le formateur peut-il corriger une présence ? | Oui, sur ses propres séances (RG-20) |
| — | Le moteur écarte-t-il les prérequis manquants ? | Non, il les propose avec mention. Rattrapage par alerte croissante (RG-90, RG-246, RG-247) |
| — | Confirmation à l'ajout d'un candidat | Oui, énonçant unités engagées et restantes (RG-248) |
| — | Groupement de la liste à faire | Temporel par défaut (RG-235) |
| — | Ressources : liste unique ou onglets ? | Onglets par type, filtres et tri communs (RG-258) |
| — | Propagation d'un paramètre aux parcours dérogés | Exclus par défaut, inclusion nommée possible (RG-252) |
| — | Planning modifié sous un brouillon | Refus de l'enregistrement, brouillon conservé, modifications extérieures affichées (RG-259) |
| — | Langue du code | Anglais, avec `GLOSSARY.md` comme table de correspondance |
| — | Architecture | Binaire Go conteneurisé, pas de serverless pour le cœur. PostgreSQL. React en SPA. Voir `CLAUDE.md` |

## Encore ouvertes

| ID | Question | Statut | Bloque |
|---|---|---|---|
| Q-139 | Échéance réelle d'envoi des demandes de places | **EN ATTENTE TERRAIN** | F-33, RG-177 |
| Q-130 | Fréquence chiffrée des modifications de planning | EN ATTENTE TERRAIN | dimensionnement F-09 |
| Q-12 | Un troisième incident terrain reste à recueillir | EN ATTENTE TERRAIN | règles de gestion |
| Q-132 | Périmètre du suivi des versements en autofinancement | OUVERTE | F-34, hors lot 1 |
| Q-43 | Formation où une absence invalide le stage malgré rattrapage | OUVERTE | RG-04 |
| — | D-07 durabilité et export : non spécifiée | OUVERTE | exigence transverse |

## Point réglementaire ajouté

| ID | Point | Source à consulter |
|---|---|---|
| V-13 | Durées de conservation : contrats de formation, pièces comptables, feuilles de présence, mandats ANTS, avis médical, journaux d'accès | code de commerce · code civil (prescription) · fiches pratiques et référentiels CNIL. **Aucune valeur retenue à ce stade**, toutes en paramétrage |
