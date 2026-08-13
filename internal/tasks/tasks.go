// Package tasks is the F-36 seed: the to-do list. It owns NO storage —
// every line is derived from the state of the other features (C-06,
// D-08), which is exactly why adding a task kind later means adding a
// query here, nothing else.
package tasks

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
)

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tasks", h.list)
	h.dashboardRoutes(mux)
}

// Task is one line: what, why it matters, and where to act. The
// consequence is in clear text (RG-250), the action is named (RG-249).
type Task struct {
	Kind     string     `json:"kind"`
	Title    string     `json:"title"`
	Detail   string     `json:"detail"`
	Severity string     `json:"severity"`
	Due      *time.Time `json:"due"`
	// What the primary action needs.
	ResourceID       *uuid.UUID `json:"resource_id,omitempty"`
	UnavailabilityID *uuid.UUID `json:"unavailability_id,omitempty"`
	Count            int        `json:"count"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}

	out := []Task{}

	// 1. Ongoing unavailabilities: how many planned lessons still count
	// on the resource, and when the next one starts (the mockup's
	// "9 séances restantes · la plus proche demain 08h").
	rows, err := h.pool.Query(r.Context(), `
		SELECT u.id, u.resource_id, res.label, u.reason, u.starts_at, u.ends_at,
		       (SELECT count(*) FROM lesson_resource lr
		        JOIN lesson l ON l.id = lr.lesson_id
		        WHERE lr.resource_id = u.resource_id AND l.status = 'PLANNED'
		          AND l.ends_at > GREATEST(u.starts_at, now())
		          AND (u.ends_at IS NULL OR l.starts_at < u.ends_at)),
		       (SELECT min(l.starts_at) FROM lesson_resource lr
		        JOIN lesson l ON l.id = lr.lesson_id
		        WHERE lr.resource_id = u.resource_id AND l.status = 'PLANNED'
		          AND l.starts_at > now()
		          AND (u.ends_at IS NULL OR l.starts_at < u.ends_at))
		FROM unavailability u
		JOIN resource res ON res.id = u.resource_id
		WHERE u.school_id = $1 AND u.status = 'ONGOING' AND u.starts_at < now() + interval '30 days'
		ORDER BY u.starts_at`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var uID, resID uuid.UUID
		var label, reason string
		var starts time.Time
		var ends, next *time.Time
		var impacted int
		if err := rows.Scan(&uID, &resID, &label, &reason, &starts, &ends, &impacted, &next); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		t := Task{
			Kind: "unavailability", ResourceID: &resID, UnavailabilityID: &uID,
			Count: impacted, Due: next,
		}
		if ends == nil {
			days := int(time.Since(starts).Hours() / 24)
			t.Title = label + " sans date de retour"
			t.Detail = reason
			if days > 0 {
				t.Detail += " · indisponible depuis " + itoa(days) + " jour" + plural(days)
			}
			t.Severity = "WARNING"
		} else {
			t.Title = label + " indisponible"
			t.Detail = reason + " · jusqu'au " + ends.Format("02/01")
			t.Severity = "WARNING"
		}
		if impacted > 0 {
			t.Detail += " · " + itoa(impacted) + " séance" + plural(impacted) + " restante" + plural(impacted)
			if next != nil {
				t.Detail += " · la plus proche " + relativeDay(*next)
			}
			t.Severity = "CRITICAL"
		} else {
			t.Detail += " · aucune séance impactée"
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 2. Past lessons still unrecorded (feeds from F-12).
	var unrecorded int
	var oldest *time.Time
	err = h.pool.QueryRow(r.Context(), `
		SELECT count(DISTINCT l.id), min(l.starts_at)
		FROM lesson l
		JOIN lesson_assignment la ON la.lesson_id = l.id
		LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
		WHERE l.school_id = $1 AND l.status = 'PLANNED' AND l.ends_at < now()
		  AND a.id IS NULL`, id.SchoolID).Scan(&unrecorded, &oldest)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if unrecorded > 0 {
		out = append(out, Task{
			Kind:  "unrecorded",
			Title: itoa(unrecorded) + " séance" + plural(unrecorded) + " sans présences",
			Detail: "Les compteurs d'heures restent faux tant que les présences ne sont pas saisies" +
				" · la plus ancienne du " + oldest.Format("02/01"),
			Severity: "WARNING", Count: unrecorded, Due: oldest,
		})
	}

	// 2b. Exam results still unrecorded (RG-112): committed bookings on a
	// past session — "non renseignée" is derived, this is its reminder.
	var unresulted int
	var oldestExam *time.Time
	err = h.pool.QueryRow(r.Context(), `
		SELECT count(*), min(es.starts_at)
		FROM exam_booking b
		JOIN exam_session es ON es.id = b.exam_session_id
		WHERE b.school_id = $1 AND b.status = 'COMMITTED'
		  AND es.starts_at < now() AND es.status <> 'CANCELLED'`,
		id.SchoolID).Scan(&unresulted, &oldestExam)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if unresulted > 0 {
		out = append(out, Task{
			Kind:  "exam_results",
			Title: itoa(unresulted) + " résultat" + plural(unresulted) + " d'examen non renseigné" + plural(unresulted),
			Detail: "Les crédits de la session restent engagés tant que le résultat n'est pas saisi" +
				" · session du " + oldestExam.Format("02/01"),
			Severity: "WARNING", Count: unresulted, Due: oldestExam,
		})
	}

	// 3. Hour gaps (feeds from F-13): who is late against a target date.
	var gaps int
	var nearest *time.Time
	err = h.pool.QueryRow(r.Context(), `
		SELECT count(*), min(g.target_date)
		FROM enrollment_hours eh
		JOIN enrollment e ON e.id = eh.enrollment_id AND e.life_status = 'ACTIVE'
		CROSS JOIN LATERAL (VALUES
		    (eh.offroad_target_date, eh.projected_offroad_hours, eh.hours_before_offroad),
		    (eh.onroad_target_date, eh.projected_onroad_hours, eh.total_hours)
		) AS g(target_date, projected, threshold)
		WHERE eh.school_id = $1 AND g.target_date IS NOT NULL
		  AND g.projected IS NOT NULL AND g.projected < g.threshold`,
		id.SchoolID).Scan(&gaps, &nearest)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if gaps > 0 {
		out = append(out, Task{
			Kind:  "gaps",
			Title: itoa(gaps) + " écart" + plural(gaps) + " d'heures sur échéance",
			Detail: "Des élèves n'atteindront pas leur seuil au rythme prévu" +
				" · première échéance le " + nearest.Format("02/01"),
			Severity: "WARNING", Count: gaps, Due: nearest,
		})
	}

	// 4. Seat request (F-33): the reminder that outlives the incident it
	// was born from. Appears 10 days before the deadline, critical the
	// last 2, gone once generated.
	var deadlineDay int
	if err := h.pool.QueryRow(r.Context(),
		`SELECT seat_request_deadline_day FROM school WHERE id = $1`, id.SchoolID,
	).Scan(&deadlineDay); err == nil {
		now := time.Now()
		target := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 2, 0)
		deadline := time.Date(now.Year(), now.Month(), deadlineDay, 23, 59, 0, 0, now.Location())
		var generated bool
		_ = h.pool.QueryRow(r.Context(), `
			SELECT EXISTS (SELECT 1 FROM seat_request
			               WHERE school_id = $1 AND target_month = $2)`,
			id.SchoolID, target).Scan(&generated)
		daysLeft := int(time.Until(deadline).Hours() / 24)
		if !generated && daysLeft <= 10 && daysLeft >= -3 {
			sev := "WARNING"
			detail := "Pour " + frMonth(target) + " · à envoyer avant le " + deadline.Format("02/01")
			if daysLeft <= 2 {
				sev = "CRITICAL"
				if daysLeft <= 0 {
					detail = "Pour " + frMonth(target) + " · à envoyer aujourd'hui"
				}
			}
			out = append(out, Task{
				Kind: "seat_request", Title: "Demande de places à envoyer",
				Detail: detail, Severity: sev, Due: &deadline,
			})
		}
	}

	reply(w, http.StatusOK, out)
}

func frMonth(t time.Time) string {
	months := []string{"janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	return months[int(t.Month())-1]
}

func itoa(n int) string {
	if n < 0 {
		return "0"
	}
	digits := []byte{}
	if n == 0 {
		return "0"
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func plural(n int) string {
	if n > 1 {
		return "s"
	}
	return ""
}

func relativeDay(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	switch int(day.Sub(today).Hours() / 24) {
	case 0:
		return "aujourd'hui " + t.Format("15h04")
	case 1:
		return "demain " + t.Format("15h04")
	default:
		return "le " + t.Format("02/01 à 15h04")
	}
}

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}