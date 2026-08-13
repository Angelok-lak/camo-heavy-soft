// F-33 — the seat request, exactly the centre's real table: one row per
// WEEK of the target month, three group columns (BE · Isolés C/D/C1/D1 ·
// Ensembles CE C1E DE D1E), counts in exam units. The numbers are FREE:
// the system only contributes the deadline, a suggestion and the names
// behind it.

import { useEffect, useState } from 'react'
import { api, type WeekLine } from './api'
import Modal from './Modal'

function monthKey(offset: number): string {
  const d = new Date()
  d.setDate(1)
  d.setMonth(d.getMonth() + offset)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

function monthLabel(key: string): string {
  const [y, m] = key.split('-').map(Number)
  return new Date(y, m - 1, 1).toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })
}

export default function SeatRequestModal({ onClose }: { onClose: () => void }) {
  const [month, setMonth] = useState(monthKey(2))
  const [deadline, setDeadline] = useState<string | null>(null)
  const [deadlinePassed, setDeadlinePassed] = useState(false)
  const [generatedAt, setGeneratedAt] = useState<string | null>(null)
  const [lines, setLines] = useState<WeekLine[]>([])
  const [why, setWhy] = useState<Map<string, string[]>>(new Map())
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)

  useEffect(() => {
    setDone(false)
    api
      .seatRequestSuggestion(month)
      .then((s) => {
        setDeadline(s.deadline)
        setDeadlinePassed(s.deadline_passed)
        setGeneratedAt(s.generated_at)
        setWhy(new Map(s.suggested.map((l) => [l.week, l.students ?? []])))
        // The already-sent numbers win over the suggestion.
        setLines(
          s.generated_lines?.length > 0
            ? s.generated_lines.map((l) => ({
                ...l,
                range: s.suggested.find((x) => x.week === l.week)?.range ?? l.range,
              }))
            : s.suggested,
        )
      })
      .catch((e) => setError(String(e)))
  }, [month])

  const update = (i: number, field: 'be' | 'isoles' | 'ensembles', value: number) => {
    setLines(lines.map((l, j) => (j === i ? { ...l, [field]: Math.max(0, value) } : l)))
  }

  const total = lines.reduce((s, l) => s + l.be + l.isoles + l.ensembles, 0)

  return (
    <Modal title="Demande de places d'examen" onClose={onClose} wide>
      <div className="form-row" style={{ maxWidth: 240 }}>
        <label>Mois demandé</label>
        <select value={month} onChange={(e) => setMonth(e.target.value)}>
          {[1, 2, 3].map((o) => (
            <option key={o} value={monthKey(o)}>{monthLabel(monthKey(o))}</option>
          ))}
        </select>
      </div>

      {deadline && !deadlinePassed && (
        <p className="muted">
          À envoyer avant le{' '}
          <strong>{new Date(deadline).toLocaleDateString('fr-FR', { day: 'numeric', month: 'long' })}</strong>
          {generatedAt &&
            ` · dernière génération le ${new Date(generatedAt).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}`}
        </p>
      )}
      {deadline && deadlinePassed && (
        <p className="amber-note" style={{ margin: '0 0 10px' }}>
          L'échéance d'envoi du{' '}
          {new Date(deadline).toLocaleDateString('fr-FR', { day: 'numeric', month: 'long' })} est
          dépassée pour ce mois — la demande ne peut plus être transmise.
          {generatedAt &&
            ` Une demande avait été générée le ${new Date(generatedAt).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}.`}
        </p>
      )}
      {error && <div className="error-box">{error}</div>}

      <table className="data seat-grid">
        <thead>
          <tr>
            <th>Semaine</th>
            <th>Dates</th>
            <th>BE</th>
            <th>Isolés C/D/C1/D1</th>
            <th>Ensembles CE C1E DE D1E</th>
          </tr>
        </thead>
        <tbody>
          {lines.map((l, i) => (
            <tr key={l.week}>
              <td><strong>{l.week}</strong></td>
              <td className="muted">{l.range}</td>
              {(['be', 'isoles', 'ensembles'] as const).map((f) => (
                <td key={f}>
                  <input
                    type="number"
                    min={0}
                    value={l[f]}
                    onChange={(e) => update(i, f, Number(e.target.value))}
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      <p className="muted" style={{ margin: '4px 0 10px' }}>Comptes en unités d'examen — les nombres sont libres.</p>

      {[...why.entries()].filter(([, s]) => s.length > 0).length > 0 && (
        <>
          <h3>Pourquoi la suggestion</h3>
          {[...why.entries()]
            .filter(([, s]) => s.length > 0)
            .map(([week, students]) => (
              <div key={week} className="detail-row">
                <span className="lead">{week}</span>
                <span className="meta" style={{ whiteSpace: 'normal' }}>{students.join(' · ')}</span>
              </div>
            ))}
          <p className="muted" style={{ marginTop: 6 }}>
            Une unité par plateau, deux par circulation, posées sur la semaine de l'échéance de
            chaque élève. La suggestion n'engage à rien.
          </p>
        </>
      )}

      <div className="form-actions" style={{ marginTop: 14 }}>
        <span className="muted" style={{ alignSelf: 'center' }}>{total} unité{total > 1 ? 's' : ''} au total</span>
        <div className="spacer" style={{ flex: 1 }} />
        <button
          className="primary"
          disabled={total === 0 || deadlinePassed}
          onClick={() =>
            api
              .generateSeatRequest(month, lines)
              .then(() => {
                setDone(true)
                setGeneratedAt(new Date().toISOString())
                window.open(`/api/seat-requests/${month}/file`, '_blank')
              })
              .catch((e) => setError(String(e)))
          }
        >
          Générer le fichier
        </button>
      </div>
      {done && (
        <p className="muted">
          Fichier téléchargé au format du tableau officiel — la demande est tracée, le rappel du
          tableau de bord s'éteint.
        </p>
      )}
    </Modal>
  )
}
