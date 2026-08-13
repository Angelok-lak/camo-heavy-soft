// Package exams is the first cut of F-06: dated exam sessions with a
// place, resources, and the entered credit allowance (RG-36). Bookings,
// results and the credit engine come with the full feature.
//
// A session ties up its resources for its duration plus the round trip
// (RG-140). It is NOT editable from the planner (RG-152): this package is
// its only write path. Cancelling frees the resources; the lessons and
// bookings fallout is handled by the office in the planner.
package exams

import (
	"encoding/json"
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
	mux.HandleFunc("GET /api/exam-places", h.listPlaces)
	mux.HandleFunc("POST /api/exam-places", h.createPlace)
	mux.HandleFunc("PATCH /api/exam-places/{id}", h.patchPlace)

	mux.HandleFunc("GET /api/exam-sessions", h.list)
	mux.HandleFunc("POST /api/exam-sessions", h.create)
	mux.HandleFunc("POST /api/exam-sessions/{id}/cancel", h.cancel)
	h.bookingRoutes(mux)
	h.seatRequestRoutes(mux)
	h.resultRoutes(mux)
}

// ---------------------------------------------------------------------
// Exam places (reference data, RG-140)
// ---------------------------------------------------------------------

type Place struct {
	ID            uuid.UUID `json:"id"`
	Label         string    `json:"label"`
	TravelMinutes int       `json:"travel_minutes"`
	Active        bool      `json:"active"`
}

func (h *Handler) listPlaces(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, label, travel_minutes, active FROM exam_place
		WHERE school_id = $1 ORDER BY label`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []Place{}
	for rows.Next() {
		var p Place
		if err := rows.Scan(&p.ID, &p.Label, &p.TravelMinutes, &p.Active); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

func (h *Handler) createPlace(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	var body struct {
		Label         string `json:"label"`
		TravelMinutes int    `json:"travel_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" || body.TravelMinutes < 0 {
		fail(w, http.StatusUnprocessableEntity, "label and a non-negative travel_minutes are required")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO exam_place (id, school_id, label, travel_minutes)
		VALUES ($1, $2, $3, $4)`, newID, id.SchoolID, body.Label, body.TravelMinutes)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, Place{ID: newID, Label: body.Label, TravelMinutes: body.TravelMinutes, Active: true})
}

func (h *Handler) patchPlace(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	placeID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown place")
		return
	}
	var body struct {
		Label         *string `json:"label"`
		TravelMinutes *int    `json:"travel_minutes"`
		Active        *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE exam_place SET
			label = COALESCE($3, label),
			travel_minutes = COALESCE($4, travel_minutes),
			active = COALESCE($5, active)
		WHERE school_id = $1 AND id = $2`,
		id.SchoolID, placeID, body.Label, body.TravelMinutes, body.Active)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown place")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------

type SessionView struct {
	ID              uuid.UUID `json:"id"`
	PlaceID         uuid.UUID `json:"place_id"`
	PlaceLabel      string    `json:"place_label"`
	TravelMinutes   int       `json:"travel_minutes"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
	CreditAllowance *int      `json:"credit_allowance"`
	Cancelled       bool      `json:"cancelled"`
	CancelReason    string    `json:"cancel_reason"`
	// Derived from the date (RG-44), never stored.
	Past      bool        `json:"past"`
	Resources []uuid.UUID `json:"resources"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT es.id, ep.id, ep.label, ep.travel_minutes,
		       es.starts_at, es.ends_at, es.credit_allowance,
		       es.status = 'CANCELLED', COALESCE(es.cancel_reason, ''),
		       es.ends_at < now(),
		       COALESCE((SELECT array_agg(resource_id) FROM exam_session_resource
		                 WHERE exam_session_id = es.id), '{}')
		FROM exam_session es
		JOIN exam_place ep ON ep.id = es.exam_place_id
		WHERE es.school_id = $1
		ORDER BY es.starts_at DESC`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []SessionView{}
	for rows.Next() {
		var v SessionView
		if err := rows.Scan(&v.ID, &v.PlaceID, &v.PlaceLabel, &v.TravelMinutes,
			&v.StartsAt, &v.EndsAt, &v.CreditAllowance,
			&v.Cancelled, &v.CancelReason, &v.Past, &v.Resources); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	var body struct {
		PlaceID         uuid.UUID   `json:"place_id"`
		StartsAt        time.Time   `json:"starts_at"`
		EndsAt          time.Time   `json:"ends_at"`
		CreditAllowance *int        `json:"credit_allowance"`
		Resources       []uuid.UUID `json:"resources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.PlaceID == uuid.Nil || !body.EndsAt.After(body.StartsAt) {
		fail(w, http.StatusUnprocessableEntity, "place_id and starts_at < ends_at required")
		return
	}

	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	tag, err := tx.Exec(r.Context(), `
		INSERT INTO exam_session (id, school_id, exam_place_id, starts_at, ends_at, credit_allowance)
		SELECT $1, $2, ep.id, $4, $5, $6 FROM exam_place ep
		WHERE ep.school_id = $2 AND ep.id = $3 AND ep.active`,
		newID, id.SchoolID, body.PlaceID, body.StartsAt, body.EndsAt, body.CreditAllowance)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusUnprocessableEntity, "unknown or inactive exam place")
		return
	}
	for _, resID := range body.Resources {
		tag, err := tx.Exec(r.Context(), `
			INSERT INTO exam_session_resource (exam_session_id, resource_id)
			SELECT $1, id FROM resource WHERE school_id = $2 AND id = $3`,
			newID, id.SchoolID, resID)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if tag.RowsAffected() == 0 {
			fail(w, http.StatusUnprocessableEntity, "unknown resource: "+resID.String())
			return
		}
	}
	if err := events.Emit(r.Context(), tx, id.SchoolID, "exam_session.created", "exam_session",
		newID, body, id.UserID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, map[string]uuid.UUID{"id": newID})
}

// cancel frees the resources (the planner stops seeing the session) and
// leaves the fallout to human decisions, traced (RG-63 will grow into a
// disruption case in a later slice).
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
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
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reason == "" {
		fail(w, http.StatusUnprocessableEntity, "a reason is required")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE exam_session
		SET status = 'CANCELLED', cancel_reason = $3, cancelled_by = $4, cancelled_at = now()
		WHERE school_id = $1 AND id = $2 AND status = 'SCHEDULED'`,
		id.SchoolID, sessionID, body.Reason, id.UserID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown or already cancelled session")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "exam_session.cancelled", "exam_session",
		sessionID, body, id.UserID)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Plumbing
// ---------------------------------------------------------------------

func requireManager(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return id, false
	}
	if !id.CanEditPlanning() {
		fail(w, http.StatusForbidden, "exam sessions are read-only for this profile")
		return id, false
	}
	return id, true
}

func requireSettings(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return id, false
	}
	if !id.CanManageSettings() {
		fail(w, http.StatusForbidden, "management only")
		return id, false
	}
	return id, true
}


func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
