// Package availability implements the single resource-availability component
// (RG-78, D-03). Terms follow GLOSSARY.md.
//
// It answers both directions with one mechanic:
//   - Check  : "is this slot valid?"        → F-09, F-06
//   - Find   : "which resources are free?"  → F-10, F-09
//
// Design note: this package never touches the database. RG-146 requires
// conflicts to be evaluated against the state resulting from a draft, which
// exists nowhere in storage. The package therefore receives a Snapshot that
// has already been loaded and, when editing, overlaid with pending
// operations. Detection is a set of pure functions, testable without a DB.
//
// It never refuses anything. It returns conflicts (D-04).
package availability

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------
// Time slots
// ---------------------------------------------------------------------

// TimeSlot is a half-open interval [Start, End).
type TimeSlot struct {
	Start time.Time
	End   time.Time
}

// Overlaps reports whether two slots intersect, even partially. Partial
// overlap counts: two lessons sharing ten minutes still compete for the
// same resource (F-09, edge case 9).
func (s TimeSlot) Overlaps(other TimeSlot) bool {
	return s.Start.Before(other.End) && other.Start.Before(s.End)
}

// Intersect returns the overlapping window. RG-217 requires naming it in
// every conflict message: without a window, a conflict cannot be acted on.
func (s TimeSlot) Intersect(other TimeSlot) (TimeSlot, bool) {
	if !s.Overlaps(other) {
		return TimeSlot{}, false
	}
	start, end := s.Start, s.End
	if other.Start.After(start) {
		start = other.Start
	}
	if other.End.Before(end) {
		end = other.End
	}
	return TimeSlot{Start: start, End: end}, true
}

// Expand widens a slot on both sides. Used for exam travel time (RG-140):
// a session 45 minutes away ties up its resources well beyond its stated
// hours, which is the single most misunderstood conflict cause.
func (s TimeSlot) Expand(d time.Duration) TimeSlot {
	return TimeSlot{Start: s.Start.Add(-d), End: s.End.Add(d)}
}

// ---------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------

type ResourceKind string

const (
	Vehicle     ResourceKind = "VEHICLE"
	Room        ResourceKind = "ROOM"
	TrainingPad ResourceKind = "TRAINING_PAD"
	Instructor  ResourceKind = "INSTRUCTOR"
)

type Resource struct {
	ID         uuid.UUID
	Kind       ResourceKind
	Label      string
	Archived   bool
	Categories []string // vehicles only (RG-74)
}

type LessonKind struct {
	ID             uuid.UUID
	Label          string
	RequiresVehicle bool // RG-149
}

type Lesson struct {
	ID          uuid.UUID
	Slot        TimeSlot
	Resources   []uuid.UUID
	Enrollments []uuid.UUID
	Cancelled   bool
	MaxStudents *int16
	Kinds       []LessonKind
}

// Unavailability with a zero End means "return date unknown" (RG-65).
type Unavailability struct {
	ResourceID uuid.UUID
	Slot       TimeSlot
	Reason     string
}

// ExamSession ties up its resources for its duration plus round-trip travel
// (RG-140). It is not editable from the planner (RG-152).
type ExamSession struct {
	ID          uuid.UUID
	Slot        TimeSlot
	Resources   []uuid.UUID
	PlaceLabel  string
	TravelTime  time.Duration
}

type Enrollment struct {
	ID          uuid.UUID
	StudentName string
	Category    string
	Closed      bool
	FundingOK   bool // inert until F-05 ships; always true in slice 1
}

type OpeningHours struct {
	Weekday time.Weekday
	Start   string // "08:00"
	End     string // "19:00"
}

// Snapshot holds everything needed to detect conflicts over a period.
// Loaded once, then overlaid with pending draft operations when editing.
type Snapshot struct {
	Period      TimeSlot
	Resources   map[uuid.UUID]Resource
	Enrollments map[uuid.UUID]Enrollment
	// LessonKinds is the active catalogue. Draft operations reference kinds
	// by ID (C-07: read live, never copied) and resolve them here.
	LessonKinds map[uuid.UUID]LessonKind
	Lessons     []Lesson
	Unavailable []Unavailability
	Exams       []ExamSession
	Opening     []OpeningHours
	Closures    []TimeSlot
}

// ---------------------------------------------------------------------
// Conflicts
// ---------------------------------------------------------------------

type ConflictKind string

const (
	StudentBooked      ConflictKind = "STUDENT_BOOKED"
	InstructorBooked   ConflictKind = "INSTRUCTOR_BOOKED"
	ResourceBooked     ConflictKind = "RESOURCE_BOOKED"
	ResourceOffline    ConflictKind = "RESOURCE_OFFLINE"
	OutsideHours       ConflictKind = "OUTSIDE_HOURS"
	CategoryMismatch   ConflictKind = "CATEGORY_MISMATCH"
	EnrollmentClosed   ConflictKind = "ENROLLMENT_CLOSED"
	FundingNotApproved ConflictKind = "FUNDING_NOT_APPROVED"
	VehicleMissing     ConflictKind = "VEHICLE_MISSING"
)

type Severity string

const (
	Critical Severity = "CRITICAL"
	Warning  Severity = "WARNING"
	Info     Severity = "INFO"
)

// PartyRef describes one side of a conflict. RG-217: a conflict is a
// RELATION, and showing only one side makes it unusable.
type PartyRef struct {
	Kind  string // "lesson", "exam_session", "unavailability", "enrollment"
	ID    uuid.UUID
	Label string
	Slot  TimeSlot
	Note  string // "tied up from 10:15 for travel"
}

// Conflict is always explained, never an error code (C-26).
type Conflict struct {
	Kind       ConflictKind
	Severity   Severity
	SubjectID  uuid.UUID // the resource or enrollment at stake
	Subject    string
	Overlap    *TimeSlot // nil when the notion does not apply
	Parties    []PartyRef
	// SameCauseAs points at the conflict carrying the explanation, so the
	// second one need not repeat it (RG-218).
	SameCauseAs *int
}

// ---------------------------------------------------------------------
// Direction 1 — "is this slot valid?"
// ---------------------------------------------------------------------

// Candidate is the lesson being created or moved.
type Candidate struct {
	LessonID    uuid.UUID // zero when creating
	Slot        TimeSlot
	Resources   []uuid.UUID
	Enrollments []uuid.UUID
	Kinds       []LessonKind
}

// Check returns every conflict for a candidate. It never refuses: the
// planner keeps control and overrides knowingly (D-04).
//
// Conflicts come back sorted by severity, with shared causes chained
// together (RG-218).
func Check(snap *Snapshot, cand Candidate) []Conflict {
	var out []Conflict

	out = append(out, resourceConflicts(snap, cand)...)
	out = append(out, studentConflicts(snap, cand)...)
	out = append(out, offlineConflicts(snap, cand)...)
	out = append(out, openingConflicts(snap, cand)...)
	out = append(out, categoryConflicts(snap, cand)...)
	out = append(out, vehicleKindConflicts(snap, cand)...)

	chainSharedCauses(out)
	sortBySeverity(out)
	return out
}

// resourceConflicts finds a resource already committed elsewhere, whether
// to another lesson or to an exam session — travel included (RG-140).
func resourceConflicts(snap *Snapshot, cand Candidate) []Conflict {
	var out []Conflict

	for _, resID := range cand.Resources {
		res, known := snap.Resources[resID]
		if !known {
			continue
		}

		for _, l := range snap.Lessons {
			if l.Cancelled || l.ID == cand.LessonID || !hasID(l.Resources, resID) {
				continue
			}
			overlap, ok := cand.Slot.Intersect(l.Slot)
			if !ok {
				continue
			}
			out = append(out, Conflict{
				Kind:      bookedKindFor(res.Kind),
				Severity:  Critical,
				SubjectID: resID,
				Subject:   res.Label,
				Overlap:   &overlap,
				Parties: []PartyRef{
					{Kind: "lesson", ID: cand.LessonID, Slot: cand.Slot},
					{Kind: "lesson", ID: l.ID, Slot: l.Slot},
				},
			})
		}

		for _, ex := range snap.Exams {
			if !hasID(ex.Resources, resID) {
				continue
			}
			// Actual tie-up, round-trip travel included.
			tiedUp := ex.Slot.Expand(ex.TravelTime)
			overlap, ok := cand.Slot.Intersect(tiedUp)
			if !ok {
				continue
			}
			out = append(out, Conflict{
				Kind:      bookedKindFor(res.Kind),
				Severity:  Critical,
				SubjectID: resID,
				Subject:   res.Label,
				Overlap:   &overlap,
				Parties: []PartyRef{
					{Kind: "lesson", ID: cand.LessonID, Slot: cand.Slot},
					{
						Kind:  "exam_session",
						ID:    ex.ID,
						Label: ex.PlaceLabel,
						Slot:  ex.Slot,
						Note:  travelNote(ex),
					},
				},
			})
		}
	}
	return out
}

// studentConflicts: a student cannot be in two places at once (RG-138).
func studentConflicts(snap *Snapshot, cand Candidate) []Conflict {
	var out []Conflict

	for _, enrID := range cand.Enrollments {
		enr, known := snap.Enrollments[enrID]
		if !known {
			continue
		}

		if enr.Closed {
			out = append(out, Conflict{
				Kind: EnrollmentClosed, Severity: Warning,
				SubjectID: enrID, Subject: enr.StudentName,
			})
		}
		if !enr.FundingOK {
			out = append(out, Conflict{
				Kind: FundingNotApproved, Severity: Critical,
				SubjectID: enrID, Subject: enr.StudentName,
			})
		}

		for _, l := range snap.Lessons {
			if l.Cancelled || l.ID == cand.LessonID || !hasID(l.Enrollments, enrID) {
				continue
			}
			overlap, ok := cand.Slot.Intersect(l.Slot)
			if !ok {
				continue
			}
			out = append(out, Conflict{
				Kind: StudentBooked, Severity: Critical,
				SubjectID: enrID, Subject: enr.StudentName,
				Overlap: &overlap,
				Parties: []PartyRef{
					{Kind: "lesson", ID: cand.LessonID, Slot: cand.Slot},
					{Kind: "lesson", ID: l.ID, Slot: l.Slot},
				},
			})
		}
	}
	return out
}

// offlineConflicts: the lesson KEEPS its assignment and shows the reason;
// it is neither emptied nor cancelled (RG-80). That is what makes
// restoration possible on return to service (RG-66).
func offlineConflicts(snap *Snapshot, cand Candidate) []Conflict {
	var out []Conflict

	for _, resID := range cand.Resources {
		res, known := snap.Resources[resID]
		if !known {
			continue
		}
		for _, u := range snap.Unavailable {
			if u.ResourceID != resID {
				continue
			}
			slot := u.Slot
			if slot.End.IsZero() {
				// Unknown return: unavailable until further notice.
				slot.End = snap.Period.End
			}
			overlap, ok := cand.Slot.Intersect(slot)
			if !ok {
				continue
			}
			out = append(out, Conflict{
				Kind: ResourceOffline, Severity: Critical,
				SubjectID: resID, Subject: res.Label,
				Overlap: &overlap,
				Parties: []PartyRef{
					{Kind: "lesson", ID: cand.LessonID, Slot: cand.Slot},
					{Kind: "unavailability", Label: u.Reason, Slot: slot},
				},
			})
		}
	}
	return out
}

// openingConflicts bounds on school hours. Individual instructor hours
// produce NO conflict (RG-139): they are informational, by decision.
func openingConflicts(snap *Snapshot, cand Candidate) []Conflict {
	var out []Conflict

	for _, c := range snap.Closures {
		if overlap, ok := cand.Slot.Intersect(c); ok {
			out = append(out, Conflict{
				Kind: OutsideHours, Severity: Warning,
				Subject: "Centre fermé", Overlap: &overlap,
			})
		}
	}
	if !withinOpening(snap.Opening, cand.Slot) {
		out = append(out, Conflict{
			Kind: OutsideHours, Severity: Warning,
			Subject: "Hors horaires d'ouverture",
		})
	}
	return out
}

// categoryConflicts: vehicle/category mismatch produces ONE aggregated
// conflict listing every student concerned, never N separate ones
// (RG-62, RG-87). It never removes anyone from the pool (RG-95).
func categoryConflicts(snap *Snapshot, cand Candidate) []Conflict {
	var vehicles []Resource
	for _, rID := range cand.Resources {
		if r, ok := snap.Resources[rID]; ok && r.Kind == Vehicle {
			vehicles = append(vehicles, r)
		}
	}
	if len(vehicles) == 0 {
		return nil
	}

	var parties []PartyRef
	for _, enrID := range cand.Enrollments {
		enr, ok := snap.Enrollments[enrID]
		if !ok || enr.Category == "" {
			continue
		}
		covered := false
		for _, v := range vehicles {
			if hasText(v.Categories, enr.Category) {
				covered = true
				break
			}
		}
		if !covered {
			parties = append(parties, PartyRef{
				Kind: "enrollment", ID: enrID, Label: enr.StudentName,
			})
		}
	}
	if len(parties) == 0 {
		return nil
	}
	return []Conflict{{
		Kind: CategoryMismatch, Severity: Warning,
		Subject: "Catégorie non couverte par les véhicules assignés",
		Parties: parties,
	}}
}

// vehicleKindConflicts: a lesson kind flagged "requires a vehicle" warns
// when none is assigned (RG-149). Kinds stay informational otherwise (RG-134).
func vehicleKindConflicts(snap *Snapshot, cand Candidate) []Conflict {
	needed := false
	for _, k := range cand.Kinds {
		if k.RequiresVehicle {
			needed = true
			break
		}
	}
	if !needed {
		return nil
	}
	for _, rID := range cand.Resources {
		if r, ok := snap.Resources[rID]; ok && r.Kind == Vehicle {
			return nil
		}
	}
	return []Conflict{{
		Kind: VehicleMissing, Severity: Warning,
		Subject: "Séance de conduite sans véhicule assigné",
	}}
}

// ---------------------------------------------------------------------
// Direction 2 — "which resources are free?"
// ---------------------------------------------------------------------

type Query struct {
	Slot            TimeSlot
	Kind            ResourceKind
	RequiredCategory string // optional
	IgnoreLessonID  uuid.UUID
}

// Find returns resources free over a slot. It reuses exactly the same
// predicates as Check: one mechanic, two directions (RG-78, D-03). Two
// implementations would be a guarantee that they eventually disagree.
func Find(snap *Snapshot, q Query) []Resource {
	var free []Resource

	for _, res := range snap.Resources {
		if res.Archived || res.Kind != q.Kind {
			continue
		}
		if q.RequiredCategory != "" && !hasText(res.Categories, q.RequiredCategory) {
			continue
		}
		cand := Candidate{
			LessonID:  q.IgnoreLessonID,
			Slot:      q.Slot,
			Resources: []uuid.UUID{res.ID},
		}
		if noBlockingConflict(Check(snap, cand)) {
			free = append(free, res)
		}
	}

	sort.Slice(free, func(i, j int) bool { return free[i].Label < free[j].Label })
	return free
}

// noBlockingConflict: only CRITICAL conflicts remove a resource from
// suggestions. WARNING leaves it suggested with a flag — the engine does
// not decide for the planner (D-04, RG-95).
func noBlockingConflict(conflicts []Conflict) bool {
	for _, c := range conflicts {
		if c.Severity == Critical {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

// chainSharedCauses: when one cause yields several conflicts — an exam
// session ties up both the instructor and the vehicle — the first carries
// the explanation and the others point at it (RG-218). Repeating the same
// overlap twice would suggest two distinct problems.
func chainSharedCauses(conflicts []Conflict) {
	seen := map[string]int{}
	for i := range conflicts {
		c := &conflicts[i]
		if c.Overlap == nil || len(c.Parties) < 2 {
			continue
		}
		key := c.Parties[1].ID.String() + c.Overlap.Start.String()
		if first, exists := seen[key]; exists {
			idx := first
			c.SameCauseAs = &idx
		} else {
			seen[key] = i
		}
	}
}

func sortBySeverity(conflicts []Conflict) {
	rank := map[Severity]int{Critical: 0, Warning: 1, Info: 2}
	sort.SliceStable(conflicts, func(i, j int) bool {
		return rank[conflicts[i].Severity] < rank[conflicts[j].Severity]
	})
}

func bookedKindFor(k ResourceKind) ConflictKind {
	if k == Instructor {
		return InstructorBooked
	}
	return ResourceBooked
}

func travelNote(ex ExamSession) string {
	if ex.TravelTime == 0 {
		return ""
	}
	return "mobilisé dès " + ex.Slot.Start.Add(-ex.TravelTime).Format("15:04") + " pour le trajet aller-retour"
}

func hasID(list []uuid.UUID, id uuid.UUID) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}

func hasText(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func withinOpening(hours []OpeningHours, s TimeSlot) bool {
	if len(hours) == 0 {
		return true // nothing configured, nothing constrained
	}
	day := s.Start.Weekday()
	startMin := s.Start.Hour()*60 + s.Start.Minute()
	endMin := s.End.Hour()*60 + s.End.Minute()

	for _, h := range hours {
		if h.Weekday != day {
			continue
		}
		open, err1 := time.Parse("15:04", h.Start)
		closed, err2 := time.Parse("15:04", h.End)
		if err1 != nil || err2 != nil {
			continue
		}
		if startMin >= open.Hour()*60+open.Minute() &&
			endMin <= closed.Hour()*60+closed.Minute() {
			return true
		}
	}
	return false
}
