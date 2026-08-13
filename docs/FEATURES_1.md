# FEATURES.md — Source de vérité de l'avancement

Projet : logiciel de gestion pour centre de formation aux permis du groupe lourd.
Phase en cours : **Phase 3 — Réalisation, tranche 1**.
Phases 1 et 2 closes sur le lot 1. Voir `CLAUDE.md` pour l'architecture et le découpage en tranches.

---

## Conventions

**Identifiants stables, jamais réattribués.**

| Préfixe | Objet |
|---|---|
| `F-xx` | Feature (tranche verticale de valeur métier) |
| `RG-xx` | Règle de gestion |
| `US-xx` | User story |
| `D-xx` | Décision actée |
| `P-xx` | Processus métier |

**Statuts, dans cet ordre strict.** Une feature ne saute pas d'étape et ne recule que sur décision explicite.

`À CADRER` → `SPÉCIFIÉE` → `CONÇUE` → `EN DÉV` → `LIVRÉE` · `EN ATTENTE` (bloquée, préciser par quoi)

---

## Décisions actées

| ID | Décision |
|---|---|
| **D-01** | Outil interne, centre unique, ouverture SaaS envisagée ensuite. Toute table porte un identifiant de centre et toute requête est filtrée dessus. **Aucune règle métier en dur** : tarifs, durées, intitulés, seuils, modèles de documents, catégories vont en paramétrage. |
| **D-02** | Financement et facturation hors MVP. Le MVP porte le financeur pressenti, un statut de prise en charge et un export comptable. Exclus : devis, conventions, factures, relances, intégrations financeurs. Deux extensions mesurées admises : passage manuel à `SOLDÉ` (RG-19) et suivi des versements en autofinancement (F-34, sous réserve de Q-132). |
| **D-03** | Planning en saisie manuelle avec détection de conflits + suggestion assistée. Pas d'optimisation automatique globale. Les deux sens reposent sur **un composant de disponibilité unique** (RG-78), avec des règles déclaratives et explicables. Le planificateur garde toujours la main. |
| **D-04** | **Le système alerte, il ne bloque jamais.** Sans exception. Trois sévérités (RG-113), public par type d'alerte (RG-82), alertes strictement internes (RG-72), sévérité croissante possible à l'approche d'une échéance (RG-114). Tout forçage est tracé. |
| **D-05** | **Chaque feature émet ses événements et compteurs dès sa livraison.** Un bloc « Événements et compteurs » est obligatoire dans toute spécification. F-20 est réduite à l'assemblage et à l'affichage. |
| **D-06** | **Toute action pouvant toucher plusieurs objets est conçue dès le départ en action groupée** : liste d'impact calculée, aperçu des conséquences, application, compte rendu, identifiant de lot dans la trace. Pas de feature dédiée. |
| **D-08** | **Chaque feature déclare ses éléments « à faire »** : nature, objet, raison, échéance, profil concerné, écran de destination. F-36 est réduite à l'assemblage et à l'affichage. |
| **D-07** | **Durabilité et export des données.** Issue de l'incident de perte de base signalé par le centre. Sauvegarde, restauration, export exploitable hors du logiciel. *(À spécifier — voir « Décisions à consolider ».)* |

---

## Synthèse de l'avancement

| Statut | Nombre |
|---|---|
| `CONÇUE` | **17** (lot 1 complet) |
| `À CADRER` | 16 |
| Supprimées | 4 |

**Lot 1, `CONÇUE`** : F-01, F-02, F-03, F-04, F-05, F-06, F-07.1, F-07.2, F-09, F-10, F-12, F-13, F-15, F-16, F-17, F-29, F-36.

Modèle de données, écrans et contrats d'API arrêtés pour ces 17 features.

---

## Tableau des features

| ID | Feature | Statut | Lot |
|---|---|---|---|
| F-01 | Paramétrer le catalogue et les référentiels | **CONÇUE** | Lot 1 |
| F-02 | Créer et qualifier un dossier candidat | **CONÇUE** | Lot 1 |
| F-03 | Placer les élèves sur les séances | **CONÇUE** | Lot 1 |
| F-04 | Construire et suivre le parcours d'un élève | **CONÇUE** | Lot 1 |
| F-05 | Suivre le dossier de financement | **CONÇUE** | Lot 1 |
| F-06 | Gérer les crédits et les sessions d'examen | **CONÇUE** | Lot 1 |
| F-07.1 | Affecter un candidat à une session d'examen et saisir le résultat | **CONÇUE** | Lot 1 |
| F-07.2 | Proposer une liste classée de candidats pour une session | **CONÇUE** | Lot 1 |
| F-08 | Gérer l'échec : représentation et heures rachetées | À CADRER | MVP |
| F-09 | Poser, éditer et réparer le planning | **CONÇUE** | Lot 1 |
| F-10 | Proposer les élèves à placer sur une séance | **CONÇUE** | Lot 1 |
| F-11 | Consulter son planning en tant que formateur (mobile) | À CADRER | MVP |
| F-12 | Constater la présence et produire la feuille de présence | **CONÇUE** | Lot 1 |
| F-13 | Suivre les heures et l'état de préparation | **CONÇUE** | Lot 1 |
| F-14 | Générer les documents de dossier | À CADRER | MVP |
| F-15 | Gérer les ressources matérielles et leurs indisponibilités | **CONÇUE** | Lot 1 |
| F-16 | Gérer les formateurs, disponibilités et absences | **CONÇUE** | Lot 1 |
| F-17 | Authentification, rôles et permissions | **CONÇUE** | Lot 1 |
| F-18 | Journalisation des accès et purge RGPD | À CADRER | MVP |
| F-19 | Rechercher, filtrer, importer/exporter les candidats | À CADRER | MVP |
| F-20 | Tableau de bord direction | À CADRER | MVP |
| F-21 | Relances et actions commerciales | À CADRER | V2 |
| F-24 | Export vers l'outil comptable | À CADRER | MVP |
| F-25 | Devis, conventions, factures, règlements | À CADRER | V2 |
| F-28 | Saisir et consulter le suivi pédagogique | À CADRER | MVP |
| F-29 | Gérer les prérequis d'entrée et d'examen | **CONÇUE** | Lot 1 |
| F-30 | Envoyer convocations et rappels | À CADRER | MVP |
| F-31 | Gérer les entreprises clientes et leurs stagiaires | À CADRER | MVP |
| F-32 | Envoyer un message groupé depuis un modèle | À CADRER | MVP |
| F-33 | Demander et suivre les places d'examen (fichier Excel) | À CADRER | MVP |
| F-34 | Suivre les versements en autofinancement | À CADRER | MVP |
| F-35 | e-photo par lien élève (prestataire tiers) | À CADRER | Hors MVP |
| F-36 | Liste à faire — file de travail unifiée | **CONÇUE** | Lot 1 |
| F-37 | Gestion de flotte : contrôle technique, entretien, kilométrage | À CADRER | V2 |

### Features supprimées — IDs non réattribués

| ID | Feature | Motif |
|---|---|---|
| F-22 | Dossier de preuve Qualiopi | Pas de module dédié. Le centre est certifié ; la traçabilité reste un sous-produit (F-12, F-14, F-18). |
| F-23 | Portail élève | L'élève ne se connecte pas. Il reçoit uniquement des mails (F-30). |
| F-26 | Traiter les conséquences d'un aléa | Même scope que F-09. Contenu intégralement absorbé par F-09. |
| F-27 | Positionnement initial | Volume standard par objectif, aucune évaluation individuelle. L'ajustement passe par F-04 (RG-01, RG-42). |

---

## Chaîne de livraison du lot 1

```
F-01 + F-17
      ↓
F-15 + F-16
      ↓
    F-09  ←── clé de voûte
      ↓
F-03 + F-10
      ↓
    F-12
      ↓
    F-13
      ↓
F-02 + F-05 + F-29 → F-04
      ↓
    F-06
      ↓
F-07.1 → F-07.2
```

**Note de découpage** : F-09 est de loin la plus grosse feature du lot. Elle se livrera vraisemblablement en deux temps — poser et éditer, puis traiter les impacts.

---

## Détail des features spécifiées

### F-03 — Placer les élèves sur les séances · `SPÉCIFIÉE`

Placer un élève sur une séance, l'en retirer, placement groupé dans les deux sens, inscription groupée à une session de formation comme raccourci. L'unité réelle est la **séance**.
Hors périmètre : création des séances (F-09), proposition d'élèves (F-10), présences (F-12).
**RG** : 135, 137, 138, 155 à 159. **US** : 46 à 49.

### F-04 — Construire et suivre le parcours d'un élève · `SPÉCIFIÉE`

Le parcours = élève + objectif + heures de référence + deux échéances + payeur. Porte deux axes indépendants : **statut de vie** (`ACTIF` / `CLÔTURÉ`) et **état de préparation** (dérivé : en formation, prêt pour le plateau, prêt pour la circulation, permis obtenu).
Porte les dérogations, le marqueur « déclaré prêt pour l'épreuve » et la mention « épreuve déjà obtenue ».
**RG** : 01, 24, 42, 47, 54, 59, 61, 101, 107 à 109, 115 à 118, 130, 131. **US** : 26 à 30.

### F-06 — Gérer les crédits et les sessions d'examen · `SPÉCIFIÉE`

Une session d'examen porte **une seule limite saisie** : son enveloppe de crédits (plateau 1, circulation 2). Pas de capacité en candidats. Pas de statut prévisionnel. Mobilise ses véhicules et formateurs assignés, trajet compris.
**RG** : 29, 31, 34, 36, 39 à 41, 44, 58, 62 à 64, 140, 152. **US** : 01 à 07.
*Réserve* : l'incident terrain « oubli de demander une date d'examen » n'est pas couvert. Il fonde F-33.

### F-07.1 — Affecter un candidat et saisir le résultat · `SPÉCIFIÉE`

Cycle de l'affectation : `ENGAGÉE` → `RÉUSSIE` / `ÉCHOUÉE` / `ABSENTE` / `RETIRÉE` / `NON RENSEIGNÉE`.
Un crédit est consommé à la saisie d'un résultat, perdu en cas d'absence, libéré si retrait avant la date, et **reste engagé** tant qu'aucun résultat n'est saisi.
**RG** : 24 à 26, 29, 39, 41, 62, 83, 90, 94, 104 à 112. **US** : 15 à 21.

### F-07.2 — Proposer une liste classée de candidats · `SPÉCIFIÉE`

Vivier **projeté** à la date de la session, score pondéré à quatre critères : expiration proche du plateau, ancienneté d'inscription, représentation après échec, avance ou retard sur les heures. Session par session. Pas de remplissage automatique de l'enveloppe. Recalcul automatique à l'ouverture, sans alerte de changement.
**RG** : 89 à 100, 102. **US** : 22 à 25.

### F-09 — Poser, éditer et réparer le planning · `SPÉCIFIÉE`

Deux entités : la **séance** (`PLANIFIÉE` / `ANNULÉE`) et le **traitement d'impact** (`À TRAITER` / `EN COURS` / `CLOS`).
Session d'édition avec verrou exclusif par périmètre : les modifications s'accumulent, les conflits se recalculent sur le brouillon, on enregistre en un lot ou on abandonne tout.
Absorbe le traitement d'impact, le remplacement de ressource en masse et l'absence annoncée.
**RG** : 55, 56, 60, 133, 134, 138 à 152, 162 à 171. **US** : 40 à 45, 53 à 58.

### F-10 — Proposer les élèves à placer sur une séance · `SPÉCIFIÉE`

Score pondéré : proximité de l'échéance d'examen, ancienneté sans séance, retard sur les heures projetées. Seuls exclus : les élèves déjà engagés sur le créneau ou déjà sur la séance. En vue élève, la mécanique s'inverse et propose des créneaux.
**RG** : 153, 154, 160, 161. **US** : 50 à 52.

### F-12 — Constater la présence · `SPÉCIFIÉE`

Présence par élève et par séance, quatre valeurs (`NON RENSEIGNÉE`, `PRÉSENT`, `ABSENT JUSTIFIÉ`, `ABSENT NON JUSTIFIÉ`). Saisie par défaut « tous présents », tolérante à la perte de réseau. Chaque élève présent compte la **durée totale** de la séance.
**RG** : 20, 33, 53, 76, 123 à 128, 132. **US** : 35 à 39.
*Réserve* : RG-128 contredit le terrain (Q-137).

### F-13 — Suivre les heures et l'état de préparation · `SPÉCIFIÉE`

Compteur **réalisé** (présences constatées) et **deux compteurs projetés**, un par échéance. Comparaison aux seuils propres du parcours, alertes d'écart, surveillance de l'expiration du plateau. Compteurs jamais stockés, recalculés à la lecture. Le réalisé n'est jamais saisissable.
**RG** : 04, 07 à 09, 27, 38, 43, 51, 52, 57, 103, 119 à 122. **US** : 31 à 34.

### F-15 — Ressources matérielles et indisponibilités · `SPÉCIFIÉE`

Véhicules, salles, plateaux. Cycle `ACTIVE` / `ARCHIVÉE` (suppression = archivage). Indisponibilité `EN COURS` / `TERMINÉE`, fin éventuellement inconnue. La remise en service lève les alertes et propose de traiter les séances restées assignées ; elle ne replace rien automatiquement.
**RG** : 62, 65 à 67, 73 à 75, 78 à 80. **US** : 08 à 11, 13, 14.

### F-16 — Formateurs, disponibilités et absences · `SPÉCIFIÉE`

Même mécanique d'indisponibilité. **Aucune habilitation par catégorie.** Horaires individuels informatifs, ne produisant aucun conflit. Cumul d'heures travaillées comme indicateur de charge, pas outil de paie. Compte utilisateur optionnel.
**RG** : 65, 66, 75 à 77, 81, 139. **US** : 12.

---

## Règles de gestion consolidées

### Parcours, seuils et dérogations

| ID | Règle |
|---|---|
| RG-01 | Un parcours porte des heures de référence, initialisées depuis l'objectif, modifiables avec trace |
| RG-42 | Un parcours porte ses **propres** seuils, dérogeables individuellement avec auteur, date et motif codé |
| RG-46 | Les seuils atteints ou non produisent une information, jamais un blocage |
| RG-47 | La dérogation s'effectue depuis l'alerte elle-même, sans changement d'écran |
| RG-54 | État de préparation : en formation · prêt pour le plateau · prêt pour la circulation · permis obtenu |
| RG-59 | Statut de vie `ACTIF` / `CLÔTURÉ`. Clôture explicite, avec motif, auteur et date |
| RG-101 | Marqueur « déclaré prêt pour l'épreuve », par échéance, avec auteur, date et motif |
| RG-107 | Mention « épreuve déjà obtenue », par type d'épreuve, avec date. Justificatif facultatif, aucun contrôle |
| RG-108 | Une épreuve plateau déjà obtenue alimente le calcul d'expiration comme un plateau du centre |
| RG-109 | Une circulation déjà obtenue produit l'état `permis obtenu` |
| RG-110 | Les épreuves obtenues hors système sont exclues des taux de réussite |
| RG-115 | Un élève ne porte qu'un parcours `ACTIF` à la fois |
| RG-116 | Motifs de clôture : permis obtenu, abandon, refus de financement, transfert, autre. Jamais automatique |
| RG-117 | Un parcours clôturé sort des effectifs, des viviers et des listes courantes. Reste consultable |
| RG-118 | Les deux échéances sont modifiables indépendamment. Retirer une échéance remet en alerte |
| RG-129 | Le parcours compte le nombre de présentations par type d'épreuve |
| RG-130 | Création d'un parcours : secrétariat et direction |
| RG-131 | La clôture ouvre une liste d'impact : affectations engagées et séances futures, traitables en masse |

### Compteurs d'heures

| ID | Règle |
|---|---|
| RG-04 | Prêt pour le plateau : heures ≥ seuil plateau et prérequis d'examen validés. Prêt pour la circulation : heures ≥ total, plateau obtenu et non expiré |
| RG-07 | Le parcours expose un compteur réalisé et des compteurs projetés |
| RG-08 | Projeté inférieur au seuil → alerte, avec l'écart en heures |
| RG-09 | Une séance annulée sort du projeté et n'entre pas dans le réalisé |
| RG-33 | Chaque élève présent sur une séance compte la **durée totale** de la séance |
| RG-38 | Les compteurs sont évalués contre les deux seuils. L'alerte précise lequel manque et de combien |
| RG-43 | Le compteur réalisé n'est **jamais** saisissable. Une correction porte sur une présence, jamais sur un total |
| RG-57 | Les heures au-delà d'un seuil ou après l'obtention du permis sont comptées sans effet sur l'état |
| RG-103 | Le compteur réalisé ne reflète que les heures effectuées au centre |
| RG-119 | Deux compteurs projetés, un par échéance |
| RG-120 | Une séance non renseignée n'entre ni dans le réalisé ni dans le projeté |
| RG-121 | Un parcours sans échéance n'a pas de projeté |
| RG-122 | Les compteurs ne sont jamais stockés. Recalculés à la lecture |

### Prérequis et financement

| ID | Règle |
|---|---|
| RG-02 | Un objectif porte **deux** jeux de prérequis paramétrables : entrée et présentation à l'examen |
| RG-03 | Un prérequis validé porte le valideur, la date et un commentaire optionnel. Dévalidable, avec trace |
| RG-05 | Un parcours non prêt affecté à une session déclenche une alerte, sans blocage |
| RG-10 | Le prérequis d'entrée « financement » est satisfait dès le statut `ACCORDÉ`. L'encaissement n'est pas requis |
| RG-11 | Prérequis d'examen : dossier administratif complet, NEPH obtenu, ETG validé |
| RG-12 | Le caractère requis de l'ETG est un paramètre de l'objectif |
| RG-13 | Placer un élève sans prérequis d'entrée validés déclenche une alerte, tracée, non bloquante |
| RG-14 | Le statut de prise en charge alimente directement le prérequis d'entrée. Pas de double saisie |
| RG-15 | Cycle du financement : `À MONTER` → `DÉPOSÉ` → `ACCORDÉ` → `SOLDÉ`, `REFUSÉ` depuis `DÉPOSÉ`. Chaque transition tracée |
| RG-16 | Un dossier `ACCORDÉ` depuis plus de N jours sans `SOLDÉ` remonte dans une liste de suivi |
| RG-17 | Un prérequis peut porter des documents justificatifs, avec date de dépôt et auteur |
| RG-18 | L'ETG est un prérequis coché manuellement. Aucune date d'ETG n'est planifiée dans le système |
| RG-19 | Passage `ACCORDÉ` → `SOLDÉ` par saisie manuelle : date et auteur, aucun montant |
| RG-21 | Un prérequis est validable par le secrétariat et le formateur |
| RG-22 | Le formateur ne valide pas le prérequis de financement |
| RG-23 | Le formateur ne voit ni financeur, ni statut détaillé, ni montant. Un indicateur simple : dossier en règle ou non |
| RG-48 | L'avis du formateur est un prérequis d'examen paramétrable, à trois valeurs |
| RG-49 | Un avis défavorable produit une alerte `attention` sur toute affectation |

### Sessions d'examen et crédits

| ID | Règle |
|---|---|
| RG-24 | Deux échéances par parcours. Le plateau précède obligatoirement la circulation |
| RG-25 | Un plateau obtenu porte sa date d'obtention et son expiration, durée paramétrable (initialisée à 1 an) |
| RG-26 | Prêt pour la circulation exige un plateau obtenu et non expiré |
| RG-27 | Alerte à N jours avant l'expiration d'un plateau sans circulation programmée |
| RG-28 | Un plateau expiré doit être repassé et reconsomme des crédits |
| RG-29 | Pas de stock global. Une session porte une enveloppe attribuée avec elle, non transférable |
| RG-31 | Compteurs par session : attribués, engagés, restants, puis consommés et perdus. Niveau centre = agrégation |
| RG-34 | Une session mixe librement catégories et types d'épreuve dans la limite de son enveloppe |
| RG-36 | L'enveloppe est la **seule** limite saisie. Modifiable avec trace |
| RG-39 | Un crédit est consommé à la saisie d'un résultat, perdu en cas d'absence, et reste engagé tant qu'aucun résultat n'est saisi. Les crédits non engagés à la date sont perdus. Aucune restitution, aucun report |
| RG-40 | Le motif de perte est enregistré : absence, échec, non-présentation, enveloppe non remplie |
| RG-41 | Dépasser l'enveloppe est possible et tracé. Sévérité montante à l'approche de la date |
| RG-44 | Une session n'a pas de statut prévisionnel. Elle existe dès sa saisie |
| RG-58 | Une session mobilise ses véhicules et formateurs assignés |
| RG-62 | Incompatibilité véhicule / catégorie : **une** alerte agrégée listant séances et candidats |
| RG-63 | Une session annulée libère ses ressources et signale ses affectations à replacer |
| RG-64 | Aucun contrôle de faisabilité horaire en V1 |
| RG-83 | À J-N, récapitulatif des candidats engagés et des crédits non engagés |
| RG-104 | Une désaffectation avant la date **libère** les crédits engagés |
| RG-105 | Un candidat ne porte qu'une affectation active par type d'épreuve |
| RG-106 | Résultat saisi par le secrétariat et la direction. Correction tracée avec la valeur précédente |
| RG-111 | Aucun désengagement automatique. La résolution d'un dépassement est humaine |
| RG-112 | Une affectation sans résultat après la date passe en `NON RENSEIGNÉE`. Alerte persistante |
| RG-140 | Une session d'examen immobilise ses ressources sur sa durée **augmentée du trajet aller-retour** |
| RG-152 | Les sessions d'examen apparaissent au planning, non éditables depuis celui-ci |

### Moteur d'affectation

| ID | Règle |
|---|---|
| RG-89 | Vivier **projeté** : le candidat est retenu s'il peut atteindre son seuil d'ici la date, au rythme paramétré |
| RG-90 | Un candidat sans prérequis validés n'est pas proposé, mais reste affectable manuellement avec alerte |
| RG-91 | Score pondéré, poids paramétrables : expiration du plateau, ancienneté, représentation, avance/retard |
| RG-92 | À score égal, le plateau prime sur la circulation |
| RG-93 | Aucun remplissage automatique de l'enveloppe. Le moteur classe, le centre arbitre |
| RG-94 | Un candidat peut être engagé simultanément sur un plateau et une circulation ultérieure |
| RG-95 | Une incompatibilité véhicule / catégorie ne retire pas du vivier : proposé avec signalement |
| RG-96 | Aucun délai minimum entre plateau et circulation, ni entre échec et représentation |
| RG-97 | Liste classée dans laquelle on pioche, avec explication pour les candidats proposés |
| RG-98 | Session par session. Aucune optimisation multi-sessions |
| RG-99 | Recalcul automatique à l'ouverture. Aucune alerte entre deux consultations |
| RG-100 | Le rythme hypothétique est une valeur paramétrée au niveau du centre |
| RG-102 | Un parcours « déclaré prêt » entre directement dans le vivier, sans projection |
| RG-153 | Classement des élèves à placer : proximité de l'échéance, ancienneté sans séance, retard sur le projeté |
| RG-154 | Seuls exclus du vivier F-10 : élèves déjà engagés sur le créneau ou déjà sur la séance |
| RG-160 | La liste indique l'écart à l'objectif et les jours restants avant l'échéance |
| RG-161 | En vue élève, la mécanique s'inverse : proposer les créneaux disponibles |

### Échec et représentation

| ID | Règle |
|---|---|
| RG-30 | Un échec produit deux effets tracés : heures rachetées et nouvelle présentation consommant des crédits |
| RG-32 | Un échec en circulation ne remet pas en cause le plateau, tant qu'il n'est pas expiré |
| RG-61 | Les heures rachetées sont un nombre libre, tracé et refacturable. **Aucun effet** sur les heures de référence ni sur l'état de préparation |

### Ressources

| ID | Règle |
|---|---|
| RG-65 | Une indisponibilité porte un motif, un début et une fin éventuellement inconnue |
| RG-66 | La remise en service est explicite. Elle lève les alertes et ouvre une liste d'impact pour traiter les séances restées assignées. Elle ne replace rien automatiquement |
| RG-67 | Les indisponibilités sans fin prévue sont listées par ancienneté, sévérité `attention` au-delà de N jours |
| RG-73 | Une ressource n'est jamais effacée. « Supprimer » l'archive. Consultable et restaurable |
| RG-74 | Un véhicule porte les catégories qu'il couvre |
| RG-75 | Horaires de travail individuels **informatifs**, bornés par les horaires d'ouverture du centre |
| RG-76 | Un formateur peut exister sans compte utilisateur. Le secrétariat saisit alors ses présences |
| RG-77 | Aucune habilitation par catégorie. Tout formateur peut animer toute séance |
| RG-78 | **Composant de disponibilité unique**, interrogeable dans les deux sens (D-03) |
| RG-79 | Les indisponibilités peuvent se chevaucher. Aucune fusion automatique |
| RG-80 | Une séance dont une ressource devient indisponible **conserve** son assignation et affiche le motif |
| RG-81 | Cumul d'heures travaillées par formateur : indicateur de charge, pas outil de paie |
| RG-139 | Les horaires individuels ne produisent aucun conflit |

### Séances et planning

| ID | Règle |
|---|---|
| RG-55 | L'état de préparation est visible sur chaque séance. Une séance devenue inutile est signalée |
| RG-56 | Une séance signalée peut être libérée en un geste, restituant formateur et véhicule |
| RG-60 | Le nombre de parcours actifs est affiché en permanence sur le planning |
| RG-133 | Durée d'une séance : standard paramétrable ou libre |
| RG-134 | Une séance porte zéro, un ou plusieurs types. Le type est informatif |
| RG-135 | Nombre maximum d'élèves saisi librement. Valeur indicative déductible du véhicule |
| RG-136 | Une séance est créée **vide**, puis remplie d'élèves. Deux gestes distincts |
| RG-137 | Aucun ordre obligatoire entre types de séances |
| RG-138 | Conflits détectés : élève, formateur, véhicule ou salle déjà engagé · ressource indisponible · hors horaires d'ouverture · catégorie incompatible · financement non `ACCORDÉ` · parcours clôturé. Tous forçables et tracés |
| RG-141 | Aucune règle de trajet entre séances |
| RG-142 | Vue par défaut : résumé de la semaine. Dernier choix mémorisé |
| RG-143 | Glisser-déposer et formulaire : deux moyens équivalents |
| RG-144 | Verrou exclusif par périmètre d'édition. Les autres voient qui édite |
| RG-145 | Le verrou se libère à l'enregistrement, à l'abandon, après inactivité, ou par libération forcée tracée |
| RG-146 | Les conflits sont recalculés sur l'état résultant du brouillon |
| RG-147 | L'enregistrement applique tout en un lot. L'abandon n'applique rien |
| RG-149 | Un type de séance peut être paramétré « requiert un véhicule ». Alerte `attention` à l'oubli |
| RG-150 | Annuler une séance libère ses ressources, détache ses élèves, exige un motif |
| RG-151 | Déplacer une séance conserve son identité, ses élèves et ses ressources |
| RG-155 | L'unité de placement est la **séance**. L'inscription à une session est un raccourci |
| RG-156 | Dépasser le nombre maximum d'élèves : alerte `attention` forçable |
| RG-157 | Retirer un élève d'une séance émargée : possible, avec confirmation explicite |
| RG-158 | Placement en masse dans les deux sens |
| RG-159 | Aucune vérification d'éligibilité entre objectif et type de séance |

### Traitement des aléas

| ID | Règle |
|---|---|
| RG-68 | Tout événement d'impact produit la liste des séances et affectations concernées |
| RG-69 | Une action groupée s'applique depuis cette liste, avec **aperçu des conséquences** avant application |
| RG-70 | Les modifications d'une action groupée portent un identifiant de lot commun |
| RG-71 | Compte rendu final : traité, échoué, restant |
| RG-162 | Déclencheurs : indisponibilité, remise en service, annulation de session d'examen, clôture de parcours, élève devenu prêt, absence annoncée |
| RG-163 | Actions par ligne : remplacer la ressource, annuler la séance, retirer l'élève, laisser en l'état |
| RG-164 | Le remplacement en masse interroge le composant de disponibilité créneau par créneau. Aucun forçage automatique |
| RG-165 | La liste est recalculée à chaque ouverture. Une ligne devenue sans objet en sort |
| RG-166 | « Laisser en l'état » est une décision tracée, pas une absence de décision |
| RG-167 | Une absence annoncée porte un élève, une période et un motif. Elle ouvre un traitement d'impact |
| RG-168 | Un traitement ouvert produit une alerte de sévérité croissante |
| RG-169 | Une séance impactée conserve son assignation et affiche son motif tant qu'elle n'est pas traitée |
| RG-170 | Période et motif d'absence annoncée conservés dans l'historique du parcours |
| RG-171 | Le formateur ne peut ni annuler une séance, ni déclarer un aléa |

### Présences

| ID | Règle |
|---|---|
| RG-20 | Présence saisissable par le formateur et le secrétariat. Correction tracée avec la valeur précédente |
| RG-123 | Présence par élève et par séance, quatre valeurs. Saisie par défaut « tous présents » |
| RG-124 | Aucune présence partielle |
| RG-125 | Une séance passée non renseignée produit une alerte `critique` persistante |
| RG-126 | Saisie tolérante à la perte de réseau, transmise au retour de connexion |
| RG-127 | La feuille de présence restitue des faits. Elle n'expose ni seuil, ni cumul, ni dérogation |
| RG-128 | Un `ABSENT NON JUSTIFIÉ` marque la séance refacturable *(contredit par le terrain — Q-137)* |
| RG-132 | Retirer un élève d'une séance émargée : confirmation explicite que la présence sera annulée |

### Alertes, visibilité et communications

| ID | Règle |
|---|---|
| RG-51 | Compteur, seuils et dérogations : direction et secrétariat. Le formateur voit la progression seule |
| RG-52 | Aucun document ni message externe n'expose compteur, seuils ou dérogation |
| RG-53 | Les feuilles de présence restent produites et transmissibles : ce sont des faits |
| RG-72 | Les alertes sont strictement **internes**. Toute communication externe passe par F-30, sur action humaine |
| RG-82 | Chaque type d'alerte porte un public : direction seule, direction et secrétariat, ou formateur |
| RG-84 | Un envoi groupé part d'une sélection filtrée, avec aperçu et compte rendu par destinataire |
| RG-85 | Modèles de messages paramétrables. Aucun n'expose compteur, seuils ni dérogation |
| RG-86 | Un envoi groupé résulte toujours d'une action humaine explicite |
| RG-87 | Tout événement impactant plusieurs objets produit **une** alerte agrégée, jamais N alertes unitaires |
| RG-88 | Un envoi vise soit les élèves, soit les payeurs, jamais les deux |
| RG-113 | Trois sévérités paramétrables : `critique`, `attention`, `info` |
| RG-114 | La sévérité peut croître avec le temps restant avant l'échéance. Seuils paramétrables |

### Paramétrage

| ID | Règle |
|---|---|
| RG-06 | Les valeurs standard par objectif sont en paramétrage, jamais en dur |
| RG-37 | Chaque objectif porte deux seuils. Valeurs initiales : C 49/70 · D 49/70 · CE 70/90 |
| RG-45 | *(supprimée — un échec ne modifie aucune heure de référence)* |
| RG-35 | *(remplacée par RG-39)* |
| RG-148 | *(non attribuée — trou de numérotation, ID réservé)* |

---

## Corrections de numérotation

Deux chaînes de travail parallèles ont divergé. Réattributions appliquées ici :

| ID d'origine | Nouvel ID | Objet |
|---|---|---|
| F-32 *(chaîne places d'examen)* | **F-33** | Demander et suivre les places d'examen |
| F-33 *(chaîne versements)* | **F-34** | Suivre les versements en autofinancement |
| F-32 *(chaîne e-photo)* | **F-35** | e-photo par lien élève |
| D-06 *(chaîne durabilité)* | **D-07** | Durabilité et export des données |

F-32 reste « Envoyer un message groupé depuis un modèle ». D-06 reste « Actions groupées ».
Les collisions sur `Q-xx`, `RG-xx` et `V-xx` sont traitées dans `QUESTIONS.md`.

---

## Décisions à consolider

1. **D-07 n'est pas spécifiée.** Elle naît d'un incident réel (perte de base de données). À traiter comme une exigence transverse, pas comme une feature.
2. **F-33 est fondée sur un incident réel** (oubli de demande de places → annulation de formation sur deux mois). Cycle bimestriel, quota indexé au nombre de formateurs, fichier Excel à générer. Bloquée par le format exact du fichier.
3. **RG-128 contredit le terrain.** À trancher avant toute réalisation touchant F-12 ou F-13.

---

# Ajouts des phases 2 et 3

Règles créées après la clôture de la phase 1. Les révisions annulent et remplacent la version antérieure figurant plus haut.

## Révisions

| ID | Version en vigueur |
|---|---|
| RG-10 | Le prérequis d'entrée est satisfait par `ACCORDÉ` en mode organisme, par le constat du premier versement en autofinancement |
| RG-11 | Prérequis d'examen : dossier complet · NEPH obtenu · ETG validé si requis · financement `ACCORDÉ` en mode organisme, `SOLDÉ` en autofinancement |
| RG-15 | Le dossier de financement porte un **mode**, organisme ou autofinancement, qui détermine son cycle de vie. Organisme : `À MONTER` → `DÉPOSÉ` → `ACCORDÉ` → `SOLDÉ`, `REFUSÉ` depuis `DÉPOSÉ`. Autofinancement : `À MONTER` → `EN COURS DE VERSEMENT` → `SOLDÉ`. Chaque transition tracée |
| RG-20 | La présence est saisie **et corrigée** par le formateur sur ses propres séances, et par le secrétariat ou la direction sur toutes. Correction tracée avec la valeur précédente |
| RG-90 | Un candidat dont les prérequis d'examen ne sont pas encore validés **est proposé** par le moteur, avec mention explicite de ce qui manque. Aucun candidat n'est écarté sur ce motif |
| RG-128 | Une absence, justifiée ou non, **consomme** les heures de la séance. Le motif est enregistré, sans effet sur le décompte *(tranché par le terrain, Q-137)* |
| RG-135 | Nombre maximum d'élèves saisi librement. Une valeur indicative peut être déduite du véhicule, sans jamais s'imposer |
| RG-204 | Modifier un paramètre copié affiche la liste des objets non affectés, avec possibilité de leur appliquer la nouvelle valeur, à l'unité ou en masse, avec aperçu |
| RG-235 | Le groupement par défaut de la liste à faire est **temporel** : aujourd'hui, cette semaine, plus tard. Chaque section porte son icône, son compte et ses bornes. Groupement par nature en option |

## Compteurs et absences

| ID | Règle |
|---|---|
| RG-172 | Le parcours expose deux compteurs distincts : heures **consommées** (base des seuils et de l'état de préparation) et heures **effectuées** (base des documents et des faits) |
| RG-173 | Un écart significatif entre consommées et effectuées produit une alerte `attention` : l'élève brûle son forfait sans progresser. Seuil paramétrable |
| RG-180 | Troisième compteur : heures **absentes**, ventilées entre justifiées et non justifiées. Consommées = effectuées + absentes |

## Demande de places d'examen (F-33)

| ID | Règle |
|---|---|
| RG-174 | Une demande porte un mois cible, une échéance d'envoi, et une quantité par semaine et par famille de catégories |
| RG-175 | La correspondance entre objectifs du centre et familles du formulaire (BE, isolés, ensembles) est paramétrable |
| RG-176 | Le logiciel génère le fichier au format attendu, sans le parser en retour : la réponse arrive par un autre canal |
| RG-177 | Alerte de sévérité croissante à l'approche de l'échéance d'envoi tant que la demande du mois n'est pas marquée envoyée. **Réponse directe à l'incident fondateur** |
| RG-178 | Jours fériés et fermetures du centre paramétrés et retirés des semaines proposées |
| RG-179 | Suivi par mois : demandé, obtenu, annulé, avec moyenne sur les périodes passées |
| RG-195 | Le total d'unités demandées est affiché à la saisie avec le rappel des mois précédents. Aucun plafond, aucune alerte : information seule |

Les quantités du formulaire sont des **unités**, cohérentes avec l'enveloppe des sessions (RG-29).

## Dossier candidat, prérequis, financement

| ID | Règle |
|---|---|
| RG-181 | Une personne porte son état civil complet : nom, prénoms, date, ville, département et pays de naissance |
| RG-182 | Le NEPH est porté par la **personne**, pas par le parcours. Unique, conservé d'un parcours à l'autre |
| RG-183 | Toute création est précédée d'une recherche de doublon sur nom, prénom et date de naissance. Signalé, jamais bloquant |
| RG-184 | Une personne n'est jamais effacée. « Supprimer » l'archive. Archivage refusé tant qu'un parcours `ACTIF` existe |
| RG-185 | Le dossier porte un historique métier daté et attribué, distinct de la journalisation technique |
| RG-186 | Les pièces sensibles sont réservées aux profils habilités. Toute consultation est journalisée |
| RG-187 | Le contact payeur est l'élève, une entreprise ou un organisme. Un payeur par parcours |
| RG-188 | Le type de financeur est paramétrable. Valeurs initiales : entreprise, CPF, France Travail, OPCO, Transitions Pro, AGEFIPH, autofinancement, autre |
| RG-189 | Chaque transition de financement porte auteur, date et commentaire. `REFUSÉ` exige un motif |
| RG-190 | Un parcours sans dossier de financement produit une alerte `attention` à la première pose de séance |
| RG-191 | Un prérequis peut porter une date de validité. Son dépassement le fait passer `EXPIRÉ` et alerte à N jours |
| RG-192 | L'état de complétude est dérivé : validés sur obligatoires, par jeu. Jamais saisi |
| RG-193 | Un prérequis peut être marqué non applicable, avec motif et auteur. Il sort du calcul de complétude |
| RG-194 | L'avis médical est une donnée de santé : accès restreint, journalisation des consultations, durée de conservation propre |
| RG-255 | Les jeux de prérequis sont intitulés par leur finalité — « pour entrer en formation », « pour se présenter à l'examen » — jamais par leur nom technique |
| RG-256 | L'avis médical n'expose que sa date de validité. Le document est accessible en une action supplémentaire, journalisée |

## Conservation et purge

| ID | Règle |
|---|---|
| RG-196 | Chaque type de donnée porte sa durée de conservation, paramétrable, décomptée depuis un **événement de référence** explicite |
| RG-197 | À l'échéance, purge ou anonymisation selon le type. Chaque purge journalisée : type, volume, date, règle appliquée |
| RG-198 | Une donnée servant plusieurs finalités conserve la durée la plus longue applicable. Aucune purge ne détruit une preuve encore exigible |
| RG-199 | L'avis médical porte une durée distincte et courte. Sa suppression n'affecte pas la validité historique du prérequis, qui conserve sa date et son valideur |
| RG-200 | Toute durée est livrée avec une valeur initiale et un écran de paramétrage |
| RG-201 | La purge s'effectue depuis une liste des données arrivées à échéance, sur action explicite de la direction. Jamais automatiquement, jamais silencieusement |

## Paramétrage

| ID | Règle |
|---|---|
| RG-202 | Chaque paramètre porte sa nature : copié à l'instanciation ou lu en direct. Fixée à la conception |
| RG-203 | Toute modification de paramètre est tracée : valeur précédente, auteur, date |
| RG-205 | Une valeur de liste n'est jamais supprimée si elle est référencée. Elle est désactivée |
| RG-206 | Tout paramètre est livré avec une valeur initiale exploitable. Le logiciel tourne sans paramétrage préalable |
| RG-207 | Aucune valeur métier en dur. Un écran de paramétrage existe pour chaque famille |
| RG-216 | L'aperçu d'une propagation indique les conséquences sur l'état de préparation : combien de parcours changent d'état, combien sont déjà affectés à une session d'examen |
| RG-244 | Tout écran modifiant une valeur copiée nomme sa portée dans le libellé et la confirmation. Jamais un « Modifier » nu |
| RG-245 | La confirmation d'une dérogation rappelle qu'elle ne concerne que cet élève, et propose le lien vers le paramétrage |
| RG-252 | La propagation exclut **par défaut** les objets porteurs d'une dérogation. Une case optionnelle les inclut en les nommant un par un avec leur motif |
| RG-253 | Les objets divergents sont présentés en deux populations : ancienne valeur par ancienneté, et valeur dérogée. Jamais un décompte unique |
| RG-254 | Chaque famille affiche sa portée en tête d'écran, avec l'effet chiffré sur l'état réel |

## Comptes et permissions

| ID | Règle |
|---|---|
| RG-208 | Trois profils. Un compte porte un ou plusieurs profils, ses droits sont l'**union** des profils portés |
| RG-209 | Tout compte est rattaché à un centre. Toute requête est filtrée dessus, sans exception |
| RG-210 | Un formateur peut exister sans compte. Un compte formateur est lié à une et une seule fiche formateur |
| RG-211 | Un compte n'est jamais supprimé : suspendu ou archivé. Ses traces restent attribuées |
| RG-212 | Le dernier compte direction actif ne peut être ni suspendu ni archivé |
| RG-213 | Toute action reste attribuée au compte qui l'a réalisée. Un changement de rôle n'est jamais rétroactif |
| RG-214 | **Les permissions ne suivent pas D-04** : ce qu'un profil n'a pas le droit de voir est absent, pas signalé. Une permission n'est pas une décision métier |

## Conflits, planning, aléas

| ID | Règle |
|---|---|
| RG-215 | L'alignement d'un parcours sur un nouveau standard est distinct d'une dérogation : motif automatique, tracé, **exclu des statistiques de dérogation** |
| RG-217 | Un conflit énonce toujours : la ressource ou l'élève concerné · la **plage horaire de chevauchement** · les deux objets impliqués, chacun avec ses horaires et navigable |
| RG-218 | Quand une même cause produit plusieurs conflits, le premier porte l'explication, les suivants y renvoient |
| RG-219 | Une session d'examen affiche son temps de trajet **et** la plage d'immobilisation qui en résulte, séparément |
| RG-220 | Les modifications en attente sont listées individuellement, chacune annulable avant enregistrement. Après enregistrement, aucun retour en arrière |
| RG-221 | L'écran de traitement affiche en permanence son déclencheur : nature, auteur, date, volume, avancement |
| RG-222 | L'aperçu d'une action groupée ventile en trois issues : applicable · applicable avec conflit · sans solution. Il propose l'application du sous-ensemble sûr **et** l'application complète avec forçages, comme deux choix distincts |
| RG-223 | Chaque ligne énonce sa cause en clair, jamais un libellé d'erreur |
| RG-224 | Une ligne « laissée en l'état » affiche son auteur et reste révocable |
| RG-225 | Une ligne devenue sans objet reste visible et grisée, avec son motif de sortie |
| RG-226 | Un traitement ouvert est accessible depuis trois entrées : la liste à faire · le motif d'impact affiché sur une séance, qui est un lien · l'alerte de RG-168 |
| RG-227 | L'urgence d'un traitement est portée par la **date de sa première ligne non réglée**, jamais par son volume |
| RG-228 | Chaque décision de ligne est enregistrée immédiatement. Un traitement ne se sauvegarde pas et ne se perd pas |
| RG-229 | Au-delà d'un seuil de lignes, la liste est regroupée — par jour par défaut, par ressource de remplacement ou par élève en option |
| RG-230 | Le filtre par défaut est « reste à traiter » |
| RG-259 | Un brouillon porte l'état du planning à son ouverture. Si cet état a changé à l'enregistrement, l'opération est **refusée**, le brouillon conservé intégralement, et les modifications extérieures affichées avec les conflits qu'elles créent |
| RG-260 | Le verrou d'édition protège un périmètre de planning, pas les ressources qu'il mobilise. Sessions d'examen, indisponibilités et créations hors périmètre peuvent modifier l'état sous un brouillon |

## Examens

| ID | Règle |
|---|---|
| RG-246 | Une affectation dont un prérequis d'examen manque porte une alerte de sévérité croissante à l'approche de la date |
| RG-247 | Le récapitulatif à J-N nomme les candidats dont un prérequis manque, l'unité que chacun ferait perdre, et propose le retrait. **N est paramétré au-dessus du délai nécessaire pour réaffecter et convoquer un remplaçant** |
| RG-248 | L'ajout d'un candidat depuis la liste proposée passe par une confirmation énonçant les unités engagées et restantes |

## Présences

| ID | Règle |
|---|---|
| RG-236 | La vue mobile s'ouvre directement sur la séance en cours ou la plus proche. Aucune navigation préalable |
| RG-237 | Le chemin nominal — tous présents — se termine en deux gestes. Toute exception coûte un geste, jamais un changement d'écran |
| RG-238 | Le motif d'absence se choisit en ligne, sous l'élève. `ABSENT NON JUSTIFIÉ` est un motif comme les autres |
| RG-239 | L'état de la connexion est affiché avant et après la saisie |
| RG-240 | La vue mobile enchaîne sur la séance suivante après enregistrement |
| RG-241 | La seule alerte affichée en vue mobile est celle des séances non renseignées du formateur lui-même |
| RG-242 | L'alerte de séances non renseignées est agrégée. « Ouvrir » présente la liste triée par ancienneté, le formateur choisit |
| RG-243 | Le bouton d'enregistrement énonce ce qu'il valide : « Enregistrer — 3 présents » |

## Liste à faire (F-36)

| ID | Règle |
|---|---|
| RG-231 | Une ligne entre dans la liste **si et seulement si** elle désigne une action réalisable par un profil identifié et disparaît quand l'action est faite. Les alertes d'état restent hors de la liste |
| RG-232 | Chaque ligne porte son objet, sa raison en clair, sa date d'échéance et un lien vers l'écran d'action |
| RG-233 | La liste est filtrée par profil. Le formateur ne voit que ses propres séances non renseignées |
| RG-234 | Tri par échéance de l'action, jamais par volume ni date de création |
| RG-249 | Une ligne porte son action de traitement et, lorsque la cause racine peut être levée directement, une **action secondaire de résolution**. Celle-ci fait disparaître la ligne |
| RG-250 | Le libellé énonce une condition, une date et une conséquence en langage courant. Aucun terme inventé |
| RG-251 | « Rappeler plus tard » — trois durées fixes — n'est proposé que sur les lignes sans échéance dure |

## Divers

| ID | Règle |
|---|---|
| RG-257 | Un parcours clôturé porte la mention « (clôturé) » partout où il est nommé, avec motif et date accessibles au même endroit |
| RG-258 | L'écran ressources sépare les types en onglets, chacun conservant les mêmes filtres et tri. Un onglet « toutes » rassemble les indisponibilités |

---

## Décisions à consolider

1. **D-07 n'est pas spécifiée.** Née d'un incident réel de perte de base. Exigence transverse, pas une feature.
2. **F-33 est bloquée** par l'échéance réelle d'envoi (Q-139).
3. **F-34 reste hors lot 1** : RG-19 permet déjà de solder à la main, ce qui débloque l'examen. F-34 n'ajoute que le détail des versements.
4. **36 défauts `A-xx` et 13 points `V-xx`** à revoir avant mise en production.
