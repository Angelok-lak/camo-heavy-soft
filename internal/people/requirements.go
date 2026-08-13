package people

// F-29: the requirements of an enrollment, two sets titled by purpose
// (RG-255) — entering training, presenting at the exam. Everything the
// front shows is computed here (C-26): EXPIRED from valid_until, set
// completeness from the mandatory items (RG-192).
//
// Permissions follow RG-21 / RG-22: office and management validate
// anything; an instructor validates only items whose template allows it.
// That refusal is a PERMISSION, not a business rule — it may refuse.

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

func (h *Handler) requirementRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/enrollments/{id}/requirements", h.listRequirements)
	mux.HandleFunc("POST /api/requirements/{id}/validate", h.validateRequirement)
	mux.HandleFunc("POST /api/requirements/{id}/unvalidate", h.unvalidateRequirement)
	mux.HandleFunc("POST /api/requirements/{id}/not-applicable", h.markNotApplicable)

	mux.HandleFunc("GET /api/objectives/{id}/requirement-templates", h.listTemplates)
	mux.HandleFunc("POST /api/objectives/{id}/requirement-templates", h.createTemplate)
	mux.HandleFunc("PATCH /api/requirement-templates/{id}", h.patchTemplate)
}

// ---------------------------------------------------------------------
// Reading the enrollment's requirements
// ---------------------------------------------------------------------

type Requirement struct {
	ID          uuid.UUID  `json:"id"`
	Label       string     `json:"label"`
	Set         string     `json:"set"`
	Mandatory   bool       `json:"mandatory"`
	Status      string     `json:"status"`
	ValidatedBy string     `json:"validated_by"`
	ValidatedAt *time.Time `json:"validated_at"`
	Comment     string     `json:"comment"`
	NaReason    string     `json:"na_reason"`
	ValidUntil  *time.Time `json:"valid_until"`
	// Derived, never stored (RG-193): valid_until has passed.
	Expired bool `json:"expired"`
	// Whether THIS caller may act on it (RG-21) — computed server-side.
	CanValidate bool `json:"can_validate"`
}

type RequirementSet struct {
	Title    string        `json:"title"`
	Items    []Requirement `json:"items"`
	Missing  int           `json:"missing"`
	Complete bool          `json:"complete"`
}

// listRequirements returns the two sets titled by purpose (RG-255).
// Completeness (RG-192): every mandatory item VALIDATED (not expired) or
// NOT_APPLICABLE.
func (h *Handler) listRequirements(w http.ResponseWriter, r *http.Request) {
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
		SELECT er.id, er.label, er.req_set::text, er.mandatory, er.status::text,
		       COALESCE(u.display_name, ''), er.validated_at,
		       COALESCE(er.comment, ''), COALESCE(er.na_reason, ''), er.valid_until,
		       er.instructor_may_validate
		FROM enrollment_requirement er
		JOIN enrollment e ON e.id = er.enrollment_id
		LEFT JOIN app_user u ON u.id = er.validated_by
		WHERE e.school_id = $1 AND er.enrollment_id = $2
		ORDER BY er.req_set, er.mandatory DESC, er.label`, id.SchoolID, enrollmentID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	sets := map[string]*RequirementSet{
		"ENTRY": {Title: "Pour entrer en formation", Items: []Requirement{}},
		"EXAM":  {Title: "Pour se présenter à l'examen", Items: []Requirement{}},
	}
	officeSide := id.CanManagePeople()
	for rows.Next() {
		var q Requirement
		var instructorMay bool
		if err := rows.Scan(&q.ID, &q.Label, &q.Set, &q.Mandatory, &q.Status,
			&q.ValidatedBy, &q.ValidatedAt, &q.Comment, &q.NaReason, &q.ValidUntil,
			&instructorMay); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		q.Expired = q.Status == "VALIDATED" && q.ValidUntil != nil && q.ValidUntil.Before(time.Now())
		q.CanValidate = officeSide || instructorMay
		if s, known := sets[q.Set]; known {
			s.Items = append(s.Items, q)
		}
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, s := range sets {
		for _, q := range s.Items {
			if q.Mandatory && !(q.Status == "NOT_APPLICABLE" || (q.Status == "VALIDATED" && !q.Expired)) {
				s.Missing++
			}
		}
		s.Complete = s.Missing == 0
	}
	reply(w, http.StatusOK, map[string]*RequirementSet{
		"entry": sets["ENTRY"],
		"exam":  sets["EXAM"],
	})
}

// ---------------------------------------------------------------------
// Acting on one requirement
// ---------------------------------------------------------------------

// mayAct loads the requirement and applies RG-21/RG-22: office and
// management always; an instructor only where the copy allows it.
func (h *Handler) mayAct(r *http.Request, id auth.Identity, reqID uuid.UUID) (bool, error) {
	if id.CanManagePeople() {
		return true, nil
	}
	var instructorMay bool
	err := h.pool.QueryRow(r.Context(), `
		SELECT instructor_may_validate FROM enrollment_requirement
		WHERE school_id = $1 AND id = $2`, id.SchoolID, reqID).Scan(&instructorMay)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, errNotFoundReq
	}
	return instructorMay, err
}

var errNotFoundReq = errors.New("unknown requirement")

func (h *Handler) validateRequirement(w http.ResponseWriter, r *http.Request) {
	h.actOnRequirement(w, r, "requirement.validated", func(body struct {
		Comment string `json:"comment"`
		Reason  string `json:"reason"`
	}) (string, []any) {
		// valid_until springs from the copied validity window.
		return `UPDATE enrollment_requirement SET
			status = 'VALIDATED', validated_by = $3, validated_at = now(),
			comment = NULLIF($4, ''), na_reason = NULL,
			valid_until = CASE WHEN validity_months IS NOT NULL
				THEN (now() + make_interval(months => validity_months))::date END
			WHERE school_id = $1 AND id = $2`, []any{body.Comment}
	})
}

func (h *Handler) unvalidateRequirement(w http.ResponseWriter, r *http.Request) {
	h.actOnRequirement(w, r, "requirement.unvalidated", func(body struct {
		Comment string `json:"comment"`
		Reason  string `json:"reason"`
	}) (string, []any) {
		return `UPDATE enrollment_requirement SET
			status = 'NOT_VALIDATED', validated_by = $3, validated_at = now(),
			comment = NULLIF($4, ''), na_reason = NULL, valid_until = NULL
			WHERE school_id = $1 AND id = $2`, []any{body.Comment}
	})
}

// markNotApplicable takes the item out of completeness (RG-193), reason
// required.
func (h *Handler) markNotApplicable(w http.ResponseWriter, r *http.Request) {
	h.actOnRequirement(w, r, "requirement.marked_na", func(body struct {
		Comment string `json:"comment"`
		Reason  string `json:"reason"`
	}) (string, []any) {
		if body.Reason == "" {
			return "", nil
		}
		return `UPDATE enrollment_requirement SET
			status = 'NOT_APPLICABLE', validated_by = $3, validated_at = now(),
			na_reason = $4, valid_until = NULL
			WHERE school_id = $1 AND id = $2`, []any{body.Reason}
	})
}

func (h *Handler) actOnRequirement(w http.ResponseWriter, r *http.Request, eventKind string,
	build func(struct {
		Comment string `json:"comment"`
		Reason  string `json:"reason"`
	}) (string, []any)) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	reqID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown requirement")
		return
	}
	allowed, err := h.mayAct(r, id, reqID)
	if errors.Is(err, errNotFoundReq) {
		fail(w, http.StatusNotFound, "unknown requirement")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowed {
		fail(w, http.StatusForbidden, "ce prérequis ne peut pas être validé par un formateur")
		return
	}

	var body struct {
		Comment string `json:"comment"`
		Reason  string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	sql, extra := build(body)
	if sql == "" {
		fail(w, http.StatusUnprocessableEntity, "un motif est requis")
		return
	}

	args := append([]any{id.SchoolID, reqID, id.UserID}, extra...)
	tag, err := h.pool.Exec(r.Context(), sql, args...)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown requirement")
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, eventKind, "requirement", reqID, body, id.UserID)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Templates (per objective, management side)
// ---------------------------------------------------------------------

type Template struct {
	ID                    uuid.UUID `json:"id"`
	Label                 string    `json:"label"`
	Set                   string    `json:"set"`
	Mandatory             bool      `json:"mandatory"`
	InstructorMayValidate bool      `json:"instructor_may_validate"`
	ValidityMonths        *int      `json:"validity_months"`
	Active                bool      `json:"active"`
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	objectiveID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown objective")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, label, req_set::text, mandatory, instructor_may_validate, validity_months, active
		FROM requirement_template
		WHERE school_id = $1 AND objective_id = $2
		ORDER BY req_set, label`, id.SchoolID, objectiveID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []Template{}
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.Label, &t.Set, &t.Mandatory,
			&t.InstructorMayValidate, &t.ValidityMonths, &t.Active); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	if !id.CanManageSettings() {
		fail(w, http.StatusForbidden, "management only")
		return
	}
	objectiveID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown objective")
		return
	}
	var body struct {
		Label                 string `json:"label"`
		Set                   string `json:"set"`
		Mandatory             *bool  `json:"mandatory"`
		InstructorMayValidate bool   `json:"instructor_may_validate"`
		ValidityMonths        *int   `json:"validity_months"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
		body.Label == "" || (body.Set != "ENTRY" && body.Set != "EXAM") {
		fail(w, http.StatusUnprocessableEntity, "label and set (ENTRY/EXAM) required")
		return
	}
	mandatory := true
	if body.Mandatory != nil {
		mandatory = *body.Mandatory
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		INSERT INTO requirement_template
		    (id, school_id, objective_id, label, req_set, mandatory, instructor_may_validate, validity_months)
		SELECT $1, $2, o.id, $4, $5, $6, $7, $8 FROM objective o
		WHERE o.school_id = $2 AND o.id = $3`,
		newID, id.SchoolID, objectiveID, body.Label, body.Set,
		mandatory, body.InstructorMayValidate, body.ValidityMonths)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown objective")
		return
	}
	reply(w, http.StatusCreated, map[string]uuid.UUID{"id": newID})
}

func (h *Handler) patchTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	if !id.CanManageSettings() {
		fail(w, http.StatusForbidden, "management only")
		return
	}
	templateID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown template")
		return
	}
	var body struct {
		Label                 *string `json:"label"`
		Mandatory             *bool   `json:"mandatory"`
		InstructorMayValidate *bool   `json:"instructor_may_validate"`
		ValidityMonths        *int    `json:"validity_months"`
		Active                *bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusUnprocessableEntity, "malformed body")
		return
	}
	// Deactivation, never deletion (RG-205): copies on existing files
	// keep living their own life.
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE requirement_template SET
			label = COALESCE($3, label),
			mandatory = COALESCE($4, mandatory),
			instructor_may_validate = COALESCE($5, instructor_may_validate),
			validity_months = COALESCE($6, validity_months),
			active = COALESCE($7, active)
		WHERE school_id = $1 AND id = $2`,
		id.SchoolID, templateID, body.Label, body.Mandatory,
		body.InstructorMayValidate, body.ValidityMonths, body.Active)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}