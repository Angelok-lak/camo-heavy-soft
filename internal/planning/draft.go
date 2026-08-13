// Package planning implements F-09: the planner, its edit session and its
// draft. Terms follow GLOSSARY.md.
//
// The draft is a JOURNAL OF INTENTIONS, not a copy of the planner (C-09).
// The state shown to the editor is the saved state plus the pending
// operations replayed in memory. Conflicts are computed on that resulting
// state (RG-146). Saving replays the operations in one transaction;
// discarding deletes the session — nothing ever touched the planner.
//
// This file is pure: no database, no clock, no I/O. The store loads a
// Snapshot, Apply overlays the operations, availability.Check does the rest.
package planning

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/angelok-lak/camo-heavy-soft/internal/availability"
)

// ---------------------------------------------------------------------
// Operations
// ---------------------------------------------------------------------

type OperationKind string

// Slice 1 keeps: create, move, cancel, plus resource and student wiring.
// Mass replacement and impact handling come with the full F-09.
const (
	CreateLesson     OperationKind = "CREATE_LESSON"
	MoveLesson       OperationKind = "MOVE_LESSON"
	CancelLesson     OperationKind = "CANCEL_LESSON"
	AssignResource   OperationKind = "ASSIGN_RESOURCE"
	UnassignResource OperationKind = "UNASSIGN_RESOURCE"
	PlaceStudent     OperationKind = "PLACE_STUDENT"
	RemoveStudent    OperationKind = "REMOVE_STUDENT"
)

// Operation is one pending intention. It is serialised as-is into
// edit_session.operations (jsonb, C-14): the draft survives a disconnect.
//
// LessonID is always set. For CREATE_LESSON the client generates it
// (UUID v7, C-02), which lets later operations of the same draft target a
// lesson that does not exist in storage yet — no temporary-ID mapping.
type Operation struct {
	ID       uuid.UUID     `json:"id"`
	Kind     OperationKind `json:"kind"`
	LessonID uuid.UUID     `json:"lesson_id"`

	// CREATE_LESSON and MOVE_LESSON
	Slot *availability.TimeSlot `json:"slot,omitempty"`

	// CREATE_LESSON only
	KindIDs     []uuid.UUID `json:"kind_ids,omitempty"`
	MaxStudents *int16      `json:"max_students,omitempty"`

	// ASSIGN_RESOURCE / UNASSIGN_RESOURCE
	ResourceID *uuid.UUID `json:"resource_id,omitempty"`

	// PLACE_STUDENT / REMOVE_STUDENT
	EnrollmentID *uuid.UUID `json:"enrollment_id,omitempty"`

	// CANCEL_LESSON: the reason is required by RG-150
	Reason string `json:"reason,omitempty"`
}

// ---------------------------------------------------------------------
// Applying the journal
// ---------------------------------------------------------------------

// Rejection is a STRUCTURAL impossibility: the operation targets a lesson
// that does not exist, or is malformed. It is one of the three refusal
// natures the API conventions allow. Business rules never end up here —
// they produce conflicts (D-04).
type Rejection struct {
	OperationID uuid.UUID `json:"operation_id"`
	Reason      string    `json:"reason"`
}

// DraftState is the result of overlaying a journal onto a snapshot.
// It is what "push the draft" returns: the resulting state, every conflict
// recomputed on it (RG-146), each explained (RG-217), and the list of
// pending modifications (RG-220).
type DraftState struct {
	// Result is the saved state plus the applied operations. No other
	// query in the system ever sees it (C-09).
	Result *availability.Snapshot
	// Pending are the operations that applied, in order.
	Pending []Operation
	// Rejected are the structurally impossible ones, skipped.
	Rejected []Rejection
	// Conflicts of every lesson the draft touches, computed against the
	// resulting state. Keyed by lesson ID.
	Conflicts map[uuid.UUID][]availability.Conflict
}

// Apply overlays the journal onto the snapshot and recomputes conflicts.
// The input snapshot is never mutated: the editor's view and the system's
// view must not share state.
func Apply(snap *availability.Snapshot, ops []Operation) DraftState {
	result := shallowCopy(snap)
	touched := map[uuid.UUID]bool{}

	state := DraftState{Result: result, Conflicts: map[uuid.UUID][]availability.Conflict{}}

	for _, op := range ops {
		if reason := applyOne(result, op); reason != "" {
			state.Rejected = append(state.Rejected, Rejection{OperationID: op.ID, Reason: reason})
			continue
		}
		state.Pending = append(state.Pending, op)
		touched[op.LessonID] = true
	}

	// RG-146: conflicts are evaluated on the state RESULTING from the
	// draft, not on the saved one. Check excludes the lesson itself, so a
	// moved lesson does not collide with its own former slot.
	for id := range touched {
		l, ok := findLesson(result, id)
		if !ok || l.Cancelled {
			continue
		}
		state.Conflicts[id] = availability.Check(result, availability.Candidate{
			LessonID:    l.ID,
			Slot:        l.Slot,
			Resources:   l.Resources,
			Enrollments: l.Enrollments,
			Kinds:       l.Kinds,
		})
	}
	return state
}

// applyOne mutates result in place. An empty return means applied; any
// other return is the structural reason for rejection.
func applyOne(result *availability.Snapshot, op Operation) string {
	switch op.Kind {

	case CreateLesson:
		if op.Slot == nil {
			return "a lesson needs a time slot"
		}
		if _, exists := findLesson(result, op.LessonID); exists {
			return fmt.Sprintf("lesson %s already exists", op.LessonID)
		}
		var kinds []availability.LessonKind
		for _, kid := range op.KindIDs {
			k, known := result.LessonKinds[kid]
			if !known {
				return fmt.Sprintf("unknown lesson kind %s", kid)
			}
			kinds = append(kinds, k)
		}
		result.Lessons = append(result.Lessons, availability.Lesson{
			ID:          op.LessonID,
			Slot:        *op.Slot,
			MaxStudents: op.MaxStudents,
			Kinds:       kinds,
		})
		return ""

	case MoveLesson:
		if op.Slot == nil {
			return "a move needs a target slot"
		}
		return withLesson(result, op.LessonID, func(l *availability.Lesson) string {
			l.Slot = *op.Slot
			return ""
		})

	case CancelLesson:
		if op.Reason == "" {
			return "cancelling requires a reason (RG-150)"
		}
		return withLesson(result, op.LessonID, func(l *availability.Lesson) string {
			l.Cancelled = true
			return ""
		})

	case AssignResource:
		if op.ResourceID == nil {
			return "no resource given"
		}
		if _, known := result.Resources[*op.ResourceID]; !known {
			return fmt.Sprintf("unknown resource %s", *op.ResourceID)
		}
		return withLesson(result, op.LessonID, func(l *availability.Lesson) string {
			l.Resources = appendOnce(l.Resources, *op.ResourceID)
			return ""
		})

	case UnassignResource:
		if op.ResourceID == nil {
			return "no resource given"
		}
		return withLesson(result, op.LessonID, func(l *availability.Lesson) string {
			l.Resources = remove(l.Resources, *op.ResourceID)
			return ""
		})

	case PlaceStudent:
		if op.EnrollmentID == nil {
			return "no enrollment given"
		}
		if _, known := result.Enrollments[*op.EnrollmentID]; !known {
			return fmt.Sprintf("unknown enrollment %s", *op.EnrollmentID)
		}
		return withLesson(result, op.LessonID, func(l *availability.Lesson) string {
			l.Enrollments = appendOnce(l.Enrollments, *op.EnrollmentID)
			return ""
		})

	case RemoveStudent:
		if op.EnrollmentID == nil {
			return "no enrollment given"
		}
		return withLesson(result, op.LessonID, func(l *availability.Lesson) string {
			l.Enrollments = remove(l.Enrollments, *op.EnrollmentID)
			return ""
		})
	}
	return fmt.Sprintf("unknown operation kind %q", op.Kind)
}

// ---------------------------------------------------------------------
// Copy discipline
// ---------------------------------------------------------------------

// shallowCopy clones what the draft can touch — the lesson list and each
// lesson's slices — and shares the rest. Resources, enrollments and the
// reference catalogue are read-only during an edit session.
func shallowCopy(snap *availability.Snapshot) *availability.Snapshot {
	out := *snap
	out.Lessons = make([]availability.Lesson, len(snap.Lessons))
	for i, l := range snap.Lessons {
		l.Resources = append([]uuid.UUID(nil), l.Resources...)
		l.Enrollments = append([]uuid.UUID(nil), l.Enrollments...)
		l.Kinds = append([]availability.LessonKind(nil), l.Kinds...)
		out.Lessons[i] = l
	}
	return &out
}

func findLesson(snap *availability.Snapshot, id uuid.UUID) (availability.Lesson, bool) {
	for _, l := range snap.Lessons {
		if l.ID == id {
			return l, true
		}
	}
	return availability.Lesson{}, false
}

func withLesson(snap *availability.Snapshot, id uuid.UUID, mutate func(*availability.Lesson) string) string {
	for i := range snap.Lessons {
		if snap.Lessons[i].ID == id {
			return mutate(&snap.Lessons[i])
		}
	}
	return fmt.Sprintf("unknown lesson %s", id)
}

func appendOnce(list []uuid.UUID, id uuid.UUID) []uuid.UUID {
	for _, v := range list {
		if v == id {
			return list
		}
	}
	return append(list, id)
}

func remove(list []uuid.UUID, id uuid.UUID) []uuid.UUID {
	out := list[:0]
	for _, v := range list {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
