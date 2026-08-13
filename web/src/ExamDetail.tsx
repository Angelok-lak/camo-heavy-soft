// The exam session file, per the validated mockup: header with the real
// immobilisation, the units bar, committed candidates (overrides traced
// in clear text), proposed candidates ranked with their missing
// prerequisites NAMED and excluding no one. Shared by the Exams tab and
// the planner's exam blocks.

import { useCallback, useEffect, useState } from 'react'
import { api, type Communication, type ExamSessionDetail } from './api'
import Modal from './Modal'

const TEST_LABELS = { OFFROAD: 'Plateau', ONROAD: 'Circulation' } as const
const RESULT_LABELS: Record<string, string> = {
  PASSED: 'Réussi',
  FAILED: 'Échoué',
  ABSENT: 'Absent',
}

export default function ExamDetail({
  sessionId,
  canManage,
  onClose,
}: {
  sessionId: string
  canManage: boolean
  onClose: () => void
}) {
  const [d, setD] = useState<ExamSessionDetail | null>(null)
  const [convocations, setConvocations] = useState<Communication[] | null>(null)
  const [smtpOK, setSmtpOK] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(() => {
    api.examSessionDetail(sessionId).then(setD).catch((e) => setError(String(e)))
  }, [sessionId])
  useEffect(reload, [reload])

  if (!d) {
    return (
      <Modal title="Session d'examen" onClose={onClose} wide>
        {error ? <div className="error-box">{error}</div> : <p className="muted">Chargement…</p>}
      </Modal>
    )
  }

  const s = d.session
  const start = new Date(s.starts_at)
  const end = new Date(s.ends_at)
  const grabbed = s.travel_minutes
    ? ` · ressources prises de ${hm(new Date(start.getTime() - s.travel_minutes * 60000))} à ${hm(
        new Date(end.getTime() + s.travel_minutes * 60000),
      )}`
    : ''
  const engaged = d.credits.committed + d.credits.spent + d.credits.forfeited
  const pct =
    d.credits.allowance && d.credits.allowance > 0
      ? Math.min(100, Math.round((engaged / d.credits.allowance) * 100))
      : 0

  const title = `Session du ${start.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })} · ${s.place_label.replace("Centre d'examen de ", '')}`

  return (
    <Modal title={title} onClose={onClose} wide>
      <p className="muted" style={{ marginTop: 0 }}>
        {hm(start)} → {hm(end)} · {d.resource_labels.join(' · ') || 'aucune ressource'}
        {s.travel_minutes ? ` · trajet ${s.travel_minutes} min A/R` : ''}
        {grabbed}
      </p>

      {error && <div className="error-box">{error}</div>}

      {d.credits.allowance !== null && (
        <div className="units-block">
          <div className="units-head">
            <span className="stat-num">{d.credits.committed}</span>
            <span>
              unité{d.credits.committed > 1 ? 's' : ''} engagée{d.credits.committed > 1 ? 's' : ''} sur{' '}
              {d.credits.allowance}
              {d.credits.spent > 0 && ` · ${d.credits.spent} consommée${d.credits.spent > 1 ? 's' : ''}`}
              {d.credits.forfeited > 0 && ` · ${d.credits.forfeited} perdue${d.credits.forfeited > 1 ? 's' : ''}`}
              {!s.past && ` · ${d.credits.remaining} restante${(d.credits.remaining ?? 0) > 1 ? 's' : ''}`}
            </span>
          </div>
          <div className="gauge-track">
            <div
              className={pct >= 100 ? 'gauge-fill short' : 'gauge-fill accent'}
              style={{ width: pct + '%' }}
            />
          </div>
          <p className="muted" style={{ margin: '6px 0 0' }}>
            {s.past
              ? 'Plateau 1 unité · circulation 2 unités · session passée, les unités non engagées sont perdues'
              : `Plateau 1 unité · circulation 2 unités · les unités non engagées le ${start.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })} seront perdues`}
          </p>
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'baseline', gap: 12 }}>
        <h3 style={{ flex: 1 }}>
          {d.bookings.length} candidat{d.bookings.length > 1 ? 's' : ''} engagé{d.bookings.length > 1 ? 's' : ''}
        </h3>
        {canManage && d.bookings.length > 0 && !s.past && !s.cancelled && (
          <button
            className="btn-sm primary"
            onClick={() =>
              api
                .convokeSession(sessionId)
                .then((r) => {
                  setConvocations(r.communications)
                  setSmtpOK(r.smtp_configured)
                })
                .catch((e) => setError(String(e)))
            }
          >
            Convoquer tous
          </button>
        )}
      </div>

      {convocations && (
        <div className="convocation-box">
          {!smtpOK && convocations.some((c) => c.status === 'SIMULATED') && (
            <p className="amber-note" style={{ margin: '0 0 8px' }}>
              Envoi email simulé — les messages sont tracés et partiront réellement une fois le
              SMTP configuré.
            </p>
          )}
          {convocations.map((c) => (
            <div key={c.id ?? c.recipient_label} className="detail-row">
              <span className="req-main">
                <span className="req-label">{c.recipient_label}</span>
                <span className="req-meta">
                  {c.channel === 'WHATSAPP' ? 'WhatsApp' : `Email · ${c.recipient_address}`}
                </span>
              </span>
              {c.channel === 'WHATSAPP' && c.status === 'PREPARED' && c.whatsapp_link ? (
                <button
                  className="btn-sm wa"
                  onClick={() => {
                    window.open(c.whatsapp_link, '_blank')
                    api.markCommunicationSent(c.id).then(() =>
                      setConvocations(
                        convocations.map((x) => (x.id === c.id ? { ...x, status: 'SENT' } : x)),
                      ),
                    ).catch(() => {})
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
                  {c.status === 'SENT' ? 'Envoyée' : c.status === 'SIMULATED' ? 'Simulée' : c.status === 'FAILED' ? 'Échec' : 'Préparée'}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
      {d.bookings.length === 0 && <p className="muted">Aucun candidat engagé pour l'instant.</p>}
      {d.bookings.map((b) => (
        <div key={b.id} className="detail-row">
          <span className="req-main">
            <span className="req-label">{b.student_name}</span>
            <span className="req-meta">
              Permis {b.objective} · {b.consumed_hours} h{b.total_hours > 0 ? ` sur ${b.total_hours}` : ''}
            </span>
            {b.override_note && (
              <span className="override-note">
                {b.override_note} · forcé par {b.booked_by} le{' '}
                {new Date(b.booked_at).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}
              </span>
            )}
          </span>
          <span className="muted" style={{ whiteSpace: 'nowrap' }}>
            {TEST_LABELS[b.test_kind]} · {b.units} u.
          </span>
          {s.past && b.status !== 'WITHDRAWN' && (
            canManage && !s.cancelled ? (
              <span className="req-actions">
                {(['PASSED', 'FAILED', 'ABSENT'] as const).map((st) => (
                  <button
                    key={st}
                    className={b.status === st ? 'btn-sm primary' : 'btn-sm'}
                    onClick={() =>
                      api.enterExamResult(b.id, st).then(reload).catch((e) => setError(String(e)))
                    }
                  >
                    {RESULT_LABELS[st]}
                  </button>
                ))}
                {b.unrecorded && <span className="status-pill neutral">Non renseigné</span>}
              </span>
            ) : (
              <span
                className={
                  b.status === 'PASSED'
                    ? 'status-pill ok'
                    : b.status === 'FAILED' || b.status === 'ABSENT'
                      ? 'status-pill bad'
                      : 'status-pill neutral'
                }
              >
                {RESULT_LABELS[b.status] ?? 'Non renseigné'}
              </span>
            )
          )}
          {b.status === 'WITHDRAWN' && <span className="status-pill neutral">Retiré</span>}
          {canManage && !s.past && !s.cancelled && b.status !== 'WITHDRAWN' && (
            <button
              className="btn-sm"
              onClick={() => api.withdrawBooking(b.id).then(reload).catch((e) => setError(String(e)))}
            >
              Retirer
            </button>
          )}
        </div>
      ))}

      <h3>
        Candidats proposés · {d.proposed_total} sur {d.active_total} parcours actifs
      </h3>
      {d.proposed.map((c) => (
        <div key={c.enrollment_id} className="detail-row">
          <span className="req-main">
            <span className="req-label">{c.student_name}</span>
            <span className="req-meta">
              Permis {c.objective}
              {c.projected_hours !== null &&
                ` · sera à ${c.projected_hours} h sur ${c.threshold_hours}`}
              {c.days_left !== null && ` · échéance dans ${c.days_left} j`}
            </span>
            {c.offroad_passed_at && (
              <span className="req-meta">
                Plateau obtenu le{' '}
                {new Date(c.offroad_passed_at).toLocaleDateString('fr-FR', {
                  day: 'numeric',
                  month: 'long',
                })}
                {plateauExpiry(c.offroad_expires_at)}
                {c.presentations > 0 && ` · ${ordinal(c.presentations + 1)} présentation`}
              </span>
            )}
            {c.missing_requirements.length > 0 && (
              <span className="amber-note">{c.missing_requirements.join(', ')} non validé</span>
            )}
          </span>
          {canManage && !s.past && !s.cancelled && (
            <span className="req-actions">
              <button
                className="btn-sm primary"
                onClick={() =>
                  api.bookExam(sessionId, c.enrollment_id, 'OFFROAD').then(reload).catch((e) => setError(String(e)))
                }
              >
                Plateau · 1 u.
              </button>
              <button
                className="btn-sm primary"
                onClick={() =>
                  api.bookExam(sessionId, c.enrollment_id, 'ONROAD').then(reload).catch((e) => setError(String(e)))
                }
              >
                Circulation · 2 u.
              </button>
            </span>
          )}
        </div>
      ))}
      {d.proposed_total > d.proposed.length && (
        <p className="muted">+ {d.proposed_total - d.proposed.length} autres candidats proposés</p>
      )}
    </Modal>
  )
}

function hm(d: Date): string {
  return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
}

function plateauExpiry(expiresAt: string | null): string {
  if (!expiresAt) return ''
  const days = Math.floor((new Date(expiresAt).getTime() - Date.now()) / 86400000)
  if (days < 0) return ' · expiré'
  if (days <= 60) return ` · expire dans ${days} j`
  return ''
}

function ordinal(n: number): string {
  return n === 1 ? '1re' : `${n}e`
}
