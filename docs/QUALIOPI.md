# Analyse Qualiopi — ce qui manque, ce qui existe

Rédigé en août 2026. Sources : guide de lecture du Référentiel national qualité
(ministère du Travail, V9 en vigueur depuis 2024 — une V10 circulerait,
[À VÉRIFIER] sur travail-emploi.gouv.fr avant l'audit) et synthèses des
7 critères / 32 indicateurs. Statut de chaque ligne : **couvert** (le logiciel
produit déjà la preuve) · **à développer** (feature logicielle manquante) ·
**organisationnel** (preuve hors logiciel, à produire par le centre).

---

## Pourquoi c'est décisif pour ce centre

- Qualiopi est obligatoire depuis 2022 pour tout organisme qui veut des fonds
  publics ou mutualisés : **CPF, OPCO, France Travail**.
- Les permis du **groupe lourd (C1, C1E, C, CE, D1, D1E, D, DE) restent
  mobilisables au CPF sans le plafond de 900 € ni la restriction aux demandeurs
  d'emploi** qui frappent le groupe léger depuis février 2026. Autrement dit :
  le marché du centre est précisément celui où le CPF reste ouvert — sans
  Qualiopi, pas d'accès à ce financement.
- Préalable administratif : être déclaré organisme de formation (**NDA**) ;
  Qualiopi certifie ensuite les **processus**, pas le logiciel. Le logiciel ne
  « rend pas Qualiopi » — il **produit les preuves** que l'auditeur demande.

Périmètre retenu : catégorie **Actions de formation (AF)**. Tronc commun de
22 indicateurs + indicateurs « certifiant » (3, 7, 16) si les prestations
préparent une certification enregistrée — c'est le cas des permis
[À VÉRIFIER : statut exact des permis de conduire au RS/RNCP et des titres pro
transport éventuels]. Indicateurs CFA (13, 14, 15, 20, 29) et AFEST (28)
supposés **non applicables** (pas d'apprentissage) — à confirmer.

---

## Critère 1 — Information du public

| Ind. | Exigence | Statut | Détail |
|---|---|---|---|
| 1 | Information accessible, complète (prestations, durées, tarifs, délais, accessibilité) | **organisationnel + à développer** | Le site public est hors périmètre (D-09). Le logiciel peut générer la **fiche programme** par objectif (contenu, durée, prérequis, tarifs) à publier — les données existent dans Paramétrage |
| 2 | Indicateurs de résultats adaptés | **à développer — quasi gratuit** | Les résultats d'examen (F-07) sont en base : **taux de réussite par permis et par période = une requête** (C-05). Manque l'écran/export « Nos résultats » |
| 3 | (Certifiant) Taux d'obtention, débouchés, passerelles | **à développer** | Taux d'obtention : idem ind. 2. **Débouchés** : rien — demande une enquête post-formation (voir ind. 30, même mécanique) |

## Critère 2 — Objectifs et adaptation

| Ind. | Exigence | Statut | Détail |
|---|---|---|---|
| 4 | Analyse du besoin du bénéficiaire | **à développer** | Aucune trace d'entretien/analyse initiale. Feature : **évaluation de départ** sur le parcours (besoin, expérience, financement, échéance visée) — le champ échéance existe déjà |
| 5 | Objectifs opérationnels et évaluables | **partiellement couvert** | Les objectifs (C, CE, D) portent heures plateau/total (F-01). Manque la déclinaison en **objectifs pédagogiques** formalisés par programme |
| 6 | Contenus et modalités adaptés | **partiellement couvert** | Types de séance, durées, prérequis paramétrés. Manque le **programme/déroulé pédagogique** rattaché à l'objectif (support de la fiche ind. 1) |
| 7 | (Certifiant) Adéquation aux exigences de la certification | **organisationnel** | Conformité au REMC / référentiels d'examen [À VÉRIFIER] — c'est le contenu pédagogique du centre, pas le logiciel |
| 8 | Positionnement à l'entrée + évaluation des acquis | **à développer** | Manque l'**évaluation initiale** (heures prévisionnelles justifiées — la valeur copiée C-07 en serait la sortie) et les **évaluations intermédiaires** formalisées. L'émargement (F-12) trace l'assiduité, pas les acquis |

## Critère 3 — Accueil et suivi

| Ind. | Exigence | Statut | Détail |
|---|---|---|---|
| 9 | Conditions de déroulement communiquées | **largement couvert** | Convocations d'examen ✓, message automatique de placement ✓, modèles éditables ✓. Manque : **livret d'accueil** et règlement intérieur (documents à générer/joindre) |
| 10 | Adaptation de la prestation au bénéficiaire | **couvert** | Parcours individuel (échéances, seuils copiés), planning nominatif, placement suggéré par retard — et tout mouvement est tracé (D-05) |
| 11 | Atteinte des objectifs évaluée | **couvert en grande partie** | Compteurs dérivés, jauges projeté/seuil, alertes d'écart (F-13), résultats d'examen tracés avec correction (F-07). Compléter avec les évaluations d'acquis (ind. 8) |
| 12 | Prévention des ruptures / abandons | **couvert en grande partie** | Alertes d'écart, absences motivées, liste « À traiter », modèles de relance (retard d'heures, absence). Manque une **trace du traitement** des décrochages (le futur DisruptionCase / F-36 complet) |
| 16 | (Certifiant) Inscription aux épreuves conforme | **couvert** | Demandes de places au format officiel avec échéance bloquante (F-33/A-39), engagements avec unités figées, convocations trajet déduit |

## Critère 4 — Moyens

| Ind. | Exigence | Statut | Détail |
|---|---|---|---|
| 17 | Moyens humains et techniques adaptés | **couvert** | Ressources (véhicules avec catégories, salles, formateurs), indisponibilités motivées, détection de conflits — la preuve de l'adéquation moyens/séances est le planning lui-même |
| 18 | Coordination des intervenants | **couvert** | Planning par formateur, émargement moniteur, événements |
| 19 | Ressources pédagogiques à disposition | **à développer** | Rien côté élève. Le **socle documents + portail à jeton (F-35)** est la brique naturelle : y déposer supports et fiches, trace de mise à disposition |

## Critère 5 — Personnel

| Ind. | Exigence | Statut | Détail |
|---|---|---|---|
| 21 | Compétences des intervenants gérées | **à développer — prioritaire** | Manque le **dossier formateur** : titre pro ECSR / BEPECASER mention groupe lourd, **autorisation d'enseigner avec date de validité** [À VÉRIFIER : intitulés exacts], visite médicale. Mécanique déjà écrite : documents datés + expiration dérivée + alerte D-04, comme les prérequis élèves |
| 22 | Développement des compétences des salariés | **organisationnel + léger** | Plan de formation du personnel : preuve RH. Le dossier formateur (ind. 21) peut tracer les formations suivies |

## Critère 6 — Environnement professionnel

| Ind. | Exigence | Statut | Détail |
|---|---|---|---|
| 23 | Veille légale et réglementaire | **organisationnel** | Registre de veille (abonnements, comptes rendus). Hors logiciel — au mieux un journal léger |
| 24 | Veille emplois/métiers | **organisationnel** | Idem (liens OPCO Mobilités, branches transport) |
| 25 | Veille pédagogique et technologique | **organisationnel** | Idem |
| 26 | Accueil des publics en situation de handicap | **organisationnel + léger** | Référent handicap, réseau d'orientation : organisationnel. Côté logiciel : champ **aménagements** sur le dossier candidat (repris sur convocations et séances) |
| 27 | (Sous-traitance) Conformité des sous-traitants | **selon le centre** | Si formateurs indépendants : contrats + contrôle Qualiopi/NDA des sous-traitants. À trancher (question 2) |

## Critère 7 — Amélioration continue

| Ind. | Exigence | Statut | Détail |
|---|---|---|---|
| 30 | Recueil des appréciations (bénéficiaires, financeurs, équipes) | **à développer — LE gros manque** | Rien aujourd'hui. Feature : **enquêtes de satisfaction** à chaud/à froid par **lien à jeton** (mécanique F-35 réutilisée telle quelle), envoyées via les communications, stats dérivées au tableau de bord |
| 31 | Traitement des réclamations | **à développer — léger** | **Registre des réclamations** : ligne datée, objet, traitement, délai — même moule que les corrections tracées existantes |
| 32 | Mesures d'amélioration mises en œuvre | **partiellement couvert** | La liste « À traiter » incarne déjà la boucle. Manque la trace formelle « constat → action → effet » reliant enquêtes (30) et réclamations (31) |

---

## Synthèse — le backlog Qualiopi côté logiciel

Par ordre de valeur (preuves les plus demandées d'abord) :

1. **Enquêtes de satisfaction** (ind. 30, alimente 3 et 32) — liens à jeton
   façon e-photo, à chaud après examen / à froid à N mois, relances via comms,
   restitution au tableau de bord. *La* preuve que tout auditeur ouvre.
2. **Indicateurs de résultats** (ind. 2, 3) — taux de réussite par permis et
   période, dérivés des résultats F-07 déjà en base. Écran + export publiable.
3. **Dossier formateur** (ind. 21, 17, 22) — qualifications et autorisations
   datées, expiration dérivée, alerte D-04. Réutilise la mécanique prérequis.
4. **Évaluation de départ / positionnement** (ind. 4, 8) — formalise l'entrée
   en formation et justifie le volume d'heures du parcours.
5. **Registre des réclamations** (ind. 31) — petit, isolé, vite fait.
6. **Documents cadre générés** (ind. 1, 5, 6, 9) — fiche programme par
   objectif et livret d'accueil, depuis les données de Paramétrage.
7. **Champ aménagements handicap** (ind. 26) — léger, sur le dossier.

Ce qui existe déjà et pèse lourd à l'audit : émargements horodatés avec motifs
(assiduité — aussi exigée par les financeurs), compteurs et alertes d'écart,
convocations et messages tracés, demandes de places au format officiel,
résultats d'examen avec corrections tracées, événements sur toutes les
écritures (D-05). La colonne vertébrale « tout est dérivé, tout est tracé »
est exactement ce qu'un auditeur appelle une preuve.

À produire hors logiciel quoi qu'il arrive : NDA et BPF à jour, procédures
écrites (veille, handicap, réclamations), CV et titres des formateurs,
règlement intérieur, CGV, registre accessibilité.

---

## Questions ouvertes (les plus bloquantes d'abord)

1. **Quel calendrier et quel périmètre de certification ?** Le centre a-t-il
   déjà son NDA, et vise-t-il l'audit initial sur « actions de formation »
   seules ? (Sans NDA, rien ne démarre.)
2. **Y a-t-il apprentissage (CFA) ou formateurs sous-traitants ?** Ça active
   ou éteint les indicateurs 13-15, 20, 27, 29 — et change la taille du
   chantier.
3. **Les prestations préparent-elles aussi des titres professionnels**
   (conducteur routier marchandises/voyageurs) en plus des permis ? Ça fixe
   l'ampleur des indicateurs « certifiant » (3, 7, 16) et des taux à publier.
