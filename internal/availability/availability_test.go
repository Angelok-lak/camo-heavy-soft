package availability

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Test fixtures ------------------------------------------------------

var (
	vehicleCE  = uuid.New()
	vehicleCE2 = uuid.New()
	instructor = uuid.New()
	student    = uuid.New()
	lessonA    = uuid.New()
	examID     = uuid.New()
)

func at(day int, hhmm string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", "2026-10-1"+string(rune('0'+day))+" "+hhmm)
	if err != nil {
		panic(err)
	}
	return t
}

func slot(day int, from, to string) TimeSlot {
	return TimeSlot{Start: at(day, from), End: at(day, to)}
}

func baseSnapshot() *Snapshot {
	return &Snapshot{
		Period: TimeSlot{Start: at(3, "00:00"), End: at(7, "23:59")},
		Resources: map[uuid.UUID]Resource{
			vehicleCE: {
				ID: vehicleCE, Kind: Vehicle, Label: "CE-01",
				Categories: []string{"C", "CE"},
			},
			vehicleCE2: {
				ID: vehicleCE2, Kind: Vehicle, Label: "CE-02",
				Categories: []string{"CE"},
			},
			instructor: {ID: instructor, Kind: Instructor, Label: "Alhan"},
		},
		Enrollments: map[uuid.UUID]Enrollment{
			student: {
				ID: student, StudentName: "Chiffleau Aurore",
				Category: "CE", FundingOK: true,
			},
		},
		Opening: []OpeningHours{
			{Weekday: time.Tuesday, Start: "08:00", End: "19:00"},
			{Weekday: time.Wednesday, Start: "08:00", End: "19:00"},
		},
	}
}

func kindsOf(conflicts []Conflict) map[ConflictKind]int {
	out := map[ConflictKind]int{}
	for _, c := range conflicts {
		out[c.Kind]++
	}
	return out
}

// TimeSlot -----------------------------------------------------------

func TestOverlapIsPartialAware(t *testing.T) {
	cases := []struct {
		name     string
		a, b     TimeSlot
		expected bool
	}{
		{"disjoint", slot(4, "08:00", "11:00"), slot(4, "11:00", "14:00"), false},
		{"touching is not overlapping", slot(4, "08:00", "11:00"), slot(4, "11:00", "12:00"), false},
		{"ten minutes is an overlap", slot(4, "08:00", "11:00"), slot(4, "10:50", "13:00"), true},
		{"fully contained", slot(4, "08:00", "17:00"), slot(4, "10:00", "11:00"), true},
		{"identical", slot(4, "08:00", "11:00"), slot(4, "08:00", "11:00"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Overlaps(tc.b); got != tc.expected {
				t.Fatalf("Overlaps = %v, want %v", got, tc.expected)
			}
			if got := tc.b.Overlaps(tc.a); got != tc.expected {
				t.Fatalf("not symmetric")
			}
		})
	}
}

// RG-217: a conflict without a named window is not actionable.
func TestIntersectReturnsTheActualWindow(t *testing.T) {
	a := slot(4, "08:00", "11:00")
	b := slot(4, "10:15", "17:45")

	got, ok := a.Intersect(b)
	if !ok {
		t.Fatal("expected an intersection")
	}
	if !got.Start.Equal(at(4, "10:15")) || !got.End.Equal(at(4, "11:00")) {
		t.Fatalf("window = %v–%v, want 10:15–11:00", got.Start, got.End)
	}
}

// Resource conflicts -------------------------------------------------

func TestSameVehicleTwiceIsCritical(t *testing.T) {
	snap := baseSnapshot()
	snap.Lessons = []Lesson{{
		ID: lessonA, Slot: slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{vehicleCE},
	}}

	conflicts := Check(snap, Candidate{
		Slot: slot(4, "10:00", "13:00"), Resources: []uuid.UUID{vehicleCE},
	})

	if kindsOf(conflicts)[ResourceBooked] != 1 {
		t.Fatalf("expected one ResourceBooked, got %v", kindsOf(conflicts))
	}
	c := conflicts[0]
	if c.Severity != Critical {
		t.Fatalf("severity = %v, want CRITICAL", c.Severity)
	}
	if c.Overlap == nil || !c.Overlap.Start.Equal(at(4, "10:00")) {
		t.Fatal("conflict must carry its overlap window (RG-217)")
	}
	if len(c.Parties) != 2 {
		t.Fatal("conflict must name both sides (RG-217)")
	}
}

// A lesson being moved must not conflict with itself.
func TestLessonDoesNotConflictWithItself(t *testing.T) {
	snap := baseSnapshot()
	snap.Lessons = []Lesson{{
		ID: lessonA, Slot: slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{vehicleCE},
	}}

	conflicts := Check(snap, Candidate{
		LessonID: lessonA, Slot: slot(4, "09:00", "12:00"),
		Resources: []uuid.UUID{vehicleCE},
	})

	if kindsOf(conflicts)[ResourceBooked] != 0 {
		t.Fatalf("a lesson conflicted with itself: %v", conflicts)
	}
}

func TestCancelledLessonReleasesItsResources(t *testing.T) {
	snap := baseSnapshot()
	snap.Lessons = []Lesson{{
		ID: lessonA, Slot: slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{vehicleCE}, Cancelled: true,
	}}

	conflicts := Check(snap, Candidate{
		Slot: slot(4, "08:00", "11:00"), Resources: []uuid.UUID{vehicleCE},
	})

	if kindsOf(conflicts)[ResourceBooked] != 0 {
		t.Fatal("a cancelled lesson must not hold its resources (RG-150)")
	}
}

// RG-140: this is the conflict nobody understands without the travel note.
func TestExamTravelTimeCreatesTheConflict(t *testing.T) {
	snap := baseSnapshot()
	snap.Exams = []ExamSession{{
		ID: examID, Slot: slot(4, "11:00", "17:00"),
		Resources: []uuid.UUID{vehicleCE, instructor},
		PlaceLabel: "Meaux", TravelTime: 45 * time.Minute,
	}}

	// A lesson ending exactly when the exam starts: no overlap without travel.
	conflicts := Check(snap, Candidate{
		Slot:      slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{vehicleCE, instructor},
	})

	counts := kindsOf(conflicts)
	if counts[ResourceBooked] != 1 || counts[InstructorBooked] != 1 {
		t.Fatalf("expected one conflict per resource, got %v", counts)
	}

	c := conflicts[0]
	if c.Overlap == nil || !c.Overlap.Start.Equal(at(4, "10:15")) {
		t.Fatalf("overlap must start at 10:15 (travel), got %v", c.Overlap)
	}
	if c.Parties[1].Note == "" {
		t.Fatal("the exam party must carry its travel note")
	}
}

// RG-218: one cause, several conflicts, a single explanation.
func TestSharedCauseIsChainedNotRepeated(t *testing.T) {
	snap := baseSnapshot()
	snap.Exams = []ExamSession{{
		ID: examID, Slot: slot(4, "11:00", "17:00"),
		Resources: []uuid.UUID{vehicleCE, instructor},
		PlaceLabel: "Meaux", TravelTime: 45 * time.Minute,
	}}

	conflicts := Check(snap, Candidate{
		Slot:      slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{vehicleCE, instructor},
	})

	chained := 0
	for _, c := range conflicts {
		if c.SameCauseAs != nil {
			chained++
		}
	}
	if chained != 1 {
		t.Fatalf("expected exactly one chained conflict, got %d", chained)
	}
}

// Unavailability -----------------------------------------------------

// RG-65: a breakdown with no known return date blocks until further notice.
func TestUnknownReturnDateBlocksThroughThePeriod(t *testing.T) {
	snap := baseSnapshot()
	snap.Unavailable = []Unavailability{{
		ResourceID: vehicleCE,
		Slot:       TimeSlot{Start: at(3, "07:00")}, // zero End
		Reason:     "Breakdown",
	}}

	conflicts := Check(snap, Candidate{
		Slot: slot(6, "08:00", "11:00"), Resources: []uuid.UUID{vehicleCE},
	})

	if kindsOf(conflicts)[ResourceOffline] != 1 {
		t.Fatalf("expected ResourceOffline, got %v", kindsOf(conflicts))
	}
}

// Categories ---------------------------------------------------------

// RG-62 + RG-87: one aggregated conflict, never one per student.
func TestCategoryMismatchIsAggregated(t *testing.T) {
	snap := baseSnapshot()
	second := uuid.New()
	snap.Enrollments[second] = Enrollment{
		ID: second, StudentName: "Martin Julien", Category: "D", FundingOK: true,
	}
	third := uuid.New()
	snap.Enrollments[third] = Enrollment{
		ID: third, StudentName: "Rossi Anna", Category: "D", FundingOK: true,
	}

	conflicts := Check(snap, Candidate{
		Slot:        slot(4, "08:00", "11:00"),
		Resources:   []uuid.UUID{vehicleCE}, // covers C and CE, not D
		Enrollments: []uuid.UUID{student, second, third},
	})

	mismatches := 0
	var c Conflict
	for _, x := range conflicts {
		if x.Kind == CategoryMismatch {
			mismatches++
			c = x
		}
	}
	if mismatches != 1 {
		t.Fatalf("expected 1 aggregated conflict, got %d", mismatches)
	}
	if len(c.Parties) != 2 {
		t.Fatalf("expected the 2 uncovered students listed, got %d", len(c.Parties))
	}
	if c.Severity != Warning {
		t.Fatal("a category mismatch alerts, it never blocks (RG-95)")
	}
}

// Lesson kinds -------------------------------------------------------

func TestDrivingLessonWithoutVehicleWarns(t *testing.T) {
	snap := baseSnapshot()
	driving := LessonKind{ID: uuid.New(), Label: "Driving", RequiresVehicle: true}

	conflicts := Check(snap, Candidate{
		Slot:      slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{instructor},
		Kinds:     []LessonKind{driving},
	})

	if kindsOf(conflicts)[VehicleMissing] != 1 {
		t.Fatalf("expected VehicleMissing (RG-149), got %v", kindsOf(conflicts))
	}
}

// Opening hours ------------------------------------------------------

func TestOutsideOpeningHoursWarnsButNeverBlocks(t *testing.T) {
	snap := baseSnapshot()

	conflicts := Check(snap, Candidate{
		Slot: slot(4, "06:00", "07:30"), Resources: []uuid.UUID{vehicleCE},
	})

	found := false
	for _, c := range conflicts {
		if c.Kind == OutsideHours {
			found = true
			if c.Severity == Critical {
				t.Fatal("outside hours must not be critical (D-04)")
			}
		}
	}
	if !found {
		t.Fatal("expected an OutsideHours conflict")
	}
}

// Find ---------------------------------------------------------------

// RG-95: a warning-level conflict leaves the resource suggested.
func TestFindKeepsResourcesWithNonCriticalConflictsOnly(t *testing.T) {
	snap := baseSnapshot()
	snap.Lessons = []Lesson{{
		ID: lessonA, Slot: slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{vehicleCE},
	}}

	free := Find(snap, Query{Slot: slot(4, "09:00", "12:00"), Kind: Vehicle})

	if len(free) != 1 || free[0].ID != vehicleCE2 {
		t.Fatalf("expected only CE-02 free, got %v", free)
	}
}

func TestFindFiltersOnCategory(t *testing.T) {
	snap := baseSnapshot()

	free := Find(snap, Query{
		Slot: slot(4, "08:00", "11:00"), Kind: Vehicle, RequiredCategory: "C",
	})

	if len(free) != 1 || free[0].ID != vehicleCE {
		t.Fatalf("only CE-01 covers category C, got %v", free)
	}
}

func TestFindIgnoresArchivedResources(t *testing.T) {
	snap := baseSnapshot()
	r := snap.Resources[vehicleCE2]
	r.Archived = true
	snap.Resources[vehicleCE2] = r

	free := Find(snap, Query{Slot: slot(4, "08:00", "11:00"), Kind: Vehicle})

	for _, f := range free {
		if f.ID == vehicleCE2 {
			t.Fatal("an archived resource must never be suggested (RG-73)")
		}
	}
}

// Nothing ever blocks ------------------------------------------------

// D-04 has no exception: Check reports, it never refuses.
func TestCheckNeverRefuses(t *testing.T) {
	snap := baseSnapshot()
	snap.Lessons = []Lesson{{
		ID: lessonA, Slot: slot(4, "08:00", "11:00"),
		Resources: []uuid.UUID{vehicleCE, instructor},
		Enrollments: []uuid.UUID{student},
	}}
	snap.Unavailable = []Unavailability{{
		ResourceID: vehicleCE, Slot: slot(4, "00:00", "23:00"), Reason: "Breakdown",
	}}

	conflicts := Check(snap, Candidate{
		Slot:        slot(4, "08:00", "11:00"),
		Resources:   []uuid.UUID{vehicleCE, instructor},
		Enrollments: []uuid.UUID{student},
	})

	if len(conflicts) == 0 {
		t.Fatal("expected several conflicts")
	}
	// Sorted by severity: critical first (planner reads the worst first).
	if conflicts[0].Severity != Critical {
		t.Fatalf("first conflict = %v, want CRITICAL", conflicts[0].Severity)
	}
}
