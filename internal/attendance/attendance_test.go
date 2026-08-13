package attendance

// Integration tests: idempotence, correction tracing and the counter
// view are database behaviour. Own database, gated on TEST_DATABASE_URL.

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixture struct {
	pool       *pgxpool.Pool
	schoolID   uuid.UUID
	userID     uuid.UUID
	enrollment uuid.UUID
	lesson     uuid.UUID
	assignment uuid.UUID
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
	if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS crit_attendance_test WITH (FORCE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `CREATE DATABASE crit_attendance_test`); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/crit_attendance_test"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	for _, m := range []string{"001_slice1_schema.sql", "002_auth_sessions.sql", "003_attendance.sql"} {
		sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", m))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}

	f := &fixture{
		pool: pool, schoolID: uuid.New(), userID: uuid.New(),
		enrollment: uuid.New(), lesson: uuid.New(), assignment: uuid.New(),
	}
	personID, objectiveID := uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`INSERT INTO school (id, label) VALUES ($1, 'CRIT')`, f.schoolID)
	exec(`INSERT INTO app_user (id, school_id, login, display_name, secret_hash)
	      VALUES ($1, $2, 'sec', 'Angelo', 'x')`, f.userID, f.schoolID)
	exec(`INSERT INTO person (id, school_id, last_name, first_names)
	      VALUES ($1, $2, 'Chiffleau', 'Aurore')`, personID, f.schoolID)
	exec(`INSERT INTO objective (id, school_id, label, hours_before_offroad, total_hours)
	      VALUES ($1, $2, 'CE', 70, 90)`, objectiveID, f.schoolID)
	exec(`INSERT INTO enrollment (id, school_id, person_id, objective_id,
	          hours_before_offroad, total_hours, offroad_target_date)
	      VALUES ($1, $2, $3, $4, 70, 90, (now() + interval '30 days')::date)`,
		f.enrollment, f.schoolID, personID, objectiveID)
	// A 3-hour lesson, already past.
	exec(`INSERT INTO lesson (id, school_id, starts_at, ends_at)
	      VALUES ($1, $2, now() - interval '5 hours', now() - interval '2 hours')`,
		f.lesson, f.schoolID)
	exec(`INSERT INTO lesson_assignment (id, lesson_id, enrollment_id, assigned_by)
	      VALUES ($1, $2, $3, $4)`, f.assignment, f.lesson, f.enrollment, f.userID)
	return f
}

func (f *fixture) record(t *testing.T, value string) {
	t.Helper()
	ctx := context.Background()
	// Mirrors the handler's write path at the SQL level.
	var attendanceID *uuid.UUID
	var current *string
	err := f.pool.QueryRow(ctx, `
		SELECT a.id, a.value::text FROM lesson_assignment la
		LEFT JOIN attendance a ON a.lesson_assignment_id = la.id
		WHERE la.id = $1`, f.assignment).Scan(&attendanceID, &current)
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case attendanceID == nil:
		if _, err := f.pool.Exec(ctx, `
			INSERT INTO attendance (id, school_id, lesson_assignment_id, value, recorded_by)
			VALUES ($1, $2, $3, $4, $5)`,
			uuid.New(), f.schoolID, f.assignment, value, f.userID); err != nil {
			t.Fatal(err)
		}
	case *current == value:
		// idempotent no-op
	default:
		if _, err := f.pool.Exec(ctx, `
			INSERT INTO attendance_correction
			    (id, school_id, attendance_id, previous_value, new_value, corrected_by)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			uuid.New(), f.schoolID, *attendanceID, *current, value, f.userID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `
			UPDATE attendance SET value = $2 WHERE id = $1`, *attendanceID, value); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *fixture) counters(t *testing.T) (attended, consumed float64, projected *float64) {
	t.Helper()
	err := f.pool.QueryRow(context.Background(), `
		SELECT attended_hours, consumed_hours, projected_offroad_hours
		FROM enrollment_hours WHERE enrollment_id = $1`, f.enrollment,
	).Scan(&attended, &consumed, &projected)
	if err != nil {
		t.Fatal(err)
	}
	return
}

// RG-33: a present student counts the FULL lesson duration.
func TestPresentCountsTheFullLessonDuration(t *testing.T) {
	f := setup(t)
	f.record(t, "PRESENT")

	attended, consumed, _ := f.counters(t)
	if attended != 3 || consumed != 3 {
		t.Fatalf("attended=%v consumed=%v, want 3 and 3 (RG-33)", attended, consumed)
	}
}

// RG-172: an absence consumes hours without attending them.
func TestAbsenceConsumesWithoutAttending(t *testing.T) {
	f := setup(t)
	f.record(t, "UNEXCUSED")

	attended, consumed, _ := f.counters(t)
	if attended != 0 || consumed != 3 {
		t.Fatalf("attended=%v consumed=%v, want 0 and 3 (RG-172)", attended, consumed)
	}
}

// RG-119: projection = consumed + planned lessons before the target date,
// and recording a lesson must not double-count it.
func TestProjectionCountsPlannedOnceRecordedLessonsOnce(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// A future 2-hour lesson before the target date, not yet recorded.
	future := uuid.New()
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO lesson (id, school_id, starts_at, ends_at)
		VALUES ($1, $2, now() + interval '2 days', now() + interval '2 days 2 hours')`,
		future, f.schoolID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO lesson_assignment (id, lesson_id, enrollment_id, assigned_by)
		VALUES ($1, $2, $3, $4)`, uuid.New(), future, f.enrollment, f.userID); err != nil {
		t.Fatal(err)
	}

	// Past lesson unrecorded: projection counts BOTH lessons as planned.
	_, _, projected := f.counters(t)
	if projected == nil || *projected != 5 {
		t.Fatalf("projected=%v, want 5 (3h past-planned + 2h future)", projected)
	}

	// Recording the past lesson moves it from planned to consumed —
	// the projection must not change (no double count).
	f.record(t, "PRESENT")
	_, consumed, projected := f.counters(t)
	if consumed != 3 || projected == nil || *projected != 5 {
		t.Fatalf("consumed=%v projected=%v, want 3 and 5", consumed, projected)
	}
}

// RG-126 + RG-20: replaying is silent, changing is a traced correction.
func TestIdempotentReplayAndTracedCorrection(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	f.record(t, "PRESENT")
	f.record(t, "PRESENT") // replay: no correction
	f.record(t, "EXCUSED") // change: one correction

	var corrections int
	if err := f.pool.QueryRow(ctx,
		`SELECT count(*) FROM attendance_correction`).Scan(&corrections); err != nil {
		t.Fatal(err)
	}
	if corrections != 1 {
		t.Fatalf("corrections=%d, want exactly 1 (RG-126, RG-20)", corrections)
	}

	var prev, next string
	if err := f.pool.QueryRow(ctx, `
		SELECT previous_value::text, new_value::text FROM attendance_correction`,
	).Scan(&prev, &next); err != nil {
		t.Fatal(err)
	}
	if prev != "PRESENT" || next != "EXCUSED" {
		t.Fatalf("correction %s→%s, want PRESENT→EXCUSED", prev, next)
	}

	var rows int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM attendance`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("attendance rows=%d, want 1 (idempotent upsert)", rows)
	}
}

// A cancelled lesson consumes nothing even if it was once recorded is a
// non-case (cancelled lessons cannot be recorded); but a cancelled
// PLANNED lesson must not inflate projections.
func TestCancelledLessonLeavesTheProjection(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.pool.Exec(ctx, `
		UPDATE lesson SET status = 'CANCELLED', cancel_reason = 'x' WHERE id = $1`,
		f.lesson); err != nil {
		t.Fatal(err)
	}
	_, _, projected := f.counters(t)
	if projected == nil || *projected != 0 {
		t.Fatalf("projected=%v, want 0: a cancelled lesson projects nothing", projected)
	}
}
