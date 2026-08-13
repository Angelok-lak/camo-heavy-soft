# MODELE_DONNEES.md — Phase 2, lot 1

Projet : logiciel de gestion pour centre de formation aux permis du groupe lourd.
Périmètre : les 16 features `SPÉCIFIÉE` du lot 1.
Statut des features couvertes : `SPÉCIFIÉE` → **`CONÇUE`** une fois ce document validé.

---

## Conventions

Nouveau préfixe introduit ici : **`C-xx`** — choix de conception. Stable, jamais réattribué.
Distinct de `D-xx`, qui reste réservé aux décisions produit.

Les entités sont décrites en attributs métier. Types, index et contraintes physiques relèvent de la réalisation.

---

## 1. Socle technique

### C-01 — Base relationnelle, PostgreSQL

Le domaine est transactionnel et fortement relationnel : un placement d'élève touche simultanément une séance, un parcours, des ressources et un compteur de crédits, avec des invariants à respecter. Une base documentaire imposerait de gérer ces invariants dans le code applicatif, et de dupliquer les données pour éviter les jointures.

Trois besoins tranchent définitivement :
- **Détection de conflits** (RG-138) : intersections de plages horaires sur plusieurs ressources. C'est une requête ensembliste.
- **Compteurs dérivés** (RG-122) : agrégations à la lecture, jamais stockées.
- **Atomicité** : l'enregistrement d'une session d'édition (RG-147) applique N modifications ou aucune.

PostgreSQL apporte en plus les types `daterange` et `tstzrange` avec index GiST, qui traitent les chevauchements nativement. C'est exactement le problème du planning.

### C-02 — Identifiants opaques

Clés primaires en UUID v7. Ordonnées dans le temps, donc peu coûteuses en index, et sans révéler de volumétrie dans les URL.

### C-03 — Portée organisationnelle systématique

Toute table métier porte `centre_id` (RG-209, D-01). Le filtrage n'est pas laissé à l'appelant : il est appliqué au niveau de l'accès aux données, une seule fois. Aucune requête ne peut l'oublier.

### C-04 — Horodatage en UTC

Stockage en `timestamptz`, restitution en Europe/Paris. Les séances traversent les changements d'heure ; une durée de 3 h reste 3 h.

---

## 2. Trois principes structurants

### C-05 — Aucun compteur n'est stocké

RG-122 le pose comme règle métier ; ce document le confirme comme choix technique.

**Volumétrie réelle** : 300 parcours actifs, ~30 séances chacun, soit ~9 000 présences et ~10 000 placements sur une année. Une agrégation sur cet ordre de grandeur est instantanée avec les bons index. La liste « qui est en retard » (US-32) se calcule en une requête sur les 300 parcours.

Ce qui coûterait cher, ce serait de stocker : chaque correction de présence, chaque annulation de séance, chaque déplacement d'échéance devrait déclencher un recalcul, et la moindre voie oubliée ferait diverger le compteur.

**Réserve honnête** : si le volume était dix fois supérieur, une vue matérialisée rafraîchie à l'écriture deviendrait justifiable. Le seuil de bascule s'observe, il ne se devine pas — on livre sans, on mesure.

### C-06 — Les alertes sont des requêtes, pas des lignes

Toutes les alertes du lot 1 sont **dérivables** de l'état : un seuil non atteint, un plateau qui expire, une séance passée non renseignée, un dépassement d'enveloppe. Les stocker imposerait de les créer, les mettre à jour, les éteindre et les purger — quatre occasions de désynchronisation, pour aucun gain.

Ce qui est stocké, ce sont les **faits** dont les alertes découlent, et les **décisions humaines** prises à leur sujet : un forçage, une dérogation, une ligne d'impact laissée en l'état.

**Deux conséquences** :
- La sévérité et le public ne sont pas des colonnes mais des paramètres appliqués au calcul (RG-82, RG-113, RG-114). Changer une sévérité prend effet immédiatement (F-01, cas 4) sans toucher une seule ligne.
- Un registre d'alertes unique expose un catalogue de types, chacun avec sa requête, sa sévérité paramétrée et son public. Ajouter une alerte, c'est ajouter une entrée au catalogue.

**Exception assumée** : le `traitement_impact` est une entité stockée. Ce n'est pas une alerte mais une file de travail avec des décisions accumulées, qu'on peut quitter et reprendre.

### C-07 — Deux natures de paramètre, matérialisées différemment

RG-202 distingue les paramètres **copiés à l'instanciation** des paramètres **lus en direct**. Le modèle rend la distinction impossible à confondre :

| Nature | Matérialisation |
|---|---|
| Copié | L'objet porte **sa propre colonne**, initialisée depuis le paramètre. Aucune clé étrangère vers le paramètre |
| Direct | L'objet porte une **clé étrangère** vers la table de référence, ou rien du tout — le paramètre est lu au calcul |

Un seuil horaire est une colonne de `parcours`. Une sévérité d'alerte est lue dans la configuration au moment du calcul. On ne peut pas se tromper : la présence ou l'absence de clé étrangère dit la nature.

Corollaire : **aucun `enum` applicatif** sur les listes métier. Motifs, catégories, types de séance, financeurs sont des tables de référence avec un indicateur d'activation (RG-205). D-01 l'exige.

---

## 3. Référentiel et paramétrage — F-01, F-17

### `centre`
Racine de la portée organisationnelle. Libellé, coordonnées, fuseau. Horaires d'ouverture par jour.

### `utilisateur`
Identité de connexion, `statut` (`ACTIF` / `SUSPENDU` / `ARCHIVÉ`, RG-211), secret d'authentification, dates. Jamais supprimé.

### `utilisateur_profil`
Association vers `direction` / `secretariat` / `formateur`. Plusieurs profils par compte, droits en union (RG-208). Historisée : dates de début et de fin, pour que RG-213 reste vérifiable.

### `objectif`
Le produit vendu. Intitulé, `heures_avant_plateau`, `heures_total`, `etg_requis`, tarif indicatif, actif.
Valeurs initiales : C 49/70 · D 49/70 · CE 70/90 (RG-37).

### `prerequis_modele`
Rattaché à un objectif. Libellé, `jeu` (`ENTREE` / `EXAMEN`), obligatoire, `validable_par_formateur` (RG-21, A-14), durée de validité éventuelle, justificatif attendu.

### Tables de référence
`categorie_vehicule` · `type_seance` (avec `requiert_vehicule`, RG-149) · `duree_seance_standard` · `lieu_examen` (avec `temps_trajet_minutes`, RG-140) · `type_financeur` · `motif` (typé par domaine : clôture, dérogation, absence, annulation, perte de crédit, indisponibilité, refus) · `fermeture_centre` (jours fériés et fermetures, RG-178).

Chacune porte un indicateur d'activation. Une valeur référencée n'est jamais supprimée (RG-205).

### `parametre_centre`
Clé-valeur typée pour les paramètres scalaires lus en direct : barème des crédits, durée de validité du plateau, délais d'alerte, rythme hypothétique, délai de verrou, poids des moteurs, seuils de bascule de sévérité.

### `journal_parametre`
Toute modification de paramètre : valeur précédente, nouvelle valeur, auteur, date (RG-203).

---

## 4. Ressources — F-15, F-16

### C-08 — Une seule notion de ressource, deux tables spécialisées

Le composant de disponibilité (RG-78, D-03) doit interroger indifféremment un véhicule, une salle, un plateau ou un formateur. Trois options ont été pesées :

| Option | Pour | Contre |
|---|---|---|
| Une table `ressource` fourre-tout | Requête de disponibilité triviale | Un formateur porte un compte utilisateur et un cumul d'heures, un véhicule porte des catégories. Colonnes nulles en masse |
| Tables totalement séparées | Chaque entité est propre | La disponibilité devient une union de requêtes, à maintenir à chaque ajout de type |
| **Retenue** — `ressource` porte l'identité et le type, deux tables satellites portent le spécifique | Disponibilité et indisponibilité sur une seule table. Spécifique isolé | Une jointure de plus à la lecture du détail |

### `ressource`
`centre_id`, `type` (`VEHICULE` / `SALLE` / `PLATEAU` / `FORMATEUR`), libellé, `statut` (`ACTIVE` / `ARCHIVÉE`, RG-73).

### `ressource_vehicule`
Extension : catégories couvertes (RG-74), capacité indicative en élèves (RG-135).

### `ressource_formateur`
Extension : `utilisateur_id` optionnel (RG-76, RG-210), horaires individuels informatifs (RG-75, RG-139).

### `indisponibilite`
`ressource_id`, `motif_id`, `debut`, `fin` nullable (RG-65), `statut` (`EN COURS` / `TERMINÉE`), auteur et date de remise en service (RG-66). Les chevauchements sont autorisés (RG-79).

**Une seule table pour tous les types de ressource** : c'est le gain de C-08.

---

## 5. Personnes et parcours — F-02, F-04, F-05, F-29

### `personne`
État civil complet : nom, prénoms, date, ville, département et pays de naissance (RG-181). Coordonnées. `neph` unique (RG-182). `statut` (`ACTIF` / `ARCHIVÉ`, RG-184).

**La personne survit à ses parcours.** C'est ce qui permet à un ancien élève de revenir sans double saisie.

### `payeur`
Élève lui-même, entreprise ou organisme. Contact référent (RG-187). Préfigure F-31 sans la précéder.

### `parcours`
Le pivot du produit.

| Attribut | Nature | Règle |
|---|---|---|
| `personne_id`, `objectif_id`, `payeur_id` | — | RG-115 : un seul `ACTIF` par personne |
| `heures_avant_plateau`, `heures_total` | **Copiés** de l'objectif | RG-01, RG-42, C-07 |
| `echeance_plateau`, `echeance_circulation` | Dates cibles, modifiables indépendamment | RG-24, RG-118 |
| `statut_vie` | `ACTIF` / `CLÔTURÉ` + motif, auteur, date | RG-59, RG-116 |
| `heures_rachetees` | Nombre libre cumulé, sans effet sur les seuils | RG-61 |

**L'état de préparation n'est pas une colonne.** Il est dérivé de RG-04, RG-26 et des marqueurs (C-05). Le stocker le ferait diverger à la première correction de présence.

### `parcours_derogation`
Historise chaque modification de seuil : échéance concernée, valeur avant, valeur après, `nature` (`DÉROGATION` / `ALIGNEMENT`, RG-215), motif codé, auteur, date. La nature `ALIGNEMENT` sort des statistiques de dérogation.

### `parcours_marqueur`
`type` (`DÉCLARÉ PRÊT` RG-101 / `ÉPREUVE DÉJÀ OBTENUE` RG-107), épreuve concernée, date d'obtention pour le second, motif, auteur, justificatif optionnel (RG-107, facultatif).

Une épreuve plateau déjà obtenue alimente le calcul d'expiration comme un plateau du centre (RG-108) : le calcul lit indifféremment cette table et les résultats d'affectation.

### `prerequis_parcours`
Copié depuis `prerequis_modele` à la création (C-07, F-29 cas 3). Statut `NON VALIDÉ` / `VALIDÉ` / `NON APPLICABLE` (RG-193). Valideur, date, commentaire, date de validité éventuelle. L'état `EXPIRÉ` est **dérivé** de la date, jamais stocké.

### `document`
Pièce jointe polymorphe : rattachée à un prérequis, un parcours ou une personne. `type_document`, sensibilité, dépôt (auteur, date). Les pièces sensibles suivent RG-186 et RG-194.

### `dossier_financement`
Un par parcours. `type_financeur_id`, `statut` (RG-15), montant absent (D-02).

### `financement_transition`
Chaque changement d'état : avant, après, auteur, date, commentaire, motif obligatoire sur `REFUSÉ` (RG-189).

---

## 6. Planning — F-09, F-03, F-12

### `session_formation`
Simple regroupement de séances servant de raccourci de saisie (RG-155). Libellé, dates. **Ne porte aucune règle métier.**

### `seance`
| Attribut | Règle |
|---|---|
| `centre_id`, `session_formation_id` nullable | RG-124 |
| `plage` — intervalle horaire | RG-133 |
| `nb_eleves_max` | Saisi librement, indicatif déductible du véhicule (RG-135) |
| `statut` — `PLANIFIÉE` / `ANNULÉE` + motif | RG-150 |

Le cycle réduit à deux états est un acquis de la phase 1 : la réalisation se lit dans les présences, un report est un déplacement.

### `seance_type` et `seance_ressource`
Associations multiples. Une séance porte zéro à N types, informatifs (RG-134), et zéro à N ressources.

### `placement`
Élève sur séance. Sans état : il existe ou non. Auteur et date.

### `presence`
`placement_id`, `valeur` (`NON RENSEIGNÉE` / `PRÉSENT` / `ABSENT JUSTIFIÉ` / `ABSENT NON JUSTIFIÉ`, RG-123), motif, auteur, date.

### `presence_correction`
Valeur précédente, nouvelle valeur, auteur, date (RG-20). Table distincte : une correction est un fait à conserver, pas un écrasement.

### Les trois compteurs, tous dérivés de `presence`

| Compteur | Calcul | Sert à |
|---|---|---|
| **Effectuées** | Somme des durées où `PRÉSENT` | Feuilles de présence, preuve, suivi réel |
| **Absentes** | Somme des durées absentes, ventilées | RG-180 |
| **Consommées** | Effectuées + absentes | Les seuils, l'état de préparation, le rebours (RG-128, RG-172) |

Chaque élève présent compte la **durée totale** de la séance (RG-33) : le compteur ne divise pas par le nombre d'élèves en rotation.

**Projetés** : consommées + durées des séances `PLANIFIÉE` antérieures à l'échéance concernée. Deux valeurs, une par échéance (RG-119).

---

## 7. La session d'édition — le point technique du lot

### C-09 — Le brouillon est un journal d'intentions, pas une copie du planning

RG-144 à RG-147 décrivent une session d'édition : les modifications s'accumulent, les conflits se recalculent sur l'état résultant, on enregistre tout ou on abandonne tout. Trois façons de le construire :

| Option | Fonctionnement | Verdict |
|---|---|---|
| Copie du périmètre | Dupliquer les séances de la semaine dans des tables miroir | Rejetée. Duplication du schéma, réconciliation à l'enregistrement, divergence garantie |
| Drapeau de brouillon | Écrire les séances avec un indicateur « non publié » | Rejetée. Chaque requête du système devrait filtrer ce drapeau. Un oubli, et un brouillon fuite dans le planning de tous |
| **Journal d'opérations** | La session d'édition porte une liste ordonnée d'intentions : créer, déplacer, annuler, assigner, placer | **Retenue** |

**Comment ça marche** : l'état affiché à l'éditeur est l'état enregistré auquel on applique les opérations en attente, en mémoire. Les conflits se calculent sur cet état résultant (RG-146). Enregistrer, c'est rejouer les opérations en une transaction. Abandonner, c'est supprimer la ligne — rien n'a jamais touché le planning.

**Trois bénéfices** qui ne sont pas des effets de bord :
- Aucune autre requête du système n'a à connaître l'existence du brouillon.
- L'identifiant de la session d'édition **est** l'identifiant de lot de RG-70. Rien à inventer.
- Le même mécanisme sert aux actions groupées de D-06 : une action groupée est une session d'édition ouverte, remplie et enregistrée en un geste.

### `session_edition`
`utilisateur_id`, périmètre (bornes de dates, éventuellement une ressource), `statut` (`OUVERTE` / `ENREGISTRÉE` / `ABANDONNÉE`), ouverture, dernière activité (RG-145), libération forcée éventuelle avec auteur.

**Le verrou est cette ligne.** Une contrainte d'unicité sur périmètre + statut `OUVERTE` le garantit en base, pas dans le code applicatif.

### `edition_operation`
`session_edition_id`, ordre, `type_operation`, cible, charge utile. Supprimées à l'abandon, conservées à l'enregistrement — elles constituent la trace du lot.

### `conflit_force`
Un conflit détecté puis passé outre : type (RG-138), objets concernés, auteur, date, session d'édition (A-08). C'est un fait, donc c'est stocké.

---

## 8. Examens et crédits — F-06, F-07.1, F-07.2

### `session_examen`
| Attribut | Règle |
|---|---|
| `date`, `plage`, `lieu_examen_id` | RG-140 : les ressources sont immobilisées durée + trajet aller-retour |
| `enveloppe_credits` | **Seule limite saisie** (RG-36) |
| `statut` | `ANNULÉE` uniquement. `À VENIR` et `PASSÉE` sont dérivés de la date (RG-44) |

### `session_examen_ressource`
Véhicules et formateurs assignés (RG-58).

### `affectation_examen`
`parcours_id`, `session_examen_id`, `type_epreuve` (`PLATEAU` / `CIRCULATION`), `statut` (`ENGAGÉE` / `RÉUSSIE` / `ÉCHOUÉE` / `ABSENTE` / `RETIRÉE` / `NON RENSEIGNÉE`), résultat saisi par, date.
Le passage à `NON RENSEIGNÉE` est **dérivé** de la date tant qu'aucun résultat n'est saisi (RG-112) : rien à faire basculer par traitement différé.

### `affectation_correction`
Valeur précédente d'un résultat corrigé (RG-106).

### Les compteurs de crédits, tous dérivés

| Compteur | Calcul |
|---|---|
| Attribués | `enveloppe_credits` |
| Engagés | Somme du barème sur les affectations `ENGAGÉE` et `NON RENSEIGNÉE` |
| Consommés | Idem sur `RÉUSSIE` et `ÉCHOUÉE` |
| Perdus | `ABSENTE`, plus le reliquat non engagé après la date, avec motif (RG-39, RG-40) |
| Restants | Attribués − engagés − consommés − perdus |

Le barème est un paramètre lu en direct (C-07), **sauf** au moment où un crédit est consommé : la consommation fige le nombre d'unités appliqué, sinon changer le barème réécrirait l'histoire (F-01, cas 6).

### Expiration du plateau
Dérivée de la date d'obtention — issue d'une affectation `RÉUSSIE` ou d'un `parcours_marqueur` — et de la durée paramétrée (RG-25). Jamais stockée : changer le paramètre doit se propager.

---

## 9. Traitement des aléas — F-09

### `traitement_impact`
`declencheur` (RG-162), objet déclencheur, `statut` (`À TRAITER` / `EN COURS` / `CLOS`), clôture éventuelle avec motif.

**Seule entité dérivée du domaine des alertes à être stockée** (C-06) : elle porte des décisions humaines accumulées.

### `ligne_impact`
Cible (séance ou affectation), `sort` (`à traiter` / `remplacée` / `annulée` / `élève retiré` / `laissée en l'état` / `sans objet`), auteur, date.

La liste est **recalculée à chaque ouverture** (RG-165) : seuls les sorts décidés sont conservés. Une ligne devenue sans objet sort d'elle-même.

### `absence_annoncee`
`parcours_id`, période, motif, auteur (RG-167, RG-170). Ouvre un traitement d'impact. Ne préjuge d'aucune présence.

---

## 10. Traçabilité

### C-10 — Deux traçabilités distinctes, à ne pas fusionner

| Trace | Contenu | Destinataire |
|---|---|---|
| **Métier** | Événements signifiants : parcours créé, seuil dérogé, résultat saisi, présence corrigée | Le secrétariat, sur la fiche concernée (RG-185) |
| **Technique** | Toute lecture et écriture, y compris les consultations de pièces sensibles (RG-186, RG-194) | La direction, en audit. Volumineuse, purgée (F-18) |

Les fusionner produirait un historique de fiche illisible, noyé sous les consultations.

### `evenement_metier`
Type, objets concernés, charge utile, auteur, date, `lot_id` optionnel (RG-70).
**C'est aussi le socle de D-05** : les compteurs du tableau de bord (F-20) se calculent depuis cette table sans qu'aucune feature ait à prévoir son propre stockage d'indicateurs.

### `journal_acces`
Écrit systématiquement, hors du modèle métier. Rétention propre.

---

## 11. Ce que le modèle ne fait pas, volontairement

| Absence | Raison |
|---|---|
| Table `alerte` | C-06 : dérivées |
| Colonne « état de préparation » sur `parcours` | C-05 : dérivé de RG-04 |
| Colonne « heures réalisées » | RG-43 : jamais saisissable, donc jamais stockée |
| Table `place_examen` unitaire | RG-29 : le crédit est un nombre porté par l'enveloppe, pas un objet |
| Capacité en candidats sur `session_examen` | Q-51 : une seule limite saisie |
| Table `session_formation` porteuse de règles | RG-155 : simple regroupement |
| Matrice de permissions en base | La matrice de F-17 est stable et se code en dur dans la couche d'autorisation. Ce n'est pas une règle métier du centre, c'est la structure du produit |

Cette dernière ligne mérite d'être assumée : D-01 interdit les règles **métier** en dur, pas la structure du logiciel. Rendre la matrice paramétrable ajouterait un écran d'administration complexe pour trois profils qui ne bougeront pas.

---

## 12. Points à trancher

| Réf | Point | Défaut appliqué |
|---|---|---|
| C-11 | Suppression physique ou logique en base | Aucune suppression physique sur les entités métier. Archivage partout |
| C-12 | Le `payeur` est-il une table ou des colonnes sur `parcours` ? | Table dès maintenant. Colonnes en texte libre coûteraient une migration à l'arrivée de F-31 |
| C-13 | Fuseau et durées | Séances en `timestamptz`, durées en minutes entières. Pas de secondes |
| C-14 | Le brouillon survit-il à une déconnexion ? | Oui : les opérations sont persistées. Le verrou se libère par inactivité (RG-145), les opérations restent consultables |

---

## 13. Suite de la phase 2

1. **Ce document validé** → les 16 features passent `SPÉCIFIÉE` → `CONÇUE` sur le volet données.
2. **Contrats d'API**, par tranche verticale et non par entité — un contrat par feature, pas un CRUD par table.
3. **Écrans**, dans l'ordre de la chaîne de réalisation : F-17, F-01, F-15/F-16, puis F-09 qui concentre l'essentiel de la complexité d'interface.

La chaîne de réalisation reste celle de `FEATURES.md` :

```
F-17 → F-01 → F-15 + F-16 → F-09 → F-03 + F-10 → F-12 → F-13
     → F-02 + F-05 + F-29 → F-04 → F-06 → F-07.1 → F-07.2
```
