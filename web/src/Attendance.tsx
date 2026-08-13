// F-12: recording attendance. The instructor's current lesson sits on
// top as the primary action; below, the backlog of unrecorded lessons,
// oldest first (RG-125), with stable columns. Recording happens in a
// dialog, students pre-filled "Présent" (RG-236).

import { useCallback, useEffect, useState } from 'react'
import {
  api,
  type AbsenceReason,
  type LessonAttendance,
  type UnrecordedLesson,
  type UserContext,
} from './api'
import Modal from './Modal'

const VALUE_LABELS: Record<string, string> = {
  PRESENT: 'Présent',
  EXCUSED: 'Absent justifié',
  UNEXCUSED: 'Absent non justifié',
}

export default function Attendance({ user }: { user: UserContext }) {
  // The instructor gets the mockup's day card; the office keeps the
  // full backlog table.
  if (user.instructor_resource_id && !user.permissions.edit_planning) {
    return <InstructorDay />
  }
  return <OfficeAttendance user={user} />
}

// ---------------------------------------------------------------------
// Instructor day — per the validated mockup
// ---------------------------------------------------------------------

// The offline queue: recordings that failed to send wait in
// localStorage and flush automatically when the network is back — the
// API's idempotence (RG-126) makes retries safe.
type QueuedRecording = {
  lessonId: string
  lines: { enrollment_id: string; value: string; reason?: string }[]
}

function readQueue(): QueuedRecording[] {
  try {
    return JSON.parse(localStorage.getItem('attendance-queue') ?? '[]') as QueuedRecording[]
  } catch {
    return []
  }
}

function writeQueue(q: QueuedRecording[]) {
  localStorage.setItem('attendance-queue', JSON.stringify(q))
}

function InstructorDay() {
  const [current, setCurrent] = useState<LessonAttendance | null>(null)
  const [backlog, setBacklog] = useState<UnrecordedLesson[]>([])
  const [open, setOpen] = useState<LessonAttendance | null>(null)
  const [reasons, setReasons] = useState<AbsenceReason[]>([])
  const [saved, setSaved] = useState(false)
  const [offline, setOffline] = useState(!navigator.onLine)
  const [queued, setQueued] = useState(readQueue().length)

  const flushQueue = useCallback(async () => {
    let q = readQueue()
    while (q.length > 0) {
      try {
        await api.recordAttendance(q[0].lessonId, q[0].lines)
        q = q.slice(1)
        writeQueue(q)
        setQueued(q.length)
      } catch {
        return // still offline: keep the rest for later
      }
    }
    setOffline(false)
  }, [])

  const reload = useCallback(() => {
    api
      .currentLesson()
      .then((l) => {
        setCurrent(l ?? null)
        setOffline(false)
        localStorage.setItem('attendance-cache', JSON.stringify(l ?? null))
      })
      .catch(() => {
        // Network down: show the cached day so the instructor can work.
        setOffline(true)
        try {
          setCurrent(JSON.parse(localStorage.getItem('attendance-cache') ?? 'null'))
        } catch {
          setCurrent(null)
        }
      })
    api.unrecordedLessons().then(setBacklog).catch(() => {})
  }, [])
  useEffect(reload, [reload])
  useEffect(() => {
    api.absenceReasons().then((r) => setReasons(r.filter((x) => x.active))).catch(() => {})
  }, [])
  useEffect(() => {
    void flushQueue()
    const onOnline = () => void flushQueue().then(reload)
    const onOffline = () => setOffline(true)
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOffline)
    return () => {
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOffline)
    }
  }, [flushQueue, reload])

  const lesson = open ?? current
  const today = new Date().toLocaleDateString('fr-FR', { weekday: 'long', day: 'numeric', month: 'short' })

  return (
    <div className="page-main">
      <div className="inst-day">
        <div className="inst-head" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h2>{today}</h2>
          {offline && <span className="chip">📡 Hors réseau</span>}
        </div>
        {queued > 0 && (
          <p className="muted" style={{ margin: '6px 0' }}>
            {queued} émargement{queued > 1 ? 's' : ''} en attente — envoyé au retour du réseau.
          </p>
        )}

        {backlog.length > 0 && !open && (
          <div className="inst-banner">
            <span>
              {backlog.length} séance{backlog.length > 1 ? 's' : ''} non renseignée{backlog.length > 1 ? 's' : ''}
            </span>
            <button
              className="btn-sm"
              onClick={() =>
                api.lessonAttendance(backlog[0].lesson_id).then(setOpen).catch(() => {})
              }
            >
              Ouvrir
            </button>
          </div>
        )}

        {saved && <div className="edit-banner" style={{ borderRadius: 10 }}>Présences enregistrées ✓</div>}

        {lesson ? (
          <InstructorLessonCard
            key={lesson.lesson_id}
            lesson={lesson}
            reasons={reasons}
            onSaved={() => {
              setSaved(true)
              setOpen(null)
              reload()
            }}
            onQueued={() => {
              setQueued(readQueue().length)
              setOpen(null)
            }}
          />
        ) : (
          <p className="muted" style={{ textAlign: 'center', marginTop: 40 }}>
            Aucune séance aujourd'hui.
          </p>
        )}
      </div>
    </div>
  )
}

function InstructorLessonCard({
  lesson,
  reasons,
  onSaved,
  onQueued,
}: {
  lesson: LessonAttendance
  reasons: AbsenceReason[]
  onSaved: () => void
  onQueued: () => void
}) {
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(lesson.students.map((s) => [s.enrollment_id, s.value || 'PRESENT'])),
  )
  const [reasonBy, setReasonBy] = useState<Record<string, string>>({})

  const hm = (iso: string) =>
    new Date(iso).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })

  return (
    <div className="inst-lesson">
      <h3>
        {hm(lesson.starts_at)} → {hm(lesson.ends_at)}
      </h3>
      <span className="muted">{lesson.students.length} élève(s)</span>

      {lesson.students.map((s) => {
        const absent = values[s.enrollment_id] !== 'PRESENT'
        return (
          <div
            key={s.enrollment_id}
            className={absent ? 'inst-student absent' : 'inst-student'}
            onClick={() =>
              setValues({
                ...values,
                [s.enrollment_id]: absent ? 'PRESENT' : 'EXCUSED',
              })
            }
          >
            <strong>{s.student_name}</strong>
            <span className="muted">
              {absent ? 'Absent — motif à choisir' : 'Présent'}
            </span>
            {absent && (
              <div className="reason-chips" onClick={(e) => e.stopPropagation()}>
                {reasons.map((r) => (
                  <button
                    key={r.id}
                    className={reasonBy[s.enrollment_id] === r.label ? 'on' : ''}
                    onClick={() => {
                      setReasonBy({ ...reasonBy, [s.enrollment_id]: r.label })
                      setValues({ ...values, [s.enrollment_id]: 'EXCUSED' })
                    }}
                  >
                    {r.label}
                  </button>
                ))}
                <button
                  className={reasonBy[s.enrollment_id] === '' && values[s.enrollment_id] === 'UNEXCUSED' ? 'on' : ''}
                  onClick={() => {
                    setReasonBy({ ...reasonBy, [s.enrollment_id]: '' })
                    setValues({ ...values, [s.enrollment_id]: 'UNEXCUSED' })
                  }}
                >
                  Sans motif
                </button>
              </div>
            )}
          </div>
        )
      })}

      <button
        className="primary inst-save"
        disabled={lesson.students.length === 0}
        onClick={() => {
          const lines = lesson.students.map((s) => ({
            enrollment_id: s.enrollment_id,
            value: values[s.enrollment_id],
            ...(reasonBy[s.enrollment_id] ? { reason: reasonBy[s.enrollment_id] } : {}),
          }))
          api
            .recordAttendance(lesson.lesson_id, lines)
            .then(onSaved)
            .catch(() => {
              // No network: park it, it will flush on its own.
              writeQueue([...readQueue(), { lessonId: lesson.lesson_id, lines }])
              onQueued()
            })
        }}
      >
        Enregistrer
      </button>
      <p className="muted" style={{ textAlign: 'center', margin: '6px 0 0' }}>
        Hors réseau, l'émargement part tout seul au retour de la connexion.
      </p>
    </div>
  )
}

// ---------------------------------------------------------------------
// Office view
// ---------------------------------------------------------------------

function OfficeAttendance({ user }: { user: UserContext }) {
  const [current, setCurrent] = useState<LessonAttendance | null>(null)
  const [backlog, setBacklog] = useState<UnrecordedLesson[]>([])
  const [recording, setRecording] = useState<LessonAttendance | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)

  const reload = useCallback(() => {
    api.currentLesson().then((l) => setCurrent(l ?? null)).catch(() => setCurrent(null))
    api.unrecordedLessons().then(setBacklog).catch((e) => setError(String(e)))
  }, [])
  useEffect(reload, [reload])

  const fmt = (iso: string) =>
    new Date(iso).toLocaleString('fr-FR', {
      weekday: 'short', day: 'numeric', month: 'short',
      hour: '2-digit', minute: '2-digit',
    })

  return (
    <div className="page">
      <div className="page-main">
        <div className="toolbar">
          <h2>Présences</h2>
          <div className="spacer" />
        </div>

        {error && <div className="error-box">{error}</div>}
        {saved && <div className="edit-banner">{saved}</div>}

        {current && (
          <div className="current-lesson">
            <div>
              <strong>Séance {new Date(current.starts_at).getTime() <= Date.now() &&
                Date.now() < new Date(current.ends_at).getTime()
                ? 'en cours'
                : 'du jour'}</strong>
              <div className="muted">
                {fmt(current.starts_at)} → {fmt(current.ends_at).split(' ').pop()} ·{' '}
                {current.students.length} élève(s)
              </div>
            </div>
            <button className="primary" onClick={() => setRecording(current)}>
              Faire l'appel
            </button>
          </div>
        )}

        <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
          <h3 style={{ marginBottom: 4 }}>Séances non renseignées</h3>
          {backlog.length > 0 && (
            <span className="muted">
              {backlog.length} séance{backlog.length > 1 ? 's' : ''} · les compteurs d'heures
              restent faux tant qu'elles ne sont pas saisies
            </span>
          )}
        </div>
        {backlog.length === 0 ? (
          <p className="muted">Tout est à jour.</p>
        ) : (
          groupByDay(backlog).map(([day, lessons]) => (
            <div key={day}>
              <h3 className="eyebrow">{day}</h3>
              {lessons.map((u) => (
                <div key={u.lesson_id} className="student-card">
                  <span className="health-dot big amber" />
                  <div className="student-id">
                    <strong>
                      {hm(u.starts_at)} → {hm(u.ends_at)}
                    </strong>
                    <span className="muted">
                      {u.instructors.join(', ') || 'sans formateur'}
                      {u.vehicles.length > 0 && ` · ${u.vehicles.join(', ')}`}
                      {` · ${u.students} élève${u.students > 1 ? 's' : ''}`}
                    </span>
                  </div>
                  <div className="spacer" />
                  <span className="status-pill neutral">
                    {u.recorded}/{u.students} saisi{u.recorded > 1 ? 's' : ''}
                  </span>
                  <div className="row-actions">
                    <button
                      className="btn-sm primary"
                      onClick={() =>
                        api.lessonAttendance(u.lesson_id).then(setRecording).catch((e) => setError(String(e)))
                      }
                    >
                      Faire l'appel
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ))
        )}

        {!current && !user.instructor_resource_id && backlog.length === 0 && (
          <p className="muted">Les séances passées à renseigner apparaîtront ici.</p>
        )}
      </div>

      {recording && (
        <Modal
          title={`Présences — ${fmt(recording.starts_at)}`}
          onClose={() => setRecording(null)}
        >
          <RecordForm
            key={recording.lesson_id}
            lesson={recording}
            onSaved={(when) => {
              setSaved(`Présences enregistrées — ${when}`)
              setRecording(null)
              reload()
            }}
          />
        </Modal>
      )}
    </div>
  )
}

function RecordForm({
  lesson,
  onSaved,
}: {
  lesson: LessonAttendance
  onSaved: (label: string) => void
}) {
  // Pre-filled "Présent": recording the normal case is one click.
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(lesson.students.map((s) => [s.enrollment_id, s.value || 'PRESENT'])),
  )
  const [reasons, setReasons] = useState<Record<string, string>>(() =>
    Object.fromEntries(lesson.students.map((s) => [s.enrollment_id, s.reason])),
  )
  const [error, setError] = useState<string | null>(null)

  const when = new Date(lesson.starts_at).toLocaleString('fr-FR', {
    weekday: 'long', day: 'numeric', month: 'long',
    hour: '2-digit', minute: '2-digit',
  })

  return (
    <div>
      {lesson.students.length === 0 && <p className="muted">Aucun élève placé sur cette séance.</p>}

      {lesson.students.map((s) => (
        <div key={s.enrollment_id} className="form-row">
          <label>
            {s.student_name}
            {s.value ? ' — déjà saisi, toute modification sera tracée' : ''}
          </label>
          <select
            value={values[s.enrollment_id]}
            onChange={(e) => setValues({ ...values, [s.enrollment_id]: e.target.value })}
          >
            {Object.entries(VALUE_LABELS).map(([v, label]) => (
              <option key={v} value={v}>{label}</option>
            ))}
          </select>
          {values[s.enrollment_id] !== 'PRESENT' && (
            <input
              placeholder="Motif"
              value={reasons[s.enrollment_id] ?? ''}
              onChange={(e) => setReasons({ ...reasons, [s.enrollment_id]: e.target.value })}
            />
          )}
        </div>
      ))}

      {error && <div className="error-box">{error}</div>}
      <div className="form-actions">
        <button
          className="primary"
          disabled={lesson.students.length === 0}
          onClick={() =>
            api
              .recordAttendance(
                lesson.lesson_id,
                lesson.students.map((s) => ({
                  enrollment_id: s.enrollment_id,
                  value: values[s.enrollment_id],
                  ...(reasons[s.enrollment_id] ? { reason: reasons[s.enrollment_id] } : {}),
                })),
              )
              .then(() => onSaved(when))
              .catch((e) => setError(String(e)))
          }
        >
          Enregistrer les présences
        </button>
      </div>
    </div>
  )
}

function hm(iso: string): string {
  return new Date(iso).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
}

// Oldest day first — the order the backlog should be cleared in.
function groupByDay(lessons: UnrecordedLesson[]): [string, UnrecordedLesson[]][] {
  const out = new Map<string, UnrecordedLesson[]>()
  for (const l of lessons) {
    const day = new Date(l.starts_at).toLocaleDateString('fr-FR', {
      weekday: 'long', day: 'numeric', month: 'long',
    })
    if (!out.has(day)) out.set(day, [])
    out.get(day)!.push(l)
  }
  return [...out.entries()]
}
