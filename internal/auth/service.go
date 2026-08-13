package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Sliding expiry: a session lives as long as it is used, and dies after
// this much silence. Product structure, not a business parameter.
const sessionIdle = 12 * time.Hour

// ErrBadCredentials is deliberately the ONLY login failure the outside
// world sees: unknown login, wrong secret and suspended account are
// indistinguishable, so the form enumerates nothing.
var ErrBadCredentials = errors.New("invalid login or secret")

var ErrNoSession = errors.New("no valid session")

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Login checks the secret and opens a session. The returned token goes
// into an HttpOnly cookie; only its hash is stored (migration 002).
func (s *Service) Login(ctx context.Context, login, secret string) (Identity, string, error) {
	var id Identity
	var hash string
	err := s.pool.QueryRow(ctx, `
		SELECT id, school_id, display_name, secret_hash
		FROM app_user WHERE login = $1 AND status = 'ACTIVE'`, login,
	).Scan(&id.UserID, &id.SchoolID, &id.Name, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		// Burn comparable time so a missing login is not measurably
		// faster than a wrong secret.
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"), []byte(secret))
		return Identity{}, "", ErrBadCredentials
	}
	if err != nil {
		return Identity{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) != nil {
		return Identity{}, "", ErrBadCredentials
	}

	if err := s.loadProfiles(ctx, &id); err != nil {
		return Identity{}, "", err
	}

	token, tokenHash, err := newToken()
	if err != nil {
		return Identity{}, "", err
	}
	sessionID, err := uuid.NewV7()
	if err != nil {
		return Identity{}, "", err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO auth_session (id, school_id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, now() + $5::interval)`,
		sessionID, id.SchoolID, id.UserID, tokenHash, sessionIdle.String())
	if err != nil {
		return Identity{}, "", err
	}
	return id, token, nil
}

// Identify resolves a token into the caller's identity and slides the
// expiry. Rights are re-read on EVERY request: a suspension or a profile
// change applies immediately (RG-211), no session invalidation dance.
func (s *Service) Identify(ctx context.Context, token string) (Identity, error) {
	var id Identity
	err := s.pool.QueryRow(ctx, `
		UPDATE auth_session AS a
		SET last_seen_at = now(), expires_at = now() + $2::interval
		FROM app_user u
		WHERE a.token_hash = $1
		  AND a.revoked_at IS NULL
		  AND a.expires_at > now()
		  AND u.id = a.user_id
		  AND u.status = 'ACTIVE'
		RETURNING u.id, u.school_id, u.display_name`,
		hashToken(token), sessionIdle.String(),
	).Scan(&id.UserID, &id.SchoolID, &id.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrNoSession
	}
	if err != nil {
		return Identity{}, err
	}
	if err := s.loadProfiles(ctx, &id); err != nil {
		return Identity{}, err
	}
	return id, nil
}

// Logout revokes the session row: the cookie may linger, it opens nothing.
func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE auth_session SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`, hashToken(token))
	return err
}

// InstructorResource returns the instructor sheet linked to the account,
// if any (RG-210): the planner highlights "my" lessons with it.
func (s *Service) InstructorResource(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	var resourceID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT resource_id FROM resource_instructor WHERE user_id = $1`, userID,
	).Scan(&resourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &resourceID, nil
}

// loadProfiles reads the OPEN profiles; rights are their union (RG-208).
func (s *Service) loadProfiles(ctx context.Context, id *Identity) error {
	rows, err := s.pool.Query(ctx, `
		SELECT profile FROM user_profile
		WHERE user_id = $1 AND ends_at IS NULL`, id.UserID)
	if err != nil {
		return err
	}
	defer rows.Close()
	id.Profiles = nil
	for rows.Next() {
		var p Profile
		if err := rows.Scan(&p); err != nil {
			return err
		}
		id.Profiles = append(id.Profiles, p)
	}
	return rows.Err()
}

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("token entropy: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
