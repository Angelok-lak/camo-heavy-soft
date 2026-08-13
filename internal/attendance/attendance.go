// Package attendance is F-12, slice 2: record who was there. The rest of
// the system derives everything from these rows (C-05): hours, gaps,
// rankings — none of that lives here.
//
// Three rules shape the endpoints:
//   - Recording is IDEMPOTENT (RG-126): a flaky connection replaying the
//     same values must not produce corrections or duplicates.
//   - Changing a recorded value is a CORRECTION, traced with the
//     previous value (RG-20) — never an overwrite.
//   - An instructor records their OWN lessons; the office records
//     anywhere (RG-20, RG-236).
package attendance

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
	"github.com/angelok-lak/camo-heavy-soft/internal/events"
)

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/attendance/current", h.currentLesson)
	mux.HandleFunc("GET /api/attendance/unrecorded", h.unrecorded)
	mux.HandleFunc("GET /api/lessons/{id}/attendance", h.read)
	mux.HandleFunc("PUT /api/lessons/{id}/attendance", h.record)
	mux.HandleFunc("GET /api/attendance/sheet", h.sheet)
	mux.HandleFunc("GET /api/enrollments/{id}/lessons", h.studentLessons)
}

// studentLessons feeds the student file's "Séances" tab: everything past
// with its recorded value, everything planned ahead. Distinct from the
// attendance sheet (RG-127), which stays past-facts-only for proof.
func (h *Handler) studentLessons(w http.ResponseWriter, r *http.Request) {
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

	rows, err := h.pool.Query(r.Context(), `
		SELECT l.starts_at, l.ends_at,
		       EXTRACT(EPOCH FROM l.ends_at - l.starts_at)::int / 60,
		       l.status = 'CANCELLED',
		       l.ends_at > now(),
		       COALESCE(a.value::text, ''),
		       COALESCE((SELECT array_agg(res.label ORDER BY res.label)
		                 FROM lesson_resource lr JOIN resource res ON res.id = lr.resource_id
		                 WHERE lr.lesson_id = l.id), '{}')
		FROM lesson_assignment la
		JOIN lesson l ON l.id = la.lesson_id
		JOIN enrollment e ON e.id = la.enrollment_id
		LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
		WHERE e.school_id = $1 AND la.enrollment_id = $2
		ORDER BY l.starts_at DESC
		LIMIT 200`, id.SchoolID, enrollmentID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type line struct {
		StartsAt  time.Time `json:"starts_at"`
		EndsAt    time.Time `json:"ends_at"`
		Minutes   int       `json:"minutes"`
		Cancelled bool      `json:"cancelled"`
		Upcoming  bool      `json:"upcoming"`
		Value     string    `json:"value"`
		Resources []string  `json:"resources"`
	}
	out := []line{}
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.StartsAt, &l.EndsAt, &l.Minutes, &l.Cancelled, &l.Upcoming, &l.Value, &l.Resources); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------
// Views
// ---------------------------------------------------------------------

type StudentLine struct {
	AssignmentID uuid.UUID `json:"assignment_id"`
	EnrollmentID uuid.UUID `json:"enrollment_id"`
	StudentName  string    `json:"student_name"`
	// Value is empty when nothing is recorded yet — UNRECORDED is the
	// absence of a row (RG-123). The front pre-fills PRESENT (RG-236).
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

type LessonAttendance struct {
	LessonID uuid.UUID     `json:"lesson_id"`
	StartsAt time.Time     `json:"starts_at"`
	EndsAt   time.Time     `json:"ends_at"`
	Students []StudentLine `json:"students"`
}

// ---------------------------------------------------------------------
// Current lesson (RG-236)
// ---------------------------------------------------------------------

// currentLesson: the linked instructor's lesson in progress, or failing
// that the nearest one of the day. 204 when the account has no
// instructor sheet or no lesson today.
func (h *Handler) currentLesson(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}

	var lessonID uuid.UUID
	err := h.pool.QueryRow(r.Context(), `
		SELECT l.id
		FROM lesson l
		JOIN lesson_resource lr ON lr.lesson_id = l.id
		JOIN resource_instructor ri ON ri.resource_id = lr.resource_id
		WHERE l.school_id = $1 AND ri.user_id = $2
		  AND l.status = 'PLANNED'
		  AND l.starts_at::date = now()::date
		ORDER BY (now() BETWEEN l.starts_at AND l.ends_at) DESC,
		         abs(EXTRACT(EPOCH FROM l.starts_at - now()))
		LIMIT 1`, id.SchoolID, id.UserID).Scan(&lessonID)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.replyLesson(w, r, id.SchoolID, lessonID)
}

// ---------------------------------------------------------------------
// Unrecorded lessons (RG-125)
// ---------------------------------------------------------------------

// Instructors and material resources come as separate, sorted lists so
// every row of the screen reads the same way.
type UnrecordedLesson struct {
	LessonID    uuid.UUID `json:"lesson_id"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Students    int       `json:"students"`
	Recorded    int       `json:"recorded"`
	Instructors []string  `json:"instructors"`
	Vehicles    []string  `json:"vehicles"`
}

// unrecorded lists past lessons with at least one missing value, OLDEST
// first (RG-125): the longer it drags, the less reliable memory gets.
// Instructors see their own; the office sees everything (RG-242).
func (h *Handler) unrecorded(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	officeWide := id.CanEditPlanning()

	rows, err := h.pool.Query(r.Context(), `
		SELECT l.id, l.starts_at, l.ends_at,
		       count(la.id),
		       count(a.id),
		       COALESCE((SELECT array_agg(res.label ORDER BY res.label)
		                 FROM lesson_resource lr JOIN resource res ON res.id = lr.resource_id
		                 WHERE lr.lesson_id = l.id AND res.kind = 'INSTRUCTOR'), '{}'),
		       COALESCE((SELECT array_agg(res.label ORDER BY res.label)
		                 FROM lesson_resource lr JOIN resource res ON res.id = lr.resource_id
		                 WHERE lr.lesson_id = l.id AND res.kind <> 'INSTRUCTOR'), '{}')
		FROM lesson l
		JOIN lesson_assignment la ON la.lesson_id = l.id
		LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
		WHERE l.school_id = $1 AND l.status = 'PLANNED' AND l.ends_at < now()
		  AND ($2 OR EXISTS (
		        SELECT 1 FROM lesson_resource lr
		        JOIN resource_instructor ri ON ri.resource_id = lr.resource_id
		        WHERE lr.lesson_id = l.id AND ri.user_id = $3))
		GROUP BY l.id
		HAVING count(a.id) < count(la.id)
		ORDER BY l.starts_at`,
		id.SchoolID, officeWide, id.UserID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []UnrecordedLesson{}
	for rows.Next() {
		var u UnrecordedLesson
		if err := rows.Scan(&u.LessonID, &u.StartsAt, &u.EndsAt, &u.Students, &u.Recorded, &u.Instructors, &u.Vehicles); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------
// Read one lesson's values
// ---------------------------------------------------------------------

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	lessonID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown lesson")
		return
	}
	h.replyLesson(w, r, id.SchoolID, lessonID)
}

func (h *Handler) replyLesson(w http.ResponseWriter, r *http.Request, schoolID, lessonID uuid.UUID) {
	var la LessonAttendance
	la.LessonID = lessonID
	err := h.pool.QueryRow(r.Context(), `
		SELECT starts_at, ends_at FROM lesson
		WHERE school_id = $1 AND id = $2`, schoolID, lessonID,
	).Scan(&la.StartsAt, &la.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown lesson")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT la.id, la.enrollment_id, p.last_name || ' ' || p.first_names,
		       COALESCE(a.value::text, ''), COALESCE(a.reason, '')
		FROM lesson_assignment la
		JOIN enrollment e ON e.id = la.enrollment_id
		JOIN person p ON p.id = e.person_id
		LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
		WHERE la.lesson_id = $1
		ORDER BY p.last_name, p.first_names`, lessonID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	la.Students = []StudentLine{}
	for rows.Next() {
		var s StudentLine
		if err := rows.Scan(&s.AssignmentID, &s.EnrollmentID, &s.StudentName, &s.Value, &s.Reason); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		la.Students = append(la.Students, s)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, la)
}

// ---------------------------------------------------------------------
// Record (RG-126) and correct (RG-20)
// ---------------------------------------------------------------------

var validValues = map[string]bool{"PRESENT": true, "EXCUSED": true, "UNEXCUSED": true}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	lessonID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown lesson")
		return
	}

	allowed, err := h.mayRecord(r, id, lessonID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowed {
		fail(w, http.StatusForbidden, "instructors record their own lessons only (RG-20)")
		return
	}

	var body []struct {
		EnrollmentID uuid.UUID `json:"enrollment_id"`
		Value        string    `json:"value"`
		Reason       string    `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) == 0 {
		fail(w, http.StatusUnprocessableEntity, "a list of {enrollment_id, value} is required")
		return
	}
	for _, line := range body {
		if !validValues[line.Value] {
			fail(w, http.StatusUnprocessableEntity, "value must be PRESENT, EXCUSED or UNEXCUSED")
			return
		}
	}

	tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	corrections := 0
	for _, line := range body {
		// Resolve the assignment; a student not on the lesson is a
		// structural impossibility.
		var assignmentID uuid.UUID
		var current *string
		var attendanceID *uuid.UUID
		err := tx.QueryRow(r.Context(), `
			SELECT la.id, a.id, a.value::text
			FROM lesson_assignment la
			LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
			WHERE la.lesson_id = $1 AND la.enrollment_id = $2`,
			lessonID, line.EnrollmentID).Scan(&assignmentID, &attendanceID, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, http.StatusUnprocessableEntity, "student not placed on this lesson: "+line.EnrollmentID.String())
			return
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}

		switch {
		case attendanceID == nil:
			// First recording.
			newID, err := uuid.NewV7()
			if err != nil {
				fail(w, http.StatusInternalServerError, err.Error())
				return
			}
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO attendance (id, school_id, lesson_assignment_id, value, reason, recorded_by)
				VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)`,
				newID, id.SchoolID, assignmentID, line.Value, line.Reason, id.UserID); err != nil {
				fail(w, http.StatusInternalServerError, err.Error())
				return
			}

		case *current == line.Value:
			// Idempotence (RG-126): same value replayed, nothing happens.

		default:
			// A change of a recorded value is a traced correction (RG-20).
			corrID, err := uuid.NewV7()
			if err != nil {
				fail(w, http.StatusInternalServerError, err.Error())
				return
			}
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO attendance_correction
				    (id, school_id, attendance_id, previous_value, new_value, corrected_by)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				corrID, id.SchoolID, *attendanceID, *current, line.Value, id.UserID); err != nil {
				fail(w, http.StatusInternalServerError, err.Error())
				return
			}
			if _, err := tx.Exec(r.Context(), `
				UPDATE attendance SET value = $2, reason = NULLIF($3, ''),
				    recorded_by = $4, recorded_at = now()
				WHERE id = $1`,
				*attendanceID, line.Value, line.Reason, id.UserID); err != nil {
				fail(w, http.StatusInternalServerError, err.Error())
				return
			}
			corrections++
		}
	}

	if err := events.Emit(r.Context(), tx, id.SchoolID, "attendance.recorded", "lesson", lessonID,
		map[string]int{"lines": len(body), "corrections": corrections}, id.UserID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.replyLesson(w, r, id.SchoolID, lessonID)
}

// mayRecord: office and management anywhere; an instructor only where
// their own sheet is assigned (RG-20).
func (h *Handler) mayRecord(r *http.Request, id auth.Identity, lessonID uuid.UUID) (bool, error) {
	if id.CanEditPlanning() {
		return true, nil
	}
	var mine bool
	err := h.pool.QueryRow(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM lesson_resource lr
			JOIN resource_instructor ri ON ri.resource_id = lr.resource_id
			WHERE lr.lesson_id = $1 AND ri.user_id = $2)`,
		lessonID, id.UserID).Scan(&mine)
	return mine, err
}

// ---------------------------------------------------------------------
// Attendance sheet (RG-127)
// ---------------------------------------------------------------------

type SheetLine struct {
	Date        time.Time `json:"date"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Minutes     int       `json:"minutes"`
	StudentName string    `json:"student_name"`
	Value       string    `json:"value"`
}

// sheet returns FACTS ONLY: dates, durations, statuses. No totals, no
// thresholds, no waivers (RG-127, RG-52) — a sheet is proof, not
// analysis, and mixing the two produces false certificates.
func (h *Handler) sheet(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	enrollmentID, err := uuid.Parse(r.URL.Query().Get("enrollment_id"))
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "enrollment_id required")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT l.starts_at::date, l.starts_at, l.ends_at,
		       EXTRACT(EPOCH FROM l.ends_at - l.starts_at)::int / 60,
		       p.last_name || ' ' || p.first_names,
		       COALESCE(a.value::text, 'UNRECORDED')
		FROM lesson_assignment la
		JOIN lesson l ON l.id = la.lesson_id
		JOIN enrollment e ON e.id = la.enrollment_id
		JOIN person p ON p.id = e.person_id
		LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
		WHERE e.school_id = $1 AND la.enrollment_id = $2
		  AND l.status = 'PLANNED' AND l.ends_at < now()
		ORDER BY l.starts_at`, id.SchoolID, enrollmentID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []SheetLine{}
	for rows.Next() {
		var s SheetLine
		if err := rows.Scan(&s.Date, &s.StartsAt, &s.EndsAt, &s.Minutes, &s.StudentName, &s.Value); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------
// Plumbing
// ---------------------------------------------------------------------

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
