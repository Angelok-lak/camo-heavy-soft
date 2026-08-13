package auth

// Integration tests: sessions and their revocation live in PostgreSQL.
// Skipped unless TEST_DATABASE_URL is set. They run in their own
// database (crit_auth_test) so the planning package's tests, which reset
// the schema of the target database, cannot race them.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type fixture struct {
	svc      *Service
	pool     *pgxpool.Pool
	schoolID uuid.UUID
	userID   uuid.UUID
}

func setup(t *testing.T) *fixture {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS crit_auth_test WITH (FORCE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `CREATE DATABASE crit_auth_test`); err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/crit_auth_test"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	for _, m := range []string{"001_slice1_schema.sql", "002_auth_sessions.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", m))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}

	f := &fixture{svc: NewService(pool), pool: pool, schoolID: uuid.New(), userID: uuid.New()}
	hash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, f, `INSERT INTO school (id, label) VALUES ($1, 'CRIT')`, f.schoolID)
	mustExec(t, f, `INSERT INTO app_user (id, school_id, login, display_name, secret_hash)
		VALUES ($1, $2, 'angelo', 'Angelo', $3)`, f.userID, f.schoolID, string(hash))
	mustExec(t, f, `INSERT INTO user_profile (id, user_id, profile) VALUES ($1, $2, 'OFFICE')`,
		uuid.New(), f.userID)
	return f
}

func mustExec(t *testing.T, f *fixture, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func TestLoginThenIdentify(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	id, token, err := f.svc.Login(ctx, "angelo", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if id.Name != "Angelo" || !id.CanEditPlanning() {
		t.Fatalf("unexpected identity: %+v", id)
	}

	back, err := f.svc.Identify(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if back.UserID != f.userID {
		t.Fatal("token resolved to the wrong account")
	}
}

// One generic failure for every cause: nothing to enumerate.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, _, err := f.svc.Login(ctx, "angelo", "wrong"); err != ErrBadCredentials {
		t.Fatalf("wrong secret: %v", err)
	}
	if _, _, err := f.svc.Login(ctx, "nobody", "s3cret"); err != ErrBadCredentials {
		t.Fatalf("unknown login: %v", err)
	}
	mustExec(t, f, `UPDATE app_user SET status = 'SUSPENDED' WHERE id = $1`, f.userID)
	if _, _, err := f.svc.Login(ctx, "angelo", "s3cret"); err != ErrBadCredentials {
		t.Fatalf("suspended account: %v", err)
	}
}

// RG-211: suspension cuts a session already open, on the next request.
func TestSuspensionKillsTheOpenSession(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	_, token, err := f.svc.Login(ctx, "angelo", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, f, `UPDATE app_user SET status = 'SUSPENDED' WHERE id = $1`, f.userID)

	if _, err := f.svc.Identify(ctx, token); err != ErrNoSession {
		t.Fatalf("a suspended account must lose its session immediately: %v", err)
	}
}

func TestLogoutRevokes(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	_, token, err := f.svc.Login(ctx, "angelo", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Identify(ctx, token); err != ErrNoSession {
		t.Fatalf("revoked session still resolves: %v", err)
	}
}

// RG-208: rights are the union of open profiles; a closed one drops out.
func TestRightsAreTheUnionOfOpenProfiles(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	profileID := uuid.New()
	mustExec(t, f, `INSERT INTO user_profile (id, user_id, profile) VALUES ($1, $2, 'MANAGEMENT')`,
		profileID, f.userID)

	id, _, err := f.svc.Login(ctx, "angelo", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if !id.CanForceRelease() {
		t.Fatal("management profile must grant force-release (union, RG-208)")
	}

	// Closing the period removes the right — historised, not deleted (RG-213).
	mustExec(t, f, `UPDATE user_profile SET ends_at = now() WHERE id = $1`, profileID)
	_, token, err := f.svc.Login(ctx, "angelo", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	back, err := f.svc.Identify(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if back.CanForceRelease() {
		t.Fatal("a closed profile period must not keep granting rights")
	}
	if fmt.Sprintf("%v", back.Profiles) != "[OFFICE]" {
		t.Fatalf("profiles = %v, want [OFFICE]", back.Profiles)
	}
}
