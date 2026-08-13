package planning

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/angelok-lak/camo-heavy-soft/internal/availability"
)

// Fixtures -----------------------------------------------------------

var (
	vehicleCE  = uuid.New()
	instructor = uuid.New()
	studentA   = uuid.New()
	lessonSaved = uuid.New()
	drivingKind = uuid.New()
)

func at(hhmm string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", "2026-10-14 "+hhmm)
	if err != nil {
		panic(err)
	}
	return t
}

func slot(from, to string) availability.TimeSlot {
	return availability.TimeSlot{Start: at(from), End: at(to)}
}

func baseSnapshot() *availability.Snapshot {
	return &availability.Snapshot{
		Period: availability.TimeSlot{
			Start: at("00:00"), End: at("23:59"),
		},
		Resources: map[uuid.UUID]availability.Resource{
			vehicleCE:  {ID: vehicleCE, Kind: availability.Vehicle, Label: "CE-01", Categories: []string{"CE"}},
			instructor: {ID: instructor, Kind: availability.Instructor, Label: "Alhan"},
		},
		Enrollments: map[uuid.UUID]availability.Enrollment{
			studentA: {ID: studentA, StudentName: "Chiffleau Aurore", Category: "CE", FundingOK: true},
		},
		LessonKinds: map[uuid.UUID]availability.LessonKind{
			drivingKind: {ID: drivingKind, Label: "Driving", RequiresVehicle: true},
		},
		Lessons: []availability.Lesson{{
			ID:        lessonSaved,
			Slot:      slot("08:00", "11:00"),
			Resources: []uuid.UUID{vehicleCE, instructor},
		}},
	}
}

func op(kind OperationKind, lessonID uuid.UUID) Operation {
	return Operation{ID: uuid.New(), Kind: kind, LessonID: lessonID}
}

// C-09: the journal overlays, the input never changes ----------------

func TestApplyNeverMutatesTheSavedState(t *testing.T) {
	snap := baseSnapshot()
	newLesson := uuid.New()

	create := op(CreateLesson, newLesson)
	s := slot("14:00", "17:00")
	create.Slot = &s

	move := op(MoveLesson, lessonSaved)
	m := slot("09:00", "12:00")
	move.Slot = &m

	assign := op(AssignResource, newLesson)
	assign.ResourceID = &vehicleCE

	state := Apply(snap, []Operation{create, move, assign})

	// The resulting state sees everything…
	if len(state.Result.Lessons) != 2 {
		t.Fatalf("result should hold 2 lessons, got %d", len(state.Result.Lessons))
	}
	// …the saved state saw nothing (C-09: no other query knows the draft).
	if len(snap.Lessons) != 1 {
		t.Fatal("the draft leaked into the saved snapshot")
	}
	if !snap.Lessons[0].Slot.Start.Equal(at("08:00")) {
		t.Fatal("moving a lesson in the draft moved it in the saved state")
	}
	if len(snap.Lessons[0].Resources) != 2 {
		t.Fatal("assigning in the draft touched the saved lesson")
	}
}

// RG-146: conflicts are computed on the RESULTING state --------------

func TestConflictsSeeTheResultingStateNotTheSavedOne(t *testing.T) {
	snap := baseSnapshot()
	newLesson := uuid.New()

	// Create a lesson over the saved one, sharing its vehicle: conflict.
	create := op(CreateLesson, newLesson)
	s := slot("10:00", "13:00")
	create.Slot = &s
	assign := op(AssignResource, newLesson)
	assign.ResourceID = &vehicleCE

	state := Apply(snap, []Operation{create, assign})

	found := false
	for _, c := range state.Conflicts[newLesson] {
		if c.Kind == availability.ResourceBooked {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ResourceBooked against the saved lesson, got %v", state.Conflicts[newLesson])
	}
}

func TestMovingAwayResolvesTheConflictInTheDraft(t *testing.T) {
	snap := baseSnapshot()
	newLesson := uuid.New()

	create := op(CreateLesson, newLesson)
	s := slot("10:00", "13:00")
	create.Slot = &s
	assign := op(AssignResource, newLesson)
	assign.ResourceID = &vehicleCE

	// Then the editor moves the SAVED lesson out of the way. On the
	// resulting state there is no overlap left.
	move := op(MoveLesson, lessonSaved)
	m := slot("14:00", "17:00")
	move.Slot = &m

	state := Apply(snap, []Operation{create, assign, move})

	for id, conflicts := range state.Conflicts {
		for _, c := range conflicts {
			if c.Kind == availability.ResourceBooked {
				t.Fatalf("lesson %s still conflicts after the move: %v", id, c)
			}
		}
	}
}

func TestTwoDraftLessonsConflictWithEachOther(t *testing.T) {
	snap := baseSnapshot()
	a, b := uuid.New(), uuid.New()

	ops := []Operation{}
	for _, id := range []uuid.UUID{a, b} {
		create := op(CreateLesson, id)
		s := slot("14:00", "16:00")
		create.Slot = &s
		assign := op(AssignResource, id)
		assign.ResourceID = &instructor
		ops = append(ops, create, assign)
	}

	state := Apply(snap, ops)

	if kinds(state.Conflicts[a])[availability.InstructorBooked] != 1 ||
		kinds(state.Conflicts[b])[availability.InstructorBooked] != 1 {
		t.Fatal("two lessons born in the same draft must see each other (RG-146)")
	}
}

// RG-150: cancelling needs a reason, and releases resources ----------

func TestCancelReleasesResourcesForTheRestOfTheDraft(t *testing.T) {
	snap := baseSnapshot()
	newLesson := uuid.New()

	cancel := op(CancelLesson, lessonSaved)
	cancel.Reason = "Vehicle recalled"

	create := op(CreateLesson, newLesson)
	s := slot("08:00", "11:00") // exactly over the cancelled one
	create.Slot = &s
	assign := op(AssignResource, newLesson)
	assign.ResourceID = &vehicleCE

	state := Apply(snap, []Operation{cancel, create, assign})

	if kinds(state.Conflicts[newLesson])[availability.ResourceBooked] != 0 {
		t.Fatal("a lesson cancelled in the draft must not hold its resources")
	}
}

func TestCancelWithoutReasonIsRejected(t *testing.T) {
	snap := baseSnapshot()
	cancel := op(CancelLesson, lessonSaved)

	state := Apply(snap, []Operation{cancel})

	if len(state.Rejected) != 1 {
		t.Fatalf("expected 1 rejection, got %v", state.Rejected)
	}
	if len(state.Pending) != 0 {
		t.Fatal("a rejected operation must not be pending")
	}
}

// Structural impossibilities are rejections, not conflicts (D-04) ----

func TestUnknownLessonIsARejectionNotAConflict(t *testing.T) {
	snap := baseSnapshot()
	move := op(MoveLesson, uuid.New())
	m := slot("09:00", "10:00")
	move.Slot = &m

	state := Apply(snap, []Operation{move})

	if len(state.Rejected) != 1 {
		t.Fatalf("expected a rejection, got %v", state.Rejected)
	}
	if len(state.Conflicts) != 0 {
		t.Fatal("a structural impossibility is not a conflict")
	}
}

func TestRejectedOperationDoesNotStopTheRest(t *testing.T) {
	snap := baseSnapshot()

	bad := op(MoveLesson, uuid.New())
	b := slot("09:00", "10:00")
	bad.Slot = &b

	good := op(MoveLesson, lessonSaved)
	g := slot("15:00", "18:00")
	good.Slot = &g

	state := Apply(snap, []Operation{bad, good})

	if len(state.Pending) != 1 || len(state.Rejected) != 1 {
		t.Fatalf("pending=%d rejected=%d, want 1 and 1", len(state.Pending), len(state.Rejected))
	}
	l, _ := findLesson(state.Result, lessonSaved)
	if !l.Slot.Start.Equal(at("15:00")) {
		t.Fatal("the valid operation after a rejected one must still apply")
	}
}

// Idempotent wiring --------------------------------------------------

func TestAssignTwiceHoldsOnce(t *testing.T) {
	snap := baseSnapshot()
	a1 := op(AssignResource, lessonSaved)
	a1.ResourceID = &vehicleCE
	a2 := op(AssignResource, lessonSaved)
	a2.ResourceID = &vehicleCE

	state := Apply(snap, []Operation{a1, a2})

	l, _ := findLesson(state.Result, lessonSaved)
	if n := count(l.Resources, vehicleCE); n != 1 {
		t.Fatalf("vehicle held %d times, want 1", n)
	}
}

func TestPlaceThenRemoveLeavesNoTrace(t *testing.T) {
	snap := baseSnapshot()
	place := op(PlaceStudent, lessonSaved)
	place.EnrollmentID = &studentA
	rem := op(RemoveStudent, lessonSaved)
	rem.EnrollmentID = &studentA

	state := Apply(snap, []Operation{place, rem})

	l, _ := findLesson(state.Result, lessonSaved)
	if count(l.Enrollments, studentA) != 0 {
		t.Fatal("removed student still placed")
	}
}

// Lesson kinds resolve from the live catalogue (C-07) ----------------

func TestCreateResolvesKindsFromTheCatalogue(t *testing.T) {
	snap := baseSnapshot()
	newLesson := uuid.New()

	create := op(CreateLesson, newLesson)
	s := slot("14:00", "16:00")
	create.Slot = &s
	create.KindIDs = []uuid.UUID{drivingKind}

	state := Apply(snap, []Operation{create})

	// Driving kind without a vehicle: RG-149 must fire on the draft lesson.
	if kinds(state.Conflicts[newLesson])[availability.VehicleMissing] != 1 {
		t.Fatalf("expected VehicleMissing, got %v", state.Conflicts[newLesson])
	}
}

// The journal survives serialisation (C-14) --------------------------

func TestOperationsRoundTripThroughJSON(t *testing.T) {
	newLesson := uuid.New()
	create := op(CreateLesson, newLesson)
	s := slot("14:00", "16:00")
	create.Slot = &s
	create.KindIDs = []uuid.UUID{drivingKind}
	assign := op(AssignResource, newLesson)
	assign.ResourceID = &vehicleCE

	raw, err := json.Marshal([]Operation{create, assign})
	if err != nil {
		t.Fatal(err)
	}
	var back []Operation
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}

	snap := baseSnapshot()
	state := Apply(snap, back)
	if len(state.Pending) != 2 || len(state.Rejected) != 0 {
		t.Fatalf("replayed journal diverged: pending=%d rejected=%v", len(state.Pending), state.Rejected)
	}
}

// Helpers ------------------------------------------------------------

func kinds(conflicts []availability.Conflict) map[availability.ConflictKind]int {
	out := map[availability.ConflictKind]int{}
	for _, c := range conflicts {
		out[c.Kind]++
	}
	return out
}

func count(list []uuid.UUID, id uuid.UUID) int {
	n := 0
	for _, v := range list {
		if v == id {
			n++
		}
	}
	return n
}
