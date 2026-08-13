package planning

// Integration tests against a real PostgreSQL: the lock is an EXCLUDE
// constraint (C-09) and the save path is a transaction — neither can be
// proven with mocks. Skipped unless TEST_DATABASE_URL is set, e.g.:
//
//	docker run -d -p 5544:5432 -e POSTGRES_PASSWORD=test postgres:16
//	TEST_DATABASE_URL=postgres://postgres:test@localhost:5544/postgres go test ./internal/planning/

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/angelok-lak/camo-heavy-soft/internal/availability"
)

type fixture struct {
	store    *Store
	pool     *pgxpool.Pool
	schoolID uuid.UUID
	userID   uuid.UUID
	otherID  uuid.UUID
	vehicle  uuid.UUID
}

func setup(t *testing.T) *fixture {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	// Own database per package, recreated per run: the target database
	// (often a dev one with seeded data) is never touched.
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS crit_planning_test WITH (FORCE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `CREATE DATABASE crit_planning_test`); err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/crit_planning_test"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	migrations, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(migrations)
	for _, m := range migrations {
		sql, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", filepath.Base(m), err)
		}
	}

	f := &fixture{
		store: NewStore(pool), pool: pool,
		schoolID: uuid.New(), userID: uuid.New(), otherID: uuid.New(),
		vehicle: uuid.New(),
	}
	mustExec(t, pool, `INSERT INTO school (id, label) VALUES ($1, 'CRIT')`, f.schoolID)
	mustExec(t, pool, `INSERT INTO app_user (id, school_id, login, display_name, secret_hash)
		VALUES ($1, $2, 'sec', 'Angelo', 'x'), ($3, $2, 'form', 'Alhan', 'x')`,
		f.userID, f.schoolID, f.otherID)
	mustExec(t, pool, `INSERT INTO resource (id, school_id, kind, label) VALUES ($1, $2, 'VEHICLE', 'CE-01')`,
		f.vehicle, f.schoolID)
	mustExec(t, pool, `INSERT INTO resource_vehicle (resource_id) VALUES ($1)`, f.vehicle)
	return f
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func week(t *testing.T) availability.TimeSlot {
	t.Helper()
	start, err := time.Parse("2006-01-02", "2026-10-12")
	if err != nil {
		t.Fatal(err)
	}
	return availability.TimeSlot{Start: start, End: start.AddDate(0, 0, 7)}
}

// RG-144: the lock lives in the database, and the refusal names the holder.
func TestLockIsHeldByTheDatabaseAndNamesItsHolder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	scope := week(t)

	first, err := f.store.OpenEditSession(ctx, f.schoolID, f.userID, scope)
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.store.OpenEditSession(ctx, f.schoolID, f.otherID, scope)
	held, ok := err.(*LockHeldError)
	if !ok {
		t.Fatalf("expected LockHeldError, got %v", err)
	}
	if held.HolderName != "Angelo" {
		t.Fatalf("holder = %q, want the FIRST editor named (RG-144)", held.HolderName)
	}

	// Discarding releases the lock: the second editor gets in (RG-147).
	if err := f.store.Discard(ctx, f.schoolID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.OpenEditSession(ctx, f.schoolID, f.otherID, scope); err != nil {
		t.Fatalf("lock must fall with the discard: %v", err)
	}
}

// The full journey: open, push, save. The batch is atomic and traced.
func TestSaveReplaysTheJournalAtomically(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	scope := week(t)

	session, err := f.store.OpenEditSession(ctx, f.schoolID, f.userID, scope)
	if err != nil {
		t.Fatal(err)
	}

	lessonID := uuid.New()
	slot := availability.TimeSlot{
		Start: scope.Start.Add(9 * time.Hour),
		End:   scope.Start.Add(12 * time.Hour),
	}
	create := Operation{ID: uuid.New(), Kind: CreateLesson, LessonID: lessonID, Slot: &slot}
	assign := Operation{ID: uuid.New(), Kind: AssignResource, LessonID: lessonID, ResourceID: &f.vehicle}

	if err := f.store.ReplaceOperations(ctx, f.schoolID, session.ID, []Operation{create, assign}); err != nil {
		t.Fatal(err)
	}

	report, err := f.store.Save(ctx, f.schoolID, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.BatchID != session.ID {
		t.Fatal("the batch id must be the session id (RG-70)")
	}
	if len(report.Applied) != 2 {
		t.Fatalf("applied = %d, want 2", len(report.Applied))
	}

	// The lesson landed with its resource.
	snap, err := f.store.LoadSnapshot(ctx, f.schoolID, scope)
	if err != nil {
		t.Fatal(err)
	}
	l, found := findLesson(snap, lessonID)
	if !found || len(l.Resources) != 1 {
		t.Fatalf("saved lesson not found or without its vehicle: %+v", l)
	}

	// D-05: the events carry the batch id.
	var events int
	err = f.pool.QueryRow(ctx,
		`SELECT count(*) FROM domain_event WHERE batch_id = $1`, session.ID).Scan(&events)
	if err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Fatalf("domain events = %d, want 2 (D-05)", events)
	}
}

// RG-259: a scope that moved outside the session refuses with a description.
func TestOutsideModificationRefusesTheSaveAndSaysWhat(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	scope := week(t)

	// A saved lesson pre-exists.
	preexisting := uuid.New()
	mustExec(t, f.pool, `INSERT INTO lesson (id, school_id, starts_at, ends_at)
		VALUES ($1, $2, $3, $4)`,
		preexisting, f.schoolID, scope.Start.Add(8*time.Hour), scope.Start.Add(10*time.Hour))

	session, err := f.store.OpenEditSession(ctx, f.schoolID, f.userID, scope)
	if err != nil {
		t.Fatal(err)
	}
	move := availability.TimeSlot{Start: scope.Start.Add(14 * time.Hour), End: scope.Start.Add(16 * time.Hour)}
	err = f.store.ReplaceOperations(ctx, f.schoolID, session.ID, []Operation{
		{ID: uuid.New(), Kind: MoveLesson, LessonID: preexisting, Slot: &move},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Someone else bumps the lesson meanwhile (e.g. a force-released
	// session saved before ours).
	mustExec(t, f.pool, `UPDATE lesson SET version = version + 1 WHERE id = $1`, preexisting)

	_, err = f.store.Save(ctx, f.schoolID, session.ID)
	stale, ok := err.(*StaleScopeError)
	if !ok {
		t.Fatalf("expected StaleScopeError, got %v", err)
	}
	if len(stale.ChangedLessons) != 1 || stale.ChangedLessons[0] != preexisting {
		t.Fatalf("the refusal must NAME what changed (RG-259), got %v", stale.ChangedLessons)
	}
}

// Arbitrage RG-259: only a write colliding with a write refuses. An
// outside change on a lesson the draft never targets lets the save pass —
// its consequences surface as recomputed conflicts, not as a refusal.
func TestOutsideChangeOnUntouchedLessonDoesNotRefuse(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	scope := week(t)

	untouched := uuid.New()
	mustExec(t, f.pool, `INSERT INTO lesson (id, school_id, starts_at, ends_at)
		VALUES ($1, $2, $3, $4)`,
		untouched, f.schoolID, scope.Start.Add(8*time.Hour), scope.Start.Add(10*time.Hour))

	session, err := f.store.OpenEditSession(ctx, f.schoolID, f.userID, scope)
	if err != nil {
		t.Fatal(err)
	}
	// The draft only creates a NEW lesson, elsewhere in the week.
	lessonID := uuid.New()
	slot := availability.TimeSlot{Start: scope.Start.Add(30 * time.Hour), End: scope.Start.Add(32 * time.Hour)}
	err = f.store.ReplaceOperations(ctx, f.schoolID, session.ID, []Operation{
		{ID: uuid.New(), Kind: CreateLesson, LessonID: lessonID, Slot: &slot},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Meanwhile the untouched lesson moves.
	mustExec(t, f.pool, `UPDATE lesson SET version = version + 1 WHERE id = $1`, untouched)

	if _, err := f.store.Save(ctx, f.schoolID, session.ID); err != nil {
		t.Fatalf("an outside change on an untouched lesson must not refuse: %v", err)
	}
}

// D-04 + A-08: saving over a live conflict never blocks, and leaves a trace.
func TestSavingOverAConflictTracesTheOverride(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	scope := week(t)

	// The vehicle is down for the whole week.
	mustExec(t, f.pool, `INSERT INTO unavailability (id, school_id, resource_id, reason, starts_at, declared_by)
		VALUES ($1, $2, $3, 'Breakdown', $4, $5)`,
		uuid.New(), f.schoolID, f.vehicle, scope.Start, f.userID)

	session, err := f.store.OpenEditSession(ctx, f.schoolID, f.userID, scope)
	if err != nil {
		t.Fatal(err)
	}
	lessonID := uuid.New()
	slot := availability.TimeSlot{
		Start: scope.Start.Add(9 * time.Hour), End: scope.Start.Add(11 * time.Hour),
	}
	err = f.store.ReplaceOperations(ctx, f.schoolID, session.ID, []Operation{
		{ID: uuid.New(), Kind: CreateLesson, LessonID: lessonID, Slot: &slot},
		{ID: uuid.New(), Kind: AssignResource, LessonID: lessonID, ResourceID: &f.vehicle},
	})
	if err != nil {
		t.Fatal(err)
	}

	report, err := f.store.Save(ctx, f.schoolID, session.ID)
	if err != nil {
		t.Fatalf("the system alerts, it never blocks (D-04): %v", err)
	}
	if len(report.Overridden) == 0 {
		t.Fatal("the report must carry the overridden conflicts")
	}

	var stored int
	err = f.pool.QueryRow(ctx,
		`SELECT count(*) FROM overridden_conflict WHERE edit_session_id = $1`, session.ID).Scan(&stored)
	if err != nil {
		t.Fatal(err)
	}
	if stored == 0 {
		t.Fatal("an override is a fact, it is stored (A-08)")
	}
}
