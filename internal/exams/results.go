package exams

// F-07 results: the credit engine's second half.
//
//   - A result is entered once the session has started; "non renseignée"
//     is DERIVED — a committed booking past the date with no result
//     (RG-112), nothing flips by batch job.
//   - Changing an entered result is a traced correction (RG-106).
//   - Credits: committed (still engaged), spent (passed + failed),
//     forfeited (absent, plus the unengaged remainder once the session
//     is past — RG-39/RG-40).
//   - A PASSED off-road result carries a derived expiry: result date +
//     the school's validity parameter (RG-25). Never stored.

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/angelok-lak/camo-heavy-soft/internal/events"
)

func (h *Handler) resultRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/exam-bookings/{id}/result", h.enterResult)
}

var validResults = map[string]bool{"PASSED": true, "FAILED": true, "ABSENT": true}

func (h *Handler) enterResult(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	bookingID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown booking")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validResults[body.Status] {
		fail(w, http.StatusUnprocessableEntity, "status must be PASSED, FAILED or ABSENT")
		return
	}

	tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback(r.Context())

	var current string
	var sessionStart time.Time
	err = tx.QueryRow(r.Context(), `
		SELECT b.status::text, es.starts_at
		FROM exam_booking b JOIN exam_session es ON es.id = b.exam_session_id
		WHERE b.school_id = $1 AND b.id = $2 FOR UPDATE OF b`,
		id.SchoolID, bookingID).Scan(&current, &sessionStart)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown booking")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Structural guards: no result before the session, none on a
	// withdrawn booking, and no silent re-entry of the same value.
	if sessionStart.After(time.Now()) {
		fail(w, http.StatusUnprocessableEntity, "la session n'a pas encore eu lieu")
		return
	}
	if current == "WITHDRAWN" {
		fail(w, http.StatusUnprocessableEntity, "candidat retiré de la session")
		return
	}
	if current == body.Status {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// A change of an already-entered result is a traced correction.
	if current != "COMMITTED" {
		corrID, err := uuid.NewV7()
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := tx.Exec(r.Context(), `
			INSERT INTO exam_booking_correction
			    (id, school_id, exam_booking_id, previous_status, new_status, corrected_by)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			corrID, id.SchoolID, bookingID, current, body.Status, id.UserID); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if _, err := tx.Exec(r.Context(), `
		UPDATE exam_booking SET status = $2, result_by = $3, result_at = now()
		WHERE id = $1`, bookingID, body.Status, id.UserID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := events.Emit(r.Context(), tx, id.SchoolID, "exam_result.entered", "exam_booking",
		bookingID, map[string]string{"from": current, "to": body.Status}, id.UserID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// offroadValidityMonths reads the school parameter (RG-25, A-16).
func (h *Handler) offroadValidityMonths(r *http.Request, schoolID uuid.UUID) int {
	months := 12
	_ = h.pool.QueryRow(r.Context(),
		`SELECT offroad_validity_months FROM school WHERE id = $1`, schoolID).Scan(&months)
	return months
}