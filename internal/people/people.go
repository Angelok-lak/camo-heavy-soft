// Package people is F-02, slice 1: name, first names, contact details —
// create, modify, list, search. Cut from the slice: documents, NEPH,
// duplicate detection, merge.
//
// The person OUTLIVES their enrollments (data model §5): archiving is the
// only removal, and it is refused while an ACTIVE enrollment exists
// (RG-184) — a structural refusal, not a business rule.
package people

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
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
	mux.HandleFunc("GET /api/persons", h.list)
	mux.HandleFunc("POST /api/persons", h.create)
	mux.HandleFunc("PATCH /api/persons/{id}", h.patch)
	mux.HandleFunc("GET /api/persons/{id}/history", h.history)
	h.requirementRoutes(mux)
}

type PersonView struct {
	ID          uuid.UUID  `json:"id"`
	LastName    string     `json:"last_name"`
	FirstNames  string     `json:"first_names"`
	DateOfBirth *time.Time `json:"date_of_birth"`
	Phone       *string    `json:"phone"`
	Email       *string    `json:"email"`
	Neph        *string    `json:"neph"`
	Status      string     `json:"status"`
	// The active enrollment rides along: the list is where the office
	// checks "is this student engaged, on what, for when".
	Enrollment *EnrollmentSummary `json:"enrollment"`
	Health     Health             `json:"health"`
}

// EnrollmentSummary carries what the list card shows, counters included
// — computed by the enrollment_hours view, never here (C-05).
type EnrollmentSummary struct {
	ID                uuid.UUID  `json:"id"`
	Objective         string     `json:"objective"`
	OffroadTargetDate *time.Time `json:"offroad_target_date"`
	OnroadTargetDate  *time.Time `json:"onroad_target_date"`
	ConsumedHours     float64    `json:"consumed_hours"`
	TotalHours        float64    `json:"total_hours"`
	UpcomingLessons   int        `json:"upcoming_lessons"`
	FundingStatus     string     `json:"funding_status"`
	FunderLabel       string     `json:"funder_label"`
}

// Health is the list's traffic light, computed HERE (C-26): the front
// shows the dot and the reasons, it decides nothing.
type Health struct {
	Color   string   `json:"color"` // green / amber / red
	Reasons []string `json:"reasons"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	search := r.URL.Query().Get("search")

	// One search box, every identifier the office knows a student by:
	// name, NEPH, email, phone.
	rows, err := h.pool.Query(r.Context(), `
		SELECT p.id, p.last_name, p.first_names, p.date_of_birth, p.phone, p.email, p.neph, p.status,
		       e.id, o.label, e.offroad_target_date, e.onroad_target_date,
		       COALESCE(eh.consumed_hours, 0), COALESCE(eh.total_hours, 0),
		       COALESCE((SELECT count(*) FROM lesson_assignment la
		                 JOIN lesson l ON l.id = la.lesson_id
		                 WHERE la.enrollment_id = e.id AND l.status = 'PLANNED'
		                   AND l.starts_at > now()), 0),
		       COALESCE(f.status::text, ''), COALESCE(fk.label, ''),
		       COALESCE(fk.self_funded, false),
		       COALESCE(eh.projected_offroad_hours < eh.hours_before_offroad, false)
		         OR COALESCE(eh.projected_onroad_hours < eh.total_hours, false),
		       COALESCE((SELECT count(*) FROM enrollment_requirement er
		                 WHERE er.enrollment_id = e.id AND er.req_set = 'ENTRY'
		                   AND er.mandatory AND er.status = 'NOT_VALIDATED'), 0)
		FROM person p
		LEFT JOIN enrollment e ON e.person_id = p.id AND e.life_status = 'ACTIVE'
		LEFT JOIN objective o ON o.id = e.objective_id
		LEFT JOIN enrollment_hours eh ON eh.enrollment_id = e.id
		LEFT JOIN funding f ON f.enrollment_id = e.id
		LEFT JOIN funder_kind fk ON fk.id = f.funder_kind_id
		WHERE p.school_id = $1
		  AND ($2 = ''
		       OR p.last_name ILIKE '%' || $2 || '%'
		       OR p.first_names ILIKE '%' || $2 || '%'
		       OR p.neph ILIKE '%' || $2 || '%'
		       OR p.email ILIKE '%' || $2 || '%'
		       OR p.phone ILIKE '%' || $2 || '%')
		ORDER BY p.last_name, p.first_names
		LIMIT 200`, id.SchoolID, search)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []PersonView{}
	for rows.Next() {
		var v PersonView
		var enrID *uuid.UUID
		var objective *string
		var offroad, onroad *time.Time
		var consumed, total float64
		var upcoming, entryMissing int
		var fundingStatus, funderLabel string
		var selfFunded, hasGap bool
		if err := rows.Scan(&v.ID, &v.LastName, &v.FirstNames, &v.DateOfBirth,
			&v.Phone, &v.Email, &v.Neph, &v.Status, &enrID, &objective, &offroad, &onroad,
			&consumed, &total, &upcoming, &fundingStatus, &funderLabel, &selfFunded,
			&hasGap, &entryMissing); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if enrID != nil {
			v.Enrollment = &EnrollmentSummary{
				ID: *enrID, Objective: *objective,
				OffroadTargetDate: offroad, OnroadTargetDate: onroad,
				ConsumedHours: consumed, TotalHours: total, UpcomingLessons: upcoming,
				FundingStatus: fundingStatus, FunderLabel: funderLabel,
			}
		}
		v.Health = computeHealth(v, fundingStatus, selfFunded, hasGap, upcoming, entryMissing)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

// computeHealth: the global dot. Red when a critical will bite (rejected
// funding, projected hours under a threshold); amber for anything the
// office should tidy; green when the file is clean.
func computeHealth(v PersonView, fundingStatus string, selfFunded, hasGap bool, upcoming, entryMissing int) Health {
	h := Health{Color: "green", Reasons: []string{}}
	amber := func(reason string) {
		if h.Color == "green" {
			h.Color = "amber"
		}
		h.Reasons = append(h.Reasons, reason)
	}
	red := func(reason string) {
		h.Color = "red"
		h.Reasons = append(h.Reasons, reason)
	}

	if v.Enrollment == nil {
		amber("Aucun parcours actif")
		return h
	}
	switch {
	case fundingStatus == "REJECTED":
		red("Financement refusé")
	case selfFunded:
		// Self-funding follows its own cycle (RG-15): no approval to wait for.
	case fundingStatus == "DRAFT":
		amber("Dossier de financement à monter")
	case fundingStatus == "SUBMITTED":
		amber("Financement déposé, en attente d'accord")
	}
	if hasGap {
		red("Heures projetées sous le seuil d'une échéance")
	}
	if v.Neph == nil || *v.Neph == "" {
		amber("NEPH non renseigné")
	}
	if v.Enrollment.OffroadTargetDate == nil {
		amber("Échéances non posées")
	}
	if upcoming == 0 {
		amber("Aucune séance à venir")
	}
	if entryMissing > 0 {
		amber("Prérequis d'entrée incomplets")
	}
	return h
}

// history is the BUSINESS trace of the file (RG-185, C-10): meaningful
// events, dated and attributed — never the technical access log.
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	personID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown person")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT de.kind, de.occurred_at, COALESCE(u.display_name, '')
		FROM domain_event de
		LEFT JOIN app_user u ON u.id = de.author_id
		WHERE de.school_id = $1
		  AND (de.subject_id = $2
		       OR de.subject_id IN (SELECT id FROM enrollment WHERE person_id = $2))
		ORDER BY de.occurred_at DESC
		LIMIT 100`, id.SchoolID, personID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type event struct {
		Kind       string    `json:"kind"`
		OccurredAt time.Time `json:"occurred_at"`
		Author     string    `json:"author"`
	}
	out := []event{}
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.Kind, &e.OccurredAt, &e.Author); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

type personBody struct {
	LastName    string     `json:"last_name"`
	FirstNames  string     `json:"first_names"`
	DateOfBirth *time.Time `json:"date_of_birth"`
	Phone       *string    `json:"phone"`
	Email       *string    `json:"email"`
	Neph        *string    `json:"neph"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	var body personBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.LastName == "" || body.FirstNames == "" {
		fail(w, http.StatusUnprocessableEntity, "last_name and first_names required")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO person (id, school_id, last_name, first_names, date_of_birth, phone, email, neph)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))`,
		newID, id.SchoolID, body.LastName, body.FirstNames, body.DateOfBirth, body.Phone, body.Email, body.Neph)
	if err != nil {
		// The partial unique index: one NEPH per person (RG-182).
		fail(w, http.StatusConflict, "ce NEPH est déjà porté par un autre dossier")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "person.created", "person", newID,
		map[string]string{"name": body.LastName + " " + body.FirstNames}, id.UserID)
	reply(w, http.StatusCreated, map[string]uuid.UUID{"id": newID})
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	personID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown person")
		return
	}
	var body struct {
		personBody
		Status *string `json:"status"` // ACTIVE / ARCHIVED (RG-184)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}

	if body.Status != nil && *body.Status == "ARCHIVED" {
		// RG-184: structurally impossible while an enrollment is ACTIVE.
		var active bool
		err := h.pool.QueryRow(r.Context(), `
			SELECT EXISTS (SELECT 1 FROM enrollment
			               WHERE person_id = $1 AND life_status = 'ACTIVE')`, personID).Scan(&active)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if active {
			fail(w, http.StatusConflict, "un parcours actif existe : le clôturer d'abord")
			return
		}
	}

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE person SET
			last_name = COALESCE(NULLIF($3, ''), last_name),
			first_names = COALESCE(NULLIF($4, ''), first_names),
			date_of_birth = COALESCE($5, date_of_birth),
			phone = COALESCE($6, phone),
			email = COALESCE($7, email),
			neph = COALESCE(NULLIF($8, ''), neph),
			status = COALESCE($9, status)
		WHERE school_id = $1 AND id = $2`,
		id.SchoolID, personID, body.LastName, body.FirstNames,
		body.DateOfBirth, body.Phone, body.Email, body.Neph, body.Status)
	if err != nil {
		fail(w, http.StatusConflict, "ce NEPH est déjà porté par un autre dossier")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown person")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireManager(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return id, false
	}
	if !id.CanManagePeople() {
		fail(w, http.StatusForbidden, "student records are read-only for this profile")
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
