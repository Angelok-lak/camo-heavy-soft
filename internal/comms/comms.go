// Package comms sends what the centre says to people: exam convocations
// first, free messages second. One recipient = one message = one traced
// row, whatever the channel.
//
// Channels (Angelo's arbitrage, option A):
//   - EMAIL sends itself when SMTP_HOST/SMTP_PORT/SMTP_FROM (+ optional
//     SMTP_USER/SMTP_PASS) are set; without them the message is stored
//     SIMULATED, complete, ready to really leave once configured.
//   - WHATSAPP is a prepared wa.me link — one recipient per link, the
//     office clicks, WhatsApp opens pre-filled, and the row is marked
//     SENT on click. No Business API, no Meta validation.
//
// Templates live in communication_template (D-01), with {{variables}}.
package comms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
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
	mux.HandleFunc("POST /api/exam-sessions/{id}/convocations", h.convokeSession)
	mux.HandleFunc("GET /api/persons/{id}/communications", h.listForPerson)
	mux.HandleFunc("POST /api/persons/{id}/communications", h.sendFree)
	mux.HandleFunc("POST /api/communications/{id}/mark-sent", h.markSent)
	mux.HandleFunc("GET /api/communication-templates", h.listTemplates)
	mux.HandleFunc("PUT /api/communication-templates/{kind}", h.putTemplate)
}

// ---------------------------------------------------------------------
// The message row
// ---------------------------------------------------------------------

type Communication struct {
	ID           uuid.UUID  `json:"id"`
	Channel      string     `json:"channel"`
	Kind         string     `json:"kind"`
	Recipient    string     `json:"recipient_label"`
	Address      string     `json:"recipient_address"`
	Subject      string     `json:"subject"`
	Body         string     `json:"body"`
	Status       string     `json:"status"`
	WhatsappLink string     `json:"whatsapp_link,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	SentAt       *time.Time `json:"sent_at"`
}

// waLink builds the one-recipient prepared link. French numbers: a
// leading 0 becomes +33.
func waLink(phone, body string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if strings.HasPrefix(digits, "0") {
		digits = "33" + digits[1:]
	}
	return "https://wa.me/" + digits + "?text=" + url.QueryEscape(body)
}

func smtpConfigured() bool {
	return os.Getenv("SMTP_HOST") != "" && os.Getenv("SMTP_FROM") != ""
}

func sendEmail(to, subject, body string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	from := os.Getenv("SMTP_FROM")
	msg := []byte("From: " + from + "\r\nTo: " + to + "\r\nSubject: " + subject +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + body)
	var a smtp.Auth
	if user := os.Getenv("SMTP_USER"); user != "" {
		a = smtp.PlainAuth("", user, os.Getenv("SMTP_PASS"), host)
	}
	return smtp.SendMail(host+":"+port, a, from, []string{to}, msg)
}

// deliver creates the row and pushes it through its channel. WhatsApp
// stays PREPARED (the click sends); email goes SENT, SIMULATED or FAILED.
func (h *Handler) deliver(ctx context.Context, schoolID, authorID uuid.UUID,
	personID, payerID *uuid.UUID, channel, kind, label, address, subject, body string,
	aboutKind string, aboutID *uuid.UUID) (Communication, error) {

	c := Communication{
		Channel: channel, Kind: kind, Recipient: label, Address: address,
		Subject: subject, Body: body, Status: "PREPARED", CreatedAt: time.Now(),
	}
	var sentAt *time.Time
	if channel == "EMAIL" {
		now := time.Now()
		if smtpConfigured() {
			if err := sendEmail(address, subject, body); err != nil {
				c.Status = "FAILED"
			} else {
				c.Status = "SENT"
				sentAt = &now
			}
		} else {
			c.Status = "SIMULATED"
			sentAt = &now
		}
	} else {
		c.WhatsappLink = waLink(address, body)
	}
	c.SentAt = sentAt

	id, err := uuid.NewV7()
	if err != nil {
		return c, err
	}
	c.ID = id
	_, err = h.pool.Exec(ctx, `
		INSERT INTO communication
		    (id, school_id, person_id, payer_id, channel, kind, recipient_label,
		     recipient_address, subject, body, status, about_kind, about_id, created_by, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10, $11, NULLIF($12, ''), $13, $14, $15)`,
		id, schoolID, personID, payerID, channel, kind, label, address,
		subject, body, c.Status, aboutKind, aboutID, authorID, sentAt)
	if err != nil {
		return c, err
	}
	_ = events.Emit(ctx, h.pool, schoolID, "communication."+strings.ToLower(c.Status),
		"communication", id, map[string]string{"kind": kind, "channel": channel, "to": label}, authorID)
	return c, nil
}

// ---------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------

func (h *Handler) template(ctx context.Context, schoolID uuid.UUID, kind string) (subject, body string, err error) {
	err = h.pool.QueryRow(ctx, `
		SELECT subject, body FROM communication_template
		WHERE school_id = $1 AND kind = $2`, schoolID, kind).Scan(&subject, &body)
	return
}

func fill(tpl string, vars map[string]string) string {
	for k, v := range vars {
		tpl = strings.ReplaceAll(tpl, "{{"+k+"}}", v)
	}
	return tpl
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT kind, subject, body FROM communication_template
		WHERE school_id = $1 ORDER BY kind`, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type tpl struct{ Kind, Subject, Body string }
	out := []map[string]string{}
	for rows.Next() {
		var t tpl
		if err := rows.Scan(&t.Kind, &t.Subject, &t.Body); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, map[string]string{"kind": t.Kind, "subject": t.Subject, "body": t.Body})
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

func (h *Handler) putTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || !id.CanManageSettings() {
		fail(w, http.StatusForbidden, "management only")
		return
	}
	kind := r.PathValue("kind")
	var body struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Body == "" {
		fail(w, http.StatusUnprocessableEntity, "subject and body required")
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO communication_template (id, school_id, kind, subject, body)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (school_id, kind) DO UPDATE SET subject = $4, body = $5`,
		newID, id.SchoolID, kind, body.Subject, body.Body)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Exam convocations — one personal message per committed candidate
// ---------------------------------------------------------------------

func (h *Handler) convokeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || !id.CanEditPlanning() {
		fail(w, http.StatusForbidden, "office and management only")
		return
	}
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown session")
		return
	}
	// Optional: restrict to one booking.
	var body struct {
		BookingID *uuid.UUID `json:"booking_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	tplSubject, tplBody, err := h.template(r.Context(), id.SchoolID, "exam_convocation")
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "aucun modèle de convocation — voir Paramétrage")
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT b.id, e.id, p.id, p.first_names, p.last_name,
		       COALESCE(p.email, ''), COALESCE(p.phone, ''), p.preferred_channel::text,
		       b.test_kind::text, es.starts_at, ep.label, ep.travel_minutes,
		       e.payer_id, COALESCE(py.label, ''), COALESCE(py.contact_email, ''),
		       COALESCE(py.contact_phone, ''), COALESCE(py.preferred_channel::text, 'EMAIL')
		FROM exam_booking b
		JOIN enrollment e ON e.id = b.enrollment_id
		JOIN person p ON p.id = e.person_id
		JOIN exam_session es ON es.id = b.exam_session_id
		JOIN exam_place ep ON ep.id = es.exam_place_id
		LEFT JOIN payer py ON py.id = e.payer_id
		WHERE b.school_id = $1 AND b.exam_session_id = $2 AND b.status = 'COMMITTED'
		  AND ($3::uuid IS NULL OR b.id = $3)`,
		id.SchoolID, sessionID, body.BookingID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type row struct {
		bookingID, enrollmentID, personID              uuid.UUID
		first, last, email, phone, channel, test       string
		starts                                         time.Time
		place                                          string
		travel                                         int
		payerID                                        *uuid.UUID
		payerLabel, payerEmail, payerPhone, payerChan  string
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.bookingID, &x.enrollmentID, &x.personID, &x.first, &x.last,
			&x.email, &x.phone, &x.channel, &x.test, &x.starts, &x.place, &x.travel,
			&x.payerID, &x.payerLabel, &x.payerEmail, &x.payerPhone, &x.payerChan); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		list = append(list, x)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := []Communication{}
	for _, x := range list {
		test := "plateau"
		if x.test == "ONROAD" {
			test = "circulation"
		}
		present := x.starts.Add(-time.Duration(x.travel) * time.Minute)
		vars := map[string]string{
			"prenom":  x.first,
			"nom":     x.last,
			"epreuve": test,
			"date":    x.starts.Format("02/01/2006"),
			"heure":   x.starts.Format("15h04"),
			"lieu":    x.place,
			"presentation": present.Format("15h04"),
		}
		subject := fill(tplSubject, vars)
		msg := fill(tplBody, vars)

		// The candidate, on their preferred channel (fallback: whatever
		// contact exists).
		channel, address := pickChannel(x.channel, x.email, x.phone)
		if address == "" {
			out = append(out, Communication{
				Recipient: x.first + " " + x.last, Status: "FAILED",
				Body: "aucune coordonnée sur le dossier",
			})
		} else {
			pid := x.personID
			c, err := h.deliver(r.Context(), id.SchoolID, id.UserID, &pid, nil, channel,
				"exam_convocation", x.first+" "+x.last, address, subject, msg,
				"exam_session", &sessionID)
			if err != nil {
				fail(w, http.StatusInternalServerError, err.Error())
				return
			}
			out = append(out, c)
		}

		// The payer receives convocations too (A-06), never results.
		if x.payerID != nil {
			channel, address := pickChannel(x.payerChan, x.payerEmail, x.payerPhone)
			if address != "" {
				c, err := h.deliver(r.Context(), id.SchoolID, id.UserID, nil, x.payerID, channel,
					"exam_convocation", x.payerLabel, address, subject, msg,
					"exam_session", &sessionID)
				if err != nil {
					fail(w, http.StatusInternalServerError, err.Error())
					return
				}
				out = append(out, c)
			}
		}
	}
	reply(w, http.StatusOK, map[string]any{"communications": out, "smtp_configured": smtpConfigured()})
}

func pickChannel(preferred, email, phone string) (string, string) {
	if preferred == "WHATSAPP" && phone != "" {
		return "WHATSAPP", phone
	}
	if email != "" {
		return "EMAIL", email
	}
	if phone != "" {
		return "WHATSAPP", phone
	}
	return "EMAIL", ""
}

// ---------------------------------------------------------------------
// Free messages + history + mark-sent
// ---------------------------------------------------------------------

func (h *Handler) sendFree(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || !id.CanManagePeople() {
		fail(w, http.StatusForbidden, "office and management only")
		return
	}
	personID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown person")
		return
	}
	var body struct {
		Channel string `json:"channel"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Body == "" ||
		(body.Channel != "EMAIL" && body.Channel != "WHATSAPP") {
		fail(w, http.StatusUnprocessableEntity, "channel (EMAIL/WHATSAPP) et body requis")
		return
	}

	var first, last, email, phone string
	err = h.pool.QueryRow(r.Context(), `
		SELECT first_names, last_name, COALESCE(email, ''), COALESCE(phone, '')
		FROM person WHERE school_id = $1 AND id = $2`,
		id.SchoolID, personID).Scan(&first, &last, &email, &phone)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown person")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	address := email
	if body.Channel == "WHATSAPP" {
		address = phone
	}
	if address == "" {
		fail(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("aucune coordonnée %s sur le dossier", strings.ToLower(body.Channel)))
		return
	}
	c, err := h.deliver(r.Context(), id.SchoolID, id.UserID, &personID, nil, body.Channel,
		"free", first+" "+last, address, body.Subject, body.Body, "", nil)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusCreated, c)
}

func (h *Handler) listForPerson(w http.ResponseWriter, r *http.Request) {
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
		SELECT c.id, c.channel::text, c.kind, c.recipient_label, c.recipient_address,
		       COALESCE(c.subject, ''), c.body, c.status::text, c.created_at, c.sent_at
		FROM communication c
		WHERE c.school_id = $1
		  AND (c.person_id = $2
		       OR c.payer_id = (SELECT payer_id FROM enrollment
		                        WHERE person_id = $2 AND life_status = 'ACTIVE'))
		ORDER BY c.created_at DESC LIMIT 100`, id.SchoolID, personID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []Communication{}
	for rows.Next() {
		var c Communication
		if err := rows.Scan(&c.ID, &c.Channel, &c.Kind, &c.Recipient, &c.Address,
			&c.Subject, &c.Body, &c.Status, &c.CreatedAt, &c.SentAt); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if c.Channel == "WHATSAPP" && c.Status == "PREPARED" {
			c.WhatsappLink = waLink(c.Address, c.Body)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

// markSent: the office clicked the WhatsApp link — the trace records it.
func (h *Handler) markSent(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	commID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown communication")
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE communication SET status = 'SENT', sent_at = now()
		WHERE school_id = $1 AND id = $2 AND status = 'PREPARED'`,
		id.SchoolID, commID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown or already settled communication")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------
// Automatic assignment message
// ---------------------------------------------------------------------

// NotifyLessonAssignment sends the automatic message when a student is
// placed on a lesson (both placement paths call it after the write
// lands). Missing template or missing contact → no message and no error:
// the placement itself must never fail on a notification.
func (h *Handler) NotifyLessonAssignment(ctx context.Context,
	schoolID, lessonID, enrollmentID, authorID uuid.UUID) (*Communication, error) {

	tplSubject, tplBody, err := h.template(ctx, schoolID, "lesson_assignment")
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var starts, ends time.Time
	var personID uuid.UUID
	var first, last, email, phone, preferred, kinds, instructors string
	err = h.pool.QueryRow(ctx, `
		SELECT l.starts_at, l.ends_at,
		       p.id, p.first_names, p.last_name,
		       COALESCE(p.email, ''), COALESCE(p.phone, ''), p.preferred_channel::text,
		       COALESCE((SELECT string_agg(lk.label, ', ' ORDER BY lk.label)
		                 FROM lesson_lesson_kind llk
		                 JOIN lesson_kind lk ON lk.id = llk.lesson_kind_id
		                 WHERE llk.lesson_id = l.id), ''),
		       COALESCE((SELECT string_agg(res.label, ', ' ORDER BY res.label)
		                 FROM lesson_resource lr
		                 JOIN resource res ON res.id = lr.resource_id AND res.kind = 'INSTRUCTOR'
		                 WHERE lr.lesson_id = l.id), '')
		FROM lesson l, enrollment e
		JOIN person p ON p.id = e.person_id
		WHERE l.school_id = $1 AND l.id = $2 AND e.id = $3`,
		schoolID, lessonID, enrollmentID,
	).Scan(&starts, &ends, &personID, &first, &last, &email, &phone, &preferred,
		&kinds, &instructors)
	if err != nil {
		return nil, err
	}

	if starts.Before(time.Now()) {
		return nil, nil // history correction, not news — nothing to announce
	}
	channel, address := pickChannel(preferred, email, phone)
	if address == "" {
		return nil, nil // no contact on file — nothing automatic can leave
	}
	avec := ""
	if instructors != "" {
		avec = " avec " + instructors
	}
	if kinds == "" {
		kinds = "de formation"
	}
	vars := map[string]string{
		"prenom":      first,
		"nom":         last,
		"type":        kinds,
		"date":        starts.Format("02/01/2006"),
		"heure_debut": starts.Format("15h04"),
		"heure_fin":   ends.Format("15h04"),
		"avec":        avec,
	}
	c, err := h.deliver(ctx, schoolID, authorID, &personID, nil, channel,
		"lesson_assignment", first+" "+last, address,
		fill(tplSubject, vars), fill(tplBody, vars), "lesson", &lessonID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}