package enrollment

// F-13, slice 2: the three counters, the two projections, the gap
// alerts. Everything reads the enrollment_hours VIEW (migration 003) —
// counters are queries, never columns (C-05), and every consumer goes
// through the same implementation.

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
)

func (h *Handler) hoursRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/enrollments/{id}", h.read)
	mux.HandleFunc("GET /api/enrollments/gaps", h.gaps)
}

type Hours struct {
	Attended  float64  `json:"attended"`
	Excused   float64  `json:"excused"`
	Unexcused float64  `json:"unexcused"`
	Consumed  float64  `json:"consumed"`
	// nil when the matching target date is not set (nothing to project to).
	ProjectedOffroad *float64 `json:"projected_offroad"`
	ProjectedOnroad  *float64 `json:"projected_onroad"`
}

// Alert is derived at read time, joined to its object (C-06, C-29).
// Severity stays WARNING in slice 2: the escalation thresholds belong to
// the settings work of a later slice (RG-114).
type Alert struct {
	Kind     string  `json:"kind"`
	Severity string  `json:"severity"`
	Target   string  `json:"target"` // OFFROAD / ONROAD
	GapHours float64 `json:"gap_hours"`
	DaysLeft int     `json:"days_left"`
	Message  string  `json:"message"`
}

type EnrollmentView struct {
	ID                 uuid.UUID  `json:"id"`
	StudentName        string     `json:"student_name"`
	Objective          string     `json:"objective"`
	LifeStatus         string     `json:"life_status"`
	HoursBeforeOffroad float64    `json:"hours_before_offroad"`
	TotalHours         float64    `json:"total_hours"`
	OffroadTargetDate  *time.Time `json:"offroad_target_date"`
	OnroadTargetDate   *time.Time `json:"onroad_target_date"`
	Hours              Hours      `json:"hours"`
	// Off-road pass and its DERIVED expiry (RG-25): result date + the
	// school's validity parameter — recomputed at read, never stored.
	OffroadPassedAt  *time.Time `json:"offroad_passed_at"`
	OffroadExpiresAt *time.Time `json:"offroad_expires_at"`
	Alerts           []Alert    `json:"alerts"`
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	enrollmentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown enrollment")
		return
	}

	var v EnrollmentView
	err = h.pool.QueryRow(r.Context(), `
		SELECT e.id, p.last_name || ' ' || p.first_names, o.label, e.life_status,
		       eh.hours_before_offroad, eh.total_hours,
		       eh.offroad_target_date, eh.onroad_target_date,
		       eh.attended_hours, eh.excused_hours, eh.unexcused_hours, eh.consumed_hours,
		       eh.projected_offroad_hours, eh.projected_onroad_hours
		FROM enrollment_hours eh
		JOIN enrollment e ON e.id = eh.enrollment_id
		JOIN person p ON p.id = e.person_id
		JOIN objective o ON o.id = e.objective_id
		WHERE eh.school_id = $1 AND eh.enrollment_id = $2`,
		id.SchoolID, enrollmentID,
	).Scan(&v.ID, &v.StudentName, &v.Objective, &v.LifeStatus,
		&v.HoursBeforeOffroad, &v.TotalHours,
		&v.OffroadTargetDate, &v.OnroadTargetDate,
		&v.Hours.Attended, &v.Hours.Excused, &v.Hours.Unexcused, &v.Hours.Consumed,
		&v.Hours.ProjectedOffroad, &v.Hours.ProjectedOnroad)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown enrollment")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	var passedAt *time.Time
	var validityMonths int
	err = h.pool.QueryRow(r.Context(), `
		SELECT (SELECT max(b.result_at) FROM exam_booking b
		        WHERE b.enrollment_id = $2 AND b.test_kind = 'OFFROAD' AND b.status = 'PASSED'),
		       s.offroad_validity_months
		FROM school s WHERE s.id = $1`, id.SchoolID, enrollmentID,
	).Scan(&passedAt, &validityMonths)
	if err == nil && passedAt != nil {
		v.OffroadPassedAt = passedAt
		exp := passedAt.AddDate(0, validityMonths, 0)
		v.OffroadExpiresAt = &exp
	}

	v.Alerts = gapAlerts(v)
	if v.OffroadExpiresAt != nil && time.Until(*v.OffroadExpiresAt) < 60*24*time.Hour {
		days := int(time.Until(*v.OffroadExpiresAt).Hours() / 24)
		v.Alerts = append(v.Alerts, Alert{
			Kind: "OFFROAD_EXPIRY", Severity: "WARNING", Target: "OFFROAD",
			DaysLeft: days,
			Message:  "le plateau obtenu approche de son expiration",
		})
	}
	reply(w, http.StatusOK, v)
}

// gapAlerts: a projection under its threshold alerts — it never blocks
// anything anywhere (D-04). One alert per target date concerned.
func gapAlerts(v EnrollmentView) []Alert {
	out := []Alert{}
	add := func(target string, projected *float64, threshold float64, date *time.Time) {
		if projected == nil || date == nil || *projected >= threshold {
			return
		}
		out = append(out, Alert{
			Kind:     "HOURS_GAP",
			Severity: "WARNING",
			Target:   target,
			GapHours: threshold - *projected,
			DaysLeft: int(time.Until(*date).Hours() / 24),
			Message:  "projected hours below the threshold for this target date",
		})
	}
	add("OFFROAD", v.Hours.ProjectedOffroad, v.HoursBeforeOffroad, v.OffroadTargetDate)
	add("ONROAD", v.Hours.ProjectedOnroad, v.TotalHours, v.OnroadTargetDate)
	return out
}

// GapLine is one row of the US-32 list: who is late, on what, by how
// much — sorted by how soon the target date bites.
type GapLine struct {
	EnrollmentID uuid.UUID `json:"enrollment_id"`
	StudentName  string    `json:"student_name"`
	Objective    string    `json:"objective"`
	Target       string    `json:"target"`
	TargetDate   time.Time `json:"target_date"`
	DaysLeft     int       `json:"days_left"`
	GapHours     float64   `json:"gap_hours"`
	Projected    float64   `json:"projected_hours"`
	Threshold    float64   `json:"threshold_hours"`
}

func (h *Handler) gaps(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}

	// One row per (enrollment, target date) whose projection is short.
	rows, err := h.pool.Query(r.Context(), `
		SELECT eh.enrollment_id, p.last_name || ' ' || p.first_names, o.label,
		       g.target, g.target_date, g.threshold - g.projected, g.projected, g.threshold
		FROM enrollment_hours eh
		JOIN enrollment e ON e.id = eh.enrollment_id AND e.life_status = 'ACTIVE'
		JOIN person p ON p.id = e.person_id
		JOIN objective o ON o.id = e.objective_id
		CROSS JOIN LATERAL (
			VALUES
			  ('OFFROAD', eh.offroad_target_date, eh.projected_offroad_hours, eh.hours_before_offroad),
			  ('ONROAD',  eh.onroad_target_date,  eh.projected_onroad_hours,  eh.total_hours)
		) AS g(target, target_date, projected, threshold)
		WHERE eh.school_id = $1
		  AND g.target_date IS NOT NULL
		  AND g.projected IS NOT NULL
		  AND g.projected < g.threshold
		ORDER BY g.target_date`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []GapLine{}
	for rows.Next() {
		var g GapLine
		if err := rows.Scan(&g.EnrollmentID, &g.StudentName, &g.Objective,
			&g.Target, &g.TargetDate, &g.GapHours, &g.Projected, &g.Threshold); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		g.DaysLeft = int(time.Until(g.TargetDate).Hours() / 24)
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}
