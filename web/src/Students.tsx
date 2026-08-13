// F-02 + F-04 + F-13 — the student list as cards: one search box for
// every identifier the office knows (NEPH, name, email, phone), a permit
// badge, a progress ring, and chips for what is MISSING on the file.
// Counters and gaps come computed from the server (C-05, C-26).
//
// The full file opens as a tabbed dialog: Dossier · Parcours · Séances ·
// Historique. Financement (F-05) and Documents (F-29) get their tabs
// when their features ship.

import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  api,
  ApiError,
  type EnrollmentDetail,
  type GapLine,
  type HistoryEvent,
  type Communication,
  type FunderKind,
  type FundingView,
  type Payer,
  type Objective,
  type RequirementSet,
  type PersonRow,
  type StudentLesson,
  type UserContext,
} from './api'
import Modal from './Modal'

export default function Students({ user }: { user: UserContext }) {
  const [rows, setRows] = useState<PersonRow[]>([])
  const [gaps, setGaps] = useState<GapLine[]>([])
  const [objectives, setObjectives] = useState<Objective[]>([])
  const [search, setSearch] = useState('')
  const [permitFilter, setPermitFilter] = useState('')
  const [onlyLate, setOnlyLate] = useState(false)
  const [showArchived, setShowArchived] = useState(false)
  const [sortBy, setSortBy] = useState<'name' | 'gap' | 'target'>('name')
  const [creating, setCreating] = useState(false)
  const [enrolling, setEnrolling] = useState<PersonRow | null>(null)
  const [detail, setDetail] = useState<PersonRow | null>(null)
  const [messaging, setMessaging] = useState<PersonRow | null>(null)
  const [error, setError] = useState<string | null>(null)
  const canManage = user.permissions.manage_people

  const reload = useCallback(() => {
    api.persons(search).then(setRows).catch((e) => setError(String(e)))
    api.gaps().then(setGaps).catch(() => {})
  }, [search])
  useEffect(reload, [reload])
  useEffect(() => {
    api.objectives().then(setObjectives).catch(() => {})
  }, [])

  const gapByEnrollment = useMemo(() => {
    const m = new Map<string, GapLine>()
    for (const g of gaps) {
      const known = m.get(g.enrollment_id)
      if (!known || g.gap_hours > known.gap_hours) m.set(g.enrollment_id, g)
    }
    return m
  }, [gaps])

  const visible = useMemo(() => {
    const filtered = rows.filter(
      (p) =>
        (showArchived || p.status === 'ACTIVE') &&
        (!permitFilter || p.enrollment?.objective === permitFilter) &&
        (!onlyLate || (p.enrollment && gapByEnrollment.has(p.enrollment.id))),
    )
    const gapOf = (p: PersonRow) =>
      p.enrollment ? gapByEnrollment.get(p.enrollment.id)?.gap_hours ?? 0 : 0
    return [...filtered].sort((a, b) => {
      if (sortBy === 'gap') return gapOf(b) - gapOf(a)
      if (sortBy === 'target')
        return (a.enrollment?.offroad_target_date ?? '9999').localeCompare(
          b.enrollment?.offroad_target_date ?? '9999',
        )
      return (a.last_name + a.first_names).localeCompare(b.last_name + b.first_names)
    })
  }, [rows, showArchived, permitFilter, onlyLate, gapByEnrollment, sortBy])

  return (
    <div className="page">
      <div className="page-main">
        <div className="toolbar">
          <h2>Élèves</h2>
          <div className="spacer" />
          {canManage && (
            <button className="primary" onClick={() => setCreating(true)}>
              Inscrire un nouvel élève
            </button>
          )}
        </div>

        <input
          type="search"
          className="big-search"
          placeholder="Rechercher un élève (NEPH, nom, prénom, email, téléphone)…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />

        <div className="toolbar">
          <select value={permitFilter} onChange={(e) => setPermitFilter(e.target.value)}>
            <option value="">Tous les permis</option>
            {objectives.map((o) => (
              <option key={o.id} value={o.label}>Permis {o.label}</option>
            ))}
          </select>
          <button className={onlyLate ? 'chip on' : 'chip'} onClick={() => setOnlyLate(!onlyLate)}>
            En écart<span className="count">{gapByEnrollment.size}</span>
          </button>
          <button
            className={showArchived ? 'chip on' : 'chip'}
            onClick={() => setShowArchived(!showArchived)}
          >
            Afficher les archivés
          </button>
          <div className="spacer" />
          <select value={sortBy} onChange={(e) => setSortBy(e.target.value as typeof sortBy)}>
            <option value="name">Trier par nom</option>
            <option value="gap">Trier par écart</option>
            <option value="target">Trier par échéance</option>
          </select>
        </div>

        {error && <div className="error-box">{error}</div>}
        <p className="muted">
          {visible.length} élève{visible.length > 1 ? 's' : ''} trouvé{visible.length > 1 ? 's' : ''}
        </p>

        {visible.map((p) => {
          const gap = p.enrollment ? gapByEnrollment.get(p.enrollment.id) : undefined
          const missing: string[] = []
          if (!p.neph) missing.push('NEPH')
          if (p.enrollment && !p.enrollment.offroad_target_date) missing.push('Échéances')
          if (p.enrollment && p.enrollment.upcoming_lessons === 0) missing.push('Aucune séance à venir')
          const pct =
            p.enrollment && p.enrollment.total_hours > 0
              ? Math.min(100, Math.round((p.enrollment.consumed_hours / p.enrollment.total_hours) * 100))
              : null
          return (
            <div
              key={p.id}
              className={`student-card ${p.status === 'ARCHIVED' ? 'archived' : ''}`}
              onClick={() => setDetail(p)}
            >
              <span className={`health-dot big ${p.health.color}`}>
                {p.health.reasons.length > 0 && (
                  <span className="health-tip">
                    {p.health.reasons.map((r, i) => (
                      <span key={i}>{r}</span>
                    ))}
                  </span>
                )}
              </span>
              <span className="permit-badge">{p.enrollment?.objective ?? '—'}</span>
              <div className="student-id">
                <strong>
                  {p.first_names} {p.last_name}
                </strong>
                <span className="muted">
                  {p.neph ? `NEPH ${p.neph}` : 'NEPH non renseigné'}
                  {p.enrollment?.funder_label ? ` · ${p.enrollment.funder_label}` : ''}
                  {p.enrollment?.funding_status
                    ? ` · ${FUNDING_LABELS[p.enrollment.funding_status] ?? p.enrollment.funding_status}`
                    : ''}
                  {p.phone ? ` · ${p.phone}` : ''}
                </span>
              </div>
              <div className="spacer" />
              <div className="student-chips">
                {gap && <span className="chip-missing critical">Retard {gap.gap_hours} h</span>}
                {missing.slice(0, gap ? 1 : 2).map((m) => (
                  <span key={m} className="chip-missing">{m}</span>
                ))}
                {missing.length > (gap ? 1 : 2) && (
                  <span className="chip-missing more">+{missing.length - (gap ? 1 : 2)}</span>
                )}
              </div>
              {pct !== null && p.enrollment && (
                <div className="hours-mini" title={`${p.enrollment.consumed_hours} h consommées sur ${p.enrollment.total_hours}`}>
                  <span>
                    {p.enrollment.consumed_hours} h <small>/ {p.enrollment.total_hours}</small>
                  </span>
                  <div className="mini-track">
                    <div className="mini-fill" style={{ width: pct + '%' }} />
                  </div>
                </div>
              )}
              <div className="row-actions" onClick={(e) => e.stopPropagation()}>
                <button onClick={() => setDetail(p)}>Détail</button>
                {canManage && p.status === 'ACTIVE' && (
                  <button onClick={() => setMessaging(p)}>Message</button>
                )}
                {canManage && !p.enrollment && p.status === 'ACTIVE' && (
                  <button onClick={() => setEnrolling(p)}>Créer un parcours</button>
                )}
              </div>
            </div>
          )
        })}
        {visible.length === 0 && (
          <p className="muted">Aucun élève ne correspond aux filtres.</p>
        )}
      </div>

      {creating && (
        <Modal title="Inscrire un nouvel élève" onClose={() => setCreating(false)}>
          <CreatePersonForm
            onDone={() => {
              setCreating(false)
              reload()
            }}
          />
        </Modal>
      )}
      {enrolling && (
        <Modal
          title={`Parcours — ${enrolling.first_names} ${enrolling.last_name}`}
          onClose={() => setEnrolling(null)}
        >
          <EnrollForm
            person={enrolling}
            objectives={objectives}
            onDone={() => {
              setEnrolling(null)
              reload()
            }}
          />
        </Modal>
      )}
      {messaging && (
        <Modal
          title={`Message — ${messaging.first_names} ${messaging.last_name}`}
          onClose={() => setMessaging(null)}
        >
          <SendMessageForm
            person={messaging}
            onSent={() => setMessaging(null)}
            onCancel={() => setMessaging(null)}
          />
        </Modal>
      )}
      {detail && (
        <PersonFile
          person={detail}
          canManage={canManage}
          onClose={() => setDetail(null)}
          onChanged={reload}
        />
      )}
    </div>
  )
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' })
}

function fmtWhen(iso: string): string {
  return new Date(iso).toLocaleString('fr-FR', {
    weekday: 'short', day: 'numeric', month: 'short',
    hour: '2-digit', minute: '2-digit',
  })
}

const FUNDING_LABELS: Record<string, string> = {
  DRAFT: 'financement à monter',
  SUBMITTED: 'financement déposé',
  APPROVED: 'financement accordé',
  SETTLED: 'financement soldé',
  REJECTED: 'financement refusé',
}

const VALUE_LABELS: Record<string, string> = {
  PRESENT: 'Présent',
  EXCUSED: 'Absent justifié',
  UNEXCUSED: 'Absent non justifié',
  '': 'Non renseignée',
}

const EVENT_LABELS: Record<string, string> = {
  'lesson.created': 'Séance posée',
  'lesson.moved': 'Séance déplacée',
  'lesson.cancelled': 'Séance annulée',
  'lesson.resource_assigned': 'Ressource assignée',
  'lesson.resource_unassigned': 'Ressource retirée',
  'lesson.student_placed': 'Placé sur une séance',
  'lesson.student_removed': 'Retiré d’une séance',
  'attendance.recorded': 'Présences saisies',
  'person.created': 'Dossier créé',
  'enrollment.created': 'Parcours créé',
  'enrollment.target_moved': 'Échéance déplacée',
}

// ---------------------------------------------------------------------
// The tabbed file
// ---------------------------------------------------------------------

type Tab = 'dossier' | 'parcours' | 'seances' | 'messages' | 'historique'

function PersonFile({
  person,
  canManage,
  onClose,
  onChanged,
}: {
  person: PersonRow
  canManage: boolean
  onClose: () => void
  onChanged: () => void
}) {
  const [tab, setTab] = useState<Tab>('dossier')
  const [hours, setHours] = useState<EnrollmentDetail | null>(null)
  const [lessons, setLessons] = useState<StudentLesson[]>([])
  const [history, setHistory] = useState<HistoryEvent[]>([])
  const [reqs, setReqs] = useState<{ entry: RequirementSet; exam: RequirementSet } | null>(null)
  const [editing, setEditing] = useState(false)

  const reloadReqs = () => {
    if (person.enrollment)
      api.requirements(person.enrollment.id).then(setReqs).catch(() => {})
  }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  useEffect(() => {
    api.personHistory(person.id).then(setHistory).catch(() => {})
    if (!person.enrollment) return
    api.enrollmentDetail(person.enrollment.id).then(setHours).catch(() => {})
    api.studentLessons(person.enrollment.id).then(setLessons).catch(() => {})
    api.requirements(person.enrollment.id).then(setReqs).catch(() => {})
  }, [person])

  const upcoming = lessons.filter((l) => l.upcoming && !l.cancelled)


  return (
    <div className="file-overlay">
      <div className="file-page">
        <header className="file-head">
          <div>
            <h1>
              {person.first_names} {person.last_name}
            </h1>
            <span className="muted">
              {person.neph ? `NEPH ${person.neph}` : 'NEPH non renseigné'}
              {person.date_of_birth ? ` · né(e) le ${fmtDate(person.date_of_birth)}` : ''}
            </span>
          </div>
          <div className="row-actions">
            {canManage && !editing && (
              <button onClick={() => { setEditing(true); setTab('dossier') }}>Modifier</button>
            )}
            <button onClick={onClose}>Fermer</button>
          </div>
        </header>

        <nav className="file-tabs">
          {(
            [
              ['dossier', 'Dossier'],
              ['parcours', person.enrollment ? `Parcours ${person.enrollment.objective}` : 'Parcours'],
              ['seances', `Séances${upcoming.length > 0 ? ` (${upcoming.length} à venir)` : ''}`],
              ['messages', 'Messages'],
              ['historique', 'Historique'],
            ] as [Tab, string][]
          ).map(([t, label]) => (
            <button key={t} className={tab === t ? 'active' : ''} onClick={() => setTab(t)}>
              {label}
            </button>
          ))}
        </nav>

        {tab === 'dossier' && (
          <div className="file-cols">
            <div>
              <h3 className="eyebrow">Contacts</h3>
              {editing ? (
                <EditPersonForm
                  person={person}
                  onDone={() => {
                    setEditing(false)
                    onChanged()
                    onClose()
                  }}
                  onCancel={() => setEditing(false)}
                />
              ) : (
                <>
                  <p>
                    <strong>Élève</strong> ·{' '}
                    {[person.phone, person.email].filter(Boolean).join(' · ') || (
                      <span className="muted">aucune coordonnée saisie</span>
                    )}
                  </p>
                  {person.enrollment && (
                    <PayerBlock enrollmentId={person.enrollment.id} canManage={canManage} />
                  )}
                </>
              )}

              {person.enrollment ? (
                <FundingBlock enrollmentId={person.enrollment.id} canManage={canManage} />
              ) : (
                <>
                  <h3 className="eyebrow">Financement</h3>
                  <p className="muted">Le dossier de financement naîtra avec le parcours.</p>
                </>
              )}
            </div>

            <div>
              {reqs ? (
                <>
                  <ReqSet set={reqs.entry} onChanged={reloadReqs} />
                  <ReqSet set={reqs.exam} onChanged={reloadReqs} />
                  <DocumentsBlock person={person} canManage={canManage} onChanged={reloadReqs} />
                </>
              ) : (
                <p className="muted">Les prérequis apparaîtront avec le parcours.</p>
              )}
            </div>
          </div>
        )}

        {tab === 'parcours' &&
          (hours ? (
            <div>
              <div className="stat-row">
                <div className="stat">
                  <span className="stat-num">{hours.hours.consumed}<small> h</small></span>
                  <span className="stat-label">consommées / {hours.total_hours} h</span>
                </div>
                <div className="stat">
                  <span className="stat-num">{hours.hours.attended}<small> h</small></span>
                  <span className="stat-label">effectuées</span>
                </div>
                <div className="stat">
                  <span className="stat-num">{hours.hours.excused + hours.hours.unexcused}<small> h</small></span>
                  <span className="stat-label">absences</span>
                </div>
                <div className="stat">
                  <span className="stat-num">{upcoming.length}</span>
                  <span className="stat-label">séances à venir</span>
                </div>
              </div>

              {hours.offroad_passed_at && (
                <div className="detail-row">
                  <span className="req-main">
                    <span className="req-label">Plateau obtenu</span>
                    <span className="req-meta">
                      le {fmtDate(hours.offroad_passed_at)}
                      {hours.offroad_expires_at && ` · valable jusqu'au ${fmtDate(hours.offroad_expires_at)}`}
                    </span>
                  </span>
                  <span className="status-pill ok">Réussi</span>
                </div>
              )}

              {hours.hours.projected_offroad !== null && !hours.offroad_passed_at && (
                <GaugeRow
                  label={`Plateau · échéance ${fmtDate(hours.offroad_target_date!)}`}
                  value={hours.hours.projected_offroad}
                  threshold={hours.hours_before_offroad}
                />
              )}
              {hours.hours.projected_onroad !== null && (
                <GaugeRow
                  label={`Circulation · échéance ${fmtDate(hours.onroad_target_date!)}`}
                  value={hours.hours.projected_onroad}
                  threshold={hours.total_hours}
                />
              )}

              {hours.alerts.map((a, i) =>
                a.kind === 'OFFROAD_EXPIRY' ? (
                  <div key={i} className="conflict warning">
                    <div className="kind">Plateau — expire dans {a.days_left} jours</div>
                    <div className="party">passé cette date, l'épreuve sera à repasser</div>
                  </div>
                ) : (
                  <div key={i} className="conflict warning">
                    <div className="kind">
                      Écart {a.target === 'OFFROAD' ? 'plateau' : 'circulation'} — manque {a.gap_hours} h
                    </div>
                    <div className="party">échéance dans {a.days_left} jours</div>
                  </div>
                ),
              )}
            </div>
          ) : (
            <p className="muted">Aucun parcours actif.</p>
          ))}

        {tab === 'seances' && (
          <div>
            {lessons.length === 0 && <p className="muted">Aucune séance.</p>}
            {upcoming.length > 0 && <h3 className="eyebrow">À venir</h3>}
            {[...upcoming].reverse().map((l, i) => (
              <div key={'u' + i} className="file-lesson">
                <span>{fmtWhen(l.starts_at)}</span>
                <span className="muted">{l.resources.join(' · ')}</span>
                <span className="pill upcoming">Planifiée</span>
              </div>
            ))}
            {lessons.some((l) => !l.upcoming) && <h3 className="eyebrow">Passées</h3>}
            {lessons
              .filter((l) => !l.upcoming)
              .map((l, i) => (
                <div key={'p' + i} className="file-lesson">
                  <span>{fmtWhen(l.starts_at)}</span>
                  <span className="muted">
                    {l.minutes} min · {l.resources.join(' · ')}
                  </span>
                  <span className={l.cancelled ? 'muted' : l.value === 'PRESENT' ? 'pill ok' : 'pill warn'}>
                    {l.cancelled ? 'Annulée' : VALUE_LABELS[l.value] ?? l.value}
                  </span>
                </div>
              ))}
          </div>
        )}

        {tab === 'messages' && <MessagesTab person={person} canManage={canManage} />}

      {tab === 'historique' && (
          <div>
            {history.length === 0 && (
              <p className="muted">
                Aucun événement tracé pour l'instant — l'historique se remplit au fil des actions.
              </p>
            )}
            {history.map((e, i) => (
              <div key={i} className="file-lesson">
                <span>{EVENT_LABELS[e.kind] ?? e.kind}</span>
                <span className="muted">
                  {fmtWhen(e.occurred_at)}
                  {e.author ? ` · par ${e.author}` : ''}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

// F-35: the file's documents and the e-photo request. The office
// creates a one-week tokenized link and sends it in one click through
// the communications pipeline; the upload validates the requirement.
function DocumentsBlock({
  person,
  canManage,
  onChanged,
}: {
  person: PersonRow
  canManage: boolean
  onChanged: () => void
}) {
  const [docs, setDocs] = useState<
    { id: string; kind: string; filename: string; ants_code: string; via: string; uploaded_at: string }[]
  >([])
  const [link, setLink] = useState<string | null>(null)
  const [sent, setSent] = useState<string | null>(null)

  const reload = () => {
    api.personDocuments(person.id).then(setDocs).catch(() => {})
  }
  useEffect(reload, [person.id])

  const fullLink = link ? window.location.origin + link : null

  return (
    <div>
      <h3 className="eyebrow">Documents</h3>
      {docs.length === 0 && <p className="muted">Aucun document reçu.</p>}
      {docs.map((d) => (
        <div key={d.id} className="detail-row">
          <span className="req-main">
            <span className="req-label">
              {d.kind === 'EPHOTO' ? 'E-photo' : d.filename}
              {d.ants_code && <span className="muted"> · code ANTS {d.ants_code}</span>}
            </span>
            <span className="req-meta">
              {d.via === 'PORTAL' ? 'reçue via le portail' : 'déposée au centre'} le{' '}
              {new Date(d.uploaded_at).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}
            </span>
          </span>
          <a className="link-action" href={`/api/documents/${d.id}/content`} target="_blank" rel="noreferrer">
            Voir
          </a>
        </div>
      ))}

      {canManage && !link && (
        <div className="form-actions">
          <button
            className="btn-sm"
            onClick={() =>
              api.requestEphoto(person.id).then((r) => setLink(r.path)).catch(() => {})
            }
          >
            Demander l'e-photo
          </button>
        </div>
      )}
      {fullLink && (
        <div className="setting-card" style={{ marginTop: 8 }}>
          <p style={{ margin: '0 0 6px' }}>
            Lien personnel (valable 7 jours, un seul envoi) :
          </p>
          <p className="muted" style={{ wordBreak: 'break-all', margin: '0 0 10px' }}>{fullLink}</p>
          <div className="form-actions" style={{ marginTop: 0 }}>
            {person.phone && (
              <button
                className="btn-sm wa"
                onClick={() =>
                  api
                    .sendFreeMessage(person.id, {
                      channel: 'WHATSAPP',
                      body: `Bonjour ${person.first_names}, pour finaliser votre inscription, prenez votre e-photo ici : ${fullLink} — CAMO-EDUCASER`,
                    })
                    .then((c) => {
                      if (c.whatsapp_link) {
                        window.open(c.whatsapp_link, '_blank')
                        api.markCommunicationSent(c.id).catch(() => {})
                      }
                      setSent('WhatsApp')
                    })
                    .catch(() => {})
                }
              >
                Envoyer par WhatsApp
              </button>
            )}
            {person.email && (
              <button
                className="btn-sm"
                onClick={() =>
                  api
                    .sendFreeMessage(person.id, {
                      channel: 'EMAIL',
                      subject: 'Votre e-photo pour finaliser votre inscription',
                      body: `Bonjour ${person.first_names},

Pour finaliser votre inscription, prenez votre e-photo depuis votre téléphone : ${fullLink}

CAMO-EDUCASER`,
                    })
                    .then(() => setSent('email'))
                    .catch(() => {})
                }
              >
                Envoyer par email
              </button>
            )}
            <button
              className="btn-sm"
              onClick={() => navigator.clipboard?.writeText(fullLink).then(() => setSent('copié'))}
            >
              Copier le lien
            </button>
          </div>
          {sent && <p className="muted" style={{ margin: '8px 0 0' }}>Lien {sent === 'copié' ? 'copié' : `envoyé par ${sent}`} ✓</p>}
        </div>
      )}
      <span style={{ display: 'none' }}>{typeof onChanged === 'function' ? '' : ''}</span>
    </div>
  )
}

// Communications: the file's message history, plus the free message —
// Angelo's arbitrage: email sends itself (or is simulated without SMTP),
// WhatsApp is a prepared one-recipient link the office clicks.
// SendMessageForm: one free message to this person, email or prepared
// WhatsApp link (one recipient per message). Shared by the file's
// Messages tab and the quick action on the list.
export function SendMessageForm({
  person,
  onSent,
  onCancel,
}: {
  person: PersonRow
  onSent: () => void
  onCancel: () => void
}) {
  const [channel, setChannel] = useState<'EMAIL' | 'WHATSAPP'>(
    person.email ? 'EMAIL' : 'WHATSAPP',
  )
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [templates, setTemplates] = useState<{ kind: string; subject: string; body: string }[]>([])
  useEffect(() => {
    api.communicationTemplates()
      .then((ts) => setTemplates(ts.filter((t) => t.kind.startsWith('office_'))))
      .catch(() => {})
  }, [])

  const OFFICE_LABELS: Record<string, string> = {
    office_welcome: 'Bienvenue',
    office_documents: 'Pièces manquantes',
    office_hours_gap: "Retard d'heures",
    office_no_show: 'Absence non prévenue',
  }
  const applyTemplate = (kind: string) => {
    const t = templates.find((x) => x.kind === kind)
    if (!t) return
    const fillVars = (s: string) =>
      s
        .split('{{prenom}}').join(person.first_names)
        .split('{{nom}}').join(person.last_name)
        .split('{{objectif}}').join(person.enrollment?.objective ?? '')
    setSubject(fillVars(t.subject))
    setBody(fillVars(t.body))
  }

  return (
    <div>
      {templates.length > 0 && (
        <div className="form-row" style={{ maxWidth: 300 }}>
          <label>Partir d'un modèle — facultatif</label>
          <select defaultValue="" onChange={(e) => e.target.value && applyTemplate(e.target.value)}>
            <option value="">Message vierge</option>
            {templates.map((t) => (
              <option key={t.kind} value={t.kind}>{OFFICE_LABELS[t.kind] ?? t.kind}</option>
            ))}
          </select>
        </div>
      )}
      <div className="form-row" style={{ maxWidth: 260 }}>
        <label>Canal</label>
        <select value={channel} onChange={(e) => setChannel(e.target.value as 'EMAIL' | 'WHATSAPP')}>
          <option value="EMAIL">Email{person.email ? ` (${person.email})` : ' — aucune adresse'}</option>
          <option value="WHATSAPP">WhatsApp{person.phone ? ` (${person.phone})` : ' — aucun numéro'}</option>
        </select>
      </div>
      {channel === 'EMAIL' && (
        <div className="form-row">
          <label>Sujet</label>
          <input value={subject} onChange={(e) => setSubject(e.target.value)} />
        </div>
      )}
      <div className="form-row">
        <label>Message</label>
        <textarea
          rows={5}
          value={body}
          onChange={(e) => setBody(e.target.value)}
          autoFocus
          style={{ font: 'inherit', padding: 10, borderRadius: 10, border: '1px solid var(--line)' }}
        />
      </div>
      {error && <div className="error-box">{error}</div>}
      <div className="form-actions">
        <button
          className="primary"
          disabled={!body.trim()}
          onClick={() =>
            api
              .sendFreeMessage(person.id, {
                channel,
                ...(subject.trim() ? { subject: subject.trim() } : {}),
                body: body.trim(),
              })
              .then((c) => {
                if (c.channel === 'WHATSAPP' && c.whatsapp_link) {
                  window.open(c.whatsapp_link, '_blank')
                  api.markCommunicationSent(c.id).catch(() => {})
                }
                onSent()
              })
              .catch((e) =>
                setError(e instanceof ApiError ? String(e.body.error ?? e.message) : String(e)),
              )
          }
        >
          {channel === 'WHATSAPP' ? 'Préparer et ouvrir WhatsApp' : 'Envoyer'}
        </button>
        <button onClick={onCancel}>Annuler</button>
      </div>
    </div>
  )
}

function MessagesTab({ person, canManage }: { person: PersonRow; canManage: boolean }) {
  const [comms, setComms] = useState<Communication[]>([])
  const [writing, setWriting] = useState(false)

  const reload = () => {
    api.personCommunications(person.id).then(setComms).catch(() => {})
  }
  useEffect(reload, [person.id])

  const KIND_LABELS: Record<string, string> = {
    exam_convocation: 'Convocation examen',
    free: 'Message',
  }

  return (
    <div>
      {canManage && !writing && (
        <div className="form-actions" style={{ marginTop: 0 }}>
          <button className="primary" onClick={() => setWriting(true)}>Nouveau message</button>
        </div>
      )}
      {writing && (
        <div className="setting-card">
          <SendMessageForm
            person={person}
            onSent={() => {
              setWriting(false)
              reload()
            }}
            onCancel={() => setWriting(false)}
          />
        </div>
      )}

      {comms.length === 0 && <p className="muted">Aucun message envoyé pour l'instant.</p>}
      {comms.map((c) => (
        <div key={c.id} className="detail-row">
          <span className="req-main">
            <span className="req-label">
              {KIND_LABELS[c.kind] ?? c.kind}
              {c.subject ? ` — ${c.subject}` : ''}
            </span>
            <span className="req-meta">
              {c.channel === 'WHATSAPP' ? 'WhatsApp' : 'Email'} · {c.recipient_label} ·{' '}
              {new Date(c.created_at).toLocaleString('fr-FR', {
                day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
              })}
            </span>
            <span className="req-meta" style={{ whiteSpace: 'normal' }}>{c.body.slice(0, 140)}{c.body.length > 140 ? '…' : ''}</span>
          </span>
          {c.channel === 'WHATSAPP' && c.status === 'PREPARED' && c.whatsapp_link ? (
            <button
              className="btn-sm wa"
              onClick={() => {
                window.open(c.whatsapp_link, '_blank')
                api.markCommunicationSent(c.id).then(reload).catch(() => {})
              }}
            >
              Ouvrir WhatsApp
            </button>
          ) : (
            <span
              className={
                c.status === 'SENT' || c.status === 'SIMULATED'
                  ? 'status-pill ok'
                  : c.status === 'FAILED'
                    ? 'status-pill bad'
                    : 'status-pill neutral'
              }
            >
              {c.status === 'SENT' ? 'Envoyé' : c.status === 'SIMULATED' ? 'Simulé' : c.status === 'FAILED' ? 'Échec' : 'Préparé'}
            </span>
          )}
        </div>
      ))}
    </div>
  )
}

// The payer (C-12): shown with its reference contact; NULL = the
// student pays. Pick an existing one or create it inline.
function PayerBlock({ enrollmentId, canManage }: { enrollmentId: string; canManage: boolean }) {
  const [funding, setFunding] = useState<FundingView | null>(null)
  const [payers, setPayers] = useState<Payer[]>([])
  const [creating, setCreating] = useState(false)
  const [label, setLabel] = useState('')
  const [contact, setContact] = useState('')
  const [email, setEmail] = useState('')

  const reload = () => {
    api.funding(enrollmentId).then(setFunding).catch(() => {})
    api.payers().then(setPayers).catch(() => {})
  }
  useEffect(reload, [enrollmentId])

  if (!funding) return null

  return (
    <div style={{ marginBottom: 4 }}>
      {funding.payer ? (
        <p style={{ marginBottom: 2 }}>
          <strong>Payeur</strong> · {funding.payer.label}
          {(funding.payer.contact_name || funding.payer.contact_email) && (
            <span className="muted" style={{ display: 'block' }}>
              {[funding.payer.contact_name, funding.payer.contact_email, funding.payer.contact_phone]
                .filter(Boolean)
                .join(' · ')}
            </span>
          )}
        </p>
      ) : (
        <p style={{ marginBottom: 2 }}>
          <strong>Payeur</strong> · <span className="muted">l'élève lui-même</span>
        </p>
      )}
      {canManage && !creating && (
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <select
            value={funding.payer_id ?? ''}
            onChange={(e) =>
              api.setPayer(enrollmentId, e.target.value || null).then(reload).catch(() => {})
            }
            style={{ maxWidth: 220 }}
          >
            <option value="">L'élève lui-même</option>
            {payers.filter((p) => p.active).map((p) => (
              <option key={p.id} value={p.id}>{p.label}</option>
            ))}
          </select>
          <button className="link-action" onClick={() => setCreating(true)}>
            Nouveau payeur
          </button>
        </div>
      )}
      {creating && (
        <div className="setting-card" style={{ marginTop: 8 }}>
          <div className="form-row">
            <label>Entreprise ou organisme</label>
            <input value={label} onChange={(e) => setLabel(e.target.value)} autoFocus />
          </div>
          <div className="form-row">
            <label>Contact référent</label>
            <input value={contact} onChange={(e) => setContact(e.target.value)} />
          </div>
          <div className="form-row">
            <label>Email du contact</label>
            <input value={email} onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div className="form-actions">
            <button
              className="primary btn-sm"
              disabled={!label.trim()}
              onClick={() =>
                api
                  .createPayer({
                    label: label.trim(),
                    ...(contact.trim() ? { contact_name: contact.trim() } : {}),
                    ...(email.trim() ? { contact_email: email.trim() } : {}),
                  })
                  .then((r) => api.setPayer(enrollmentId, r.id))
                  .then(() => {
                    setCreating(false)
                    setLabel('')
                    setContact('')
                    setEmail('')
                    reload()
                  })
                  .catch(() => {})
              }
            >
              Créer et associer
            </button>
            <button className="btn-sm" onClick={() => setCreating(false)}>Annuler</button>
          </div>
        </div>
      )}
    </div>
  )
}

// F-05: the funding file — funder, coverage cycle, traced transitions.
const CYCLE: [FundingView['status'], string][] = [
  ['DRAFT', 'À monter'],
  ['SUBMITTED', 'Déposé'],
  ['APPROVED', 'Accordé'],
  ['SETTLED', 'Soldé'],
]

function FundingBlock({ enrollmentId, canManage }: { enrollmentId: string; canManage: boolean }) {
  const [funding, setFunding] = useState<FundingView | null>(null)
  const [kinds, setKinds] = useState<FunderKind[]>([])
  const [rejectReason, setRejectReason] = useState('')
  const [rejecting, setRejecting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const reload = () => {
    api.funding(enrollmentId).then(setFunding).catch(() => {})
  }
  useEffect(reload, [enrollmentId])
  useEffect(() => {
    api.funderKinds().then(setKinds).catch(() => {})
  }, [])

  if (!funding) return null

  const go = (status: string, reason?: string) =>
    api
      .fundingTransition(enrollmentId, status, reason)
      .then(() => {
        setRejecting(false)
        setRejectReason('')
        setError(null)
        reload()
      })
      .catch((e) => setError(e instanceof ApiError ? String(e.body.error ?? e.message) : String(e)))

  const idx = CYCLE.findIndex(([s]) => s === funding.status)
  const last = funding.transitions?.[0]

  return (
    <div>
      <h3 className="eyebrow">Financement</h3>
      <div className="cycle">
        {CYCLE.map(([s, label], i) => (
          <span key={s} className={`cycle-step ${s === funding.status ? 'on' : i < idx ? 'done' : 'off'}`}>
            {label}
          </span>
        ))}
        {funding.status === 'REJECTED' && <span className="cycle-step rejected">Refusé</span>}
      </div>

      {canManage ? (
        <div className="form-row" style={{ maxWidth: 260, marginTop: 8 }}>
          <label>Financeur</label>
          <select
            value={funding.funder_kind_id ?? ''}
            onChange={(e) =>
              api.patchFunding(enrollmentId, e.target.value || null).then(reload).catch(() => {})
            }
          >
            <option value="">—</option>
            {kinds.filter((k) => k.active).map((k) => (
              <option key={k.id} value={k.id}>{k.label}</option>
            ))}
          </select>
        </div>
      ) : (
        funding.funder_label && <p>{funding.funder_label}</p>
      )}

      {last && (
        <p className="muted" style={{ margin: '4px 0' }}>
          {FUNDING_LABELS[last.to] ?? last.to} le{' '}
          {new Date(last.at).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })} par {last.author}
          {last.reason ? ` — ${last.reason}` : ''}
        </p>
      )}

      {canManage && (
        <div className="form-actions" style={{ flexWrap: 'wrap' }}>
          {funding.status === 'DRAFT' && (
            <button className="btn-sm primary" onClick={() => void go('SUBMITTED')}>Déposer</button>
          )}
          {funding.status === 'SUBMITTED' && (
            <button className="btn-sm primary" onClick={() => void go('APPROVED')}>Accorder</button>
          )}
          {funding.status === 'APPROVED' && (
            <button className="btn-sm primary" onClick={() => void go('SETTLED')}>Solder</button>
          )}
          {(funding.status === 'DRAFT' || funding.status === 'SUBMITTED') && (
            <button className="btn-sm" onClick={() => setRejecting(!rejecting)}>Refuser</button>
          )}
          {funding.status === 'REJECTED' && (
            <button className="btn-sm" onClick={() => void go('DRAFT')}>Remonter le dossier</button>
          )}
        </div>
      )}
      {rejecting && (
        <div className="req-na-form" style={{ paddingLeft: 0 }}>
          <input
            placeholder="Motif du refus (obligatoire)"
            value={rejectReason}
            autoFocus
            onChange={(e) => setRejectReason(e.target.value)}
          />
          <button
            className="btn-sm primary"
            disabled={!rejectReason.trim()}
            onClick={() => void go('REJECTED', rejectReason.trim())}
          >
            Confirmer le refus
          </button>
        </div>
      )}
      {error && <div className="error-box">{error}</div>}
    </div>
  )
}

// F-29: one requirement set, titled by purpose. Every state shown was
// computed by the server; buttons only appear when THIS caller may act.
function ReqSet({ set, onChanged }: { set: RequirementSet; onChanged: () => void }) {
  const [naFor, setNaFor] = useState<string | null>(null)
  const [naReason, setNaReason] = useState('')

  return (
    <div className="req-set">
      <h3 className="eyebrow">
        {set.title}
        {set.complete ? (
          <span className="check-count ok"> Complet</span>
        ) : (
          <span className="check-count"> {set.missing} manquant{set.missing > 1 ? 's' : ''} sur {set.items.length}</span>
        )}
      </h3>
      {set.items.map((q) => {
        const ok = q.status === 'VALIDATED' && !q.expired
        const na = q.status === 'NOT_APPLICABLE'
        return (
          <div key={q.id}>
            <div className="req-row">
              <span className={ok ? 'req-mark ok' : na ? 'req-mark na' : 'req-mark ko'}>
                {ok ? '✓' : na ? '—' : '✗'}
              </span>
              <span className="req-main">
                <span className={na ? 'req-label muted' : 'req-label'}>{q.label}</span>
                <span className="req-meta">
                  {q.expired && <span className="amber-note">expiré · </span>}
                  {ok && q.valid_until && <>valide jusqu'au {fmtDate(q.valid_until)} · </>}
                  {ok && q.validated_by && <>validé par {q.validated_by}</>}
                  {na && <>sans objet — {q.na_reason}</>}
                </span>
              </span>
              {q.can_validate && !ok && !na && (
                <span className="req-actions">
                  <button
                    className="btn-sm primary"
                    onClick={() => api.actRequirement(q.id, 'validate').then(onChanged).catch(() => {})}
                  >
                    Valider
                  </button>
                  <button className="btn-sm" onClick={() => setNaFor(naFor === q.id ? null : q.id)}>
                    Non applicable
                  </button>
                </span>
              )}
              {q.can_validate && (ok || q.expired || na) && (
                <span className="req-actions">
                  <button
                    className="btn-sm"
                    onClick={() => api.actRequirement(q.id, 'unvalidate').then(onChanged).catch(() => {})}
                  >
                    {na ? 'Rétablir' : 'Dévalider'}
                  </button>
                </span>
              )}
            </div>
            {naFor === q.id && (
              <div className="req-na-form">
                <input
                  placeholder="Motif (pourquoi ce prérequis ne s'applique pas)"
                  value={naReason}
                  autoFocus
                  onChange={(e) => setNaReason(e.target.value)}
                />
                <button
                  className="btn-sm primary"
                  disabled={!naReason.trim()}
                  onClick={() =>
                    api
                      .actRequirement(q.id, 'not-applicable', { reason: naReason.trim() })
                      .then(() => {
                        setNaFor(null)
                        setNaReason('')
                        onChanged()
                      })
                      .catch(() => {})
                  }
                >
                  Confirmer
                </button>
                <button className="btn-sm" onClick={() => setNaFor(null)}>Annuler</button>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

function GaugeRow({ label, value, threshold }: { label: string; value: number; threshold: number }) {
  const pct = threshold > 0 ? Math.min(100, Math.round((value / threshold) * 100)) : 0
  const short = value < threshold
  return (
    <div className="gauge-row">
      <div className="gauge-head">
        <span>{label}</span>
        <span className={short ? 'gauge-val short' : 'gauge-val'}>
          {value} h projetées / seuil {threshold} h
        </span>
      </div>
      <div className="gauge-track">
        <div className={short ? 'gauge-fill short' : 'gauge-fill'} style={{ width: pct + '%' }} />
      </div>
    </div>
  )
}

function EditPersonForm({
  person,
  onDone,
  onCancel,
}: {
  person: PersonRow
  onDone: () => void
  onCancel: () => void
}) {
  const [phone, setPhone] = useState(person.phone ?? '')
  const [email, setEmail] = useState(person.email ?? '')
  const [neph, setNeph] = useState(person.neph ?? '')
  const [error, setError] = useState<string | null>(null)

  return (
    <div>
      <div className="form-row">
        <label>Téléphone</label>
        <input value={phone} onChange={(e) => setPhone(e.target.value)} autoFocus />
      </div>
      <div className="form-row">
        <label>Email</label>
        <input value={email} onChange={(e) => setEmail(e.target.value)} />
      </div>
      <div className="form-row">
        <label>NEPH</label>
        <input value={neph} onChange={(e) => setNeph(e.target.value)} />
      </div>
      {error && <div className="error-box">{error}</div>}
      <div className="form-actions">
        <button
          className="primary"
          onClick={() =>
            api
              .patchPerson(person.id, {
                ...(phone.trim() ? { phone: phone.trim() } : {}),
                ...(email.trim() ? { email: email.trim() } : {}),
                ...(neph.trim() ? { neph: neph.trim() } : {}),
              })
              .then(onDone)
              .catch((e) =>
                setError(e instanceof ApiError ? String(e.body.error ?? e.message) : String(e)),
              )
          }
        >
          Enregistrer
        </button>
        <button onClick={onCancel}>Annuler</button>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------
// Create / enroll forms
// ---------------------------------------------------------------------

function CreatePersonForm({ onDone }: { onDone: () => void }) {
  const [lastName, setLastName] = useState('')
  const [firstNames, setFirstNames] = useState('')
  const [phone, setPhone] = useState('')
  const [email, setEmail] = useState('')
  const [neph, setNeph] = useState('')
  const [error, setError] = useState<string | null>(null)

  return (
    <div>
      <div className="form-row">
        <label>Nom</label>
        <input value={lastName} onChange={(e) => setLastName(e.target.value)} autoFocus />
      </div>
      <div className="form-row">
        <label>Prénoms</label>
        <input value={firstNames} onChange={(e) => setFirstNames(e.target.value)} />
      </div>
      <div className="form-row">
        <label>NEPH (peut être ajouté plus tard)</label>
        <input value={neph} onChange={(e) => setNeph(e.target.value)} />
      </div>
      <div className="form-row">
        <label>Téléphone</label>
        <input value={phone} onChange={(e) => setPhone(e.target.value)} />
      </div>
      <div className="form-row">
        <label>Email</label>
        <input value={email} onChange={(e) => setEmail(e.target.value)} />
      </div>
      {error && <div className="error-box">{error}</div>}
      <div className="form-actions">
        <button
          className="primary"
          disabled={!lastName.trim() || !firstNames.trim()}
          onClick={() =>
            api
              .createPerson({
                last_name: lastName.trim(),
                first_names: firstNames.trim(),
                ...(phone.trim() ? { phone: phone.trim() } : {}),
                ...(email.trim() ? { email: email.trim() } : {}),
                ...(neph.trim() ? { neph: neph.trim() } : {}),
              })
              .then(onDone)
              .catch((e) =>
                setError(e instanceof ApiError ? String(e.body.error ?? e.message) : String(e)),
              )
          }
        >
          Créer le dossier
        </button>
      </div>
    </div>
  )
}

function EnrollForm({
  person,
  objectives,
  onDone,
}: {
  person: PersonRow
  objectives: Objective[]
  onDone: () => void
}) {
  const [objectiveID, setObjectiveID] = useState('')
  const [offroad, setOffroad] = useState('')
  const [onroad, setOnroad] = useState('')
  const [error, setError] = useState<string | null>(null)

  const chosen = objectives.find((o) => o.id === objectiveID)

  return (
    <div>
      <div className="form-row">
        <label>Objectif</label>
        <select value={objectiveID} onChange={(e) => setObjectiveID(e.target.value)} autoFocus>
          <option value="">—</option>
          {objectives.map((o) => (
            <option key={o.id} value={o.id}>{o.label}</option>
          ))}
        </select>
        {chosen && (
          <span className="muted">
            Seuils repris de l'objectif : plateau à {chosen.hours_before_offroad} h, total{' '}
            {chosen.total_hours} h
          </span>
        )}
      </div>
      <div className="form-row">
        <label>Échéance plateau</label>
        <input type="date" value={offroad} onChange={(e) => setOffroad(e.target.value)} />
      </div>
      <div className="form-row">
        <label>Échéance circulation</label>
        <input type="date" value={onroad} onChange={(e) => setOnroad(e.target.value)} />
      </div>
      {error && <div className="error-box">{error}</div>}
      <div className="form-actions">
        <button
          className="primary"
          disabled={!objectiveID}
          onClick={() =>
            api
              .createEnrollment({
                person_id: person.id,
                objective_id: objectiveID,
                ...(offroad ? { offroad_target_date: new Date(offroad).toISOString() } : {}),
                ...(onroad ? { onroad_target_date: new Date(onroad).toISOString() } : {}),
              })
              .then(onDone)
              .catch((e) =>
                setError(e instanceof ApiError ? String(e.body.error ?? e.message) : String(e)),
              )
          }
        >
          Créer le parcours
        </button>
      </div>
    </div>
  )
}
