// Package events writes the business trace (C-10) every feature emits
// from its first delivery (D-05). Dashboard counters will be computed
// from this table — no feature plans its own indicator storage.
package events

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is satisfied by *pgxpool.Pool and pgx.Tx alike, so an event can
// join the transaction of the write it traces.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Emit records one event. Payload marshals to jsonb; nil is an empty
// object. Failing to trace fails the write: an untraced mutation is worse
// than a retried one.
func Emit(ctx context.Context, db Execer, schoolID uuid.UUID, kind, subjectKind string, subjectID uuid.UUID, payload any, authorID uuid.UUID) error {
	raw := []byte(`{}`)
	if payload != nil {
		var err error
		if raw, err = json.Marshal(payload); err != nil {
			return err
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO domain_event (id, school_id, kind, subject_kind, subject_id, payload, author_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, schoolID, kind, subjectKind, subjectID, raw, authorID)
	return err
}
