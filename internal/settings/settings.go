// Package settings is F-01, slice 1: lesson kinds, standard durations,
// licence categories, opening hours. The rest of F-01 (objectives,
// requirements, alert tuning, reasons, credits) comes in later slices.
//
// One rule shapes everything here: a referenced list value is NEVER
// deleted, it is deactivated (RG-205). D-01 forbids hard-coded business
// values — these tables are where they live instead.
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings/lesson-kinds", h.listLessonKinds)
	mux.HandleFunc("POST /api/settings/lesson-kinds", h.createLessonKind)
	mux.HandleFunc("PATCH /api/settings/lesson-kinds/{id}", h.patchLessonKind)

	mux.HandleFunc("GET /api/settings/lesson-durations", h.listDurations)
	mux.HandleFunc("POST /api/settings/lesson-durations", h.createDuration)
	mux.HandleFunc("PATCH /api/settings/lesson-durations/{id}", h.patchDuration)

	mux.HandleFunc("GET /api/settings/licence-categories", h.listCategories)
	mux.HandleFunc("POST /api/settings/licence-categories", h.createCategory)
	mux.HandleFunc("PATCH /api/settings/licence-categories/{id}", h.patchCategory)

	mux.HandleFunc("GET /api/settings/opening-hours", h.listOpening)
	mux.HandleFunc("PUT /api/settings/opening-hours", h.replaceOpening)

	mux.HandleFunc("GET /api/absence-reasons", h.listAbsenceReasons)
	mux.HandleFunc("POST /api/absence-reasons", h.createAbsenceReason)
	mux.HandleFunc("PATCH /api/absence-reasons/{id}", h.patchAbsenceReason)
}

// Absence reasons: read by everyone (the instructor's screen shows them
// as chips), managed by the management like every reference list.
type AbsenceReason struct {
	ID     uuid.UUID `json:"id"`
	Label  string    `json:"label"`
	Active bool      `json:"active"`
}

func (h *Handler) listAbsenceReasons(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	rows, err := h.store.pool.Query(r.Context(), `
		SELECT id, label, active FROM absence_reason
		WHERE school_id = $1 ORDER BY label`, id.SchoolID)
	listReply(w, rows, err, func(row pgx.Rows) (AbsenceReason, error) {
		var a AbsenceReason
		err := row.Scan(&a.ID, &a.Label, &a.Active)
		return a, err
	})
}

func (h *Handler) createAbsenceReason(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	if err := decode(r, &body); err != nil || body.Label == "" {
		fail(w, http.StatusUnprocessableEntity, "label required")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.store.pool.Exec(r.Context(), `
		INSERT INTO absence_reason (id, school_id, label) VALUES ($1, $2, $3)`,
		newID, id.SchoolID, body.Label)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, AbsenceReason{ID: newID, Label: body.Label, Active: true})
}

func (h *Handler) patchAbsenceReason(w http.ResponseWriter, r *http.Request) {
	h.patchValue(w, r, func(ctx context.Context, schoolID, targetID uuid.UUID, body patchBody) (int64, error) {
		tag, err := h.store.pool.Exec(ctx, `
			UPDATE absence_reason SET
				label = COALESCE($3, label),
				active = COALESCE($4, active)
			WHERE school_id = $1 AND id = $2`,
			schoolID, targetID, body.Label, body.Active)
		return tag.RowsAffected(), err
	})
}

// requireSettings: management only (slice-1 hypothesis, see auth).
// Reading is open to any signed-in profile: the planner shows durations
// and kinds to everyone.
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

func identityOr401(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
	}
	return id, ok
}

// ---------------------------------------------------------------------
// Lesson kinds
// ---------------------------------------------------------------------

type LessonKind struct {
	ID              uuid.UUID `json:"id"`
	Label           string    `json:"label"`
	RequiresVehicle bool      `json:"requires_vehicle"`
	Active          bool      `json:"active"`
}

func (h *Handler) listLessonKinds(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	rows, err := h.store.pool.Query(r.Context(), `
		SELECT id, label, requires_vehicle, active FROM lesson_kind
		WHERE school_id = $1 ORDER BY label`, id.SchoolID)
	listReply(w, rows, err, func(row pgx.Rows) (LessonKind, error) {
		var k LessonKind
		err := row.Scan(&k.ID, &k.Label, &k.RequiresVehicle, &k.Active)
		return k, err
	})
}

func (h *Handler) createLessonKind(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	var body struct {
		Label           string `json:"label"`
		RequiresVehicle bool   `json:"requires_vehicle"`
	}
	if err := decode(r, &body); err != nil || body.Label == "" {
		fail(w, http.StatusUnprocessableEntity, "label required")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.store.pool.Exec(r.Context(), `
		INSERT INTO lesson_kind (id, school_id, label, requires_vehicle)
		VALUES ($1, $2, $3, $4)`, newID, id.SchoolID, body.Label, body.RequiresVehicle)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, LessonKind{ID: newID, Label: body.Label, RequiresVehicle: body.RequiresVehicle, Active: true})
}

func (h *Handler) patchLessonKind(w http.ResponseWriter, r *http.Request) {
	h.patchValue(w, r, func(ctx context.Context, schoolID, targetID uuid.UUID, body patchBody) (int64, error) {
		tag, err := h.store.pool.Exec(ctx, `
			UPDATE lesson_kind SET
				label = COALESCE($3, label),
				requires_vehicle = COALESCE($4, requires_vehicle),
				active = COALESCE($5, active)
			WHERE school_id = $1 AND id = $2`,
			schoolID, targetID, body.Label, body.RequiresVehicle, body.Active)
		return tag.RowsAffected(), err
	})
}

// ---------------------------------------------------------------------
// Standard durations
// ---------------------------------------------------------------------

type Duration struct {
	ID      uuid.UUID `json:"id"`
	Minutes int       `json:"minutes"`
	Active  bool      `json:"active"`
}

func (h *Handler) listDurations(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	rows, err := h.store.pool.Query(r.Context(), `
		SELECT id, minutes, active FROM standard_lesson_duration
		WHERE school_id = $1 ORDER BY minutes`, id.SchoolID)
	listReply(w, rows, err, func(row pgx.Rows) (Duration, error) {
		var d Duration
		err := row.Scan(&d.ID, &d.Minutes, &d.Active)
		return d, err
	})
}

func (h *Handler) createDuration(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	var body struct {
		Minutes int `json:"minutes"`
	}
	if err := decode(r, &body); err != nil || body.Minutes <= 0 {
		fail(w, http.StatusUnprocessableEntity, "minutes must be positive")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.store.pool.Exec(r.Context(), `
		INSERT INTO standard_lesson_duration (id, school_id, minutes)
		VALUES ($1, $2, $3)`, newID, id.SchoolID, body.Minutes)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, Duration{ID: newID, Minutes: body.Minutes, Active: true})
}

func (h *Handler) patchDuration(w http.ResponseWriter, r *http.Request) {
	h.patchValue(w, r, func(ctx context.Context, schoolID, targetID uuid.UUID, body patchBody) (int64, error) {
		tag, err := h.store.pool.Exec(ctx, `
			UPDATE standard_lesson_duration SET active = COALESCE($3, active)
			WHERE school_id = $1 AND id = $2`,
			schoolID, targetID, body.Active)
		return tag.RowsAffected(), err
	})
}

// ---------------------------------------------------------------------
// Licence categories
// ---------------------------------------------------------------------

type Category struct {
	ID     uuid.UUID `json:"id"`
	Code   string    `json:"code"`
	Active bool      `json:"active"`
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	rows, err := h.store.pool.Query(r.Context(), `
		SELECT id, code, active FROM licence_category
		WHERE school_id = $1 ORDER BY code`, id.SchoolID)
	listReply(w, rows, err, func(row pgx.Rows) (Category, error) {
		var c Category
		err := row.Scan(&c.ID, &c.Code, &c.Active)
		return c, err
	})
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := decode(r, &body); err != nil || body.Code == "" {
		fail(w, http.StatusUnprocessableEntity, "code required")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.store.pool.Exec(r.Context(), `
		INSERT INTO licence_category (id, school_id, code)
		VALUES ($1, $2, $3)`, newID, id.SchoolID, body.Code)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "code already exists or invalid")
		return
	}
	reply(w, http.StatusCreated, Category{ID: newID, Code: body.Code, Active: true})
}

func (h *Handler) patchCategory(w http.ResponseWriter, r *http.Request) {
	h.patchValue(w, r, func(ctx context.Context, schoolID, targetID uuid.UUID, body patchBody) (int64, error) {
		tag, err := h.store.pool.Exec(ctx, `
			UPDATE licence_category SET
				code = COALESCE($3, code),
				active = COALESCE($4, active)
			WHERE school_id = $1 AND id = $2`,
			schoolID, targetID, body.Code, body.Active)
		return tag.RowsAffected(), err
	})
}

// patchBody covers every list value: label/code, the vehicle flag, and
// `active` — deactivation being the only removal there is (RG-205).
type patchBody struct {
	Label           *string `json:"label"`
	Code            *string `json:"code"`
	RequiresVehicle *bool   `json:"requires_vehicle"`
	Active          *bool   `json:"active"`
}

func (h *Handler) patchValue(w http.ResponseWriter, r *http.Request,
	update func(context.Context, uuid.UUID, uuid.UUID, patchBody) (int64, error)) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	targetID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown value")
		return
	}
	var body patchBody
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	affected, err := update(r.Context(), id.SchoolID, targetID, body)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if affected == 0 {
		fail(w, http.StatusNotFound, "unknown value")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Opening hours
// ---------------------------------------------------------------------

type OpeningHour struct {
	Weekday int    `json:"weekday"` // ISO: 1 = Monday … 7 = Sunday
	Start   string `json:"start"`   // "08:00"
	End     string `json:"end"`
}

func (h *Handler) listOpening(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	rows, err := h.store.pool.Query(r.Context(), `
		SELECT weekday, to_char(starts_at, 'HH24:MI'), to_char(ends_at, 'HH24:MI')
		FROM opening_hours WHERE school_id = $1 ORDER BY weekday, starts_at`, id.SchoolID)
	listReply(w, rows, err, func(row pgx.Rows) (OpeningHour, error) {
		var o OpeningHour
		err := row.Scan(&o.Weekday, &o.Start, &o.End)
		return o, err
	})
}

// replaceOpening swaps the whole week in one transaction: opening hours
// are a single coherent object, not seven independent rows.
func (h *Handler) replaceOpening(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSettings(w, r)
	if !ok {
		return
	}
	var body []OpeningHour
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	for _, o := range body {
		if o.Weekday < 1 || o.Weekday > 7 || !validHour(o.Start) || !validHour(o.End) || o.End <= o.Start {
			fail(w, http.StatusUnprocessableEntity, "each entry needs weekday 1–7 and start < end (HH:MM)")
			return
		}
	}

	err := h.store.withTx(r.Context(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(),
			`DELETE FROM opening_hours WHERE school_id = $1`, id.SchoolID); err != nil {
			return err
		}
		for _, o := range body {
			rowID, err := uuid.NewV7()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO opening_hours (id, school_id, weekday, starts_at, ends_at)
				VALUES ($1, $2, $3, $4::time, $5::time)`,
				rowID, id.SchoolID, o.Weekday, o.Start, o.End); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Store) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validHour(v string) bool {
	_, err := time.Parse("15:04", v)
	return err == nil
}

// ---------------------------------------------------------------------
// Plumbing
// ---------------------------------------------------------------------

func decode(r *http.Request, into any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	return json.NewDecoder(r.Body).Decode(into)
}

func listReply[T any](w http.ResponseWriter, rows pgx.Rows, err error, scan func(pgx.Rows) (T, error)) {
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
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

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
