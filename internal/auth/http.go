package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/google/uuid"
)

const cookieName = "crit_session"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// PublicRoutes mounts what works WITHOUT an identity.
func (h *Handler) PublicRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.logout)
}

// PrivateRoutes mounts what needs the middleware in front.
func (h *Handler) PrivateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/me", h.me)
}

// Middleware resolves the session cookie into an Identity. Rights are
// re-read per request, so a suspension applies immediately (RG-211).
func Middleware(svc *Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		id, err := svc.Identify(r.Context(), c.Value)
		if errors.Is(err, ErrNoSession) {
			jsonError(w, http.StatusUnauthorized, "session expired")
			return
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
	})
}

// contextView answers "read my context" (F-17 contract): account,
// profiles, school, linked instructor sheet (RG-210), and the effective
// permissions — computed HERE, the front decides nothing (C-26).
type contextView struct {
	UserID       uuid.UUID  `json:"user_id"`
	Name         string     `json:"name"`
	SchoolID     uuid.UUID  `json:"school_id"`
	Profiles     []Profile  `json:"profiles"`
	InstructorID *uuid.UUID `json:"instructor_resource_id"`
	Permissions  struct {
		EditPlanning    bool `json:"edit_planning"`
		ForceRelease    bool `json:"force_release"`
		ManageResources bool `json:"manage_resources"`
		ManagePeople    bool `json:"manage_people"`
		ManageSettings  bool `json:"manage_settings"`
	} `json:"permissions"`
}

func (h *Handler) contextViewFor(r *http.Request, id Identity) (contextView, error) {
	view := contextView{
		UserID:   id.UserID,
		Name:     id.Name,
		SchoolID: id.SchoolID,
		Profiles: id.Profiles,
	}
	if view.Profiles == nil {
		view.Profiles = []Profile{}
	}
	view.Permissions.EditPlanning = id.CanEditPlanning()
	view.Permissions.ForceRelease = id.CanForceRelease()
	view.Permissions.ManageResources = id.CanManageResources()
	view.Permissions.ManagePeople = id.CanManagePeople()
	view.Permissions.ManageSettings = id.CanManageSettings()

	instructorID, err := h.svc.InstructorResource(r.Context(), id.UserID)
	if err != nil {
		return view, err
	}
	view.InstructorID = instructorID
	return view, nil
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Login  string `json:"login"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Login == "" {
		jsonError(w, http.StatusUnprocessableEntity, "login and secret required")
		return
	}

	id, token, err := h.svc.Login(r.Context(), body.Login, body.Secret)
	if errors.Is(err, ErrBadCredentials) {
		jsonError(w, http.StatusUnauthorized, "invalid login or secret")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	http.SetCookie(w, sessionCookie(token, int(sessionIdle.Seconds())))
	view, err := h.contextViewFor(r, id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	replyJSON(w, http.StatusOK, view)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, ok := FromContext(r.Context())
	if !ok {
		jsonError(w, http.StatusUnauthorized, "no identity")
		return
	}
	view, err := h.contextViewFor(r, id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	replyJSON(w, http.StatusOK, view)
}

func sessionCookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Dev runs over plain http; anything deployed sets COOKIE_SECURE.
		Secure: os.Getenv("COOKIE_SECURE") == "1",
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	replyJSON(w, status, map[string]string{"error": msg})
}

func replyJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
