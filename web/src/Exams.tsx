// F-06, first cut: the exam session calendar. Date, place, committed
// resources, entered allowance. The session shows on the planner with its
// travel immobilisation; it is never edited from there (RG-152) — this
// screen is its only write path.

import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  api,
  type ExamPlace,
  type ExamSessionRow,
  type ResourceRow,
  type UserContext,
} from './api'
import Modal from './Modal'
import ExamDetail from './ExamDetail'
import SeatRequestModal from './SeatRequestModal'

export default function Exams({ user }: { user: UserContext }) {
  const [rows, setRows] = useState<ExamSessionRow[]>([])
  const [resources, setResources] = useState<ResourceRow[]>([])
  const [places, setPlaces] = useState<ExamPlace[]>([])
  const [showPast, setShowPast] = useState(false)
  const [creating, setCreating] = useState(false)
  const [cancelling, setCancelling] = useState<ExamSessionRow | null>(null)
  const [detailID, setDetailID] = useState<string | null>(null)
  const [seatModal, setSeatModal] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const canManage = user.permissions.edit_planning

  const reload = useCallback(() => {
    api.examSessions().then(setRows).catch((e) => setError(String(e)))
  }, [])
  useEffect(reload, [reload])
  useEffect(() => {
    api.resources().then(setResources).catch(() => {})
    api.examPlaces().then(setPlaces).catch(() => {})
  }, [])

  const labelOf = useMemo(() => {
    const m = new Map(resources.map((r) => [r.id, r.label]))
    return (id: string) => m.get(id) ?? '?'
  }, [resources])

  const visible = rows.filter((r) => showPast || !r.past)

  const fmt = (iso: string) =>
    new Date(iso).toLocaleString('fr-FR', {
      weekday: 'long', day: 'numeric', month: 'long',
      hour: '2-digit', minute: '2-digit',
    })

  return (
    <div className="page">
      <div className="page-main">
        <div className="toolbar">
          <h2>Sessions d'examen</h2>
          <button className={showPast ? 'chip on' : 'chip'} onClick={() => setShowPast(!showPast)}>
            Passées
          </button>
          <div className="spacer" />
          {canManage && (
            <button onClick={() => setSeatModal(true)}>Demande de places</button>
          )}
          {canManage && (
            <button className="primary" onClick={() => setCreating(true)}>
              Créer une session
            </button>
          )}
        </div>

        {error && <div className="error-box">{error}</div>}

        <table className="data">
          <thead>
            <tr>
              <th>Date</th>
              <th>Lieu</th>
              <th>Trajet</th>
              <th>Ressources immobilisées</th>
              <th>Enveloppe</th>
              <th>État</th>
              {canManage && <th />}
            </tr>
          </thead>
          <tbody>
            {visible.map((s) => (
              <tr
                key={s.id}
                className={s.cancelled ? 'archived' : 'clickable'}
                onClick={() => !s.cancelled && setDetailID(s.id)}
              >
                <td>
                  {fmt(s.starts_at)} → {new Date(s.ends_at).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}
                </td>
                <td>{s.place_label}</td>
                <td className="muted">
                  {s.travel_minutes > 0
                    ? `${s.travel_minutes} min — mobilise dès ${new Date(
                        new Date(s.starts_at).getTime() - s.travel_minutes * 60000,
                      ).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}`
                    : '—'}
                </td>
                <td>{s.resources.map(labelOf).join(', ') || <span className="muted">aucune</span>}</td>
                <td>{s.credit_allowance ?? <span className="muted">—</span>}</td>
                <td>
                  {s.cancelled ? `Annulée — ${s.cancel_reason}` : s.past ? 'Passée' : 'À venir'}
                </td>
                {canManage && (
                  <td className="row-actions" onClick={(e) => e.stopPropagation()}>
                    {!s.cancelled && <button onClick={() => setDetailID(s.id)}>Candidats</button>}
                    {!s.cancelled && !s.past && (
                      <button onClick={() => setCancelling(s)}>Annuler</button>
                    )}
                  </td>
                )}
              </tr>
            ))}
            {visible.length === 0 && (
              <tr><td colSpan={7} className="muted">Aucune session à venir. Créez-en une pour la voir apparaître sur le planning.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {creating && (
        <Modal title="Créer une session d'examen" onClose={() => setCreating(false)} wide>
          <CreateForm
            places={places}
            resources={resources}
            onDone={() => {
              setCreating(false)
              reload()
            }}
          />
        </Modal>
      )}
      {seatModal && <SeatRequestModal onClose={() => setSeatModal(false)} />}
      {detailID && (
        <ExamDetail sessionId={detailID} canManage={canManage} onClose={() => setDetailID(null)} />
      )}
      {cancelling && (
        <Modal title={`Annuler — ${cancelling.place_label}`} onClose={() => setCancelling(null)}>
          <CancelForm
            session={cancelling}
            onDone={() => {
              setCancelling(null)
              reload()
            }}
          />
        </Modal>
      )}
    </div>
  )
}

function CreateForm({
  places,
  resources,
  onDone,
}: {
  places: ExamPlace[]
  resources: ResourceRow[]
  onDone: () => void
}) {
  const [placeID, setPlaceID] = useState('')
  const [date, setDate] = useState('')
  const [start, setStart] = useState('08:30')
  const [end, setEnd] = useState('17:00')
  const [allowance, setAllowance] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [error, setError] = useState<string | null>(null)

  const place = places.find((p) => p.id === placeID)
  const committable = resources.filter(
    (r) => r.status === 'ACTIVE' && (r.kind === 'VEHICLE' || r.kind === 'INSTRUCTOR'),
  )

  const toggle = (id: string) => {
    const next = new Set(selected)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    setSelected(next)
  }

  return (
    <div>
      <div className="form-row">
        <label>Lieu d'examen</label>
        <select value={placeID} onChange={(e) => setPlaceID(e.target.value)} autoFocus>
          <option value="">—</option>
          {places.filter((p) => p.active).map((p) => (
            <option key={p.id} value={p.id}>{p.label}</option>
          ))}
        </select>
        {place && place.travel_minutes > 0 && (
          <span className="muted">
            Trajet {place.travel_minutes} min : les ressources seront mobilisées {place.travel_minutes} min
            avant et après la session.
          </span>
        )}
      </div>
      <div className="form-row">
        <label>Date</label>
        <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
      </div>
      <div className="form-row" style={{ flexDirection: 'row', gap: 12 }}>
        <div className="form-row" style={{ margin: 0 }}>
          <label>Début</label>
          <input type="time" value={start} onChange={(e) => setStart(e.target.value)} />
        </div>
        <div className="form-row" style={{ margin: 0 }}>
          <label>Fin</label>
          <input type="time" value={end} onChange={(e) => setEnd(e.target.value)} />
        </div>
        <div className="form-row" style={{ margin: 0 }}>
          <label>Enveloppe de places</label>
          <input type="number" min={0} value={allowance} onChange={(e) => setAllowance(e.target.value)} />
        </div>
      </div>
      <div className="form-row">
        <label>Ressources immobilisées (véhicules et formateurs)</label>
        <div className="check-list">
          {committable.map((r) => (
            <label key={r.id} className="check-item">
              <input type="checkbox" checked={selected.has(r.id)} onChange={() => toggle(r.id)} />
              {r.label} {r.kind === 'INSTRUCTOR' ? '(formateur)' : `(${r.categories.join(', ')})`}
            </label>
          ))}
        </div>
      </div>
      {error && <div className="error-box">{error}</div>}
      <div className="form-actions">
        <button
          className="primary"
          disabled={!placeID || !date || !start || !end || end <= start}
          onClick={() =>
            api
              .createExamSession({
                place_id: placeID,
                starts_at: new Date(`${date}T${start}`).toISOString(),
                ends_at: new Date(`${date}T${end}`).toISOString(),
                ...(allowance ? { credit_allowance: Number(allowance) } : {}),
                resources: [...selected],
              })
              .then(onDone)
              .catch((e) => setError(String(e)))
          }
        >
          Créer la session
        </button>
      </div>
    </div>
  )
}

function CancelForm({ session, onDone }: { session: ExamSessionRow; onDone: () => void }) {
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | null>(null)
  return (
    <div>
      <p className="muted">
        Les ressources seront libérées. Les séances et candidats concernés restent à traiter dans le
        planning.
      </p>
      <div className="form-row">
        <label>Motif</label>
        <input value={reason} onChange={(e) => setReason(e.target.value)} autoFocus />
      </div>
      {error && <div className="error-box">{error}</div>}
      <div className="form-actions">
        <button
          className="danger"
          disabled={!reason.trim()}
          onClick={() =>
            api.cancelExamSession(session.id, reason.trim()).then(onDone).catch((e) => setError(String(e)))
          }
        >
          Annuler la session
        </button>
      </div>
    </div>
  )
}
