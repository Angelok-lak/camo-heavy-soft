package planning

// HTTP surface of F-09, slice 1. One contract per feature, operations
// named by intent (C-25): pushing a draft, not updating lessons.
//
// Refusals keep their three allowed natures apart (API conventions):
//   403 — forbidden by permissions (F-17, RG-214)
//   404/422 — structurally impossible
//   409 — lock held (naming the holder) or version conflict (RG-259)
// A business rule NEVER refuses: it comes back as a conflict in the
// payload, severity attached (D-04).

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
	"github.com/angelok-lak/camo-heavy-soft/internal/availability"
	"github.com/angelok-lak/camo-heavy-soft/internal/events"
)

type Handler struct {
	store *Store
	// NotifyAssignment is wired at startup to the comms package: it
	// sends the automatic message when a student lands on a lesson.
	// Never blocks a placement — errors are logged, not returned.
	NotifyAssignment func(ctx context.Context, schoolID, lessonID, enrollmentID, authorID uuid.UUID) (any, error)
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// Routes mounts F-09 on a mux (Go 1.22 method patterns).
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/planning", h.readPlanning)
	mux.HandleFunc("POST /api/edit-sessions", h.openSession)
	mux.HandleFunc("GET /api/edit-sessions/{id}", h.readSession)
	mux.HandleFunc("PUT /api/edit-sessions/{id}/operations", h.pushDraft)
	mux.HandleFunc("DELETE /api/edit-sessions/{id}/operations/{opID}", h.removeOperation)
	mux.HandleFunc("POST /api/edit-sessions/{id}/save", h.save)
	mux.HandleFunc("POST /api/edit-sessions/{id}/discard", h.discard)
	mux.HandleFunc("POST /api/edit-sessions/{id}/force-release", h.forceRelease)
	mux.HandleFunc("GET /api/lessons/{id}/suggested-students", h.suggestStudents)
	// F-03 direct placement: its own vertical, separate from the F-09
	// draft (arbitrage A-38: multi-editing is rare at the centre; the
	// draft stays the only write path for create/move/cancel).
	mux.HandleFunc("POST /api/lessons/{id}/assignments", h.placeStudent)
	mux.HandleFunc("DELETE /api/lessons/{id}/assignments/{enrollmentID}", h.removeStudent)
}

// placeStudent places immediately and answers with the lesson's
// recomputed conflicts — joined alerts, never a refusal (D-04).
func (h *Handler) placeStudent(w http.ResponseWriter, r *http.Request) {
	id, ok := requireEditor(w, r)
	if !ok {
		return
	}
	lessonID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown lesson")
		return
	}
	var body struct {
		EnrollmentID uuid.UUID `json:"enrollment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EnrollmentID == uuid.Nil {
		fail(w, http.StatusUnprocessableEntity, "enrollment_id required")
		return
	}

	assignID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	tag, err := h.store.pool.Exec(r.Context(), `
		INSERT INTO lesson_assignment (id, lesson_id, enrollment_id, assigned_by)
		SELECT $1, l.id, e.id, $4
		FROM lesson l, enrollment e
		WHERE l.school_id = $2 AND l.id = $3 AND l.status = 'PLANNED'
		  AND e.school_id = $2 AND e.id = $5
		ON CONFLICT (lesson_id, enrollment_id) DO NOTHING`,
		assignID, id.SchoolID, lessonID, id.UserID, body.EnrollmentID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	var comm any
	if tag.RowsAffected() > 0 {
		_, _ = h.store.pool.Exec(r.Context(),
			`UPDATE lesson SET version = version + 1 WHERE id = $1`, lessonID)
		_ = events.Emit(r.Context(), h.store.pool, id.SchoolID, "lesson.student_placed", "lesson",
			lessonID, map[string]uuid.UUID{"enrollment_id": body.EnrollmentID}, id.UserID)
		if h.NotifyAssignment != nil {
			if c, err := h.NotifyAssignment(r.Context(), id.SchoolID, lessonID,
				body.EnrollmentID, id.UserID); err != nil {
				log.Printf("assignment message failed: %v", err)
			} else {
				comm = c
			}
		}
	}
	h.replyLessonConflicts(w, r, id.SchoolID, lessonID, comm)
}

func (h *Handler) removeStudent(w http.ResponseWriter, r *http.Request) {
	id, ok := requireEditor(w, r)
	if !ok {
		return
	}
	lessonID, err1 := uuid.Parse(r.PathValue("id"))
	enrollmentID, err2 := uuid.Parse(r.PathValue("enrollmentID"))
	if err1 != nil || err2 != nil {
		fail(w, http.StatusNotFound, "unknown lesson or enrollment")
		return
	}
	tag, err := h.store.pool.Exec(r.Context(), `
		DELETE FROM lesson_assignment la
		USING lesson l
		WHERE la.lesson_id = l.id AND l.school_id = $1
		  AND la.lesson_id = $2 AND la.enrollment_id = $3`,
		id.SchoolID, lessonID, enrollmentID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() > 0 {
		_, _ = h.store.pool.Exec(r.Context(),
			`UPDATE lesson SET version = version + 1 WHERE id = $1`, lessonID)
		_ = events.Emit(r.Context(), h.store.pool, id.SchoolID, "lesson.student_removed", "lesson",
			lessonID, map[string]uuid.UUID{"enrollment_id": enrollmentID}, id.UserID)
	}
	h.replyLessonConflicts(w, r, id.SchoolID, lessonID, nil)
}

// replyLessonConflicts recomputes the lesson's conflicts on the saved
// state and returns the lesson as the planner view knows it.
func (h *Handler) replyLessonConflicts(w http.ResponseWriter, r *http.Request, schoolID, lessonID uuid.UUID, comm any) {
	var starts, ends time.Time
	err := h.store.pool.QueryRow(r.Context(), `
		SELECT starts_at, ends_at FROM lesson WHERE school_id = $1 AND id = $2`,
		schoolID, lessonID).Scan(&starts, &ends)
	if err != nil {
		fail(w, http.StatusNotFound, "unknown lesson")
		return
	}
	day := availability.TimeSlot{
		Start: starts.Add(-12 * time.Hour), End: ends.Add(12 * time.Hour),
	}
	snap, err := h.store.LoadSnapshot(r.Context(), schoolID, day)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, l := range snap.Lessons {
		if l.ID == lessonID {
			conflicts := availability.Check(snap, availability.Candidate{
				LessonID: l.ID, Slot: l.Slot, Resources: l.Resources,
				Enrollments: l.Enrollments, Kinds: l.Kinds,
			})
			if conflicts == nil {
				conflicts = []availability.Conflict{}
			}
			out := map[string]any{"lesson": l, "conflicts": conflicts}
			if comm != nil {
				out["communication"] = comm
			}
			reply(w, http.StatusOK, out)
			return
		}
	}
	fail(w, http.StatusNotFound, "unknown lesson")
}

// ---------------------------------------------------------------------
// Read the planning
// ---------------------------------------------------------------------

// lessonView is a lesson with its conflicts already computed and
// explained — the front calculates nothing (C-26).
type lessonView struct {
	availability.Lesson
	Conflicts []availability.Conflict `json:"conflicts"`
}

type planningView struct {
	Period    availability.TimeSlot                 `json:"period"`
	Lessons   []lessonView                          `json:"lessons"`
	Exams     []availability.ExamSession            `json:"exam_sessions"`
	Resources map[uuid.UUID]availability.Resource   `json:"resources"`
	Kinds     map[uuid.UUID]availability.LessonKind `json:"lesson_kinds"`
	Students  map[uuid.UUID]availability.Enrollment `json:"enrollments"`
	Opening   []availability.OpeningHours           `json:"opening_hours"`
	Closures  []availability.TimeSlot               `json:"closures"`
}

func (h *Handler) readPlanning(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	period, err := parsePeriod(r)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	snap, err := h.store.LoadSnapshot(r.Context(), id.SchoolID, period)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	view := planningView{
		Period:    period,
		Lessons:   []lessonView{},
		Exams:     append([]availability.ExamSession{}, snap.Exams...),
		Resources: snap.Resources,
		Kinds:     snap.LessonKinds,
		Students:  snap.Enrollments,
		Opening:   snap.Opening,
		Closures:  snap.Closures,
	}
	// Existing conflicts ride along with each lesson (C-29): the state
	// may already be in conflict — overridden yesterday, still true today.
	for _, l := range snap.Lessons {
		lv := lessonView{Lesson: l, Conflicts: []availability.Conflict{}}
		if !l.Cancelled {
			if cs := availability.Check(snap, availability.Candidate{
				LessonID: l.ID, Slot: l.Slot, Resources: l.Resources,
				Enrollments: l.Enrollments, Kinds: l.Kinds,
			}); cs != nil {
				lv.Conflicts = cs
			}
		}
		view.Lessons = append(view.Lessons, lv)
	}
	reply(w, http.StatusOK, view)
}

// ---------------------------------------------------------------------
// Edit session lifecycle
// ---------------------------------------------------------------------

func (h *Handler) openSession(w http.ResponseWriter, r *http.Request) {
	id, ok := requireEditor(w, r)
	if !ok {
		return
	}
	var body struct {
		From time.Time `json:"scope_starts_at"`
		To   time.Time `json:"scope_ends_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !body.To.After(body.From) {
		fail(w, http.StatusUnprocessableEntity, "scope needs scope_starts_at < scope_ends_at")
		return
	}

	session, err := h.store.OpenEditSession(r.Context(), id.SchoolID, id.UserID,
		availability.TimeSlot{Start: body.From, End: body.To})

	var held *LockHeldError
	if errors.As(err, &held) {
		// RG-144: the refusal names the editor and the opening time.
		reply(w, http.StatusConflict, map[string]any{"error": "lock_held", "holder": held})
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, session)
}

// sessionView is a session plus its draft applied: resulting conflicts
// (RG-146) and the pending list (RG-220).
type sessionView struct {
	Session *EditSession `json:"session"`
	Draft   draftView    `json:"draft"`
}

// draftView carries the RESULTING lessons too: the front displays the
// overlaid state, it never replays the journal itself (C-26).
type draftView struct {
	Lessons   []availability.Lesson                 `json:"lessons"`
	Pending   []Operation                           `json:"pending"`
	Rejected  []Rejection                           `json:"rejected"`
	Conflicts map[uuid.UUID][]availability.Conflict `json:"conflicts"`
}

func (h *Handler) readSession(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	h.replyWithDraft(w, r, id.SchoolID, http.StatusOK)
}

// pushDraft replaces the whole journal — the client owns the ordering —
// and answers with conflicts recomputed on the resulting state (RG-146).
func (h *Handler) pushDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := requireEditor(w, r)
	if !ok {
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}
	var ops []Operation
	if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed operations")
		return
	}

	if err := h.store.ReplaceOperations(r.Context(), id.SchoolID, sessionID, ops); err != nil {
		failStore(w, err)
		return
	}
	h.replyWithDraft(w, r, id.SchoolID, http.StatusOK)
}

func (h *Handler) removeOperation(w http.ResponseWriter, r *http.Request) {
	id, ok := requireEditor(w, r)
	if !ok {
		return
	}
	sessionID, err1 := uuid.Parse(r.PathValue("id"))
	opID, err2 := uuid.Parse(r.PathValue("opID"))
	if err1 != nil || err2 != nil {
		fail(w, http.StatusNotFound, "unknown session or operation")
		return
	}

	session, err := h.store.GetOpenSession(r.Context(), id.SchoolID, sessionID)
	if err != nil {
		failStore(w, err)
		return
	}
	kept := make([]Operation, 0, len(session.Operations))
	for _, op := range session.Operations {
		if op.ID != opID {
			kept = append(kept, op)
		}
	}
	if err := h.store.ReplaceOperations(r.Context(), id.SchoolID, sessionID, kept); err != nil {
		failStore(w, err)
		return
	}
	h.replyWithDraft(w, r, id.SchoolID, http.StatusOK)
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	id, ok := requireEditor(w, r)
	if !ok {
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}

	report, err := h.store.Save(r.Context(), id.SchoolID, sessionID)

	var stale *StaleScopeError
	if errors.As(err, &stale) {
		// RG-259: describe what moved, never overwrite.
		reply(w, http.StatusConflict, map[string]any{"error": "scope_changed", "detail": stale})
		return
	}
	if err != nil {
		failStore(w, err)
		return
	}
	if h.NotifyAssignment != nil {
		for _, op := range report.Applied {
			if op.Kind == PlaceStudent && op.EnrollmentID != nil {
				if _, err := h.NotifyAssignment(r.Context(), id.SchoolID,
					op.LessonID, *op.EnrollmentID, id.UserID); err != nil {
					log.Printf("assignment message failed: %v", err)
				}
			}
		}
	}
	reply(w, http.StatusOK, report)
}

func (h *Handler) discard(w http.ResponseWriter, r *http.Request) {
	id, ok := requireEditor(w, r)
	if !ok {
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}
	if err := h.store.Discard(r.Context(), id.SchoolID, sessionID); err != nil {
		failStore(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) forceRelease(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	if !id.CanForceRelease() {
		// A permission refusal, not a business rule (RG-214): allowed.
		fail(w, http.StatusForbidden, "management only (RG-145)")
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}
	if err := h.store.ForceRelease(r.Context(), id.SchoolID, sessionID, id.UserID); err != nil {
		failStore(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Plumbing
// ---------------------------------------------------------------------

func (h *Handler) replyWithDraft(w http.ResponseWriter, r *http.Request, schoolID uuid.UUID, status int) {
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}
	session, err := h.store.GetOpenSession(r.Context(), schoolID, sessionID)
	if err != nil {
		failStore(w, err)
		return
	}
	snap, err := h.store.LoadSnapshot(r.Context(), schoolID, session.Scope)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	state := Apply(snap, session.Operations)
	reply(w, status, sessionView{
		Session: session,
		Draft: draftView{
			Lessons:   state.Result.Lessons,
			Pending:   state.Pending,
			Rejected:  state.Rejected,
			Conflicts: state.Conflicts,
		},
	})
}

func requireEditor(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return id, false
	}
	if !id.CanEditPlanning() {
		fail(w, http.StatusForbidden, "planning is read-only for this profile (RG-214)")
		return id, false
	}
	return id, true
}

func parsePeriod(r *http.Request) (availability.TimeSlot, error) {
	from, err1 := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	to, err2 := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	if err1 != nil || err2 != nil || !to.After(from) {
		return availability.TimeSlot{}, errors.New("period needs from < to, RFC 3339")
	}
	return availability.TimeSlot{Start: from, End: to}, nil
}

func failStore(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNoOpenSession) {
		fail(w, http.StatusNotFound, ErrNoOpenSession.Error())
		return
	}
	fail(w, http.StatusInternalServerError, err.Error())
}

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
