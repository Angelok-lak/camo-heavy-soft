package tasks

// F-20 first cut: the dashboard numbers. Every figure is computed here
// from the live state or the domain events (D-05) — no indicator is
// stored anywhere (C-05), so none can drift.

import (
	"net/http"
	"time"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
)

func (h *Handler) dashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/dashboard", h.dashboard)
}

type WeekPoint struct {
	WeekStart time.Time `json:"week_start"`
	Lessons   int       `json:"lessons"`
	Hours     float64   `json:"hours"`
}

type Breakdown struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type Dashboard struct {
	ActiveStudents   int         `json:"active_students"`
	LessonsThisWeek  int         `json:"lessons_this_week"`
	HoursThisMonth   float64     `json:"hours_this_month"`
	UpcomingExams    int         `json:"upcoming_exams"`
	CommittedUnits   int         `json:"committed_units"`
	GapCount         int         `json:"gap_count"`
	UnrecordedCount  int         `json:"unrecorded_count"`
	Weekly           []WeekPoint `json:"weekly"`
	FundingBreakdown []Breakdown `json:"funding_breakdown"`
	PermitBreakdown  []Breakdown `json:"permit_breakdown"`
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	ctx := r.Context()
	var d Dashboard

	err := h.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM enrollment WHERE school_id = $1 AND life_status = 'ACTIVE'),
		  (SELECT count(*) FROM lesson WHERE school_id = $1 AND status = 'PLANNED'
		     AND starts_at >= date_trunc('week', now())
		     AND starts_at < date_trunc('week', now()) + interval '7 days'),
		  COALESCE((SELECT sum(EXTRACT(EPOCH FROM l.ends_at - l.starts_at) / 3600.0)
		     FROM attendance a
		     JOIN lesson_assignment la ON la.id = a.lesson_assignment_id
		     JOIN lesson l ON l.id = la.lesson_id
		     WHERE a.school_id = $1 AND a.value = 'PRESENT'
		       AND l.starts_at >= date_trunc('month', now())), 0),
		  (SELECT count(*) FROM exam_session WHERE school_id = $1
		     AND status = 'SCHEDULED' AND starts_at > now()),
		  COALESCE((SELECT sum(b.committed_units) FROM exam_booking b
		     JOIN exam_session es ON es.id = b.exam_session_id
		     WHERE b.school_id = $1 AND b.status = 'COMMITTED'
		       AND es.starts_at > now()), 0),
		  (SELECT count(DISTINCT l.id) FROM lesson l
		     JOIN lesson_assignment la ON la.lesson_id = l.id
		     LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
		     WHERE l.school_id = $1 AND l.status = 'PLANNED'
		       AND l.ends_at < now() AND a.id IS NULL)`,
		id.SchoolID).Scan(&d.ActiveStudents, &d.LessonsThisWeek, &d.HoursThisMonth,
		&d.UpcomingExams, &d.CommittedUnits, &d.UnrecordedCount)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	err = h.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM enrollment_hours eh
		JOIN enrollment e ON e.id = eh.enrollment_id AND e.life_status = 'ACTIVE'
		CROSS JOIN LATERAL (VALUES
		    (eh.offroad_target_date, eh.projected_offroad_hours, eh.hours_before_offroad),
		    (eh.onroad_target_date, eh.projected_onroad_hours, eh.total_hours)
		) AS g(target_date, projected, threshold)
		WHERE eh.school_id = $1 AND g.target_date IS NOT NULL
		  AND g.projected IS NOT NULL AND g.projected < g.threshold`,
		id.SchoolID).Scan(&d.GapCount)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Eight weeks of activity: planned-or-done lessons and attended hours.
	rows, err := h.pool.Query(ctx, `
		WITH weeks AS (
		  SELECT generate_series(
		    date_trunc('week', now()) - interval '7 weeks',
		    date_trunc('week', now()), interval '1 week') AS w
		)
		SELECT w,
		  (SELECT count(*) FROM lesson l
		   WHERE l.school_id = $1 AND l.status = 'PLANNED'
		     AND l.starts_at >= w AND l.starts_at < w + interval '7 days'),
		  COALESCE((SELECT sum(EXTRACT(EPOCH FROM l.ends_at - l.starts_at) / 3600.0)
		   FROM attendance a
		   JOIN lesson_assignment la ON la.id = a.lesson_assignment_id
		   JOIN lesson l ON l.id = la.lesson_id
		   WHERE a.school_id = $1 AND a.value = 'PRESENT'
		     AND l.starts_at >= w AND l.starts_at < w + interval '7 days'), 0)
		FROM weeks ORDER BY w`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	d.Weekly = []WeekPoint{}
	for rows.Next() {
		var p WeekPoint
		if err := rows.Scan(&p.WeekStart, &p.Lessons, &p.Hours); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		d.Weekly = append(d.Weekly, p)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	d.FundingBreakdown, err = h.breakdown(r, `
		SELECT f.status::text, count(*)
		FROM funding f
		JOIN enrollment e ON e.id = f.enrollment_id AND e.life_status = 'ACTIVE'
		WHERE f.school_id = $1 GROUP BY f.status ORDER BY f.status`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	d.PermitBreakdown, err = h.breakdown(r, `
		SELECT o.label, count(*)
		FROM enrollment e
		JOIN objective o ON o.id = e.objective_id
		WHERE e.school_id = $1 AND e.life_status = 'ACTIVE'
		GROUP BY o.label ORDER BY o.label`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, d)
}

func (h *Handler) breakdown(r *http.Request, sql string, args ...any) ([]Breakdown, error) {
	rows, err := h.pool.Query(r.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Breakdown{}
	for rows.Next() {
		var b Breakdown
		if err := rows.Scan(&b.Label, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}