package enrollment

// F-05 within D-02's limit: the funder and the coverage status, cycle
// à monter › déposé › accordé › soldé, rejection with a mandatory reason
// (RG-189). Every change is a transition row — the file's history is the
// list of them.

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
	"github.com/angelok-lak/camo-heavy-soft/internal/events"
)

func (h *Handler) fundingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/enrollments/{id}/funding", h.readFunding)
	mux.HandleFunc("POST /api/enrollments/{id}/funding/transition", h.transitionFunding)
	mux.HandleFunc("PATCH /api/enrollments/{id}/funding", h.patchFunding)
	mux.HandleFunc("GET /api/funder-kinds", h.listFunderKinds)
	mux.HandleFunc("GET /api/payers", h.listPayers)
	mux.HandleFunc("POST /api/payers", h.createPayer)
	mux.HandleFunc("PUT /api/enrollments/{id}/payer", h.setPayer)
}

// ---------------------------------------------------------------------
// Payer (C-12): who the centre talks money with. NULL = the student.
// ---------------------------------------------------------------------

type Payer struct {
	ID           uuid.UUID `json:"id"`
	Label        string    `json:"label"`
	ContactName  string    `json:"contact_name"`
	ContactEmail string    `json:"contact_email"`
	ContactPhone string    `json:"contact_phone"`
	Active       bool      `json:"active"`
}

func (h *Handler) listPayers(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, label, COALESCE(contact_name, ''), COALESCE(contact_email, ''),
		       COALESCE(contact_phone, ''), active
		FROM payer WHERE school_id = $1 ORDER BY label`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []Payer{}
	for rows.Next() {
		var p Payer
		if err := rows.Scan(&p.ID, &p.Label, &p.ContactName, &p.ContactEmail, &p.ContactPhone, &p.Active); err != nil {
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

func (h *Handler) createPayer(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	var body struct {
		Label        string `json:"label"`
		ContactName  string `json:"contact_name"`
		ContactEmail string `json:"contact_email"`
		ContactPhone string `json:"contact_phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" {
		fail(w, http.StatusUnprocessableEntity, "label required")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO payer (id, school_id, label, contact_name, contact_email, contact_phone)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))`,
		newID, id.SchoolID, body.Label, body.ContactName, body.ContactEmail, body.ContactPhone)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, map[string]uuid.UUID{"id": newID})
}

// setPayer: NULL body payer_id = the student pays themselves.
func (h *Handler) setPayer(w http.ResponseWriter, r *http.Request) {
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
		PayerID *uuid.UUID `json:"payer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE enrollment SET payer_id = $3 WHERE school_id = $1 AND id = $2`,
		id.SchoolID, enrollmentID, body.PayerID)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "unknown payer")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown enrollment")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "enrollment.payer_changed", "enrollment",
		enrollmentID, body, id.UserID)
	w.WriteHeader(http.StatusNoContent)
}

type FundingView struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	FunderKind  *uuid.UUID `json:"funder_kind_id"`
	FunderLabel string     `json:"funder_label"`
	SelfFunded  bool       `json:"self_funded"`
	PayerID     *uuid.UUID `json:"payer_id"`
	Payer       *Payer     `json:"payer"`
	Transitions []struct {
		From   string    `json:"from"`
		To     string    `json:"to"`
		Reason string    `json:"reason"`
		Author string    `json:"author"`
		At     time.Time `json:"at"`
	} `json:"transitions"`
}

func (h *Handler) readFunding(w http.ResponseWriter, r *http.Request) {
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

	var v FundingView
	var payer Payer
	var payerID *uuid.UUID
	err = h.pool.QueryRow(r.Context(), `
		SELECT f.id, f.status::text, f.funder_kind_id,
		       COALESCE(fk.label, ''), COALESCE(fk.self_funded, false),
		       e.payer_id, COALESCE(py.label, ''), COALESCE(py.contact_name, ''),
		       COALESCE(py.contact_email, ''), COALESCE(py.contact_phone, '')
		FROM funding f
		JOIN enrollment e ON e.id = f.enrollment_id
		LEFT JOIN funder_kind fk ON fk.id = f.funder_kind_id
		LEFT JOIN payer py ON py.id = e.payer_id
		WHERE f.school_id = $1 AND f.enrollment_id = $2`,
		id.SchoolID, enrollmentID,
	).Scan(&v.ID, &v.Status, &v.FunderKind, &v.FunderLabel, &v.SelfFunded,
		&payerID, &payer.Label, &payer.ContactName, &payer.ContactEmail, &payer.ContactPhone)
	if payerID != nil {
		payer.ID = *payerID
		v.PayerID = payerID
		v.Payer = &payer
	}
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "no funding file for this enrollment")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT ft.from_status::text, ft.to_status::text, COALESCE(ft.reason, ''),
		       COALESCE(u.display_name, ''), ft.at
		FROM funding_transition ft
		LEFT JOIN app_user u ON u.id = ft.author_id
		WHERE ft.funding_id = $1 ORDER BY ft.at DESC`, v.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		var t struct {
			From   string    `json:"from"`
			To     string    `json:"to"`
			Reason string    `json:"reason"`
			Author string    `json:"author"`
			At     time.Time `json:"at"`
		}
		if err := rows.Scan(&t.From, &t.To, &t.Reason, &t.Author, &t.At); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		v.Transitions = append(v.Transitions, t)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, v)
}

var validFundingStatus = map[string]bool{
	"DRAFT": true, "SUBMITTED": true, "APPROVED": true, "SETTLED": true, "REJECTED": true,
}

func (h *Handler) transitionFunding(w http.ResponseWriter, r *http.Request) {
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
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validFundingStatus[body.Status] {
		fail(w, http.StatusUnprocessableEntity, "status must be DRAFT, SUBMITTED, APPROVED, SETTLED or REJECTED")
		return
	}
	// RG-189: a rejection carries its reason, always.
	if body.Status == "REJECTED" && body.Reason == "" {
		fail(w, http.StatusUnprocessableEntity, "un motif est obligatoire pour un refus")
		return
	}

	tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	var fundingID uuid.UUID
	var from string
	err = tx.QueryRow(r.Context(), `
		SELECT id, status::text FROM funding
		WHERE school_id = $1 AND enrollment_id = $2 FOR UPDATE`,
		id.SchoolID, enrollmentID).Scan(&fundingID, &from)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "no funding file for this enrollment")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if from == body.Status {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if _, err := tx.Exec(r.Context(), `
		UPDATE funding SET status = $2, updated_at = now() WHERE id = $1`,
		fundingID, body.Status); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	trID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(r.Context(), `
		INSERT INTO funding_transition (id, school_id, funding_id, from_status, to_status, reason, author_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)`,
		trID, id.SchoolID, fundingID, from, body.Status, body.Reason, id.UserID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := events.Emit(r.Context(), tx, id.SchoolID, "funding.transitioned", "enrollment",
		enrollmentID, map[string]string{"from": from, "to": body.Status}, id.UserID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) patchFunding(w http.ResponseWriter, r *http.Request) {
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
		FunderKindID *uuid.UUID `json:"funder_kind_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE funding SET funder_kind_id = $3, updated_at = now()
		WHERE school_id = $1 AND enrollment_id = $2`,
		id.SchoolID, enrollmentID, body.FunderKindID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "no funding file for this enrollment")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listFunderKinds(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, label, self_funded, active FROM funder_kind
		WHERE school_id = $1 ORDER BY label`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type kind struct {
		ID         uuid.UUID `json:"id"`
		Label      string    `json:"label"`
		SelfFunded bool      `json:"self_funded"`
		Active     bool      `json:"active"`
	}
	out := []kind{}
	for rows.Next() {
		var k kind
		if err := rows.Scan(&k.ID, &k.Label, &k.SelfFunded, &k.Active); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}