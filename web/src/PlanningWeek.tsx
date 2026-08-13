// F-09 — the planner on FullCalendar timeGrid (D-09; the resource
// timeline was tried and rejected by Angelo — days as columns won),
// STRICTLY CONTROLLED: the calendar renders state and reports gestures;
// every gesture becomes a draft operation, and availability + conflicts
// stay in the single Go component (RG-78). The calendar decides nothing.
//
// Rules the UI embodies:
//  - The system alerts, it never blocks (D-04).
//  - A conflict names its window and both sides (RG-217).
//  - Editing goes through the draft (C-09): nothing touches the planner
//    until "Enregistrer"; "Abandonner" leaves no trace (RG-147).

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import FullCalendar from '@fullcalendar/react'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin from '@fullcalendar/interaction'
import type { EventInput } from '@fullcalendar/core'
import frLocale from '@fullcalendar/core/locales/fr'

// Structural types for the gesture callbacks: the premium packages ship
// their own copy of the core types, which TypeScript refuses to unify.
// We only touch these fields.
interface GestureInfo {
  revert: () => void
  event: { id: string; start: Date | null; end: Date | null }
}
import {
  api,
  ApiError,
  uuidv7,
  type Communication,
  type Conflict,
  type DurationRow,
  type Lesson,
  type LockHolder,
  type Operation,
  type PlanningView,
  type SessionView,
  type SuggestedStudent,
  type UserContext,
} from './api'
import Modal from './Modal'
import ExamDetail from './ExamDetail'

const DAY_MS = 24 * 3600 * 1000

const CONFLICT_LABELS: Record<string, string> = {
  STUDENT_BOOKED: 'Élève déjà engagé',
  INSTRUCTOR_BOOKED: 'Formateur déjà engagé',
  RESOURCE_BOOKED: 'Ressource déjà engagée',
  RESOURCE_OFFLINE: 'Ressource indisponible',
  OUTSIDE_HOURS: "Hors horaires d'ouverture",
  CATEGORY_MISMATCH: 'Catégorie non couverte',
  ENROLLMENT_CLOSED: 'Parcours clôturé',
  FUNDING_NOT_APPROVED: 'Financement non accordé',
  VEHICLE_MISSING: 'Séance de conduite sans véhicule',
}

function mondayOf(d: Date): Date {
  const out = new Date(d)
  out.setHours(0, 0, 0, 0)
  out.setDate(out.getDate() - ((out.getDay() + 6) % 7))
  return out
}

function fmtHour(iso: string): string {
  return new Date(iso).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
}

function fmtDay(iso: string): string {
  return new Date(iso).toLocaleDateString('fr-FR', { weekday: 'short', day: 'numeric', month: 'short' })
}

function worstSeverity(conflicts: Conflict[]): 'critical' | 'warning' | null {
  if (conflicts.some((c) => c.Severity === 'CRITICAL')) return 'critical'
  if (conflicts.some((c) => c.Severity === 'WARNING')) return 'warning'
  return null
}

export default function PlanningWeek({ user }: { user: UserContext }) {
  const [weekStart, setWeekStart] = useState(() => mondayOf(new Date()))
  const weekEnd = useMemo(() => new Date(weekStart.getTime() + 7 * DAY_MS), [weekStart])
  const [view, setView] = useState<'timeGridDay' | 'timeGridWeek'>('timeGridWeek')

  const [planning, setPlanning] = useState<PlanningView | null>(null)
  const [session, setSession] = useState<SessionView | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [lockHolder, setLockHolder] = useState<LockHolder | null>(null)
  const [report, setReport] = useState<string | null>(null)
  const [creating, setCreating] = useState<{ at: Date; resourceID?: string } | null>(null)
  const [examDetailID, setExamDetailID] = useState<string | null>(null)
  const [durations, setDurations] = useState<DurationRow[]>([])
  // Display filters: they hide events on screen, nothing more — the
  // draft and its conflicts always cover the full week.
  const [instructorFilter, setInstructorFilter] = useState('')
  const [vehicleFilter, setVehicleFilter] = useState('')

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const calendarRef = useRef<any>(null)
  const editing = session !== null

  useEffect(() => {
    api.durations().then((d) => setDurations(d.filter((x) => x.active))).catch(() => {})
  }, [])

  const reload = useCallback(async () => {
    try {
      setPlanning(await api.planning(weekStart.toISOString(), weekEnd.toISOString()))
      setError(null)
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        window.location.reload()
        return
      }
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [weekStart, weekEnd])

  useEffect(() => {
    void reload()
  }, [reload])

  useEffect(() => {
    calendarRef.current?.getApi().gotoDate(weekStart)
  }, [weekStart, view])

  // ---- displayed lessons: saved state, or saved + draft when editing ----

  const lessons: Lesson[] = useMemo(() => {
    if (!planning) return []
    if (!session) return planning.lessons
    const draftConflicts = session.draft.conflicts ?? {}
    const savedConflicts = new Map(planning.lessons.map((l) => [l.ID, l.conflicts]))
    return (session.draft.lessons ?? []).map((l) => ({
      ...l,
      conflicts: draftConflicts[l.ID] ?? savedConflicts.get(l.ID) ?? [],
    }))
  }, [planning, session])

  const draftLessonIds = useMemo(() => {
    const ids = new Set<string>()
    for (const op of session?.draft.pending ?? []) ids.add(op.lesson_id)
    return ids
  }, [session])

  const selectedLesson = lessons.find((l) => l.ID === selected) ?? null
  const pendingOps = session?.draft.pending ?? []

  // ---- controlled events: days as columns, hours as rows ----

  const matchesFilters = useCallback(
    (resourceIds: string[] | null | undefined) => {
      if (instructorFilter && !(resourceIds ?? []).includes(instructorFilter)) return false
      if (vehicleFilter && !(resourceIds ?? []).includes(vehicleFilter)) return false
      return true
    },
    [instructorFilter, vehicleFilter],
  )

  const events = useMemo<EventInput[]>(() => {
    const out: EventInput[] = []
    for (const l of lessons) {
      if (!matchesFilters(l.Resources)) continue
      const sev = l.Cancelled ? null : worstSeverity(l.conflicts)
      const who =
        (l.Resources ?? []).map((id) => planning?.resources[id]?.Label ?? '?').join(' · ') ||
        'Sans ressource'
      const detail =
        ((l.Kinds ?? []).map((k) => k.Label).join(', ') || '') +
        ((l.Enrollments?.length ?? 0) > 0
          ? `${(l.Kinds?.length ?? 0) > 0 ? ' · ' : ''}${l.Enrollments!.length} élève(s)`
          : '')
      out.push({
        id: l.ID,
        start: l.Slot.Start,
        end: l.Slot.End,
        title: who,
        editable: editing && !l.Cancelled,
        classNames: [
          'ev-lesson',
          l.Cancelled ? 'ev-cancelled' : '',
          sev === 'critical' ? 'ev-critical' : sev === 'warning' ? 'ev-warning' : '',
          draftLessonIds.has(l.ID) ? 'ev-draft' : '',
          selected === l.ID ? 'ev-selected' : '',
        ].filter(Boolean),
        extendedProps: { kind: 'lesson', conflicts: l.conflicts.length, detail },
      })
    }
    for (const ex of planning?.exam_sessions ?? []) {
      if (!matchesFilters(ex.Resources)) continue
      const travelMs = ex.TravelTime / 1e6
      const travelMin = Math.round(ex.TravelTime / 60e9)
      const who = (ex.Resources ?? [])
        .map((id) => planning?.resources[id]?.Label ?? '?')
        .join(' · ')
      out.push({
        id: 'exam-' + ex.ID,
        start: new Date(new Date(ex.Slot.Start).getTime() - travelMs).toISOString(),
        end: new Date(new Date(ex.Slot.End).getTime() + travelMs).toISOString(),
        title: `Examen · ${ex.PlaceLabel}` + (travelMin > 0 ? ` (+${travelMin} min trajet)` : ''),
        editable: false,
        classNames: ['ev-exam'],
        extendedProps: { kind: 'exam', conflicts: 0, detail: who },
      })
    }
    return out
  }, [lessons, planning, editing, draftLessonIds, selected, matchesFilters])

  // ---- edit session ----

  async function openEdit() {
    setError(null)
    setLockHolder(null)
    setReport(null)
    try {
      const s = await api.openSession(weekStart.toISOString(), weekEnd.toISOString())
      setSession(await api.pushDraft(s.id, s.operations ?? []))
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setLockHolder(e.body.holder as LockHolder)
      } else {
        setError(e instanceof Error ? e.message : String(e))
      }
    }
  }

  async function push(ops: Operation[]) {
    if (!session) return
    try {
      setSession(await api.pushDraft(session.session.id, ops))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function save() {
    if (!session) return
    try {
      const r = await api.save(session.session.id)
      const overridden = r.overridden?.length ?? 0
      setReport(
        `Enregistré : ${r.applied?.length ?? 0} opération(s)` +
          (overridden > 0 ? ` — ${overridden} conflit(s) passé(s) outre, tracé(s)` : ''),
      )
      setSession(null)
      setSelected(null)
      await reload()
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setError('Le planning a été modifié en dehors de votre session. Rouvrez une édition.')
      } else {
        setError(e instanceof Error ? e.message : String(e))
      }
    }
  }

  async function discard() {
    if (!session) return
    await api.discard(session.session.id).catch(() => {})
    setSession(null)
    setSelected(null)
    await reload()
  }

  // ---- calendar gestures → draft operations. The calendar itself is
  // reverted every time: the truth comes back from the server. ----

  function onEventDrop(info: GestureInfo) {
    info.revert()
    const lesson = lessons.find((l) => l.ID === info.event.id)
    if (!lesson || !info.event.start) return
    const durationMs =
      new Date(lesson.Slot.End).getTime() - new Date(lesson.Slot.Start).getTime()
    const start = info.event.start
    const ops: Operation[] = [
      {
        id: uuidv7(),
        kind: 'MOVE_LESSON',
        lesson_id: lesson.ID,
        slot: {
          Start: start.toISOString(),
          End: new Date(start.getTime() + durationMs).toISOString(),
        },
      },
    ]
    void push([...pendingOps, ...ops])
  }

  function onEventResize(info: GestureInfo) {
    info.revert()
    const lesson = lessons.find((l) => l.ID === info.event.id)
    if (!lesson || !info.event.start || !info.event.end) return
    void push([
      ...pendingOps,
      {
        id: uuidv7(),
        kind: 'MOVE_LESSON',
        lesson_id: lesson.ID,
        slot: { Start: info.event.start.toISOString(), End: info.event.end.toISOString() },
      },
    ])
  }

  // ---- render ----

  return (
    <>
      <header className="topbar">
        <h1>Planning</h1>
        <button onClick={() => setWeekStart(new Date(weekStart.getTime() - 7 * DAY_MS))}>←</button>
        <span className="week-label">
          Semaine du {weekStart.toLocaleDateString('fr-FR', { day: 'numeric', month: 'long' })}
        </span>
        <button onClick={() => setWeekStart(new Date(weekStart.getTime() + 7 * DAY_MS))}>→</button>
        <button className="chip" onClick={() => setWeekStart(mondayOf(new Date()))}>
          Aujourd'hui
        </button>
        <select value={instructorFilter} onChange={(e) => setInstructorFilter(e.target.value)}>
          <option value="">Tous les formateurs</option>
          {Object.values(planning?.resources ?? {})
            .filter((r) => r.Kind === 'INSTRUCTOR' && !r.Archived)
            .map((r) => (
              <option key={r.ID} value={r.ID}>{r.Label}</option>
            ))}
        </select>
        <select value={vehicleFilter} onChange={(e) => setVehicleFilter(e.target.value)}>
          <option value="">Tous les véhicules</option>
          {Object.values(planning?.resources ?? {})
            .filter((r) => r.Kind === 'VEHICLE' && !r.Archived)
            .map((r) => (
              <option key={r.ID} value={r.ID}>{r.Label}</option>
            ))}
        </select>
        <div className="spacer" />
        <button
          className={view === 'timeGridDay' ? 'chip on' : 'chip'}
          onClick={() => setView('timeGridDay')}
        >
          Jour
        </button>
        <button
          className={view === 'timeGridWeek' ? 'chip on' : 'chip'}
          onClick={() => setView('timeGridWeek')}
        >
          Semaine
        </button>
        {!editing && user.permissions.edit_planning && (
          <button className="primary" onClick={() => void openEdit()}>
            Ouvrir une édition
          </button>
        )}
      </header>

      {editing && (
        <div className="edit-banner">
          <span className="who">Session d'édition ouverte</span>
          <span className="muted">
            {pendingOps.length} modification(s) en attente — cliquez un créneau, ou glissez une
            séance pour la déplacer
          </span>
          <div className="spacer" style={{ flex: 1 }} />
          <button onClick={() => setCreating({ at: new Date(weekStart.getTime() + 9 * 3600 * 1000) })}>
            Poser une séance
          </button>
          <button className="primary" onClick={() => void save()} disabled={pendingOps.length === 0}>
            Enregistrer
          </button>
          <button className="danger" onClick={() => void discard()}>
            Abandonner
          </button>
        </div>
      )}

      {lockHolder && (
        <div className="error-box lock-banner">
          <span>
            Planning en cours d'édition par <strong>{lockHolder.holder_name}</strong> depuis{' '}
            {fmtHour(lockHolder.opened_at)}.
          </span>
          {lockHolder.holder_user_id === user.user_id ? (
            <>
              <button
                onClick={() => {
                  api
                    .getSession(lockHolder.session_id)
                    .then((s) => {
                      setSession(s)
                      setLockHolder(null)
                    })
                    .catch((e) => setError(String(e)))
                }}
              >
                Reprendre ma session
              </button>
              <button
                className="danger"
                onClick={() => {
                  api
                    .discard(lockHolder.session_id)
                    .then(() => setLockHolder(null))
                    .catch((e) => setError(String(e)))
                }}
              >
                L'abandonner
              </button>
            </>
          ) : user.permissions.force_release ? (
            <button
              className="danger"
              onClick={() => {
                api
                  .forceRelease(lockHolder.session_id)
                  .then(() => setLockHolder(null))
                  .catch((e) => setError(String(e)))
              }}
            >
              Libérer de force
            </button>
          ) : (
            <span className="muted">Le verrou tombera de lui-même après 30 min d'inactivité.</span>
          )}
        </div>
      )}
      {error && <div className="error-box">{error}</div>}
      {report && <div className="edit-banner">{report}</div>}

      <div className="layout">
        <div className="grid-wrap p-2">
          <FullCalendar
            ref={calendarRef}
            plugins={[timeGridPlugin, interactionPlugin]}
            locale={frLocale}
            initialView={view}
            initialDate={weekStart}
            headerToolbar={false}
            height="100%"
            allDaySlot={false}
            events={events}
            slotMinTime="07:00:00"
            slotMaxTime="20:00:00"
            scrollTime="08:00:00"
            slotDuration="00:30:00"
            firstDay={1}
            nowIndicator
            dayHeaderFormat={{ weekday: 'long', day: 'numeric' }}
            eventClick={(info) => {
              if (info.event.extendedProps.kind === 'lesson') setSelected(info.event.id)
              // The exam session opens its file — bookable right here,
              // still never editable from the planner (RG-152).
              if (info.event.extendedProps.kind === 'exam')
                setExamDetailID(info.event.id.replace('exam-', ''))
            }}
            dateClick={(info: { date: Date }) => {
              if (!editing) return
              setCreating({ at: new Date(info.date) })
            }}
            editable={editing}
            eventDrop={onEventDrop}
            eventResize={onEventResize}
            eventContent={(arg) => {
              const conflicts = (arg.event.extendedProps.conflicts as number) ?? 0
              const detail = (arg.event.extendedProps.detail as string) ?? ''
              return (
                <div className="ev-inner ev-stack">
                  <span className="ev-time">
                    {arg.timeText}
                    {conflicts > 0 && <span className="ev-badge">⚠ {conflicts}</span>}
                  </span>
                  <span className="ev-title">{arg.event.title}</span>
                  {detail && <span className="ev-detail">{detail}</span>}
                </div>
              )
            }}
          />
        </div>

        <aside className="side">
          {selectedLesson && planning && (
            <LessonDetail
              lesson={selectedLesson}
              planning={planning}
              editing={editing}
              canEdit={user.permissions.edit_planning}
              onOps={(ops) => void push([...pendingOps, ...ops])}
              onDirectChange={() => void reload()}
            />
          )}

          {editing && pendingOps.length > 0 && (
            <>
              <h3>Modifications en attente</h3>
              <ul className="op-list">
                {pendingOps.map((op) => (
                  <li key={op.id}>
                    <span>{describeOp(op, planning)}</span>
                    <button
                      title="Retirer du brouillon"
                      onClick={() => void push(pendingOps.filter((o) => o.id !== op.id))}
                    >
                      ×
                    </button>
                  </li>
                ))}
              </ul>
            </>
          )}

          {(session?.draft.rejected ?? []).map((r) => (
            <div key={r.operation_id} className="error-box">
              Opération impossible : {r.reason}
            </div>
          ))}

          {!selectedLesson && !editing && (
            <p className="muted">
              Sélectionnez une séance pour voir son détail et ses conflits, ou ouvrez une édition
              pour modifier la semaine.
            </p>
          )}
          {!selectedLesson && editing && pendingOps.length === 0 && (
            <p className="muted">
              Cliquez un créneau sur la ligne d'une ressource pour poser une séance, glissez une
              séance pour la déplacer, ou cliquez-la pour la modifier.
            </p>
          )}
        </aside>
      </div>

      {examDetailID && (
        <ExamDetail
          sessionId={examDetailID}
          canManage={user.permissions.edit_planning}
          onClose={() => {
            setExamDetailID(null)
            void reload()
          }}
        />
      )}
      {creating && planning && (
        <Modal
          title={`Poser une séance — ${creating.at.toLocaleDateString('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })}`}
          onClose={() => setCreating(null)}
        >
          <CreateLessonModal
            at={creating.at}
            preselectedResource={creating.resourceID}
            planning={planning}
            durations={durations}
            onCreate={(ops) => {
              void push([...pendingOps, ...ops])
              setCreating(null)
            }}
          />
        </Modal>
      )}
    </>
  )
}

// ---------------------------------------------------------------------

function describeOp(op: Operation, planning: PlanningView | null): string {
  switch (op.kind) {
    case 'CREATE_LESSON':
      return `Poser une séance ${op.slot ? fmtDay(op.slot.Start) + ' ' + fmtHour(op.slot.Start) : ''}`
    case 'MOVE_LESSON':
      return `Déplacer vers ${op.slot ? fmtDay(op.slot.Start) + ' ' + fmtHour(op.slot.Start) : ''}`
    case 'CANCEL_LESSON':
      return `Annuler (${op.reason})`
    case 'ASSIGN_RESOURCE':
      return `Assigner ${planning?.resources[op.resource_id ?? '']?.Label ?? 'ressource'}`
    case 'UNASSIGN_RESOURCE':
      return `Retirer ${planning?.resources[op.resource_id ?? '']?.Label ?? 'ressource'}`
    case 'PLACE_STUDENT':
      return `Placer ${planning?.enrollments[op.enrollment_id ?? '']?.StudentName ?? 'élève'}`
    case 'REMOVE_STUDENT':
      return `Retirer ${planning?.enrollments[op.enrollment_id ?? '']?.StudentName ?? 'élève'}`
  }
}

function ConflictCard({ c }: { c: Conflict }) {
  const sev = c.Severity === 'CRITICAL' ? 'critical' : 'warning'
  return (
    <div className={`conflict ${sev}`}>
      <div className="kind">
        {CONFLICT_LABELS[c.Kind] ?? c.Kind}
        {c.Subject ? ` — ${c.Subject}` : ''}
      </div>
      {c.SameCauseAs !== null && c.SameCauseAs !== undefined ? (
        <div className="same-cause">Même cause que le conflit précédent</div>
      ) : (
        <>
          {c.Overlap && (
            <div className="window">
              Chevauchement {fmtHour(c.Overlap.Start)}–{fmtHour(c.Overlap.End)}
            </div>
          )}
          {(c.Parties ?? []).map((p, i) => (
            <div key={i} className="party">
              {p.Kind === 'lesson' && 'Séance'}
              {p.Kind === 'exam_session' && `Session d'examen ${p.Label}`}
              {p.Kind === 'unavailability' && `Indisponibilité : ${p.Label}`}
              {p.Kind === 'enrollment' && p.Label}
              {p.Slot?.Start ? ` · ${fmtDay(p.Slot.Start)} ${fmtHour(p.Slot.Start)}–${fmtHour(p.Slot.End)}` : ''}
              {p.Note && <span className="note"> — {p.Note}</span>}
            </div>
          ))}
        </>
      )}
    </div>
  )
}

function LessonDetail({
  lesson,
  planning,
  editing,
  canEdit,
  onOps,
  onDirectChange,
}: {
  lesson: Lesson
  planning: PlanningView
  editing: boolean
  canEdit: boolean
  onOps: (ops: Operation[]) => void
  onDirectChange: () => void
}) {
  const [cancelReason, setCancelReason] = useState('')
  const [resourceToAdd, setResourceToAdd] = useState('')
  const [studentToAdd, setStudentToAdd] = useState('')
  const [suggested, setSuggested] = useState<SuggestedStudent[] | null>(null)
  const [placeOpen, setPlaceOpen] = useState(false)
  const [lastComm, setLastComm] = useState<Communication | null>(null)

  const direct = !editing && canEdit && !lesson.Cancelled

  useEffect(() => {
    if (!editing && !direct) return
    api.suggestedStudents(lesson.ID).then(setSuggested).catch(() => setSuggested(null))
  }, [editing, direct, lesson.ID])

  const assignable = Object.values(planning.resources).filter(
    (r) => !r.Archived && !(lesson.Resources ?? []).includes(r.ID),
  )
  const placeable = Object.values(planning.enrollments).filter(
    (e) => !(lesson.Enrollments ?? []).includes(e.ID),
  )

  return (
    <div>
      <h2>
        Séance {fmtDay(lesson.Slot.Start)} {fmtHour(lesson.Slot.Start)}–{fmtHour(lesson.Slot.End)}
        {lesson.Cancelled ? ' (annulée)' : ''}
      </h2>

      {(lesson.Kinds ?? []).length > 0 && (
        <p className="muted">{(lesson.Kinds ?? []).map((k) => k.Label).join(', ')}</p>
      )}

      {(lesson.Resources ?? []).length > 0 && <h3>Ressources</h3>}
      {(lesson.Resources ?? []).map((id) => (
        <div key={id} className="detail-row">
          <span className="lead">{planning.resources[id]?.Label ?? id}</span>
          <span className="meta">
            {planning.resources[id]?.Kind === 'INSTRUCTOR'
              ? 'formateur'
              : planning.resources[id]?.Kind === 'VEHICLE'
                ? (planning.resources[id]?.Categories ?? []).join(', ')
                : ''}
          </span>
          {editing && !lesson.Cancelled && (
            <button
              className="link-action danger"
              onClick={() =>
                onOps([{ id: uuidv7(), kind: 'UNASSIGN_RESOURCE', lesson_id: lesson.ID, resource_id: id }])
              }
            >
              Retirer
            </button>
          )}
        </div>
      ))}
      {(lesson.Enrollments ?? []).length > 0 && <h3>Élèves</h3>}
      {(lesson.Enrollments ?? []).map((id) => (
        <div key={id} className="detail-row">
          <span className="lead">{planning.enrollments[id]?.StudentName ?? id}</span>
          <span className="meta">{planning.enrollments[id]?.Category ?? ''}</span>
          {editing && !lesson.Cancelled && (
            <button
              className="link-action danger"
              onClick={() =>
                onOps([{ id: uuidv7(), kind: 'REMOVE_STUDENT', lesson_id: lesson.ID, enrollment_id: id }])
              }
            >
              Retirer
            </button>
          )}
          {direct && (
            <button
              className="link-action danger"
              onClick={() => api.removeDirect(lesson.ID, id).then(onDirectChange).catch(() => {})}
            >
              Retirer
            </button>
          )}
        </div>
      ))}

      {lesson.conflicts.length > 0 && (
        <>
          <h3>Conflits</h3>
          <p className="muted" style={{ margin: '0 0 8px' }}>Signalés, jamais bloquants.</p>
          {lesson.conflicts.map((c, i) => (
            <ConflictCard key={i} c={c} />
          ))}
        </>
      )}

      {direct && (
        <>
          <button className="collapse-head" onClick={() => setPlaceOpen(!placeOpen)}>
            <h3 style={{ margin: 0 }}>Placer un élève</h3>
            <span className="chevron">{placeOpen ? '▾' : '›'}</span>
          </button>
          {placeOpen && (
            <>
          <p className="muted" style={{ margin: '0 0 8px' }}>
            Placement immédiat — les conflits éventuels s'affichent aussitôt, l'élève est
            prévenu automatiquement.
          </p>
          {lastComm && (
            <div className="convocation-box">
              <div className="detail-row">
                <span className="req-main">
                  <span className="req-label">Message à {lastComm.recipient_label}</span>
                  <span className="req-meta">
                    {lastComm.channel === 'WHATSAPP'
                      ? 'WhatsApp — préparé, un clic pour l\'envoyer'
                      : lastComm.status === 'SIMULATED'
                        ? 'Email simulé (SMTP non configuré) — tracé dans la fiche'
                        : lastComm.status === 'SENT'
                          ? 'Email envoyé'
                          : 'Échec de l\'envoi'}
                  </span>
                </span>
                {lastComm.channel === 'WHATSAPP' && lastComm.whatsapp_link && (
                  <button
                    className="btn-sm wa"
                    onClick={() => {
                      window.open(lastComm.whatsapp_link, '_blank')
                      api.markCommunicationSent(lastComm.id).catch(() => {})
                      setLastComm(null)
                    }}
                  >
                    Ouvrir WhatsApp
                  </button>
                )}
              </div>
            </div>
          )}
          {(suggested ?? [])
            .filter((s) => !(lesson.Enrollments ?? []).includes(s.enrollment_id))
            .slice(0, 5)
            .map((s) => (
              <div key={s.enrollment_id} className="suggestion">
                <div>
                  <strong>{s.student_name}</strong> · {s.objective}
                  <div className="muted">
                    {s.gap_hours > 0
                      ? `manque ${s.gap_hours} h · échéance dans ${s.days_left} j`
                      : 'à jour sur ses heures'}
                  </div>
                </div>
                <button
                  className="btn-sm primary"
                  onClick={() =>
                    api
                      .placeDirect(lesson.ID, s.enrollment_id)
                      .then((r) => {
                        setLastComm(r.communication ?? null)
                        onDirectChange()
                      })
                      .catch(() => {})
                  }
                >
                  Placer
                </button>
              </div>
            ))}
            </>
          )}
        </>
      )}

      {editing && !lesson.Cancelled && (
        <>
          <h3>Modifier</h3>

          <div className="form-row">
            <label>Assigner une ressource</label>
            <select value={resourceToAdd} onChange={(e) => setResourceToAdd(e.target.value)}>
              <option value="">—</option>
              {assignable.map((r) => (
                <option key={r.ID} value={r.ID}>
                  {r.Label} ({r.Kind === 'INSTRUCTOR' ? 'formateur' : r.Kind.toLowerCase()})
                </option>
              ))}
            </select>
            {resourceToAdd && (
              <button
                onClick={() => {
                  onOps([{ id: uuidv7(), kind: 'ASSIGN_RESOURCE', lesson_id: lesson.ID, resource_id: resourceToAdd }])
                  setResourceToAdd('')
                }}
              >
                Assigner
              </button>
            )}
          </div>

          {suggested && suggested.length > 0 ? (
            <div className="form-row">
              <label>Élèves proposés — classés par retard sur les heures projetées</label>
              {suggested
                .filter((s) => !(lesson.Enrollments ?? []).includes(s.enrollment_id))
                .slice(0, 5)
                .map((s) => (
                  <div key={s.enrollment_id} className="suggestion">
                    <div>
                      <strong>{s.student_name}</strong> · {s.objective}
                      <div className="muted">
                        {s.gap_hours > 0
                          ? `manque ${s.gap_hours} h · échéance dans ${s.days_left} j (${s.projected_hours} h projetées / ${s.threshold_hours} h)`
                          : 'à jour sur ses heures'}
                      </div>
                    </div>
                    <button
                      onClick={() =>
                        onOps([{ id: uuidv7(), kind: 'PLACE_STUDENT', lesson_id: lesson.ID, enrollment_id: s.enrollment_id }])
                      }
                    >
                      Placer
                    </button>
                  </div>
                ))}
            </div>
          ) : (
            <div className="form-row">
              <label>Placer un élève</label>
              <select value={studentToAdd} onChange={(e) => setStudentToAdd(e.target.value)}>
                <option value="">—</option>
                {placeable.map((s) => (
                  <option key={s.ID} value={s.ID}>
                    {s.StudentName} ({s.Category})
                  </option>
                ))}
              </select>
              {studentToAdd && (
                <button
                  onClick={() => {
                    onOps([{ id: uuidv7(), kind: 'PLACE_STUDENT', lesson_id: lesson.ID, enrollment_id: studentToAdd }])
                    setStudentToAdd('')
                  }}
                >
                  Placer
                </button>
              )}
            </div>
          )}

          <div className="form-row">
            <label>Annuler la séance (motif obligatoire)</label>
            <input
              placeholder="Motif"
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
            />
            <button
              className="danger"
              disabled={!cancelReason.trim()}
              onClick={() => {
                onOps([{ id: uuidv7(), kind: 'CANCEL_LESSON', lesson_id: lesson.ID, reason: cancelReason.trim() }])
                setCancelReason('')
              }}
            >
              Annuler la séance
            </button>
          </div>
        </>
      )}
    </div>
  )
}

// The full "pose a lesson" dialog: time and resource pre-filled from the
// clicked cell — every choice lands as draft operations.
function CreateLessonModal({
  at,
  preselectedResource,
  planning,
  durations,
  onCreate,
}: {
  at: Date
  preselectedResource?: string
  planning: PlanningView
  durations: DurationRow[]
  onCreate: (ops: Operation[]) => void
}) {
  const [time, setTime] = useState(
    `${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`,
  )
  const [durationMin, setDurationMin] = useState(durations[0]?.minutes ?? 180)
  const [kindId, setKindId] = useState('')
  const [resourceIds, setResourceIds] = useState<Set<string>>(
    () => new Set(preselectedResource ? [preselectedResource] : []),
  )
  const [studentIds, setStudentIds] = useState<Set<string>>(new Set())

  const kinds = Object.values(planning.lesson_kinds)
  const resources = Object.values(planning.resources)
    .filter((r) => !r.Archived)
    .sort((a, b) => a.Kind.localeCompare(b.Kind) || a.Label.localeCompare(b.Label))
  const students = Object.values(planning.enrollments).filter((s) => !s.Closed)

  const toggle = (set: Set<string>, update: (s: Set<string>) => void, id: string) => {
    const next = new Set(set)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    update(next)
  }

  return (
    <div>
      <div className="form-row" style={{ flexDirection: 'row', gap: 12 }}>
        <div className="form-row" style={{ margin: 0 }}>
          <label>Début</label>
          <input type="time" value={time} onChange={(e) => setTime(e.target.value)} />
        </div>
        <div className="form-row" style={{ margin: 0 }}>
          <label>Durée</label>
          {durations.length > 0 ? (
            <select value={durationMin} onChange={(e) => setDurationMin(Number(e.target.value))}>
              {durations.map((d) => (
                <option key={d.id} value={d.minutes}>{d.minutes} min</option>
              ))}
            </select>
          ) : (
            <input
              type="number"
              step={15}
              min={15}
              value={durationMin}
              onChange={(e) => setDurationMin(Number(e.target.value))}
            />
          )}
        </div>
        <div className="form-row" style={{ margin: 0 }}>
          <label>Type</label>
          <select value={kindId} onChange={(e) => setKindId(e.target.value)}>
            <option value="">—</option>
            {kinds.map((k) => (
              <option key={k.ID} value={k.ID}>{k.Label}</option>
            ))}
          </select>
        </div>
      </div>

      <div className="form-row">
        <label>Ressources (formateur, véhicule, salle)</label>
        <div className="check-list">
          {resources.map((r) => (
            <label key={r.ID} className="check-item">
              <input
                type="checkbox"
                checked={resourceIds.has(r.ID)}
                onChange={() => toggle(resourceIds, setResourceIds, r.ID)}
              />
              {r.Label}
              {r.Kind === 'INSTRUCTOR' ? ' (formateur)' : r.Kind === 'ROOM' ? ' (salle)' : ''}
            </label>
          ))}
        </div>
      </div>

      <div className="form-row">
        <label>Élèves</label>
        <div className="check-list">
          {students.map((s) => (
            <label key={s.ID} className="check-item">
              <input
                type="checkbox"
                checked={studentIds.has(s.ID)}
                onChange={() => toggle(studentIds, setStudentIds, s.ID)}
              />
              {s.StudentName} ({s.Category})
            </label>
          ))}
        </div>
      </div>

      <div className="form-actions">
        <button
          className="primary"
          disabled={!time || durationMin <= 0}
          onClick={() => {
            const [h, m] = time.split(':').map(Number)
            const start = new Date(at)
            start.setHours(h, m, 0, 0)
            const lessonID = uuidv7()
            const ops: Operation[] = [
              {
                id: uuidv7(),
                kind: 'CREATE_LESSON',
                lesson_id: lessonID,
                slot: {
                  Start: start.toISOString(),
                  End: new Date(start.getTime() + durationMin * 60000).toISOString(),
                },
                ...(kindId ? { kind_ids: [kindId] } : {}),
              },
            ]
            for (const id of resourceIds) {
              ops.push({ id: uuidv7(), kind: 'ASSIGN_RESOURCE', lesson_id: lessonID, resource_id: id })
            }
            for (const id of studentIds) {
              ops.push({ id: uuidv7(), kind: 'PLACE_STUDENT', lesson_id: lessonID, enrollment_id: id })
            }
            onCreate(ops)
          }}
        >
          Poser la séance
        </button>
      </div>
      <p className="muted">
        La séance rejoint le brouillon avec ses conflits éventuels ; rien n'est appliqué avant
        l'enregistrement.
      </p>
    </div>
  )
}
