// Package enrollment is F-04, slice 1: student + objective + target
// dates. Cut from the slice: waivers, markers, closure, threshold alerts.
//
// Two rules do the real work here:
//   - Thresholds are COPIED from the objective at creation, with no live
//     link (C-07, RG-01, RG-42). The absence of any later read of the
//     objective is what makes the copy real.
//   - The on-road test never precedes the off-road one (RG-24): that is
//     a structural refusal, one of the three the API conventions allow.
package enrollment

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	mux.HandleFunc("GET /api/objectives", h.listObjectives)
	mux.HandleFunc("POST /api/objectives", h.createObjective)
	mux.HandleFunc("PATCH /api/objectives/{id}", h.patchObjective)
	mux.HandleFunc("POST /api/enrollments", h.create)
	mux.HandleFunc("PATCH /api/enrollments/{id}", h.patch)
	h.hoursRoutes(mux)
	h.fundingRoutes(mux)
}

// listObjectives serves the creation form. Objectives themselves are
// F-01 territory (out of slice 1): read-only here.
func (h *Handler) listObjectives(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	// ?all=1 includes deactivated objectives (settings screen).
	all := r.URL.Query().Get("all") == "1"
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, label, hours_before_offroad, total_hours, active
		FROM objective WHERE school_id = $1 AND (active OR $2) ORDER BY label`,
		id.SchoolID, all)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type objective struct {
		ID                 uuid.UUID `json:"id"`
		Label              string    `json:"label"`
		HoursBeforeOffroad float64   `json:"hours_before_offroad"`
		TotalHours         float64   `json:"total_hours"`
		Active             bool      `json:"active"`
	}
	out := []objective{}
	for rows.Next() {
		var o objective
		if err := rows.Scan(&o.ID, &o.Label, &o.HoursBeforeOffroad, &o.TotalHours, &o.Active); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

// createObjective / patchObjective: the settings side of F-01. Values on
// EXISTING enrollments never move (C-07): they were copied. Propagation
// with its diverging-population preview is the full F-01, later.
func (h *Handler) createObjective(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || !id.CanManageSettings() {
		fail(w, http.StatusForbidden, "management only")
		return
	}
	var body struct {
		Label              string  `json:"label"`
		HoursBeforeOffroad float64 `json:"hours_before_offroad"`
		TotalHours         float64 `json:"total_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" ||
		body.HoursBeforeOffroad <= 0 || body.TotalHours < body.HoursBeforeOffroad {
		fail(w, http.StatusUnprocessableEntity, "label et heures cohérentes (plateau ≤ total) requis")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO objective (id, school_id, label, hours_before_offroad, total_hours)
		VALUES ($1, $2, $3, $4, $5)`,
		newID, id.SchoolID, body.Label, body.HoursBeforeOffroad, body.TotalHours)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, map[string]uuid.UUID{"id": newID})
}

func (h *Handler) patchObjective(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || !id.CanManageSettings() {
		fail(w, http.StatusForbidden, "management only")
		return
	}
	objectiveID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown objective")
		return
	}
	var body struct {
		Label              *string  `json:"label"`
		HoursBeforeOffroad *float64 `json:"hours_before_offroad"`
		TotalHours         *float64 `json:"total_hours"`
		Active             *bool    `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE objective SET
			label = COALESCE($3, label),
			hours_before_offroad = COALESCE($4, hours_before_offroad),
			total_hours = COALESCE($5, total_hours),
			active = COALESCE($6, active)
		WHERE school_id = $1 AND id = $2
		  AND COALESCE($5, total_hours) >= COALESCE($4, hours_before_offroad)`,
		id.SchoolID, objectiveID, body.Label, body.HoursBeforeOffroad, body.TotalHours, body.Active)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusUnprocessableEntity, "objectif inconnu ou heures incohérentes")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "objective.updated", "objective",
		objectiveID, body, id.UserID)
	w.WriteHeader(http.StatusNoContent)
}

type enrollmentBody struct {
	PersonID          uuid.UUID  `json:"person_id"`
	ObjectiveID       uuid.UUID  `json:"objective_id"`
	OffroadTargetDate *time.Time `json:"offroad_target_date"`
	OnroadTargetDate  *time.Time `json:"onroad_target_date"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	var body enrollmentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.PersonID == uuid.Nil || body.ObjectiveID == uuid.Nil {
		fail(w, http.StatusUnprocessableEntity, "person_id and objective_id required")
		return
	}
	if badDates(body.OffroadTargetDate, body.OnroadTargetDate) {
		fail(w, http.StatusUnprocessableEntity, "on-road date cannot precede the off-road one (RG-24)")
		return
	}

	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = h.withTx(r, func(tx pgx.Tx) error {
		// The INSERT..SELECT is the copy of C-07: thresholds leave the
		// objective once, here, and never look back.
		tag, err := tx.Exec(r.Context(), `
			INSERT INTO enrollment
			    (id, school_id, person_id, objective_id, hours_before_offroad, total_hours,
			     offroad_target_date, onroad_target_date)
			SELECT $1, $2, p.id, o.id, o.hours_before_offroad, o.total_hours, $5, $6
			FROM person p, objective o
			WHERE p.school_id = $2 AND p.id = $3 AND p.status = 'ACTIVE'
			  AND o.school_id = $2 AND o.id = $4 AND o.active`,
			newID, id.SchoolID, body.PersonID, body.ObjectiveID,
			body.OffroadTargetDate, body.OnroadTargetDate)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errUnknownTarget
		}
		// F-05: the funding file is born with the enrollment, À MONTER.
		fundingID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO funding (id, school_id, enrollment_id) VALUES ($1, $2, $3)`,
			fundingID, id.SchoolID, newID); err != nil {
			return err
		}
		// F-29: the requirements are COPIED from the objective's active
		// templates, in the same transaction (C-07). The file starts with
		// its two sets, everything NOT_VALIDATED.
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO enrollment_requirement
			    (id, school_id, enrollment_id, template_id, label, req_set,
			     mandatory, instructor_may_validate, validity_months)
			SELECT gen_random_uuid(), $2, $1, rt.id, rt.label, rt.req_set,
			       rt.mandatory, rt.instructor_may_validate, rt.validity_months
			FROM requirement_template rt
			WHERE rt.school_id = $2 AND rt.objective_id = $3 AND rt.active`,
			newID, id.SchoolID, body.ObjectiveID); err != nil {
			return err
		}
		return events.Emit(r.Context(), tx, id.SchoolID, "enrollment.created", "enrollment",
			newID, body, id.UserID)
	})

	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, errUnknownTarget):
		fail(w, http.StatusUnprocessableEntity, "unknown or inactive person or objective")
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		// The partial unique index: one ACTIVE enrollment per person (RG-115).
		fail(w, http.StatusConflict, "cette personne a déjà un parcours actif")
	case err != nil:
		fail(w, http.StatusInternalServerError, err.Error())
	default:
		reply(w, http.StatusCreated, map[string]uuid.UUID{"id": newID})
	}
}

// patch moves the target dates (slice 1 keeps only that; waivers and
// closure come later). RG-24 is also a CHECK in the schema — the refusal
// here just says it in clear text before the database would.
func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	enrollmentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown enrollment")
		return
	}
	var body struct {
		OffroadTargetDate *time.Time `json:"offroad_target_date"`
		OnroadTargetDate  *time.Time `json:"onroad_target_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE enrollment SET
			offroad_target_date = COALESCE($3, offroad_target_date),
			onroad_target_date = COALESCE($4, onroad_target_date)
		WHERE school_id = $1 AND id = $2 AND life_status = 'ACTIVE'`,
		id.SchoolID, enrollmentID, body.OffroadTargetDate, body.OnroadTargetDate)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" { // check_violation
		fail(w, http.StatusUnprocessableEntity, "on-road date cannot precede the off-road one (RG-24)")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown or closed enrollment")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "enrollment.target_moved", "enrollment",
		enrollmentID, body, id.UserID)
	w.WriteHeader(http.StatusNoContent)
}

func badDates(offroad, onroad *time.Time) bool {
	return offroad != nil && onroad != nil && onroad.Before(*offroad)
}

var errUnknownTarget = errors.New("unknown target")

func (h *Handler) withTx(r *http.Request, fn func(pgx.Tx) error) error {
	tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func requireManager(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return id, false
	}
	if !id.CanManagePeople() {
		fail(w, http.StatusForbidden, "enrollments are read-only for this profile")
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
