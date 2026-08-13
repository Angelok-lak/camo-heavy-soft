package exams

// F-07.1 first cut: committing a candidate to a session, one test at a
// time. Credit counters are DERIVED (C-05): committed = sum of frozen
// units over live bookings, remaining = allowance − committed.
//
// D-04 all the way down: missing exam prerequisites or a spent allowance
// never block a booking — they come back as alerts, and booking over
// missing prerequisites leaves a traced override on the row.
//
// The unit scale (off-road 1, on-road 2) matches the domain glossary; it
// belongs in parametre_centre (C-07) and moves there with full F-01.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
	"github.com/angelok-lak/camo-heavy-soft/internal/events"
)

func unitsFor(testKind string) int {
	if testKind == "ONROAD" {
		return 2
	}
	return 1
}

func (h *Handler) bookingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/exam-sessions/{id}", h.detail)
	mux.HandleFunc("POST /api/exam-sessions/{id}/bookings", h.book)
	mux.HandleFunc("POST /api/exam-bookings/{id}/withdraw", h.withdraw)
}

// ---------------------------------------------------------------------
// Detail: the mockup screen — header, units, committed, proposed
// ---------------------------------------------------------------------

type Booking struct {
	ID           uuid.UUID `json:"id"`
	EnrollmentID uuid.UUID `json:"enrollment_id"`
	StudentName  string    `json:"student_name"`
	Objective    string    `json:"objective"`
	TestKind     string    `json:"test_kind"`
	Units        int       `json:"units"`
	Status       string    `json:"status"`
	// Derived (RG-112): committed, session past, no result entered.
	Unrecorded bool `json:"unrecorded"`
	OverrideNote string    `json:"override_note"`
	BookedBy     string    `json:"booked_by"`
	BookedAt     time.Time `json:"booked_at"`
	Consumed     float64   `json:"consumed_hours"`
	Total        float64   `json:"total_hours"`
}

type Candidate struct {
	EnrollmentID uuid.UUID  `json:"enrollment_id"`
	StudentName  string     `json:"student_name"`
	Objective    string     `json:"objective"`
	Consumed     float64    `json:"consumed_hours"`
	Total        float64    `json:"total_hours"`
	Projected    *float64   `json:"projected_hours"`
	Threshold    float64    `json:"threshold_hours"`
	TargetDate   *time.Time `json:"target_date"`
	DaysLeft     *int       `json:"days_left"`
	// Missing EXAM prerequisites: named, never excluding (RG-90 revised).
	MissingRequirements []string `json:"missing_requirements"`
	// Off-road pass with its DERIVED expiry (RG-25), and how many times
	// the candidate already presented (the mockup's "2e présentation").
	OffroadPassedAt  *time.Time `json:"offroad_passed_at"`
	OffroadExpiresAt *time.Time `json:"offroad_expires_at"`
	Presentations    int        `json:"presentations"`
}

type SessionDetail struct {
	Session   SessionView `json:"session"`
	Resources []string    `json:"resource_labels"`
	Credits   struct {
		Allowance *int `json:"allowance"`
		Committed int  `json:"committed"`
		Spent     int  `json:"spent"`
		Forfeited int  `json:"forfeited"`
		Remaining *int `json:"remaining"`
	} `json:"credits"`
	Bookings      []Booking   `json:"bookings"`
	Proposed      []Candidate `json:"proposed"`
	ActiveTotal   int         `json:"active_total"`
	ProposedTotal int         `json:"proposed_total"`
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}

	var d SessionDetail
	err = h.pool.QueryRow(r.Context(), `
		SELECT es.id, ep.id, ep.label, ep.travel_minutes, es.starts_at, es.ends_at,
		       es.credit_allowance, es.status = 'CANCELLED', COALESCE(es.cancel_reason, ''),
		       es.ends_at < now(),
		       COALESCE((SELECT array_agg(r.label ORDER BY r.kind DESC, r.label)
		                 FROM exam_session_resource esr JOIN resource r ON r.id = esr.resource_id
		                 WHERE esr.exam_session_id = es.id), '{}')
		FROM exam_session es JOIN exam_place ep ON ep.id = es.exam_place_id
		WHERE es.school_id = $1 AND es.id = $2`, id.SchoolID, sessionID,
	).Scan(&d.Session.ID, &d.Session.PlaceID, &d.Session.PlaceLabel, &d.Session.TravelMinutes,
		&d.Session.StartsAt, &d.Session.EndsAt, &d.Session.CreditAllowance,
		&d.Session.Cancelled, &d.Session.CancelReason, &d.Session.Past, &d.Resources)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Live bookings, student context attached.
	rows, err := h.pool.Query(r.Context(), `
		SELECT b.id, b.enrollment_id, p.last_name || ' ' || p.first_names, o.label,
		       b.test_kind::text, b.committed_units, b.status::text,
		       COALESCE(b.override_note, ''), COALESCE(u.display_name, ''), b.booked_at,
		       COALESCE(eh.consumed_hours, 0), COALESCE(eh.total_hours, 0)
		FROM exam_booking b
		JOIN enrollment e ON e.id = b.enrollment_id
		JOIN person p ON p.id = e.person_id
		JOIN objective o ON o.id = e.objective_id
		LEFT JOIN enrollment_hours eh ON eh.enrollment_id = e.id
		LEFT JOIN app_user u ON u.id = b.booked_by
		WHERE b.school_id = $1 AND b.exam_session_id = $2 AND b.status <> 'WITHDRAWN'
		ORDER BY b.booked_at`, id.SchoolID, sessionID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	d.Bookings = []Booking{}
	for rows.Next() {
		var b Booking
		if err := rows.Scan(&b.ID, &b.EnrollmentID, &b.StudentName, &b.Objective,
			&b.TestKind, &b.Units, &b.Status, &b.OverrideNote, &b.BookedBy, &b.BookedAt,
			&b.Consumed, &b.Total); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		switch b.Status {
		case "COMMITTED":
			d.Credits.Committed += b.Units
			b.Unrecorded = d.Session.Past
		case "PASSED", "FAILED":
			d.Credits.Spent += b.Units
		case "ABSENT":
			// Absence forfeits the units (RG-39).
			d.Credits.Forfeited += b.Units
		}
		d.Bookings = append(d.Bookings, b)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	d.Credits.Allowance = d.Session.CreditAllowance
	if d.Session.CreditAllowance != nil {
		rem := *d.Session.CreditAllowance - d.Credits.Committed - d.Credits.Spent - d.Credits.Forfeited
		if rem < 0 {
			rem = 0
		}
		if d.Session.Past {
			// The unengaged remainder after the date is lost (RG-40).
			d.Credits.Forfeited += rem
			rem = 0
		}
		d.Credits.Remaining = &rem
	}

	if err := h.loadProposed(r, id.SchoolID, sessionID, &d); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, d)
}

// loadProposed ranks the ACTIVE, not-yet-committed enrollments by target
// date proximity — closest first. Missing EXAM prerequisites ride along,
// named, and exclude no one (RG-90 revised, RG-95).
func (h *Handler) loadProposed(r *http.Request, schoolID, sessionID uuid.UUID, d *SessionDetail) error {
	validityMonths := h.offroadValidityMonths(r, schoolID)
	rows, err := h.pool.Query(r.Context(), `
		SELECT eh.enrollment_id, p.last_name || ' ' || p.first_names, o.label,
		       eh.consumed_hours, eh.total_hours,
		       COALESCE(eh.projected_offroad_hours, eh.projected_onroad_hours),
		       COALESCE(eh.hours_before_offroad, eh.total_hours),
		       COALESCE(eh.offroad_target_date, eh.onroad_target_date),
		       COALESCE((SELECT array_agg(er.label ORDER BY er.label)
		                 FROM enrollment_requirement er
		                 WHERE er.enrollment_id = eh.enrollment_id AND er.req_set = 'EXAM'
		                   AND er.mandatory AND er.status = 'NOT_VALIDATED'), '{}'),
		       (SELECT max(b.result_at) FROM exam_booking b
		        WHERE b.enrollment_id = eh.enrollment_id
		          AND b.test_kind = 'OFFROAD' AND b.status = 'PASSED'),
		       (SELECT count(*) FROM exam_booking b
		        JOIN exam_session bs ON bs.id = b.exam_session_id
		        WHERE b.enrollment_id = eh.enrollment_id AND b.status <> 'WITHDRAWN'
		          AND bs.starts_at < now())
		FROM enrollment_hours eh
		JOIN enrollment e ON e.id = eh.enrollment_id AND e.life_status = 'ACTIVE'
		JOIN person p ON p.id = e.person_id AND p.status = 'ACTIVE'
		JOIN objective o ON o.id = e.objective_id
		WHERE eh.school_id = $1
		  AND NOT EXISTS (SELECT 1 FROM exam_booking b
		                  WHERE b.exam_session_id = $2 AND b.enrollment_id = eh.enrollment_id
		                    AND b.status <> 'WITHDRAWN')
		ORDER BY COALESCE(eh.offroad_target_date, eh.onroad_target_date) ASC NULLS LAST,
		         p.last_name`, schoolID, sessionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	all := []Candidate{}
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.EnrollmentID, &c.StudentName, &c.Objective,
			&c.Consumed, &c.Total, &c.Projected, &c.Threshold, &c.TargetDate,
			&c.MissingRequirements, &c.OffroadPassedAt, &c.Presentations); err != nil {
			return err
		}
		if c.TargetDate != nil {
			days := int(time.Until(*c.TargetDate).Hours() / 24)
			c.DaysLeft = &days
		}
		if c.OffroadPassedAt != nil {
			exp := c.OffroadPassedAt.AddDate(0, validityMonths, 0)
			c.OffroadExpiresAt = &exp
		}
		all = append(all, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	d.ActiveTotal = len(all) + len(d.Bookings)
	d.ProposedTotal = len(all)
	if len(all) > 10 {
		all = all[:10]
	}
	d.Proposed = all
	return nil
}

// ---------------------------------------------------------------------
// Booking / withdrawing
// ---------------------------------------------------------------------

func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}
	var body struct {
		EnrollmentID uuid.UUID `json:"enrollment_id"`
		TestKind     string    `json:"test_kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.EnrollmentID == uuid.Nil || (body.TestKind != "OFFROAD" && body.TestKind != "ONROAD") {
		fail(w, http.StatusUnprocessableEntity, "enrollment_id and test_kind (OFFROAD/ONROAD) required")
		return
	}

	// Missing exam prerequisites: the system alerts, the office decides —
	// booking anyway leaves the override in clear text (D-04, A-08).
	var missing []string
	err = h.pool.QueryRow(r.Context(), `
		SELECT COALESCE(array_agg(label ORDER BY label), '{}')
		FROM enrollment_requirement
		WHERE school_id = $1 AND enrollment_id = $2 AND req_set = 'EXAM'
		  AND mandatory AND status = 'NOT_VALIDATED'`,
		id.SchoolID, body.EnrollmentID).Scan(&missing)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	var overrideNote *string
	if len(missing) > 0 {
		note := strings.Join(missing, ", ") + " non validé"
		overrideNote = &note
	}

	units := unitsFor(body.TestKind)

	// A withdrawn booking on the same test is REVIVED, not duplicated:
	// the row is the trace, re-engaging refreezes the units and the
	// override, and clears the withdrawal stamp.
	var bookingID uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		UPDATE exam_booking b SET status = 'COMMITTED', committed_units = $5,
		       override_note = $6, booked_by = $7, booked_at = now(),
		       result_by = NULL, result_at = NULL
		FROM exam_session es
		WHERE b.school_id = $1 AND b.exam_session_id = $2 AND b.enrollment_id = $3
		  AND b.test_kind = $4 AND b.status = 'WITHDRAWN'
		  AND es.id = b.exam_session_id AND es.status = 'SCHEDULED'
		RETURNING b.id`,
		id.SchoolID, sessionID, body.EnrollmentID, body.TestKind, units, overrideNote, id.UserID,
	).Scan(&bookingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err == nil {
		_ = events.Emit(r.Context(), h.pool, id.SchoolID, "exam_booking.created", "exam_booking",
			bookingID, map[string]any{"session": sessionID, "enrollment": body.EnrollmentID,
				"test_kind": body.TestKind, "units": units, "override": overrideNote != nil,
				"revived": true}, id.UserID)
		reply(w, http.StatusCreated, map[string]any{"id": bookingID, "override_note": overrideNote})
		return
	}

	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		INSERT INTO exam_booking
		    (id, school_id, exam_session_id, enrollment_id, test_kind, committed_units, override_note, booked_by)
		SELECT $1, $2, es.id, e.id, $5, $6, $7, $8
		FROM exam_session es, enrollment e
		WHERE es.school_id = $2 AND es.id = $3 AND es.status = 'SCHEDULED'
		  AND e.school_id = $2 AND e.id = $4 AND e.life_status = 'ACTIVE'`,
		newID, id.SchoolID, sessionID, body.EnrollmentID, body.TestKind, units, overrideNote, id.UserID)
	if err != nil {
		fail(w, http.StatusConflict, "ce candidat est déjà engagé sur cette épreuve")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusUnprocessableEntity, "session annulée ou parcours inactif")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "exam_booking.created", "exam_booking",
		newID, map[string]any{"session": sessionID, "enrollment": body.EnrollmentID,
			"test_kind": body.TestKind, "units": units, "override": overrideNote != nil}, id.UserID)
	reply(w, http.StatusCreated, map[string]any{"id": newID, "override_note": overrideNote})
}

// withdraw before the session frees the units (RG-104): the booking
// stops counting, the row stays as trace.
func (h *Handler) withdraw(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	bookingID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown booking")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE exam_booking SET status = 'WITHDRAWN', result_by = $3, result_at = now()
		WHERE school_id = $1 AND id = $2 AND status = 'COMMITTED'`,
		id.SchoolID, bookingID, id.UserID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown or already settled booking")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "exam_booking.withdrawn", "exam_booking",
		bookingID, nil, id.UserID)
	w.WriteHeader(http.StatusNoContent)
}