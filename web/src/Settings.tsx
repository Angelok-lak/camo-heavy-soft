// F-01 — the settings, per the validated mockup: a grouped sidebar, ONE
// section per screen, each with its title, its subtitle and its own
// rules spelled out. Copied values carry the blue banner; deactivation
// is the only removal (RG-205).

import { useCallback, useEffect, useState, type ReactNode } from 'react'
import {
  api,
  ApiError,
  type AbsenceReason,
  type CategoryRow,
  type DurationRow,
  type ExamPlace,
  type FunderKind,
  type KindRow,
  type Objective,
  type OpeningHourRow,
  type RequirementTemplate,
  type UserContext,
} from './api'

const DAYS = ['', 'Lundi', 'Mardi', 'Mercredi', 'Jeudi', 'Vendredi', 'Samedi', 'Dimanche']

type Section =
  | 'templates'
  | 'objectives'
  | 'requirements'
  | 'funders'
  | 'kinds'
  | 'durations'
  | 'opening'
  | 'categories'
  | 'places'
  | 'credits'
  | 'reasons'

const NAV: { group: string; items: [Section, string][] }[] = [
  {
    group: 'Formations',
    items: [
      ['objectives', 'Objectifs'],
      ['requirements', 'Prérequis'],
      ['funders', 'Financeurs'],
    ],
  },
  {
    group: 'Planning',
    items: [
      ['kinds', 'Types de séance'],
      ['durations', 'Durées standard'],
      ['opening', 'Horaires et fermetures'],
    ],
  },
  {
    group: 'Ressources',
    items: [
      ['categories', 'Catégories véhicule'],
      ['places', "Lieux d'examen"],
    ],
  },
  {
    group: 'Examens',
    items: [['credits', 'Crédits et validité']],
  },
  {
    group: 'Système',
    items: [
      ['reasons', 'Motifs'],
      ['templates', 'Modèles de messages'],
    ],
  },
]

const DESCRIPTIONS: Record<Section, string> = {
  objectives: 'Ce que le centre vend : heures avant plateau et au total',
  requirements: "Jeux d'entrée en formation et d'examen, par objectif",
  funders: "Organismes et autofinancement",
  kinds: 'Conduite, théorie… et le marquage « requiert un véhicule »',
  durations: 'Les durées proposées à la pose de séance',
  opening: "Les plages d'ouverture du centre, jour par jour",
  categories: 'C, CE, D… portées par les véhicules et les parcours',
  places: "Chaque lieu avec son temps de trajet",
  credits: 'Barème des unités et validité du plateau',
  reasons: "Motifs d'absence proposés à l'émargement",
  templates: 'Convocations, placements, courriers du bureau — avec aperçu',
}

export default function Settings({ user }: { user: UserContext }) {
  const canManage = user.permissions.manage_settings
  const [section, setSection] = useState<Section | null>(null)
  const [error, setError] = useState<string | null>(null)

  if (section === null) {
    return (
      <div className="page">
        <div className="page-main">
          <div style={{ maxWidth: 880, margin: '0 auto' }}>
            <header className="panel-head" style={{ marginTop: 8 }}>
              <h2>Paramétrage</h2>
              <p className="muted">
                Toutes les valeurs métier du centre vivent ici — jamais dans le code.
                {!canManage && ' Consultation seule pour votre profil.'}
              </p>
            </header>
            {NAV.map((g) => (
              <div key={g.group}>
                <h3 className="eyebrow">{g.group}</h3>
                <div className="settings-grid-cards">
                  {g.items.map(([s, label]) => (
                    <button key={s} className="settings-card" onClick={() => setSection(s)}>
                      <strong>{label}</strong>
                      <span className="muted">{DESCRIPTIONS[s]}</span>
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="page">
      <div className="page-main settings-panel" style={{ margin: '0 auto' }}>
        <button className="link-action" style={{ marginBottom: 12 }} onClick={() => { setSection(null); setError(null) }}>
          ← Paramétrage
        </button>
        {!canManage && (
          <p className="muted" style={{ marginTop: 0 }}>
            Consultation seule — le paramétrage est réservé à la direction.
          </p>
        )}
        {error && <div className="error-box">{error}</div>}

        {section === 'objectives' && <ObjectivesPanel canManage={canManage} onError={setError} />}
        {section === 'requirements' && <RequirementsPanel canManage={canManage} onError={setError} />}
        {section === 'funders' && <FundersPanel canManage={canManage} onError={setError} />}
        {section === 'kinds' && <KindsPanel canManage={canManage} onError={setError} />}
        {section === 'durations' && <DurationsPanel canManage={canManage} onError={setError} />}
        {section === 'opening' && <OpeningPanel canManage={canManage} onError={setError} />}
        {section === 'categories' && <CategoriesPanel canManage={canManage} onError={setError} />}
        {section === 'places' && <PlacesPanel canManage={canManage} onError={setError} />}
        {section === 'credits' && <CreditsPanel />}
        {section === 'reasons' && <ReasonsPanel canManage={canManage} onError={setError} />}
        {section === 'templates' && <TemplatesPanel canManage={canManage} onError={setError} />}
      </div>
    </div>
  )
}

type PanelProps = { canManage: boolean; onError: (e: string) => void }

function PanelHead({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <header className="panel-head">
      <h2>{title}</h2>
      <p className="muted">{subtitle}</p>
    </header>
  )
}

function InfoBanner({ children }: { children: ReactNode }) {
  return <div className="info-banner">ⓘ {children}</div>
}

// ---------------------------------------------------------------------
// Objectifs — expandable rows, copied-value banner (mockup)
// ---------------------------------------------------------------------

function ObjectivesPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<Objective[]>([])
  const [open, setOpen] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)

  const reload = useCallback(() => {
    api.objectives().then(setRows).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead
        title="Objectifs"
        subtitle={`Ce que le centre vend · ${rows.length} objectif${rows.length > 1 ? 's' : ''} actif${rows.length > 1 ? 's' : ''}`}
      />
      <InfoBanner>
        Ces valeurs sont recopiées sur chaque parcours à sa création. Les modifier ici ne change
        aucun parcours en cours.
      </InfoBanner>

      {rows.map((o) =>
        open === o.id ? (
          <ObjectiveEditor
            key={o.id}
            objective={o}
            canManage={canManage}
            onError={onError}
            onDone={() => {
              setOpen(null)
              reload()
            }}
          />
        ) : (
          <button key={o.id} className="setting-row" onClick={() => setOpen(o.id)}>
            <span>
              <strong>Permis {o.label}</strong>
              <span className="muted" style={{ display: 'block' }}>
                {o.hours_before_offroad} h avant le plateau · {o.total_hours} h au total
              </span>
            </span>
            <span className="chevron">›</span>
          </button>
        ),
      )}

      {canManage && !adding && (
        <div className="form-actions">
          <button onClick={() => setAdding(true)}>Ajouter un objectif</button>
        </div>
      )}
      {adding && (
        <ObjectiveEditor
          canManage
          onError={onError}
          onDone={() => {
            setAdding(false)
            reload()
          }}
        />
      )}
    </section>
  )
}

function ObjectiveEditor({
  objective,
  canManage,
  onError,
  onDone,
}: {
  objective?: Objective
  canManage: boolean
  onError: (e: string) => void
  onDone: () => void
}) {
  const [label, setLabel] = useState(objective?.label ?? '')
  const [before, setBefore] = useState(String(objective?.hours_before_offroad ?? 49))
  const [total, setTotal] = useState(String(objective?.total_hours ?? 70))

  const save = () => {
    const body = {
      label: label.trim(),
      hours_before_offroad: Number(before),
      total_hours: Number(total),
    }
    const call = objective
      ? api.patchObjective(objective.id, body)
      : api.createObjective(body)
    call.then(onDone).catch((e) =>
      onError(e instanceof ApiError ? String(e.body.error ?? e.message) : String(e)),
    )
  }

  return (
    <div className="setting-card">
      <div className="form-row" style={{ maxWidth: 200 }}>
        <label>Intitulé</label>
        <input value={label} onChange={(e) => setLabel(e.target.value)} disabled={!canManage} autoFocus />
      </div>
      <div style={{ display: 'flex', gap: 14 }}>
        <div className="big-field">
          <label>Heures avant le plateau</label>
          <input
            type="number"
            value={before}
            onChange={(e) => setBefore(e.target.value)}
            disabled={!canManage}
          />
        </div>
        <div className="big-field">
          <label>Heures au total</label>
          <input
            type="number"
            value={total}
            onChange={(e) => setTotal(e.target.value)}
            disabled={!canManage}
          />
        </div>
      </div>
      <p className="muted">
        La propagation aux parcours en cours (avec l'aperçu des populations divergentes) viendra
        avec le paramétrage complet — pour l'instant, seuls les futurs parcours reçoivent ces
        valeurs.
      </p>
      <div className="form-actions">
        {canManage && (
          <button
            className="primary"
            disabled={!label.trim() || Number(total) < Number(before)}
            onClick={save}
          >
            Enregistrer
          </button>
        )}
        <button onClick={onDone}>Fermer</button>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------
// Prérequis
// ---------------------------------------------------------------------

function RequirementsPanel({ canManage, onError }: PanelProps) {
  const [objectives, setObjectives] = useState<Objective[]>([])
  const [objectiveID, setObjectiveID] = useState('')
  const [rows, setRows] = useState<RequirementTemplate[]>([])
  const [label, setLabel] = useState('')
  const [set, setSet] = useState('ENTRY')
  const [instructorOK, setInstructorOK] = useState(false)

  useEffect(() => {
    api.objectives().then((o) => {
      setObjectives(o)
      if (o.length > 0) setObjectiveID(o[0].id)
    }).catch(() => {})
  }, [])

  const reload = useCallback(() => {
    if (!objectiveID) return
    api.requirementTemplates(objectiveID).then(setRows).catch((e) => onError(String(e)))
  }, [objectiveID, onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead
        title="Prérequis"
        subtitle="Deux jeux par objectif : entrer en formation, se présenter à l'examen"
      />
      <InfoBanner>
        Copiés au parcours à sa création — modifier ici ne touche que les futurs parcours.
      </InfoBanner>

      <div className="form-row" style={{ maxWidth: 240 }}>
        <label>Objectif</label>
        <select value={objectiveID} onChange={(e) => setObjectiveID(e.target.value)}>
          {objectives.map((o) => (
            <option key={o.id} value={o.id}>Permis {o.label}</option>
          ))}
        </select>
      </div>

      {(['ENTRY', 'EXAM'] as const).map((s) => (
        <div key={s}>
          <h3 className="eyebrow">{s === 'ENTRY' ? 'Pour entrer en formation' : "Pour se présenter à l'examen"}</h3>
          {rows.filter((t) => t.set === s).map((t) => (
            <div key={t.id} className={`detail-row ${t.active ? '' : 'archived'}`}>
              <span className="lead">{t.label}</span>
              <span className="meta">
                {t.instructor_may_validate ? 'validable par un formateur' : 'secrétariat et direction'}
                {t.validity_months ? ` · validité ${t.validity_months} mois` : ''}
              </span>
              {canManage && (
                <button
                  className="btn-sm"
                  onClick={() =>
                    api.patchRequirementTemplate(t.id, { active: !t.active }).then(reload).catch((e) => onError(String(e)))
                  }
                >
                  {t.active ? 'Désactiver' : 'Réactiver'}
                </button>
              )}
            </div>
          ))}
        </div>
      ))}

      {canManage && (
        <div className="inline-form" style={{ marginTop: 14 }}>
          <input placeholder="Nouveau prérequis" value={label} onChange={(e) => setLabel(e.target.value)} />
          <select value={set} onChange={(e) => setSet(e.target.value)}>
            <option value="ENTRY">Entrée</option>
            <option value="EXAM">Examen</option>
          </select>
          <label className="muted">
            <input type="checkbox" checked={instructorOK} onChange={(e) => setInstructorOK(e.target.checked)} />{' '}
            validable formateur
          </label>
          <button
            disabled={!label.trim() || !objectiveID}
            onClick={() =>
              api
                .createRequirementTemplate(objectiveID, {
                  label: label.trim(),
                  set,
                  instructor_may_validate: instructorOK,
                })
                .then(() => {
                  setLabel('')
                  setInstructorOK(false)
                  reload()
                })
                .catch((e) => onError(String(e)))
            }
          >
            Ajouter
          </button>
        </div>
      )}
    </section>
  )
}

// ---------------------------------------------------------------------
// Financeurs
// ---------------------------------------------------------------------

function FundersPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<FunderKind[]>([])
  const [label, setLabel] = useState('')
  const [selfFunded, setSelfFunded] = useState(false)

  const reload = useCallback(() => {
    api.funderKinds().then(setRows).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead
        title="Financeurs"
        subtitle="Le dossier de financement porte le financeur et l'état de prise en charge, rien de plus"
      />
      {rows.map((k) => (
        <div key={k.id} className={`detail-row ${k.active ? '' : 'archived'}`}>
          <span className="lead">{k.label}</span>
          <span className="meta">{k.self_funded ? 'autofinancement — cycle propre' : 'organisme'}</span>
        </div>
      ))}
      {canManage && (
        <div className="inline-form">
          <input placeholder="Nouveau financeur" value={label} onChange={(e) => setLabel(e.target.value)} />
          <label className="muted">
            <input type="checkbox" checked={selfFunded} onChange={(e) => setSelfFunded(e.target.checked)} />{' '}
            autofinancement
          </label>
          <span className="muted">Gestion complète (désactivation…) : à venir</span>
        </div>
      )}
    </section>
  )
}

// ---------------------------------------------------------------------
// Types de séance / Durées / Catégories / Lieux / Motifs — same pattern
// ---------------------------------------------------------------------

function KindsPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<KindRow[]>([])
  const [label, setLabel] = useState('')
  const [requiresVehicle, setRequiresVehicle] = useState(false)
  const reload = useCallback(() => {
    api.lessonKinds().then(setRows).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead title="Types de séance" subtitle="Informatifs sur la séance ; « requiert un véhicule » alerte à l'oubli" />
      {rows.map((k) => (
        <div key={k.id} className={`detail-row ${k.active ? '' : 'archived'}`}>
          <span className="lead">{k.label}</span>
          <span className="meta">{k.requires_vehicle ? 'requiert un véhicule' : ''}</span>
          {canManage && (
            <button
              className="btn-sm"
              onClick={() => api.patchLessonKind(k.id, { active: !k.active }).then(reload).catch((e) => onError(String(e)))}
            >
              {k.active ? 'Désactiver' : 'Réactiver'}
            </button>
          )}
        </div>
      ))}
      {canManage && (
        <div className="inline-form">
          <input placeholder="Nouveau type" value={label} onChange={(e) => setLabel(e.target.value)} />
          <label className="muted">
            <input type="checkbox" checked={requiresVehicle} onChange={(e) => setRequiresVehicle(e.target.checked)} />{' '}
            véhicule requis
          </label>
          <button
            disabled={!label.trim()}
            onClick={() =>
              api.createLessonKind({ label: label.trim(), requires_vehicle: requiresVehicle }).then(() => {
                setLabel('')
                setRequiresVehicle(false)
                reload()
              }).catch((e) => onError(String(e)))
            }
          >
            Ajouter
          </button>
        </div>
      )}
    </section>
  )
}

function DurationsPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<DurationRow[]>([])
  const [minutes, setMinutes] = useState('')
  const reload = useCallback(() => {
    api.durations().then(setRows).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead title="Durées standard" subtitle="Proposées à la pose d'une séance" />
      {rows.map((d) => (
        <div key={d.id} className={`detail-row ${d.active ? '' : 'archived'}`}>
          <span className="lead">{d.minutes} min</span>
          <span className="meta" />
          {canManage && (
            <button
              className="btn-sm"
              onClick={() => api.patchDuration(d.id, { active: !d.active }).then(reload).catch((e) => onError(String(e)))}
            >
              {d.active ? 'Désactiver' : 'Réactiver'}
            </button>
          )}
        </div>
      ))}
      {canManage && (
        <div className="inline-form">
          <input type="number" step={15} min={15} placeholder="Minutes" value={minutes} onChange={(e) => setMinutes(e.target.value)} />
          <button
            disabled={!minutes || Number(minutes) <= 0}
            onClick={() =>
              api.createDuration(Number(minutes)).then(() => {
                setMinutes('')
                reload()
              }).catch((e) => onError(String(e)))
            }
          >
            Ajouter
          </button>
        </div>
      )}
    </section>
  )
}

function CategoriesPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<CategoryRow[]>([])
  const [code, setCode] = useState('')
  const reload = useCallback(() => {
    api.categories().then(setRows).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead title="Catégories véhicule" subtitle="C, CE, D… portées par les véhicules et les parcours" />
      {rows.map((c) => (
        <div key={c.id} className={`detail-row ${c.active ? '' : 'archived'}`}>
          <span className="lead">{c.code}</span>
          <span className="meta" />
          {canManage && (
            <button
              className="btn-sm"
              onClick={() => api.patchCategory(c.id, { active: !c.active }).then(reload).catch((e) => onError(String(e)))}
            >
              {c.active ? 'Désactiver' : 'Réactiver'}
            </button>
          )}
        </div>
      ))}
      {canManage && (
        <div className="inline-form">
          <input placeholder="CE, D1E…" value={code} onChange={(e) => setCode(e.target.value)} />
          <button
            disabled={!code.trim()}
            onClick={() =>
              api.createCategory(code.trim()).then(() => {
                setCode('')
                reload()
              }).catch((e) => onError(String(e)))
            }
          >
            Ajouter
          </button>
        </div>
      )}
    </section>
  )
}

function PlacesPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<ExamPlace[]>([])
  const [label, setLabel] = useState('')
  const [travel, setTravel] = useState('')
  const reload = useCallback(() => {
    api.examPlaces().then(setRows).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead
        title="Lieux d'examen"
        subtitle="Le temps de trajet immobilise les ressources avant et après chaque session"
      />
      {rows.map((p) => (
        <div key={p.id} className={`detail-row ${p.active ? '' : 'archived'}`}>
          <span className="lead">{p.label}</span>
          <span className="meta">{p.travel_minutes > 0 ? `${p.travel_minutes} min de trajet` : 'sur place'}</span>
          {canManage && (
            <button
              className="btn-sm"
              onClick={() => api.patchExamPlace(p.id, { active: !p.active }).then(reload).catch((e) => onError(String(e)))}
            >
              {p.active ? 'Désactiver' : 'Réactiver'}
            </button>
          )}
        </div>
      ))}
      {canManage && (
        <div className="inline-form">
          <input placeholder="Nouveau lieu" value={label} onChange={(e) => setLabel(e.target.value)} />
          <input type="number" min={0} placeholder="Trajet (min)" value={travel} onChange={(e) => setTravel(e.target.value)} />
          <button
            disabled={!label.trim()}
            onClick={() =>
              api.createExamPlace({ label: label.trim(), travel_minutes: Number(travel) || 0 }).then(() => {
                setLabel('')
                setTravel('')
                reload()
              }).catch((e) => onError(String(e)))
            }
          >
            Ajouter
          </button>
        </div>
      )}
    </section>
  )
}

function ReasonsPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<AbsenceReason[]>([])
  const [label, setLabel] = useState('')
  const reload = useCallback(() => {
    api.absenceReasons().then(setRows).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  return (
    <section>
      <PanelHead
        title="Motifs"
        subtitle="Motifs d'absence proposés à l'émargement — d'autres domaines de motifs viendront"
      />
      {rows.map((a) => (
        <div key={a.id} className={`detail-row ${a.active ? '' : 'archived'}`}>
          <span className="lead">{a.label}</span>
          <span className="meta" />
          {canManage && (
            <button
              className="btn-sm"
              onClick={() => api.patchAbsenceReason(a.id, { active: !a.active }).then(reload).catch((e) => onError(String(e)))}
            >
              {a.active ? 'Désactiver' : 'Réactiver'}
            </button>
          )}
        </div>
      ))}
      {canManage && (
        <div className="inline-form">
          <input placeholder="Nouveau motif" value={label} onChange={(e) => setLabel(e.target.value)} />
          <button
            disabled={!label.trim()}
            onClick={() =>
              api.createAbsenceReason(label.trim()).then(() => {
                setLabel('')
                reload()
              }).catch((e) => onError(String(e)))
            }
          >
            Ajouter
          </button>
        </div>
      )}
    </section>
  )
}

// ---------------------------------------------------------------------
// Horaires / Crédits
// ---------------------------------------------------------------------

function OpeningPanel({ canManage, onError }: PanelProps) {
  const [rows, setRows] = useState<OpeningHourRow[]>([])
  const [dirty, setDirty] = useState(false)

  const reload = useCallback(() => {
    api.openingHours().then((r) => {
      setRows(r)
      setDirty(false)
    }).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  const update = (i: number, patch: Partial<OpeningHourRow>) => {
    setRows(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)))
    setDirty(true)
  }

  return (
    <section>
      <PanelHead
        title="Horaires et fermetures"
        subtitle="Hors de ces plages, une séance s'affiche en avertissement — jamais bloquée"
      />
      {rows.map((o, i) => (
        <div key={i} className="detail-row">
          <span className="lead">{DAYS[o.weekday]}</span>
          {canManage ? (
            <span style={{ display: 'flex', gap: 8 }}>
              <input type="time" value={o.start} onChange={(e) => update(i, { start: e.target.value })} />
              <span className="muted" style={{ alignSelf: 'center' }}>→</span>
              <input type="time" value={o.end} onChange={(e) => update(i, { end: e.target.value })} />
            </span>
          ) : (
            <span className="meta">{o.start} → {o.end}</span>
          )}
          {canManage && (
            <button className="btn-sm" onClick={() => { setRows(rows.filter((_, j) => j !== i)); setDirty(true) }}>
              Retirer
            </button>
          )}
        </div>
      ))}
      {canManage && (
        <div className="form-actions">
          <button
            onClick={() => {
              const used = new Set(rows.map((r) => r.weekday))
              const next = [1, 2, 3, 4, 5, 6, 7].find((d) => !used.has(d)) ?? 1
              setRows([...rows, { weekday: next, start: '08:00', end: '19:00' }])
              setDirty(true)
            }}
          >
            Ajouter un jour
          </button>
          <button
            className="primary"
            disabled={!dirty}
            onClick={() => api.replaceOpeningHours(rows).then(reload).catch((e) => onError(String(e)))}
          >
            Enregistrer les horaires
          </button>
        </div>
      )}
      <p className="muted" style={{ marginTop: 14 }}>
        Les fermetures exceptionnelles (jours fériés…) viendront dans cette même section.
      </p>
    </section>
  )
}

function CreditsPanel() {
  return (
    <section>
      <PanelHead
        title="Crédits et validité"
        subtitle="Barème des unités et durée de validité du plateau"
      />
      <div className="detail-row">
        <span className="lead">Épreuve plateau</span>
        <span className="meta">1 unité</span>
      </div>
      <div className="detail-row">
        <span className="lead">Épreuve circulation</span>
        <span className="meta">2 unités</span>
      </div>
      <p className="muted" style={{ marginTop: 12 }}>
        Ces valeurs deviendront modifiables ici avec le moteur de crédits complet ; un changement de
        barème ne réécrira jamais les unités déjà engagées.
      </p>
    </section>
  )
}

// ---------------------------------------------------------------------
// Modèles de messages — editor with live email/WhatsApp preview
// ---------------------------------------------------------------------

type TplMeta = {
  label: string
  when: string
  auto: boolean
  vars: { name: string; label: string; example: string }[]
}

const PERSON_VARS = [
  { name: 'prenom', label: 'Prénom', example: 'Anna' },
  { name: 'nom', label: 'Nom', example: 'Rossi' },
]

const TPL_META: Record<string, TplMeta> = {
  exam_convocation: {
    label: "Convocation d'examen",
    when: 'Envoyée depuis une session d\'examen — un message par candidat engagé, copie au payeur',
    auto: false,
    vars: [
      ...PERSON_VARS,
      { name: 'epreuve', label: 'Épreuve', example: 'circulation' },
      { name: 'date', label: 'Date', example: '13/08/2026' },
      { name: 'heure', label: 'Heure', example: '08h30' },
      { name: 'lieu', label: 'Lieu', example: "Centre d'examen de Meaux" },
      { name: 'presentation', label: 'Présentation', example: '07h45' },
    ],
  },
  lesson_assignment: {
    label: 'Placement sur une séance',
    when: 'Part automatiquement dès qu\'un élève est placé sur une séance à venir',
    auto: true,
    vars: [
      ...PERSON_VARS,
      { name: 'type', label: 'Type de séance', example: 'Conduite' },
      { name: 'date', label: 'Date', example: '07/08/2026' },
      { name: 'heure_debut', label: 'Début', example: '10h00' },
      { name: 'heure_fin', label: 'Fin', example: '12h00' },
      { name: 'avec', label: 'Formateur', example: ' avec Alhan' },
    ],
  },
  office_welcome: {
    label: 'Bienvenue',
    when: 'Point de départ proposé dans le message libre, à l\'inscription',
    auto: false,
    vars: [...PERSON_VARS, { name: 'objectif', label: 'Objectif', example: 'CE' }],
  },
  office_documents: {
    label: 'Pièces manquantes',
    when: 'Point de départ proposé dans le message libre — relance de dossier',
    auto: false,
    vars: PERSON_VARS,
  },
  office_hours_gap: {
    label: "Retard d'heures",
    when: 'Point de départ proposé dans le message libre — écart sur la progression',
    auto: false,
    vars: PERSON_VARS,
  },
  office_no_show: {
    label: 'Absence non prévenue',
    when: 'Point de départ proposé dans le message libre, après une absence',
    auto: false,
    vars: [...PERSON_VARS, { name: 'date', label: 'Date de la séance', example: '05/08/2026' }],
  },
}

function fillExample(text: string, meta: TplMeta | undefined): string {
  let out = text
  for (const v of meta?.vars ?? PERSON_VARS) {
    out = out.split('{{' + v.name + '}}').join(v.example)
  }
  return out
}

function TemplatesPanel({ canManage, onError }: PanelProps) {
  const [templates, setTemplates] = useState<{ kind: string; subject: string; body: string }[]>([])
  const [kind, setKind] = useState<string | null>(null)
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [channel, setChannel] = useState<'EMAIL' | 'WHATSAPP'>('EMAIL')
  const [saved, setSaved] = useState(false)

  const reload = useCallback(() => {
    api.communicationTemplates().then((ts) => {
      setTemplates(ts)
      if (ts.length > 0) {
        setKind((k) => k ?? ts[0].kind)
      }
    }).catch((e) => onError(String(e)))
  }, [onError])
  useEffect(reload, [reload])

  useEffect(() => {
    const t = templates.find((x) => x.kind === kind)
    if (t) {
      setSubject(t.subject)
      setBody(t.body)
      setSaved(false)
    }
  }, [kind, templates])

  const meta = kind ? TPL_META[kind] : undefined
  const stored = templates.find((x) => x.kind === kind)
  const dirty = stored ? subject !== stored.subject || body !== stored.body : false

  return (
    <div>
      <PanelHead
        title="Modèles de messages"
        subtitle="Le texte de chaque message que le centre envoie — modifiable ici, jamais dans le code"
      />

      <div className="tpl-kinds">
        {templates.map((t) => {
          const m = TPL_META[t.kind]
          return (
            <button
              key={t.kind}
              className={t.kind === kind ? 'tpl-kind on' : 'tpl-kind'}
              onClick={() => setKind(t.kind)}
            >
              <strong>{m?.label ?? t.kind}</strong>
              {m?.auto && <span className="tpl-auto">auto</span>}
            </button>
          )
        })}
      </div>

      {kind && (
        <div className="tpl-layout">
          <div className="tpl-editor">
            {meta && <InfoBanner>{meta.when}</InfoBanner>}
            <div className="form-row">
              <label>Sujet (email)</label>
              <input value={subject} onChange={(e) => { setSubject(e.target.value); setSaved(false) }} disabled={!canManage} />
            </div>
            <div className="form-row">
              <label>Message</label>
              <textarea
                rows={13}
                value={body}
                onChange={(e) => { setBody(e.target.value); setSaved(false) }}
                disabled={!canManage}
                style={{ font: 'inherit', padding: 12, borderRadius: 10, border: '1px solid var(--line)', lineHeight: 1.5 }}
              />
            </div>
            {meta && (
              <div className="tpl-vars">
                <span className="muted">Variables — remplacées à l'envoi :</span>
                {meta.vars.map((v) => (
                  <button
                    key={v.name}
                    className="var-chip"
                    title={v.label + ' — ex. ' + v.example}
                    onClick={() => canManage && setBody(body + '{{' + v.name + '}}')}
                  >
                    {'{{' + v.name + '}}'}
                  </button>
                ))}
              </div>
            )}
            {canManage && (
              <div className="form-actions">
                <button
                  className="primary"
                  disabled={!dirty || !body.trim()}
                  onClick={() =>
                    api
                      .putCommunicationTemplate(kind, { subject, body })
                      .then(() => { setSaved(true); reload() })
                      .catch((e) => onError(String(e)))
                  }
                >
                  Enregistrer le modèle
                </button>
                {saved && <span className="saved-note">Modèle enregistré ✓</span>}
                {dirty && !saved && <span className="muted" style={{ alignSelf: 'center' }}>Modifications non enregistrées</span>}
              </div>
            )}
          </div>

          <div className="tpl-preview">
            <div className="tpl-preview-tabs">
              <button className={channel === 'EMAIL' ? 'on' : ''} onClick={() => setChannel('EMAIL')}>
                Aperçu email
              </button>
              <button className={channel === 'WHATSAPP' ? 'on' : ''} onClick={() => setChannel('WHATSAPP')}>
                Aperçu WhatsApp
              </button>
            </div>
            {channel === 'EMAIL' ? (
              <div className="email-card">
                <div className="email-head">
                  <span className="email-brand">CAMO-EDUCASER</span>
                  <span className="email-meta">De : secretariat@camo-educaser.fr</span>
                  <span className="email-meta">À : anna.rossi@exemple.fr</span>
                </div>
                <div className="email-subject">{fillExample(subject, meta) || '(sans sujet)'}</div>
                <div className="email-body">{fillExample(body, meta)}</div>
              </div>
            ) : (
              <div className="wa-frame">
                <div className="wa-bubble">
                  {fillExample(body, meta)}
                  <span className="wa-time">
                    {new Date().toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })} ✓✓
                  </span>
                </div>
                <p className="muted" style={{ marginTop: 10, fontSize: 13 }}>
                  WhatsApp n'a pas de sujet — seul le message part, prérempli dans la conversation.
                </p>
              </div>
            )}
            <p className="muted" style={{ marginTop: 10, fontSize: 13 }}>
              Aperçu avec des valeurs d'exemple — à l'envoi, chaque variable prend la vraie valeur
              du destinataire.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
