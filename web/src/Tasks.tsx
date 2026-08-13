// F-20 + F-36 seed — the dashboard: the global numbers first (all
// computed server-side from live state, C-05/C-26), one activity chart,
// the breakdowns, then the to-do list. Every figure and every line is
// derived — nothing here is stored, nothing can drift.

import { useCallback, useEffect, useState } from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts'
import { api, type ImpactedLesson, type Task, type UserContext } from './api'
import Modal from './Modal'
import SeatRequestModal from './SeatRequestModal'

type DashboardData = Awaited<ReturnType<typeof api.dashboard>>

const FUNDING_LABELS: Record<string, string> = {
  DRAFT: 'À monter',
  SUBMITTED: 'Déposé',
  APPROVED: 'Accordé',
  SETTLED: 'Soldé',
  REJECTED: 'Refusé',
}

export default function Tasks({
  user,
  onGo,
}: {
  user: UserContext
  onGo: (tab: 'planning' | 'attendance' | 'students' | 'resources') => void
}) {
  const [tasks, setTasks] = useState<Task[]>([])
  const [data, setData] = useState<DashboardData | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [restored, setRestored] = useState<{ title: string; kept: ImpactedLesson[] } | null>(null)
  const [seatModal, setSeatModal] = useState(false)
  const canManage = user.permissions.manage_resources

  const reload = useCallback(() => {
    api.tasks().then(setTasks).catch((e) => setError(String(e)))
    api.dashboard().then(setData).catch(() => {})
  }, [])
  useEffect(reload, [reload])

  const weekly = (data?.weekly ?? []).map((w) => ({
    ...w,
    label: new Date(w.week_start).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' }),
    hours: Math.round(w.hours),
  }))
  const maxPermit = Math.max(1, ...(data?.permit_breakdown ?? []).map((b) => b.count))
  const maxFunding = Math.max(1, ...(data?.funding_breakdown ?? []).map((b) => b.count))

  return (
    <div className="page">
      <div className="page-main">
        <div style={{ maxWidth: 980, margin: '0 auto' }}>
          <header className="panel-head" style={{ marginTop: 8 }}>
            <h2>Tableau de bord</h2>
            <p className="muted">
              L'état du centre en direct — chaque chiffre est recalculé à la lecture, rien n'est
              stocké.
            </p>
          </header>

          {error && <div className="error-box">{error}</div>}

          {data && (
            <>
              <div className="stat-row">
                <div className="stat">
                  <span className="stat-num">{data.active_students}</span>
                  <span className="stat-label">élèves en formation</span>
                </div>
                <div className="stat">
                  <span className="stat-num">{data.lessons_this_week}</span>
                  <span className="stat-label">séances cette semaine</span>
                </div>
                <div className="stat">
                  <span className="stat-num">
                    {Math.round(data.hours_this_month)}
                    <small> h</small>
                  </span>
                  <span className="stat-label">réalisées ce mois</span>
                </div>
                <div className="stat">
                  <span className="stat-num">{data.upcoming_exams}</span>
                  <span className="stat-label">
                    session{data.upcoming_exams > 1 ? 's' : ''} d'examen à venir
                    {data.committed_units > 0 ? ` · ${data.committed_units} u. engagées` : ''}
                  </span>
                </div>
              </div>

              <div className="dash-grid">
                <div className="dash-card">
                  <h3 className="eyebrow">Heures réalisées par semaine</h3>
                  <ResponsiveContainer width="100%" height={180}>
                    <BarChart data={weekly} margin={{ top: 8, right: 4, left: -22, bottom: 0 }}>
                      <CartesianGrid vertical={false} stroke="var(--line)" />
                      <XAxis
                        dataKey="label"
                        tickLine={false}
                        axisLine={false}
                        tick={{ fontSize: 11, fill: 'var(--muted)' }}
                      />
                      <YAxis
                        tickLine={false}
                        axisLine={false}
                        tick={{ fontSize: 11, fill: 'var(--muted)' }}
                        allowDecimals={false}
                      />
                      <Tooltip
                        cursor={{ fill: 'rgba(76, 111, 231, 0.06)' }}
                        formatter={(v) => [`${v} h`, 'réalisées']}
                        labelFormatter={(l) => `Semaine du ${l}`}
                      />
                      <Bar dataKey="hours" fill="var(--accent)" radius={[4, 4, 0, 0]} maxBarSize={26} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>

                <div className="dash-card">
                  <h3 className="eyebrow">Parcours par permis</h3>
                  {data.permit_breakdown.map((b) => (
                    <div key={b.label} className="mini-bar-row">
                      <span className="mini-bar-label">Permis {b.label}</span>
                      <span className="mini-bar-track">
                        <span
                          className="mini-bar-fill"
                          style={{ width: `${(b.count / maxPermit) * 100}%` }}
                        />
                      </span>
                      <span className="mini-bar-val">{b.count}</span>
                    </div>
                  ))}

                  <h3 className="eyebrow">Financements</h3>
                  {data.funding_breakdown.map((b) => (
                    <div key={b.label} className="mini-bar-row">
                      <span className="mini-bar-label">{FUNDING_LABELS[b.label] ?? b.label}</span>
                      <span className="mini-bar-track">
                        <span
                          className="mini-bar-fill soft"
                          style={{ width: `${(b.count / maxFunding) * 100}%` }}
                        />
                      </span>
                      <span className="mini-bar-val">{b.count}</span>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}

          <header className="panel-head" style={{ marginTop: 26 }}>
            <h2>
              À traiter
              {tasks.length > 0 && <span className="count-pill">{tasks.length}</span>}
            </h2>
            <p className="muted">
              Chaque ligne dit sa conséquence et porte son action — elle disparaît d'elle-même une
              fois réglée.
            </p>
          </header>

          {tasks.length === 0 && !error && <p className="muted">Rien à traiter — tout est en ordre. ✓</p>}

          {tasks.map((t, i) => (
            <div key={i} className="task-card">
              <span className={`health-dot ${t.severity === 'CRITICAL' ? 'red' : 'amber'}`} />
              <div className="task-main">
                <strong>{t.title}</strong>
                <span className="muted">{t.detail}</span>
              </div>
              <div className="row-actions">
                {t.kind === 'unavailability' && canManage && t.unavailability_id && (
                  <button
                    className="primary"
                    onClick={() =>
                      api
                        .restoreUnavailability(t.unavailability_id!)
                        .then((r) => {
                          setRestored({ title: t.title, kept: r.kept_lessons })
                          reload()
                        })
                        .catch((e) => setError(String(e)))
                    }
                  >
                    Remise en service
                  </button>
                )}
                {t.kind === 'unavailability' && (
                  <button onClick={() => onGo('planning')}>Voir le planning</button>
                )}
                {t.kind === 'unrecorded' && (
                  <button className="primary" onClick={() => onGo('attendance')}>
                    Renseigner les présences
                  </button>
                )}
                {t.kind === 'gaps' && (
                  <button className="primary" onClick={() => onGo('students')}>
                    Voir les élèves en écart
                  </button>
                )}
                {t.kind === 'seat_request' && canManage && (
                  <button className="primary" onClick={() => setSeatModal(true)}>
                    Préparer la demande
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      {seatModal && (
        <SeatRequestModal
          onClose={() => {
            setSeatModal(false)
            reload()
          }}
        />
      )}

      {restored && (
        <Modal title="Remise en service" onClose={() => setRestored(null)}>
          <p>
            {restored.title.split(' sans date')[0].split(' indisponible')[0]} est de retour en
            service.
          </p>
          {restored.kept.length === 0 ? (
            <p className="muted">Aucune séance n'attendait dessus.</p>
          ) : (
            <>
              <p className="muted">
                {restored.kept.length} séance(s) étaient restées assignées pendant
                l'indisponibilité — elles redeviennent valides telles quelles :
              </p>
              {restored.kept.map((l) => (
                <div key={l.id} className="detail-row">
                  <span className="lead">
                    {new Date(l.starts_at).toLocaleString('fr-FR', {
                      weekday: 'short', day: 'numeric', month: 'short',
                      hour: '2-digit', minute: '2-digit',
                    })}
                  </span>
                  <span className="meta">{l.students} élève(s)</span>
                </div>
              ))}
            </>
          )}
          <div className="form-actions">
            <button className="primary" onClick={() => setRestored(null)}>Fermer</button>
          </div>
        </Modal>
      )}
    </div>
  )
}
