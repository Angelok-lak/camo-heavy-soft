# CONTRATS_API.md — Phase 2, lot 1

Projet : logiciel de gestion pour centre de formation aux permis du groupe lourd.
Périmètre : les 17 features du lot 1.
Complète `MODELE_DONNEES.md`. Les choix de conception y sont référencés en `C-xx`.

---

## 1. Principes

### C-25 — Un contrat par feature, jamais un CRUD par table

Un endpoint générique sur les séances obligerait le front à orchestrer lui-même le verrou d'édition, le recalcul des conflits et l'application en lot — c'est-à-dire à réimplémenter les règles de gestion dans le navigateur, où elles sont contournables et intestables.

Conséquence pratique : les opérations portent des noms d'intention métier, pas des verbes de persistance. « Pousser le brouillon » et non « mettre à jour les séances ».

### C-26 — Le serveur calcule, le front affiche

Aucun conflit, aucun compteur, aucun score de classement n'est calculé côté client. Le front reçoit des résultats déjà explicités, avec leur justification en clair — c'est ce qu'exige D-03 sur l'explicabilité, et c'est ce qui rend RG-217 tenable.

### C-27 — Toute écriture multi-objets retourne un compte rendu

Jamais un simple succès. Le compte rendu porte : ce qui a été traité, ce qui a échoué avec sa raison, ce qui reste, et l'identifiant de lot (RG-70, RG-71).

### C-28 — Les compteurs sont en lecture seule

Aucune opération n'accepte un compteur d'heures ou de crédits en entrée. Ils sont dérivés (C-05), le poser en écriture ouvrirait la porte à la divergence que RG-43 interdit.

### C-29 — Les alertes ne sont pas un endpoint, elles accompagnent leur objet

Étant des requêtes et non des lignes (C-06), les alertes sont retournées avec l'objet qu'elles concernent. Une seule exception : F-36, dont c'est précisément le métier de les assembler.

### C-30 — La portée organisationnelle n'est jamais un paramètre

`centre_id` est déduit du compte appelant (RG-209, C-03). Aucune opération ne l'accepte en entrée : un paramètre serait un paramètre manipulable.

---

## 2. Conventions transverses

| Sujet | Convention |
|---|---|
| **Alertes jointes** | Tout objet retourné porte ses alertes applicables : type, sévérité, message explicité, public. Le front n'en calcule aucune |
| **Explicitation** | Un conflit porte la ressource concernée, la plage de chevauchement, et les deux objets impliqués (RG-217). Un classement porte les critères qui l'ont produit (RG-97) |
| **Concurrence** | Toute opération sur un objet éditable porte un jeton de version. Un jeton périmé produit un refus décrivant ce qui a changé, jamais un écrasement (RG-259) |
| **Traçabilité** | L'auteur est déduit du compte appelant, jamais transmis. Tout forçage porte son motif dans la requête |
| **Refus** | Un refus distingue trois natures : interdit par les permissions (F-17, RG-214), impossible structurellement (RG-105, RG-24), ou conflit de version. Une règle métier ne refuse jamais : elle alerte (D-04) |
| **Pagination** | Les listes susceptibles de dépasser la centaine d'éléments sont paginées avec un curseur. À 300 parcours, cela concerne les candidats, les séances sur période longue et les lignes d'impact |
| **Idempotence** | Les opérations d'application groupée acceptent une clé d'idempotence. Un réseau instable ne doit pas produire deux lots |

---

## 3. Contrats par feature

### F-17 — Authentification, rôles et permissions

| Opération | Entrée | Sortie |
|---|---|---|
| S'authentifier | Identifiant, secret | Session, profils portés, permissions effectives en union (RG-208) |
| Lire son contexte | — | Compte, profils, centre, fiche formateur liée le cas échéant (RG-210) |
| Lister les comptes | Filtres statut et profil | Comptes avec leurs profils et la description de ce qu'ils peuvent faire |
| Créer un compte | Identité, profils, fiche formateur optionnelle | Compte `ACTIF` |
| Modifier les profils | Compte, profils | Compte à jour. L'historique de profils conserve les périodes (RG-213) |
| Suspendre, archiver, réactiver | Compte, action | Compte à jour, ou refus si dernier compte direction actif (RG-212) |
| Réinitialiser un secret | Compte | Secret provisoire (Q-146) |

### F-01 — Paramétrage

| Opération | Entrée | Sortie |
|---|---|---|
| Lire les familles | — | Cinq sections, quinze familles, avec la **nature** de chacune : copiée ou lue en direct (C-07, RG-254) |
| Lire une famille | Famille | Valeurs, dernière modification avec auteur et valeur précédente (RG-203) |
| Prévisualiser une modification | Famille, valeurs | Pour une famille copiée : populations divergentes séparées — ancienneté et dérogation (RG-253) — et effet chiffré d'une propagation. Pour une famille directe : effet immédiat chiffré sur l'état réel |
| Modifier | Famille, valeurs | Valeurs à jour |
| Propager aux objets existants | Famille, sélection, inclusion explicite des dérogés | Compte rendu (C-27). Les dérogés sont exclus sauf demande nommée (RG-252) |
| Gérer une valeur de liste | Valeur, action | Désactivation, jamais suppression si référencée (RG-205) |

### F-15 — Ressources matérielles · F-16 — Formateurs

Un seul jeu d'opérations, le type de ressource étant un attribut (C-08).

| Opération | Entrée | Sortie |
|---|---|---|
| Lister | Type, statut, disponibilité | Ressources avec leur état courant et le **nombre de séances impactées** en cas d'indisponibilité |
| Créer, modifier | Type, libellé, spécifique | Ressource. Pour un véhicule : catégories et capacité indicative (RG-74, RG-135) |
| Archiver, restaurer | Ressource | Ressource. Archivage accepté avec séances futures, liste d'impact ouverte (RG-73) |
| Déclarer une indisponibilité | Ressource, motif, début, fin optionnelle | Indisponibilité `EN COURS` **et** traitement d'impact ouvert (RG-162) |
| Remettre en service | Ressource, date réelle | Indisponibilité `TERMINÉE` et traitement proposant les séances restées assignées (RG-66) |
| Interroger la disponibilité | Créneau, contraintes | Ressources libres. **Composant unique, sens « quelles ressources »** (RG-78, D-03) |
| Lire le cumul d'heures | Formateur, période | Séances réalisées et sessions d'examen accompagnées (RG-81) |

### F-02 — Dossier candidat

| Opération | Entrée | Sortie |
|---|---|---|
| Rechercher un doublon | Nom, prénom, date de naissance | Correspondances probables (RG-183) |
| Créer, modifier une personne | État civil complet, coordonnées, payeur | Personne. NEPH porté par la personne (RG-182) |
| Lister, rechercher | Filtres, curseur | Personnes paginées |
| Archiver, restaurer | Personne | Refus si un parcours `ACTIF` existe (RG-184) |
| Fusionner deux dossiers | Source, cible | Liste d'impact puis compte rendu (Q-143) |
| Lire l'historique métier | Personne | Événements datés et attribués (RG-185, C-10) |

### F-29 — Prérequis · F-05 — Financement

| Opération | Entrée | Sortie |
|---|---|---|
| Lire les prérequis d'un parcours | Parcours | Deux jeux intitulés par finalité (RG-255), état de complétude dérivé (RG-192), états `EXPIRÉ` calculés |
| Valider, dévalider | Prérequis, commentaire | Prérequis à jour, ou refus si le profil n'y a pas droit (RG-21, RG-22) |
| Marquer non applicable | Prérequis, motif | Prérequis hors calcul de complétude (RG-193) |
| Déposer, lire un document | Prérequis ou parcours, fichier | Document. **La lecture d'une pièce sensible est journalisée** (RG-186, RG-194) |
| Lire le dossier de financement | Parcours | Mode, cycle applicable, état, historique des transitions |
| Faire évoluer le financement | Parcours, état cible, motif si refus | Transition tracée. Le cycle dépend du mode : organisme ou autofinancement (RG-15) |

### F-04 — Parcours · F-13 — Heures et préparation

| Opération | Entrée | Sortie |
|---|---|---|
| Créer un parcours | Personne, objectif, payeur | Parcours `ACTIF`, seuils **copiés** de l'objectif (C-07, RG-01) |
| Lire un parcours | Parcours | Seuils propres · trois compteurs — effectuées, absentes, consommées (RG-172, RG-180) · deux projetés (RG-119) · état de préparation dérivé · échéances · alertes jointes |
| Déroger à un seuil | Parcours, échéance, valeur, motif codé | Parcours. Nature `DÉROGATION` ou `ALIGNEMENT` distinguée (RG-215) |
| Déclarer prêt pour l'épreuve | Parcours, échéance, motif | Marqueur posé (RG-101) |
| Saisir une épreuve déjà obtenue | Parcours, type, date, justificatif optionnel | Marqueur. Alimente l'expiration comme un plateau du centre (RG-108) |
| Poser, déplacer, retirer une échéance | Parcours, échéance, date | Parcours. Refus si circulation avant plateau (RG-24) |
| Clôturer, rouvrir | Parcours, motif | Parcours et **liste d'impact** sur affectations et séances futures (RG-131) |
| Lister les écarts | Filtres, curseur | Parcours dont un projeté est sous son seuil, triés par proximité d'échéance (US-32) |

### F-09 — Planning, édition et traitement d'impact

| Opération | Entrée | Sortie |
|---|---|---|
| Lire le planning | Période, axe, filtres | Séances · sessions d'examen avec immobilisation **trajet compris** (RG-140) · conflits existants · indicateurs de séance (RG-55) · nombre de parcours actifs (RG-60) |
| Ouvrir une édition | Périmètre | Session d'édition, ou refus nommant l'éditeur et l'heure d'ouverture (RG-144) |
| Pousser le brouillon | Opérations en attente | **Conflits recalculés sur l'état résultant** (RG-146), chacun explicité (RG-217), et liste des modifications en attente (RG-220) |
| Retirer une opération du brouillon | Opération | Brouillon et conflits recalculés |
| Enregistrer | Jeton de version | Compte rendu du lot, ou **refus décrivant les modifications extérieures** et les conflits qu'elles créent (RG-259) |
| Abandonner | — | Verrou levé, rien appliqué (RG-147) |
| Libérer un verrou de force | Session d'édition | Verrou levé, trace (RG-145). Direction seule |
| Lister les traitements d'impact | Statut | Traitements avec leur déclencheur, leur avancement et la **date de leur première ligne non réglée** (RG-227) |
| Lire un traitement | Traitement, groupement, filtre | Lignes recalculées (RG-165), groupées par jour ou par remplaçant (RG-229), motif des lignes sans objet (RG-225) |
| Prévisualiser une action groupée | Action, lignes visées | Ventilation en trois issues : applicable, avec conflit, sans solution (RG-222) |
| Appliquer | Action, sous-ensemble, forçages assumés | Compte rendu (C-27) |
| Décider d'une ligne | Ligne, sort, motif | Ligne. « Laissée en l'état » porte son auteur (RG-166, RG-224) |
| Clore un traitement | Traitement, motif si lignes restantes | Traitement `CLOS`, conservé (Q-129) |
| Déclarer une absence annoncée | Parcours, période, motif | Traitement d'impact ouvert (RG-167) |

### F-03 — Placement · F-10 — Proposition d'élèves

| Opération | Entrée | Sortie |
|---|---|---|
| Proposer des élèves | Séance | Liste classée avec, pour chacun, **l'écart en heures et les jours restants** (RG-153, RG-160). Exclusions limitées à RG-154 |
| Proposer des créneaux | Parcours, horizon | Créneaux où l'élève, un formateur et un véhicule sont libres. Même composant, sens inverse (RG-161, RG-78) |
| Placer, retirer | Séance, élèves | Placements et alertes jointes. Dépassement du maximum accepté avec alerte (RG-156) |
| Placer en masse | Élèves × séances | Compte rendu (RG-158) |
| Inscrire à une session de formation | Session, élève | Placement sur toutes les séances, chacune retirable seule (RG-155) |
| Retirer d'une séance émargée | Placement, confirmation explicite | Placement et présence annulés, les deux tracés (RG-157, RG-132) |

### F-12 — Présences

| Opération | Entrée | Sortie |
|---|---|---|
| Lire la séance courante du formateur | — | Séance en cours ou la plus proche, élèves prégarnis `PRÉSENT` (RG-236, RG-123) |
| Saisir les présences | Séance, valeurs, motifs | Présences. **Idempotent**, pour absorber une reprise après coupure (RG-126) |
| Corriger une présence | Présence, nouvelle valeur | Correction tracée avec la valeur précédente. Formateur sur ses séances, secrétariat partout (RG-20) |
| Lister les séances non renseignées | Profil | Séances triées par ancienneté (RG-125, RG-242) |
| Produire une feuille de présence | Élève ou séance, période | Faits seuls : élèves, dates, durées, statuts. **Ni cumul, ni seuil, ni dérogation** (RG-127, RG-52) |

### F-06 — Sessions d'examen · F-07.1 — Affectations · F-07.2 — Liste classée

| Opération | Entrée | Sortie |
|---|---|---|
| Lister, lire une session | Filtres | Enveloppe et cinq compteurs dérivés · ressources assignées avec trajet · affectations · alertes |
| Créer, modifier une session | Date, plage, lieu, enveloppe, ressources | Session. L'enveloppe est la seule limite saisie (RG-36) |
| Annuler une session | Session, motif | Ressources libérées, affectations signalées, traitement d'impact ouvert (RG-63) |
| Proposer des candidats | Session | Liste classée, vivier **projeté** (RG-89), prérequis manquants mentionnés sans exclusion (RG-90 révisée), incompatibilités signalées (RG-95), explication du rang (RG-97) |
| Lire les écartés | Session | Candidats hors vivier avec leur motif (US-24) |
| Prévisualiser une affectation | Session, candidat, type | Unités engagées, unités restantes après (RG-248) |
| Affecter, retirer | Session, candidat, type | Affectation. Un retrait avant la date libère les unités (RG-104) |
| Saisir, corriger un résultat | Affectation, valeur | Affectation. Un plateau réussi porte son expiration calculée (RG-25). Correction tracée (RG-106) |
| Lire le récapitulatif J-N | Session | Candidats engagés, prérequis manquants nommés, unités qui seront perdues (RG-83, RG-247) |

### F-36 — Liste à faire

| Opération | Entrée | Sortie |
|---|---|---|
| Lire la liste | Groupement, profil déduit | Lignes filtrées par profil (RG-233), groupées temporellement par défaut (RG-235), chacune avec sa condition, sa date, sa conséquence en clair (RG-250), son action principale et ses actions secondaires (RG-249) |
| Repousser une ligne | Ligne, durée parmi trois choix fixes | Ligne masquée jusqu'à échéance. **Refusé sur une ligne à échéance dure** (RG-251) |

**Point d'architecture** : F-36 n'a pas de source propre. Elle agrège ce que chaque feature déclare selon D-08. Ajouter une nature de tâche ne modifie pas cette feature.

---

## 4. Ce que l'API n'expose pas

| Absence | Raison |
|---|---|
| Écriture d'un compteur | C-28, RG-43 |
| Endpoint « alertes » générique | C-06, C-29 |
| CRUD sur les séances | C-25 : le brouillon est le seul chemin d'écriture du planning |
| Suppression d'une personne, d'une ressource, d'un compte | Archivage uniquement (RG-73, RG-184, RG-211) |
| Consultation par un élève ou un payeur | Aucun accès externe. F-23 supprimée |
| Modification d'une session d'examen depuis le planning | RG-152 |

---

## 5. Suite

Les 17 features du lot 1 passent `SPÉCIFIÉE` → **`CONÇUE`** une fois ce document validé : modèle de données, écrans et contrats sont arrêtés.

Reste à cadrer avant la réalisation : le découpage en tranches livrables, dont la première tranche démontrable.
