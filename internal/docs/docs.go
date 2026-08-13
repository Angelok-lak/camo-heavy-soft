// Package docs is F-35's first cut plus the documents foundation: the
// candidate photographs themselves from their own phone on a TOKENIZED
// public page, the photo (and the optional ANTS e-photo code) lands in
// their file's documents, and the matching entry requirement validates
// itself — the registration stops waiting.
//
// V-10 is still open: a raw smartphone photo may not satisfy the ANTS
// agrément. The portal therefore accepts the 22-character e-photo code
// from approved apps alongside (or instead of) the photo.
package docs

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
	"github.com/angelok-lak/camo-heavy-soft/internal/events"
)

const maxUpload = 8 << 20 // 8 MB
const tokenLife = 7 * 24 * time.Hour

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// Routes: the authenticated side.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/persons/{id}/ephoto-request", h.createRequest)
	mux.HandleFunc("GET /api/persons/{id}/documents", h.listDocuments)
	mux.HandleFunc("GET /api/documents/{id}/content", h.serveDocument)
}

// PublicRoutes: the portal, NO session — the token is the whole
// authorization, scoped to one person, one purpose, one week.
func (h *Handler) PublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /portal/{token}", h.portalPage)
	mux.HandleFunc("POST /portal/{token}", h.portalUpload)
}

// ---------------------------------------------------------------------
// Office side
// ---------------------------------------------------------------------

func (h *Handler) createRequest(w http.ResponseWriter, r *http.Request) {
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
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	tag, err := h.pool.Exec(r.Context(), `
		INSERT INTO upload_token (token, school_id, person_id, created_by, expires_at)
		SELECT $1, $2, p.id, $4, now() + $5::interval
		FROM person p WHERE p.school_id = $2 AND p.id = $3`,
		token, id.SchoolID, personID, id.UserID, tokenLife.String())
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, http.StatusNotFound, "unknown person")
		return
	}
	reply(w, http.StatusCreated, map[string]string{
		"token": token,
		"path":  "/portal/" + token,
	})
}

type DocumentView struct {
	ID         uuid.UUID `json:"id"`
	Kind       string    `json:"kind"`
	Filename   string    `json:"filename"`
	AntsCode   string    `json:"ants_code"`
	Via        string    `json:"via"`
	UploadedAt time.Time `json:"uploaded_at"`
}

func (h *Handler) listDocuments(w http.ResponseWriter, r *http.Request) {
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
		SELECT id, kind, filename, COALESCE(ants_code, ''), via, uploaded_at
		FROM document WHERE school_id = $1 AND person_id = $2
		ORDER BY uploaded_at DESC`, id.SchoolID, personID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []DocumentView{}
	for rows.Next() {
		var d DocumentView
		if err := rows.Scan(&d.ID, &d.Kind, &d.Filename, &d.AntsCode, &d.Via, &d.UploadedAt); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}

func (h *Handler) serveDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	docID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown document")
		return
	}
	var contentType string
	var bytes []byte
	err = h.pool.QueryRow(r.Context(), `
		SELECT content_type, bytes FROM document
		WHERE school_id = $1 AND id = $2`, id.SchoolID, docID).Scan(&contentType, &bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "unknown document")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(bytes)
}

// ---------------------------------------------------------------------
// The public portal
// ---------------------------------------------------------------------

func (h *Handler) tokenPerson(r *http.Request, token string) (schoolID, personID, createdBy uuid.UUID, firstName string, err error) {
	err = h.pool.QueryRow(r.Context(), `
		SELECT t.school_id, t.person_id, t.created_by, p.first_names
		FROM upload_token t JOIN person p ON p.id = t.person_id
		WHERE t.token = $1 AND t.expires_at > now() AND t.used_at IS NULL`,
		token).Scan(&schoolID, &personID, &createdBy, &firstName)
	return
}

func (h *Handler) portalPage(w http.ResponseWriter, r *http.Request) {
	_, _, _, firstName, err := h.tokenPerson(r, r.PathValue("token"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Lien expiré</h1>
			<p>Ce lien n'est plus valide. Demandez-en un nouveau à votre centre de formation.</p>`))
		return
	}
	page := fmt.Sprintf(`<h1>Bonjour %s 👋</h1>
		<p>Votre centre a besoin de votre <strong>e-photo agréée</strong> pour finaliser votre inscription. Deux étapes :</p>
		<ol class="steps">
			<li><strong>Faites votre e-photo</strong> avec une application agréée ANTS (par exemple
			<a href="https://www.smartphone-id.com/ephoto-ants" target="_blank" rel="noreferrer">Smartphone&nbsp;iD</a>,
			quelques euros, à votre charge) ou en cabine agréée. Vous recevrez un
			<strong>code de 22 caractères</strong>.</li>
			<li><strong>Revenez ici</strong> saisir ce code — et joignez la photo si vous l'avez.</li>
		</ol>
		<form method="post" enctype="multipart/form-data">
			<label class="field">Code e-photo ANTS
				<input type="text" name="ants_code" maxlength="30" placeholder="Ex. 123ABC…" autocomplete="off">
			</label>
			<label class="drop">
				<input type="file" name="photo" accept="image/*" capture="user" onchange="this.closest('label').classList.add('ok'); document.getElementById('fn').textContent = this.files[0]?.name ?? ''">
				<span>📷 Joindre ma photo (facultatif)</span>
				<span id="fn" class="fn"></span>
			</label>
			<button type="submit">Envoyer au centre</button>
		</form>
		<p class="hint">Pas encore de code ? Vous pouvez déjà envoyer une photo nette (fond clair, sans lunettes) — le centre vous guidera pour la version agréée.</p>`,
		html.EscapeString(firstName))
	_, _ = fmt.Fprint(w, portalShell(page))
}

func (h *Handler) portalUpload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	schoolID, personID, createdBy, _, err := h.tokenPerson(r, token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Lien expiré</h1><p>Demandez un nouveau lien à votre centre.</p>`))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Photo trop lourde</h1><p>8 Mo maximum — réessayez.</p>`))
		return
	}
	antsCode := r.FormValue("ants_code")

	var filename, contentType string
	var data []byte
	if file, header, err := r.FormFile("photo"); err == nil {
		defer file.Close()
		data, err = io.ReadAll(file)
		if err != nil {
			_, _ = fmt.Fprint(w, portalShell(`<h1>Échec de lecture</h1><p>Réessayez.</p>`))
			return
		}
		filename = header.Filename
		contentType = header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/jpeg"
		}
	}
	if len(data) == 0 && antsCode == "" {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Rien reçu</h1><p>Ajoutez une photo ou votre code e-photo, puis renvoyez.</p>`))
		return
	}
	if len(data) == 0 {
		filename, contentType, data = "code-ephoto.txt", "text/plain", []byte(antsCode)
	}

	docID, err := uuid.NewV7()
	if err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Erreur</h1><p>Réessayez dans un instant.</p>`))
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Erreur</h1><p>Réessayez dans un instant.</p>`))
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(r.Context(), `
		INSERT INTO document (id, school_id, person_id, kind, filename, content_type, bytes, ants_code, via)
		VALUES ($1, $2, $3, 'EPHOTO', $4, $5, $6, NULLIF($7, ''), 'PORTAL')`,
		docID, schoolID, personID, filename, contentType, data, antsCode); err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Erreur</h1><p>Réessayez dans un instant.</p>`))
		return
	}
	// One successful upload burns the token.
	if _, err := tx.Exec(r.Context(),
		`UPDATE upload_token SET used_at = now() WHERE token = $1`, token); err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Erreur</h1><p>Réessayez dans un instant.</p>`))
		return
	}
	// The e-photo requirement stops blocking, on every active enrollment
	// of the person (comment says where it came from; no human validator).
	if _, err := tx.Exec(r.Context(), `
		UPDATE enrollment_requirement er SET
			status = 'VALIDATED', validated_at = now(),
			comment = 'Reçue via le portail e-photo'
		FROM enrollment e
		WHERE e.id = er.enrollment_id AND e.person_id = $1 AND e.life_status = 'ACTIVE'
		  AND er.label ILIKE '%e-photo%' AND er.status = 'NOT_VALIDATED'`,
		personID); err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Erreur</h1><p>Réessayez dans un instant.</p>`))
		return
	}
	// The trace's author is the office account that issued the link —
	// the candidate has no app account, by design.
	if err := events.Emit(r.Context(), tx, schoolID, "document.received", "person",
		personID, map[string]string{"kind": "EPHOTO", "via": "PORTAL"}, createdBy); err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Erreur</h1><p>Réessayez dans un instant.</p>`))
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		_, _ = fmt.Fprint(w, portalShell(`<h1>Erreur</h1><p>Réessayez dans un instant.</p>`))
		return
	}

	_, _ = fmt.Fprint(w, portalShell(`<h1>Merci ! ✅</h1>
		<p>Votre e-photo est bien arrivée au centre. Votre dossier avance — rien d'autre à faire.</p>`))
}

// portalShell: one self-contained mobile page, CAMO warm look.
func portalShell(body string) string {
	return `<!doctype html><html lang="fr"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CAMO-EDUCASER — e-photo</title><style>
body{margin:0;font-family:'Segoe UI',system-ui,sans-serif;background:#f7f4ee;color:#2b2a33;
display:flex;justify-content:center;padding:24px 16px}
main{width:100%;max-width:420px;background:#fff;border-radius:16px;padding:26px 22px;
box-shadow:0 8px 30px rgba(60,50,30,.10)}
h1{font-size:21px;margin:0 0 8px}.brand{color:#4c6fe7;font-weight:800;letter-spacing:.5px;
font-size:13px;margin-bottom:14px}
p{line-height:1.5;font-size:14.5px}
.drop{display:flex;flex-direction:column;align-items:center;gap:6px;border:2px dashed #d8d1c4;
border-radius:14px;padding:26px 14px;margin:16px 0;cursor:pointer;font-weight:600}
.drop.ok{border-color:#2f9e6e;background:#f0faf5}
.drop input{display:none}.fn{font-size:12px;color:#77716a;font-weight:400}
.field{display:block;font-size:13px;color:#77716a;margin:14px 0}
.field small{display:block;margin:2px 0 6px}
.field input{width:100%;padding:11px;border:1px solid #e7e1d6;border-radius:10px;font-size:15px;
box-sizing:border-box}
button{width:100%;padding:14px;background:#4c6fe7;color:#fff;border:none;border-radius:12px;
font-size:16px;font-weight:700;margin-top:8px;cursor:pointer}
.hint{font-size:12.5px;color:#77716a}
.steps{padding-left:20px;font-size:14px;line-height:1.55}
.steps li{margin-bottom:8px}
.steps a{color:#4c6fe7}
</style></head><body><main><div class="brand">CAMO-EDUCASER</div>` + body + `</main></body></html>`
}

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}