package planning

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/angelok-lak/camo-heavy-soft/internal/availability"
)

// BatchReport is what every multi-object write returns (C-27): what was
// applied, what was skipped and why, and the batch identifier — which IS
// the edit session id (C-09, RG-70).
type BatchReport struct {
	BatchID  uuid.UUID   `json:"batch_id"`
	Applied  []Operation `json:"applied"`
	Rejected []Rejection `json:"rejected"`
	// Overridden lists the conflicts that were live at save time and
	// saved anyway. The system alerted, the planner decided: that is a
	// fact, so it is stored (D-04, A-08).
	Overridden []availability.Conflict `json:"overridden"`
}

// Save replays the journal in ONE transaction: everything applies or
// nothing does (C-01). Sequence:
//
//  1. Lock the session row and re-read the journal.
//  2. Compare lesson versions with the fingerprint taken at opening —
//     a mismatch is a refusal DESCRIBING what moved, never an overwrite
//     (RG-259).
//  3. Replay the operations onto the tables.
//  4. Record still-live conflicts as overridden (A-08) and emit domain
//     events carrying the batch id (D-05, RG-70).
//  5. Mark the session SAVED, which releases the lock.
func (s *Store) Save(ctx context.Context, schoolID, sessionID uuid.UUID) (*BatchReport, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 1. The session row, locked for the duration of the save.
	var userID uuid.UUID
	var scope availability.TimeSlot
	var opsJSON, fpJSON []byte
	err = tx.QueryRow(ctx, `
		SELECT user_id, scope_starts_at, scope_ends_at, operations, state_at_open
		FROM edit_session
		WHERE school_id = $1 AND id = $2 AND status = 'OPEN'
		FOR UPDATE`,
		schoolID, sessionID,
	).Scan(&userID, &scope.Start, &scope.End, &opsJSON, &fpJSON)
	if err == pgx.ErrNoRows {
		return nil, ErrNoOpenSession
	}
	if err != nil {
		return nil, err
	}
	var ops []Operation
	if err := json.Unmarshal(opsJSON, &ops); err != nil {
		return nil, fmt.Errorf("corrupt journal: %w", err)
	}

	// 2. RG-259 — did the scope move under the session?
	var atOpen map[uuid.UUID]int
	if len(fpJSON) > 0 {
		if err := json.Unmarshal(fpJSON, &atOpen); err != nil {
			return nil, fmt.Errorf("corrupt fingerprint: %w", err)
		}
	}
	now, err := s.scopeFingerprint(ctx, tx, schoolID, scope)
	if err != nil {
		return nil, err
	}
	if changed := diffFingerprints(atOpen, now, ops); len(changed) > 0 {
		return nil, &StaleScopeError{ChangedLessons: changed}
	}

	// 3. Replay onto the current snapshot to get final state + conflicts.
	snap, err := s.LoadSnapshot(ctx, schoolID, scope)
	if err != nil {
		return nil, err
	}
	state := Apply(snap, ops)

	report := &BatchReport{BatchID: sessionID, Applied: state.Pending, Rejected: state.Rejected}

	for _, op := range state.Pending {
		if err := s.persistOp(ctx, tx, schoolID, userID, op); err != nil {
			return nil, fmt.Errorf("op %s (%s): %w", op.ID, op.Kind, err)
		}
	}

	// 4. Conflicts still live at save time were seen and walked over:
	// store the override (A-08) with the batch as its context.
	for _, conflicts := range state.Conflicts {
		for _, c := range conflicts {
			report.Overridden = append(report.Overridden, c)
			parties, err := json.Marshal(c.Parties)
			if err != nil {
				return nil, err
			}
			ocID, err := uuid.NewV7()
			if err != nil {
				return nil, err
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO overridden_conflict
				    (id, school_id, edit_session_id, conflict_kind, parties, overridden_by)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				ocID, schoolID, sessionID, string(c.Kind), parties, userID)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, op := range state.Pending {
		if err := emitEvent(ctx, tx, schoolID, sessionID, userID, op); err != nil {
			return nil, err
		}
	}

	// 5. Saving closes the session; the EXCLUDE constraint only guards
	// OPEN rows, so the lock falls with the status.
	_, err = tx.Exec(ctx, `
		UPDATE edit_session SET status = 'SAVED', closed_at = now()
		WHERE id = $1`, sessionID)
	if err != nil {
		return nil, err
	}

	return report, tx.Commit(ctx)
}

// diffFingerprints returns lessons that changed outside the session AND
// that the journal touches. An outside change on a lesson the draft never
// targets does not refuse the save (arbitrage on RG-259): the draft's
// conflicts are recomputed on the fresh snapshot anyway, so whatever the
// outside change created surfaces there — alerted, traced, never blocking
// (D-04). Only a write colliding with a write refuses.
func diffFingerprints(atOpen, now map[uuid.UUID]int, ops []Operation) []uuid.UUID {
	touched := map[uuid.UUID]bool{}
	for _, op := range ops {
		// A created lesson does not pre-exist; if it does, applyOne
		// already rejects the create. Nothing to compare.
		if op.Kind != CreateLesson {
			touched[op.LessonID] = true
		}
	}

	var changed []uuid.UUID
	for id := range touched {
		before, existed := atOpen[id]
		if !existed {
			// Targets nothing that existed at opening: either a lesson
			// created earlier in the same draft, or an unknown target
			// that applyOne rejects. Neither is an outside change.
			continue
		}
		cur, still := now[id]
		if !still || cur != before {
			changed = append(changed, id)
		}
	}
	return changed
}

func (s *Store) persistOp(ctx context.Context, tx pgx.Tx, schoolID, userID uuid.UUID, op Operation) error {
	switch op.Kind {

	case CreateLesson:
		_, err := tx.Exec(ctx, `
			INSERT INTO lesson (id, school_id, starts_at, ends_at, max_students)
			VALUES ($1, $2, $3, $4, $5)`,
			op.LessonID, schoolID, op.Slot.Start, op.Slot.End, op.MaxStudents)
		if err != nil {
			return err
		}
		for _, kid := range op.KindIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO lesson_lesson_kind (lesson_id, lesson_kind_id)
				VALUES ($1, $2)`, op.LessonID, kid); err != nil {
				return err
			}
		}
		return nil

	case MoveLesson:
		return bumping(ctx, tx, schoolID, op.LessonID, `
			UPDATE lesson SET starts_at = $3, ends_at = $4, version = version + 1
			WHERE school_id = $1 AND id = $2`,
			op.Slot.Start, op.Slot.End)

	case CancelLesson:
		return bumping(ctx, tx, schoolID, op.LessonID, `
			UPDATE lesson
			SET status = 'CANCELLED', cancel_reason = $3, cancelled_by = $4,
			    cancelled_at = now(), version = version + 1
			WHERE school_id = $1 AND id = $2`,
			op.Reason, userID)

	case AssignResource:
		if _, err := tx.Exec(ctx, `
			INSERT INTO lesson_resource (lesson_id, resource_id)
			VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			op.LessonID, *op.ResourceID); err != nil {
			return err
		}
		return bump(ctx, tx, schoolID, op.LessonID)

	case UnassignResource:
		if _, err := tx.Exec(ctx, `
			DELETE FROM lesson_resource WHERE lesson_id = $1 AND resource_id = $2`,
			op.LessonID, *op.ResourceID); err != nil {
			return err
		}
		return bump(ctx, tx, schoolID, op.LessonID)

	case PlaceStudent:
		aID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO lesson_assignment (id, lesson_id, enrollment_id, assigned_by)
			VALUES ($1, $2, $3, $4) ON CONFLICT (lesson_id, enrollment_id) DO NOTHING`,
			aID, op.LessonID, *op.EnrollmentID, userID); err != nil {
			return err
		}
		return bump(ctx, tx, schoolID, op.LessonID)

	case RemoveStudent:
		if _, err := tx.Exec(ctx, `
			DELETE FROM lesson_assignment WHERE lesson_id = $1 AND enrollment_id = $2`,
			op.LessonID, *op.EnrollmentID); err != nil {
			return err
		}
		return bump(ctx, tx, schoolID, op.LessonID)
	}
	return fmt.Errorf("unknown operation kind %q", op.Kind)
}

func bump(ctx context.Context, tx pgx.Tx, schoolID, lessonID uuid.UUID) error {
	return bumping(ctx, tx, schoolID, lessonID, `
		UPDATE lesson SET version = version + 1
		WHERE school_id = $1 AND id = $2`)
}

// bumping runs an update expected to hit exactly one lesson. Zero rows is
// a structural surprise the transaction must not paper over.
func bumping(ctx context.Context, tx pgx.Tx, schoolID, lessonID uuid.UUID, sql string, args ...any) error {
	tag, err := tx.Exec(ctx, sql, append([]any{schoolID, lessonID}, args...)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("lesson %s not found at save time", lessonID)
	}
	return nil
}

// emitEvent writes the D-05 trace: every feature emits its events from
// its first delivery, and the batch id ties them to the lot (RG-70).
func emitEvent(ctx context.Context, tx pgx.Tx, schoolID, batchID, userID uuid.UUID, op Operation) error {
	payload, err := json.Marshal(op)
	if err != nil {
		return err
	}
	evID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_event
		    (id, school_id, kind, subject_kind, subject_id, payload, batch_id, author_id)
		VALUES ($1, $2, $3, 'lesson', $4, $5, $6, $7)`,
		evID, schoolID, eventKind(op.Kind), op.LessonID, payload, batchID, userID)
	return err
}

func eventKind(k OperationKind) string {
	return "lesson." + map[OperationKind]string{
		CreateLesson:     "created",
		MoveLesson:       "moved",
		CancelLesson:     "cancelled",
		AssignResource:   "resource_assigned",
		UnassignResource: "resource_unassigned",
		PlaceStudent:     "student_placed",
		RemoveStudent:    "student_removed",
	}[k]
}
