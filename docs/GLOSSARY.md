# GLOSSARY.md — Domaine métier · français → anglais

Les spécifications (`FEATURES.md`, `QUESTIONS.md`) sont en français, le code est en anglais.
Ce fichier est la table de correspondance. **Aucun terme métier ne doit apparaître en anglais dans le code sans figurer ici.**

Règle : un terme français = un terme anglais, sans synonyme. Si deux notions françaises se ressemblent, elles reçoivent deux mots anglais nettement distincts.

---

## Organisation

| Français | Anglais | Note |
|---|---|---|
| centre | `School` | Racine de la portée organisationnelle (RG-209) |
| direction | `Management` | Profil |
| secrétariat | `Office` | Profil |
| formateur | `Instructor` | Profil **et** ressource |
| élève, candidat, stagiaire | `Student` | Un seul mot pour les trois |
| entreprise, organisme, payeur | `Payer` | |
| horaires d'ouverture | `OpeningHours` | |
| fermeture du centre | `Closure` | Jours fériés inclus |

## Formation

| Français | Anglais | Note |
|---|---|---|
| objectif | `Objective` | Ce que l'élève achète |
| parcours | `Enrollment` | Élève + objectif + seuils + échéances + payeur |
| statut de vie | `LifeStatus` | `ACTIVE` / `CLOSED` |
| état de préparation | `Readiness` | Dérivé, jamais stocké |
| seuil | `Threshold` | `PlateauThreshold`, `TotalThreshold` |
| dérogation | `Waiver` | |
| alignement sur le paramétrage | `Realignment` | Distinct d'un `Waiver` (RG-215) |
| déclaré prêt pour l'épreuve | `DeclaredReady` | |
| épreuve déjà obtenue | `PriorPass` | Obtenue hors du centre |
| échéance | `TargetDate` | Jamais `Deadline` |
| heures de référence | `ReferenceHours` | |
| heures effectuées | `AttendedHours` | Présences seules |
| heures absentes | `MissedHours` | |
| heures consommées | `ConsumedHours` | Effectuées + absentes (RG-172) |
| heures projetées | `ProjectedHours` | |
| heures rachetées | `RepurchasedHours` | Après échec |

## Planning

| Français | Anglais | Note |
|---|---|---|
| séance | `Lesson` | **L'unité réelle** du planning (RG-155) |
| session de formation | `Course` | Simple regroupement de `Lesson` |
| créneau | `TimeSlot` | Intervalle semi-ouvert |
| placement (élève sur séance) | `LessonAssignment` | Sans état |
| session d'édition | `EditSession` | Porte le verrou |
| brouillon | `Draft` | Journal d'intentions (C-09) |
| verrou | `Lock` | |
| opération en attente | `PendingOperation` | |
| lot | `Batch` | `BatchID` = RG-70 |
| conflit | `Conflict` | |
| plage de chevauchement | `Overlap` | RG-217 |
| forçage | `Override` | |

## Ressources

| Français | Anglais | Note |
|---|---|---|
| ressource | `Resource` | Véhicule, salle, plateau **ou** formateur (C-08) |
| véhicule | `Vehicle` | |
| salle | `Room` | |
| plateau (lieu) | `TrainingPad` | Le lieu, pas l'épreuve |
| indisponibilité | `Unavailability` | |
| remise en service | `ReturnToService` | |
| catégorie de véhicule | `LicenceCategory` | C, CE, D… |

## Examens

| Français | Anglais | Note |
|---|---|---|
| session d'examen | `ExamSession` | |
| plateau (épreuve) | `OffRoadTest` | Hors circulation. **Jamais `Plateau`** |
| circulation (épreuve) | `OnRoadTest` | |
| crédit, unité | `Credit` | 1 pour `OffRoadTest`, 2 pour `OnRoadTest` |
| enveloppe | `CreditAllowance` | Seule limite saisie (RG-36) |
| affectation à un examen | `ExamBooking` | |
| engagé / consommé / perdu | `Committed` / `Spent` / `Forfeited` | RG-39 |
| place d'examen (demande) | `SeatRequest` | Le fichier envoyé (F-33) |
| expiration du plateau | `OffRoadExpiry` | 1 an, paramétrable |

## Dossier et financement

| Français | Anglais | Note |
|---|---|---|
| dossier candidat | `Person` | Survit à ses `Enrollment` |
| prérequis | `Requirement` | |
| jeu d'entrée / d'examen | `EntryRequirement` / `ExamRequirement` | RG-02 |
| pièce, justificatif | `Document` | |
| avis médical | `MedicalClearance` | Donnée de santé, régime propre |
| dossier de financement | `Funding` | |
| à monter / déposé / accordé / soldé / refusé | `DRAFT` / `SUBMITTED` / `APPROVED` / `SETTLED` / `REJECTED` | |
| autofinancement | `SelfFunded` | Cycle distinct (RG-15) |

## Présences et aléas

| Français | Anglais | Note |
|---|---|---|
| présence | `Attendance` | |
| présent / absent justifié / absent non justifié | `PRESENT` / `EXCUSED` / `UNEXCUSED` | |
| non renseignée | `UNRECORDED` | |
| feuille de présence | `AttendanceSheet` | |
| aléa | `Disruption` | Panne, absence, annulation |
| traitement d'impact | `DisruptionCase` | File de travail (RG-162) |
| ligne d'impact | `DisruptionItem` | |
| sans objet | `MOOT` | Ligne sortie d'elle-même (RG-165) |
| laissée en l'état | `LEFT_AS_IS` | Décision tracée (RG-166) |

## Transverse

| Français | Anglais | Note |
|---|---|---|
| alerte | `Alert` | Dérivée, jamais stockée (C-06) |
| sévérité | `Severity` | `CRITICAL` / `WARNING` / `INFO` |
| liste à faire | `TaskList` | F-36 |
| paramètre copié / lu en direct | `Copied` / `Live` | C-07 |
| événement métier | `DomainEvent` | Socle de D-05 |
| liste d'impact | `ImpactList` | D-06 |
| compte rendu | `BatchReport` | RG-71 |

---

## Pièges

**`Course` et `ExamSession` ne se confondent pas.** Le français dit « session » pour les deux ; l'anglais les sépare, parce que ce sont deux objets sans rapport.

**`OffRoadTest` et `TrainingPad`.** « Plateau » désigne en français l'épreuve et le lieu. Deux mots distincts en anglais, sans exception.

**`Enrollment` n'est pas `LessonAssignment`.** Le premier est le parcours complet d'un élève, le second son placement sur une séance.

**`TargetDate` n'est pas `Deadline`.** Une échéance d'examen se déplace ; un délai réglementaire non.

**`ConsumedHours` pilote les seuils, `AttendedHours` les documents.** Les inverser produit des attestations fausses (RG-128, RG-172).
