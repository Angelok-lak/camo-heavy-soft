// Package resources is F-15 and F-16, slice 1: create, list, archive,
// declare an unavailability. One set of operations, the resource kind is
// an attribute (C-08). Cut from the slice: return to service, disruption
// handling, instructor hour totals.
//
// D-04 shapes the one delicate operation here: declaring an
// unavailability NEVER touches the lessons using the resource. They keep
// their assignment and the declaration comes back with the impact list —
// informational, actionable, non-blocking (RG-80).
package resources

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
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
	mux.HandleFunc("GET /api/resources", h.list)
	mux.HandleFunc("POST /api/resources", h.create)
	mux.HandleFunc("PATCH /api/resources/{id}", h.patch)
	mux.HandleFunc("GET /api/resources/{id}", h.detail)
	mux.HandleFunc("GET /api/resources/{id}/unavailabilities", h.listUnavailabilities)
	mux.HandleFunc("POST /api/resources/{id}/unavailabilities", h.declareUnavailability)
	mux.HandleFunc("POST /api/unavailabilities/{id}/restore", h.restore)
}

// restore is the return to service (RG-66): the unavailability ends, and
// the answer lists the lessons that stayed assigned through the outage —
// they are valid again as they are, kept on purpose all along (RG-80).
func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	unavailabilityID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown unavailability")
		return
	}
	// The note is optional: what was fixed, or why the return was decided.
	var body struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	var note *string
	if s := strings.TrimSpace(body.Note); s != "" {
		note = &s
	}

	var resourceID uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		UPDATE unavailability
		SET status = 'ENDED', restored_by = $3, restored_at = now(),
		    restored_note = $4,
		    ends_at = COALESCE(ends_at, now())
		WHERE school_id = $1 AND id = $2 AND status = 'ONGOING'
		RETURNING resource_id`,
		id.SchoolID, unavailabilityID, id.UserID, note).Scan(&resourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown or already ended unavailability")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "unavailability.restored", "resource",
		resourceID, map[string]any{"note": body.Note}, id.UserID)

	kept, err := h.impactedLessons(r, id.SchoolID, resourceID, time.Now(), nil)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, map[string]any{"kept_lessons": kept})
}

// detail is the resource's file: identity, its unavailability history and
// the planned lessons that count on it.
func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	resourceID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown resource")
		return
	}

	var kind, label, status string
	err = h.pool.QueryRow(r.Context(), `
		SELECT kind, label, status FROM resource
		WHERE school_id = $1 AND id = $2`, id.SchoolID, resourceID,
	).Scan(&kind, &label, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown resource")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	unavailabilities, err := h.unavailabilitiesOf(r, id.SchoolID, resourceID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	upcoming, err := h.impactedLessons(r, id.SchoolID, resourceID, time.Now(), nil)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, map[string]any{
		"id": resourceID, "kind": kind, "label": label, "status": status,
		"unavailabilities": unavailabilities,
		"upcoming_lessons": upcoming,
	})
}

// ---------------------------------------------------------------------
// Views
// ---------------------------------------------------------------------

type Unavailability struct {
	ID           uuid.UUID  `json:"id"`
	Reason       string     `json:"reason"`
	StartsAt     time.Time  `json:"starts_at"`
	EndsAt       *time.Time `json:"ends_at"` // nil = return unknown (RG-65)
	Status       string     `json:"status"`
	RestoredNote *string    `json:"restored_note"`
}

// ResourceView carries the current state the list needs: is it out right
// now, and how many planned lessons still count on it — the number the
// office looks at before declaring a breakdown (F-15 contract).
type ResourceView struct {
	ID                 uuid.UUID  `json:"id"`
	Kind               string     `json:"kind"`
	Label              string     `json:"label"`
	Status             string     `json:"status"`
	Categories         []string   `json:"categories"`
	IndicativeCapacity *int16     `json:"indicative_capacity"`
	LinkedUserID       *uuid.UUID `json:"linked_user_id"`
	OutRightNow        bool       `json:"out_right_now"`
	FutureLessons      int        `json:"future_lessons"`
	// The ONGOING declaration, when there is one — what the list needs
	// to offer the return to service without opening the file.
	Ongoing *OngoingUnavailability `json:"ongoing_unavailability"`
}

type OngoingUnavailability struct {
	ID       uuid.UUID  `json:"id"`
	Reason   string     `json:"reason"`
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
}

// ImpactedLesson is one line of the impact list joined to a declaration:
// enough to find the lesson and act, nothing decided in its name.
type ImpactedLesson struct {
	ID       uuid.UUID `json:"id"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Students int       `json:"students"`
}

// ---------------------------------------------------------------------
// List
// ---------------------------------------------------------------------

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}

	kind := r.URL.Query().Get("kind")
	rows, err := h.pool.Query(r.Context(), `
		SELECT r.id, r.kind, r.label, r.status,
		       COALESCE((SELECT array_agg(lc.code ORDER BY lc.code)
		                 FROM resource_vehicle_category rvc
		                 JOIN licence_category lc ON lc.id = rvc.category_id
		                 WHERE rvc.resource_id = r.id), '{}'),
		       rv.indicative_capacity,
		       ri.user_id,
		       EXISTS (SELECT 1 FROM unavailability u
		               WHERE u.resource_id = r.id AND u.status = 'ONGOING'
		                 AND u.starts_at <= now()
		                 AND (u.ends_at IS NULL OR u.ends_at > now())),
		       ong.id, ong.reason, ong.starts_at, ong.ends_at,
		       (SELECT count(*) FROM lesson_resource lr
		        JOIN lesson l ON l.id = lr.lesson_id
		        WHERE lr.resource_id = r.id AND l.status = 'PLANNED' AND l.starts_at > now())
		FROM resource r
		LEFT JOIN resource_vehicle rv ON rv.resource_id = r.id
		LEFT JOIN resource_instructor ri ON ri.resource_id = r.id
		LEFT JOIN LATERAL (
			SELECT u.id, u.reason, u.starts_at, u.ends_at
			FROM unavailability u
			WHERE u.resource_id = r.id AND u.status = 'ONGOING'
			  AND (u.ends_at IS NULL OR u.ends_at > now())
			ORDER BY u.starts_at LIMIT 1
		) ong ON true
		WHERE r.school_id = $1 AND ($2 = '' OR r.kind::text = $2)
		ORDER BY r.kind, r.label`, id.SchoolID, kind)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []ResourceView{}
	for rows.Next() {
		var v ResourceView
		var ongID *uuid.UUID
		var ongReason *string
		var ongStarts, ongEnds *time.Time
		if err := rows.Scan(&v.ID, &v.Kind, &v.Label, &v.Status, &v.Categories,
			&v.IndicativeCapacity, &v.LinkedUserID, &v.OutRightNow,
			&ongID, &ongReason, &ongStarts, &ongEnds, &v.FutureLessons); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if ongID != nil {
			v.Ongoing = &OngoingUnavailability{ID: *ongID, Reason: *ongReason,
				StartsAt: *ongStarts, EndsAt: ongEnds}
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

// ---------------------------------------------------------------------
// Create / modify / archive
// ---------------------------------------------------------------------

type resourceBody struct {
	Kind               string   `json:"kind"`
	Label              string   `json:"label"`
	Categories         []string `json:"categories"`
	IndicativeCapacity *int16   `json:"indicative_capacity"`
}

var validKinds = map[string]bool{
	"VEHICLE": true, "ROOM": true, "TRAINING_PAD": true, "INSTRUCTOR": true,
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	var body resourceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" || !validKinds[body.Kind] {
		fail(w, http.StatusUnprocessableEntity, "kind (VEHICLE/ROOM/TRAINING_PAD/INSTRUCTOR) and label required")
		return
	}

	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = h.withTx(r, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO resource (id, school_id, kind, label)
			VALUES ($1, $2, $3, $4)`, newID, id.SchoolID, body.Kind, body.Label); err != nil {
			return err
		}
		switch body.Kind {
		case "VEHICLE":
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO resource_vehicle (resource_id, indicative_capacity)
				VALUES ($1, $2)`, newID, body.IndicativeCapacity); err != nil {
				return err
			}
			if err := h.setCategories(r, tx, id.SchoolID, newID, body.Categories); err != nil {
				return err
			}
		case "INSTRUCTOR":
			if _, err := tx.Exec(r.Context(), `
				INSERT INTO resource_instructor (resource_id) VALUES ($1)`, newID); err != nil {
				return err
			}
		}
		return events.Emit(r.Context(), tx, id.SchoolID, "resource.created", "resource", newID,
			map[string]string{"kind": body.Kind, "label": body.Label}, id.UserID)
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, map[string]uuid.UUID{"id": newID})
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	resourceID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown resource")
		return
	}
	var body struct {
		Label              *string   `json:"label"`
		Status             *string   `json:"status"` // ACTIVE / ARCHIVED (RG-73)
		Categories         *[]string `json:"categories"`
		IndicativeCapacity *int16    `json:"indicative_capacity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	if body.Status != nil && *body.Status != "ACTIVE" && *body.Status != "ARCHIVED" {
		fail(w, http.StatusUnprocessableEntity, "status is ACTIVE or ARCHIVED")
		return
	}

	err = h.withTx(r, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `
			UPDATE resource SET
				label = COALESCE($3, label),
				status = COALESCE($4, status)
			WHERE school_id = $1 AND id = $2`,
			id.SchoolID, resourceID, body.Label, body.Status)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFound
		}
		if body.IndicativeCapacity != nil {
			if _, err := tx.Exec(r.Context(), `
				UPDATE resource_vehicle SET indicative_capacity = $2
				WHERE resource_id = $1`, resourceID, body.IndicativeCapacity); err != nil {
				return err
			}
		}
		if body.Categories != nil {
			if _, err := tx.Exec(r.Context(), `
				DELETE FROM resource_vehicle_category WHERE resource_id = $1`, resourceID); err != nil {
				return err
			}
			if err := h.setCategories(r, tx, id.SchoolID, resourceID, *body.Categories); err != nil {
				return err
			}
		}
		if body.Status != nil {
			// Archiving with future lessons is ACCEPTED (RG-73): the
			// impact surfaces as conflicts on those lessons, it is not a
			// reason to refuse.
			return events.Emit(r.Context(), tx, id.SchoolID, "resource.status_changed", "resource",
				resourceID, map[string]string{"status": *body.Status}, id.UserID)
		}
		return nil
	})
	if errors.Is(err, errNotFound) {
		fail(w, http.StatusNotFound, "unknown resource")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setCategories(r *http.Request, tx pgx.Tx, schoolID, resourceID uuid.UUID, codes []string) error {
	for _, code := range codes {
		tag, err := tx.Exec(r.Context(), `
			INSERT INTO resource_vehicle_category (resource_id, category_id)
			SELECT $1, id FROM licence_category WHERE school_id = $2 AND code = $3
			ON CONFLICT DO NOTHING`, resourceID, schoolID, code)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("unknown licence category: " + code)
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Unavailability
// ---------------------------------------------------------------------

func (h *Handler) listUnavailabilities(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	resourceID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown resource")
		return
	}
	out, err := h.unavailabilitiesOf(r, id.SchoolID, resourceID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

func (h *Handler) unavailabilitiesOf(r *http.Request, schoolID, resourceID uuid.UUID) ([]Unavailability, error) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, reason, starts_at, ends_at, status, restored_note
		FROM unavailability
		WHERE school_id = $1 AND resource_id = $2
		ORDER BY starts_at DESC`, schoolID, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Unavailability{}
	for rows.Next() {
		var u Unavailability
		if err := rows.Scan(&u.ID, &u.Reason, &u.StartsAt, &u.EndsAt, &u.Status, &u.RestoredNote); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// declareUnavailability records the fact and answers with the impact:
// which planned lessons sit on the window. It touches NONE of them
// (RG-80) — the office decides, lesson by lesson, in the planner.
func (h *Handler) declareUnavailability(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	resourceID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown resource")
		return
	}
	var body struct {
		Reason   string     `json:"reason"`
		StartsAt time.Time  `json:"starts_at"`
		EndsAt   *time.Time `json:"ends_at"` // optional: unknown return (RG-65)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reason == "" || body.StartsAt.IsZero() {
		fail(w, http.StatusUnprocessableEntity, "reason and starts_at required")
		return
	}
	if body.EndsAt != nil && !body.EndsAt.After(body.StartsAt) {
		fail(w, http.StatusUnprocessableEntity, "ends_at must follow starts_at")
		return
	}

	// Overlapping declarations are allowed, no merge (RG-79).
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = h.withTx(r, func(tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `
			INSERT INTO unavailability (id, school_id, resource_id, reason, starts_at, ends_at, declared_by)
			SELECT $1, $2, r.id, $4, $5, $6, $7 FROM resource r
			WHERE r.school_id = $2 AND r.id = $3`,
			newID, id.SchoolID, resourceID, body.Reason, body.StartsAt, body.EndsAt, id.UserID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFound
		}
		return events.Emit(r.Context(), tx, id.SchoolID, "unavailability.declared", "resource",
			resourceID, body, id.UserID)
	})
	if errors.Is(err, errNotFound) {
		fail(w, http.StatusNotFound, "unknown resource")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	impacted, err := h.impactedLessons(r, id.SchoolID, resourceID, body.StartsAt, body.EndsAt)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, map[string]any{
		"unavailability": Unavailability{
			ID: newID, Reason: body.Reason, StartsAt: body.StartsAt,
			EndsAt: body.EndsAt, Status: "ONGOING",
		},
		// The impact list rides along (D-06 shape): seen, not acted on.
		"impacted_lessons": impacted,
	})
}

func (h *Handler) impactedLessons(r *http.Request, schoolID, resourceID uuid.UUID, from time.Time, to *time.Time) ([]ImpactedLesson, error) {
	// An open end means "until further notice": everything planned after
	// the start is concerned.
	rows, err := h.pool.Query(r.Context(), `
		SELECT l.id, l.starts_at, l.ends_at,
		       (SELECT count(*) FROM lesson_assignment la WHERE la.lesson_id = l.id)
		FROM lesson l
		JOIN lesson_resource lr ON lr.lesson_id = l.id
		WHERE l.school_id = $1 AND lr.resource_id = $2 AND l.status = 'PLANNED'
		  AND l.ends_at > $3
		  AND ($4::timestamptz IS NULL OR l.starts_at < $4)
		ORDER BY l.starts_at`, schoolID, resourceID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ImpactedLesson{}
	for rows.Next() {
		var l ImpactedLesson
		if err := rows.Scan(&l.ID, &l.StartsAt, &l.EndsAt, &l.Students); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------
// Plumbing
// ---------------------------------------------------------------------

var errNotFound = errors.New("not found")

func requireManager(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return id, false
	}
	if !id.CanManageResources() {
		fail(w, http.StatusForbidden, "resources are read-only for this profile")
		return id, false
	}
	return id, true
}

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

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
