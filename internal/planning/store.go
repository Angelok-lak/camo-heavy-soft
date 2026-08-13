package planning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/angelok-lak/camo-heavy-soft/internal/availability"
)

// Store is the single data-access point of F-09. Every query filters on
// school_id here, once, so no caller can forget it (C-03, RG-209).
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ---------------------------------------------------------------------
// Snapshot loading
// ---------------------------------------------------------------------

// LoadSnapshot assembles everything conflict detection needs over a
// period. Slice 1 has no exam sessions in storage yet: Exams stays empty
// and the detection code is already ready for them.
func (s *Store) LoadSnapshot(ctx context.Context, schoolID uuid.UUID, period availability.TimeSlot) (*availability.Snapshot, error) {
	snap := &availability.Snapshot{
		Period:      period,
		Resources:   map[uuid.UUID]availability.Resource{},
		Enrollments: map[uuid.UUID]availability.Enrollment{},
		LessonKinds: map[uuid.UUID]availability.LessonKind{},
	}

	if err := s.loadResources(ctx, schoolID, snap); err != nil {
		return nil, fmt.Errorf("resources: %w", err)
	}
	if err := s.loadEnrollments(ctx, schoolID, snap); err != nil {
		return nil, fmt.Errorf("enrollments: %w", err)
	}
	if err := s.loadLessonKinds(ctx, schoolID, snap); err != nil {
		return nil, fmt.Errorf("lesson kinds: %w", err)
	}
	if err := s.loadLessons(ctx, schoolID, period, snap); err != nil {
		return nil, fmt.Errorf("lessons: %w", err)
	}
	if err := s.loadUnavailabilities(ctx, schoolID, period, snap); err != nil {
		return nil, fmt.Errorf("unavailabilities: %w", err)
	}
	if err := s.loadExams(ctx, schoolID, period, snap); err != nil {
		return nil, fmt.Errorf("exam sessions: %w", err)
	}
	if err := s.loadOpeningAndClosures(ctx, schoolID, period, snap); err != nil {
		return nil, fmt.Errorf("opening hours: %w", err)
	}
	return snap, nil
}

func (s *Store) loadResources(ctx context.Context, schoolID uuid.UUID, snap *availability.Snapshot) error {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.kind, r.label, r.status = 'ARCHIVED',
		       COALESCE(array_agg(lc.code) FILTER (WHERE lc.code IS NOT NULL), '{}')
		FROM resource r
		LEFT JOIN resource_vehicle_category rvc ON rvc.resource_id = r.id
		LEFT JOIN licence_category lc ON lc.id = rvc.category_id
		WHERE r.school_id = $1
		GROUP BY r.id`, schoolID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r availability.Resource
		var kind string
		if err := rows.Scan(&r.ID, &kind, &r.Label, &r.Archived, &r.Categories); err != nil {
			return err
		}
		r.Kind = availability.ResourceKind(kind)
		snap.Resources[r.ID] = r
	}
	return rows.Err()
}

func (s *Store) loadEnrollments(ctx context.Context, schoolID uuid.UUID, snap *availability.Snapshot) error {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, p.last_name || ' ' || p.first_names,
		       COALESCE(o.label, ''), e.life_status = 'CLOSED',
		       COALESCE(f.status IN ('APPROVED', 'SETTLED')
		                OR COALESCE(fk.self_funded, false), false)
		FROM enrollment e
		JOIN person p ON p.id = e.person_id
		JOIN objective o ON o.id = e.objective_id
		LEFT JOIN funding f ON f.enrollment_id = e.id
		LEFT JOIN funder_kind fk ON fk.id = f.funder_kind_id
		WHERE e.school_id = $1`, schoolID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e availability.Enrollment
		if err := rows.Scan(&e.ID, &e.StudentName, &e.Category, &e.Closed, &e.FundingOK); err != nil {
			return err
		}
		snap.Enrollments[e.ID] = e
	}
	return rows.Err()
}

func (s *Store) loadLessonKinds(ctx context.Context, schoolID uuid.UUID, snap *availability.Snapshot) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, label, requires_vehicle
		FROM lesson_kind WHERE school_id = $1 AND active`, schoolID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k availability.LessonKind
		if err := rows.Scan(&k.ID, &k.Label, &k.RequiresVehicle); err != nil {
			return err
		}
		snap.LessonKinds[k.ID] = k
	}
	return rows.Err()
}

func (s *Store) loadLessons(ctx context.Context, schoolID uuid.UUID, period availability.TimeSlot, snap *availability.Snapshot) error {
	// Cancelled lessons load too: the view shows them, detection skips them.
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, l.starts_at, l.ends_at, l.max_students,
		       l.status = 'CANCELLED',
		       COALESCE((SELECT array_agg(lr.resource_id) FROM lesson_resource lr WHERE lr.lesson_id = l.id), '{}'),
		       COALESCE((SELECT array_agg(la.enrollment_id) FROM lesson_assignment la WHERE la.lesson_id = l.id), '{}'),
		       COALESCE((SELECT array_agg(llk.lesson_kind_id) FROM lesson_lesson_kind llk WHERE llk.lesson_id = l.id), '{}')
		FROM lesson l
		WHERE l.school_id = $1
		  AND l.slot && tstzrange($2, $3, '[)')`,
		schoolID, period.Start, period.End)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var l availability.Lesson
		var kindIDs []uuid.UUID
		if err := rows.Scan(&l.ID, &l.Slot.Start, &l.Slot.End, &l.MaxStudents,
			&l.Cancelled, &l.Resources, &l.Enrollments, &kindIDs); err != nil {
			return err
		}
		for _, kid := range kindIDs {
			if k, ok := snap.LessonKinds[kid]; ok {
				l.Kinds = append(l.Kinds, k)
			}
		}
		snap.Lessons = append(snap.Lessons, l)
	}
	return rows.Err()
}

func (s *Store) loadUnavailabilities(ctx context.Context, schoolID uuid.UUID, period availability.TimeSlot, snap *availability.Snapshot) error {
	// ends_at NULL means "return unknown" (RG-65): it overlaps any future
	// period, and the zero End carries the meaning into the component.
	rows, err := s.pool.Query(ctx, `
		SELECT resource_id, starts_at, ends_at, reason
		FROM unavailability
		WHERE school_id = $1 AND status = 'ONGOING'
		  AND (ends_at IS NULL OR tstzrange(starts_at, ends_at, '[)') && tstzrange($2, $3, '[)'))
		  AND starts_at < $3`,
		schoolID, period.Start, period.End)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var u availability.Unavailability
		var ends *time.Time
		if err := rows.Scan(&u.ResourceID, &u.Slot.Start, &ends, &u.Reason); err != nil {
			return err
		}
		if ends != nil {
			u.Slot.End = *ends
		}
		snap.Unavailable = append(snap.Unavailable, u)
	}
	return rows.Err()
}

// loadExams widens the window by the travel time on both sides: a
// session just outside the period can still tie up resources inside it
// (RG-140).
func (s *Store) loadExams(ctx context.Context, schoolID uuid.UUID, period availability.TimeSlot, snap *availability.Snapshot) error {
	rows, err := s.pool.Query(ctx, `
		SELECT es.id, es.starts_at, es.ends_at, ep.label, ep.travel_minutes,
		       COALESCE((SELECT array_agg(resource_id) FROM exam_session_resource
		                 WHERE exam_session_id = es.id), '{}')
		FROM exam_session es
		JOIN exam_place ep ON ep.id = es.exam_place_id
		WHERE es.school_id = $1 AND es.status = 'SCHEDULED'
		  AND tstzrange(es.starts_at - make_interval(mins => ep.travel_minutes),
		                es.ends_at + make_interval(mins => ep.travel_minutes), '[)')
		      && tstzrange($2, $3, '[)')`,
		schoolID, period.Start, period.End)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ex availability.ExamSession
		var travelMinutes int
		if err := rows.Scan(&ex.ID, &ex.Slot.Start, &ex.Slot.End,
			&ex.PlaceLabel, &travelMinutes, &ex.Resources); err != nil {
			return err
		}
		ex.TravelTime = time.Duration(travelMinutes) * time.Minute
		snap.Exams = append(snap.Exams, ex)
	}
	return rows.Err()
}

func (s *Store) loadOpeningAndClosures(ctx context.Context, schoolID uuid.UUID, period availability.TimeSlot, snap *availability.Snapshot) error {
	rows, err := s.pool.Query(ctx, `
		SELECT weekday, to_char(starts_at, 'HH24:MI'), to_char(ends_at, 'HH24:MI')
		FROM opening_hours WHERE school_id = $1`, schoolID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var o availability.OpeningHours
		var weekday int
		if err := rows.Scan(&weekday, &o.Start, &o.End); err != nil {
			return err
		}
		// ISO weekday 1–7 → Go's Sunday-based Weekday.
		o.Weekday = time.Weekday(weekday % 7)
		snap.Opening = append(snap.Opening, o)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	crows, err := s.pool.Query(ctx, `
		SELECT lower(period), upper(period)
		FROM closure
		WHERE school_id = $1 AND period && daterange($2::date, $3::date, '[)')`,
		schoolID, period.Start, period.End)
	if err != nil {
		return err
	}
	defer crows.Close()
	for crows.Next() {
		var from, to time.Time
		if err := crows.Scan(&from, &to); err != nil {
			return err
		}
		snap.Closures = append(snap.Closures, availability.TimeSlot{Start: from, End: to})
	}
	return crows.Err()
}

// ---------------------------------------------------------------------
// Edit session — the lock and the journal
// ---------------------------------------------------------------------

type EditSession struct {
	ID           uuid.UUID              `json:"id"`
	UserID       uuid.UUID              `json:"user_id"`
	Scope        availability.TimeSlot  `json:"scope"`
	Status       string                 `json:"status"`
	Operations   []Operation            `json:"operations"`
	OpenedAt     time.Time              `json:"opened_at"`
	LastActivity time.Time              `json:"last_activity_at"`
}

// The lock falls by inactivity (RG-145): an OPEN session silent for this
// long stops holding anything. Expiry is LAZY — applied when someone
// tries to open — so no scheduled job can forget to run. Will move to
// parametre_centre with the settings work.
const lockIdle = 30 * time.Minute

// LockHeldError names the holder: RG-144 requires the refusal to say WHO
// is editing and since WHEN, so the planner can walk over and ask.
// HolderUserID lets the front offer "resume" when the holder is yourself
// — the usual zombie case after a page reload.
type LockHeldError struct {
	HolderName   string    `json:"holder_name"`
	HolderUserID uuid.UUID `json:"holder_user_id"`
	OpenedAt     time.Time `json:"opened_at"`
	SessionID    uuid.UUID `json:"session_id"`
}

func (e *LockHeldError) Error() string {
	return fmt.Sprintf("planner locked by %s since %s", e.HolderName, e.OpenedAt.Format("15:04"))
}

// StaleScopeError describes what changed outside the session (RG-259):
// never an overwrite, always a description.
type StaleScopeError struct {
	ChangedLessons []uuid.UUID `json:"changed_lessons"`
}

func (e *StaleScopeError) Error() string {
	return fmt.Sprintf("%d lessons changed outside the session", len(e.ChangedLessons))
}

// scopeFingerprint captures lesson versions at opening. Compared at save
// time to detect outside modifications (RG-259).
func (s *Store) scopeFingerprint(ctx context.Context, q querier, schoolID uuid.UUID, scope availability.TimeSlot) (map[uuid.UUID]int, error) {
	rows, err := q.Query(ctx, `
		SELECT id, version FROM lesson
		WHERE school_id = $1 AND slot && tstzrange($2, $3, '[)')`,
		schoolID, scope.Start, scope.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fp := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var v int
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		fp[id] = v
	}
	return fp, rows.Err()
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// OpenEditSession takes the lock. THE LOCK IS THE EXCLUDE CONSTRAINT in
// the database (C-09): if the insert violates it, someone already edits an
// overlapping scope, and the error names them.
func (s *Store) OpenEditSession(ctx context.Context, schoolID, userID uuid.UUID, scope availability.TimeSlot) (*EditSession, error) {
	// Reap zombie sessions first (RG-145): stale locks fall here, not in
	// a cron. The journal stays on the row for consultation.
	_, err := s.pool.Exec(ctx, `
		UPDATE edit_session SET status = 'DISCARDED', closed_at = now()
		WHERE school_id = $1 AND status = 'OPEN'
		  AND last_activity_at < now() - $2::interval`,
		schoolID, lockIdle.String())
	if err != nil {
		return nil, err
	}

	fp, err := s.scopeFingerprint(ctx, s.pool, schoolID, scope)
	if err != nil {
		return nil, err
	}
	fpJSON, err := json.Marshal(fp)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	_, err = s.pool.Exec(ctx, `
		INSERT INTO edit_session
		    (id, school_id, user_id, scope_starts_at, scope_ends_at, state_at_open, opened_at, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
		id, schoolID, userID, scope.Start, scope.End, string(fpJSON), now)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23P01" { // exclusion_violation
		return nil, s.describeHolder(ctx, schoolID, scope)
	}
	if err != nil {
		return nil, err
	}

	return &EditSession{
		ID: id, UserID: userID, Scope: scope, Status: "OPEN",
		OpenedAt: now, LastActivity: now,
	}, nil
}

func (s *Store) describeHolder(ctx context.Context, schoolID uuid.UUID, scope availability.TimeSlot) error {
	var holder LockHeldError
	err := s.pool.QueryRow(ctx, `
		SELECT es.id, u.id, u.display_name, es.opened_at
		FROM edit_session es JOIN app_user u ON u.id = es.user_id
		WHERE es.school_id = $1 AND es.status = 'OPEN'
		  AND es.scope && tstzrange($2, $3, '[)')
		LIMIT 1`,
		schoolID, scope.Start, scope.End,
	).Scan(&holder.SessionID, &holder.HolderUserID, &holder.HolderName, &holder.OpenedAt)
	if err != nil {
		return fmt.Errorf("planner locked, holder lookup failed: %w", err)
	}
	return &holder
}

// GetOpenSession returns an OPEN session with its journal.
func (s *Store) GetOpenSession(ctx context.Context, schoolID, sessionID uuid.UUID) (*EditSession, error) {
	var es EditSession
	var opsJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, scope_starts_at, scope_ends_at, status, operations, opened_at, last_activity_at
		FROM edit_session
		WHERE school_id = $1 AND id = $2 AND status = 'OPEN'`,
		schoolID, sessionID,
	).Scan(&es.ID, &es.UserID, &es.Scope.Start, &es.Scope.End, &es.Status,
		&opsJSON, &es.OpenedAt, &es.LastActivity)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoOpenSession
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(opsJSON, &es.Operations); err != nil {
		return nil, fmt.Errorf("corrupt journal: %w", err)
	}
	return &es, nil
}

var ErrNoOpenSession = errors.New("no open edit session")

// ReplaceOperations persists the full journal (C-14: it survives a
// disconnect) and refreshes the activity timestamp (RG-145).
func (s *Store) ReplaceOperations(ctx context.Context, schoolID, sessionID uuid.UUID, ops []Operation) error {
	opsJSON, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE edit_session
		SET operations = $3, last_activity_at = now()
		WHERE school_id = $1 AND id = $2 AND status = 'OPEN'`,
		schoolID, sessionID, opsJSON)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoOpenSession
	}
	return nil
}

// Discard releases the lock and applies nothing (RG-147). The journal is
// kept on the row for consultation, the status alone closes it.
func (s *Store) Discard(ctx context.Context, schoolID, sessionID uuid.UUID) error {
	return s.close(ctx, schoolID, sessionID, "DISCARDED", nil)
}

// ForceRelease frees a stuck lock, tracing who did it (RG-145).
// Management only — enforced by the permission layer, not here.
func (s *Store) ForceRelease(ctx context.Context, schoolID, sessionID, releasedBy uuid.UUID) error {
	return s.close(ctx, schoolID, sessionID, "DISCARDED", &releasedBy)
}

func (s *Store) close(ctx context.Context, schoolID, sessionID uuid.UUID, status string, forcedBy *uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE edit_session
		SET status = $3, closed_at = now(), force_released_by = $4
		WHERE school_id = $1 AND id = $2 AND status = 'OPEN'`,
		schoolID, sessionID, status, forcedBy)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoOpenSession
	}
	return nil
}
