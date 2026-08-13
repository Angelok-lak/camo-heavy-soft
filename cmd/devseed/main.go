// devseed fills a development database with a believable centre:
// accounts, instructors, vehicles, students, opening hours. Idempotent —
// run it as often as you like. DEVELOPMENT ONLY: real data comes from the
// centre, never from here.
//
//	DATABASE_URL=... go run ./cmd/devseed
//
// Accounts: angelo/secretariat (OFFICE), direction/direction (MANAGEMENT),
// alhan/formateur (INSTRUCTOR, linked to the instructor resource).
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	exec := func(sql string, args ...any) {
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			log.Fatalf("%s\n→ %v", sql, err)
		}
	}
	hash := func(pw string) string {
		h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err != nil {
			log.Fatal(err)
		}
		return string(h)
	}

	schoolID := stableID("school")
	exec(`INSERT INTO school (id, label) VALUES ($1, 'CAMO-EDUCASER — Centre de formation groupe lourd')
	      ON CONFLICT (id) DO NOTHING`, schoolID)

	// Accounts and profiles
	users := []struct {
		key, login, name, pw, profile string
	}{
		{"user-angelo", "angelo", "Angelo", "secretariat", "OFFICE"},
		{"user-direction", "direction", "Direction", "direction", "MANAGEMENT"},
		{"user-alhan", "alhan", "Alhan", "formateur", "INSTRUCTOR"},
	}
	for _, u := range users {
		id := stableID(u.key)
		exec(`INSERT INTO app_user (id, school_id, login, display_name, secret_hash)
		      VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING`,
			id, schoolID, u.login, u.name, hash(u.pw))
		exec(`INSERT INTO user_profile (id, user_id, profile)
		      SELECT $1, $2, $3
		      WHERE NOT EXISTS (SELECT 1 FROM user_profile
		                        WHERE user_id = $2 AND profile = $3 AND ends_at IS NULL)`,
			stableID(u.key+"-profile"), id, u.profile)
	}

	// Licence categories
	for _, code := range []string{"C", "CE", "D"} {
		exec(`INSERT INTO licence_category (id, school_id, code) VALUES ($1, $2, $3)
		      ON CONFLICT (school_id, code) DO NOTHING`, stableID("cat-"+code), schoolID, code)
	}

	// Resources: two instructors, two vehicles, one room
	resources := []struct {
		key, kind, label string
		categories       []string
	}{
		{"res-alhan", "INSTRUCTOR", "Alhan", nil},
		{"res-karim", "INSTRUCTOR", "Karim", nil},
		{"res-ce01", "VEHICLE", "CE-01", []string{"C", "CE"}},
		{"res-c02", "VEHICLE", "C-02", []string{"C"}},
		{"res-d01", "VEHICLE", "D-01", []string{"D"}},
		{"res-salle", "ROOM", "Salle 1", nil},
	}
	for _, r := range resources {
		id := stableID(r.key)
		exec(`INSERT INTO resource (id, school_id, kind, label) VALUES ($1, $2, $3, $4)
		      ON CONFLICT (id) DO NOTHING`, id, schoolID, r.kind, r.label)
		if r.kind == "VEHICLE" {
			exec(`INSERT INTO resource_vehicle (resource_id) VALUES ($1)
			      ON CONFLICT (resource_id) DO NOTHING`, id)
			for _, c := range r.categories {
				exec(`INSERT INTO resource_vehicle_category (resource_id, category_id)
				      VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, stableID("cat-"+c))
			}
		}
	}
	exec(`INSERT INTO resource_instructor (resource_id, user_id) VALUES ($1, $2)
	      ON CONFLICT (resource_id) DO NOTHING`, stableID("res-alhan"), stableID("user-alhan"))
	exec(`INSERT INTO resource_instructor (resource_id) VALUES ($1)
	      ON CONFLICT (resource_id) DO NOTHING`, stableID("res-karim"))

	// Lesson kinds
	exec(`INSERT INTO lesson_kind (id, school_id, label, requires_vehicle)
	      VALUES ($1, $2, 'Conduite', true) ON CONFLICT (id) DO NOTHING`,
		stableID("kind-conduite"), schoolID)
	exec(`INSERT INTO lesson_kind (id, school_id, label, requires_vehicle)
	      VALUES ($1, $2, 'Théorie', false) ON CONFLICT (id) DO NOTHING`,
		stableID("kind-theorie"), schoolID)

	// Opening hours: Monday–Friday 08:00–19:00, Saturday 08:00–12:00
	for d := 1; d <= 5; d++ {
		exec(`INSERT INTO opening_hours (id, school_id, weekday, starts_at, ends_at)
		      VALUES ($1, $2, $3, '08:00', '19:00') ON CONFLICT (id) DO NOTHING`,
			stableID("open-"+string(rune('0'+d))), schoolID, d)
	}
	exec(`INSERT INTO opening_hours (id, school_id, weekday, starts_at, ends_at)
	      VALUES ($1, $2, 6, '08:00', '12:00') ON CONFLICT (id) DO NOTHING`,
		stableID("open-6"), schoolID)

	// Students with enrollments
	students := []struct{ key, last, first, objective, neph string }{
		{"std-chiffleau", "d'Arc", "Jeanne", "CE", "0123456789012"},
		{"std-martin", "Raspoutine", "Grigori", "C", "0123456789345"},
		{"std-rossi", "Calamity", "Jane", "D", "0123456789678"},
	}
	objectives := map[string][2]float64{"C": {49, 70}, "CE": {70, 90}, "D": {49, 70}}
	for label, h := range objectives {
		exec(`INSERT INTO objective (id, school_id, label, hours_before_offroad, total_hours)
		      VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING`,
			stableID("obj-"+label), schoolID, label, h[0], h[1])
	}
	for _, s := range students {
		pid := stableID(s.key)
		exec(`INSERT INTO person (id, school_id, last_name, first_names, neph)
		      VALUES ($1, $2, $3, $4, $5)
		      ON CONFLICT (id) DO UPDATE SET last_name = EXCLUDED.last_name,
		                                     first_names = EXCLUDED.first_names`,
			pid, schoolID, s.last, s.first, s.neph)
		h := objectives[s.objective]
		exec(`INSERT INTO enrollment (id, school_id, person_id, objective_id, hours_before_offroad, total_hours)
		      VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`,
			stableID(s.key+"-enr"), schoolID, pid, stableID("obj-"+s.objective), h[0], h[1])
	}

	// Exam places (RG-140: the travel time lives on the place).
	exec(`INSERT INTO exam_place (id, school_id, label, travel_minutes)
	      VALUES ($1, $2, 'Centre d''examen de Meaux', 45) ON CONFLICT (id) DO NOTHING`,
		stableID("explace-meaux"), schoolID)
	exec(`INSERT INTO exam_place (id, school_id, label, travel_minutes)
	      VALUES ($1, $2, 'Centre d''examen de Melun', 30) ON CONFLICT (id) DO NOTHING`,
		stableID("explace-melun"), schoolID)

	seedCurrentWeek(exec)
	seedHistory(exec)
	seedExamSession(exec)
	seedPastExamSession(exec)
	seedRequirements(exec)

	log.Println("seeded. Accounts: angelo/secretariat · direction/direction · alhan/formateur")
}

// seedHistory makes the counters real: target dates on the enrollments,
// three past weeks of recorded lessons, and a lesson happening right now
// for the instructor's "current lesson" screen. Jeanne accumulates hours
// steadily; Jane misses lessons — she is the one the gap list surfaces.
func seedHistory(exec func(sql string, args ...any)) {
	// Target dates: off-road in ~5 weeks, on-road in ~10.
	for _, key := range []string{"std-chiffleau", "std-martin", "std-rossi"} {
		exec(`UPDATE enrollment SET
		        offroad_target_date = (now() + interval '35 days')::date,
		        onroad_target_date  = (now() + interval '70 days')::date
		      WHERE id = $1`, stableID(key+"-enr"))
	}

	// Every history lesson is COHERENT with the rules: the vehicle covers
	// the student's category (RG-62), the instructor is free, the kind is
	// set. The deliberately conflicting lessons live in seedCurrentWeek
	// only, one per conflict type, as the demo of detection.
	now := time.Now()
	pastLesson := func(start time.Time, key, vehicle, instructor, student, value string) {
		wk := start.Format("2006-01-02")
		id := stableID("lesson-" + wk + "-" + key)
		exec(`INSERT INTO lesson (id, school_id, starts_at, ends_at)
		      VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`,
			id, stableID("school"), start, start.Add(3*time.Hour))
		exec(`INSERT INTO lesson_lesson_kind (lesson_id, lesson_kind_id)
		      VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, stableID("kind-conduite"))
		exec(`INSERT INTO lesson_resource (lesson_id, resource_id)
		      VALUES ($1, $2), ($1, $3) ON CONFLICT DO NOTHING`,
			id, stableID(vehicle), stableID(instructor))
		assignID := stableID("assign-" + wk + "-" + key)
		exec(`INSERT INTO lesson_assignment (id, lesson_id, enrollment_id, assigned_by)
		      VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			assignID, id, stableID(student+"-enr"), stableID("user-angelo"))
		exec(`INSERT INTO attendance (id, school_id, lesson_assignment_id, value, recorded_by)
		      VALUES ($1, $2, $3, $4, $5) ON CONFLICT (lesson_assignment_id) DO NOTHING`,
			stableID("att-"+wk+"-"+key), stableID("school"), assignID, value, stableID("user-alhan"))
	}
	for week := 1; week <= 3; week++ {
		day := now.AddDate(0, 0, -7*week)
		at := func(h int) time.Time {
			return time.Date(day.Year(), day.Month(), day.Day(), h, 0, 0, 0, time.Local)
		}
		// Jeanne (CE) on CE-01 with Alhan; Grigori (C) on C-02 with Karim;
		// Jane (D) on D-01 with Karim — one unexcused absence for Jane.
		pastLesson(at(9), "hist-aurore", "res-ce01", "res-alhan", "std-chiffleau", "PRESENT")
		pastLesson(at(14), "hist-julien", "res-c02", "res-karim", "std-martin", "PRESENT")
		annaValue := "PRESENT"
		if week == 2 {
			annaValue = "UNEXCUSED"
		}
		pastLesson(at(9), "hist-anna", "res-d01", "res-karim", "std-rossi", annaValue)
	}

	// Today's lesson for Alhan on CE-01 (covers C and CE: both students
	// fit), left unrecorded on purpose — it is what the instructor's
	// attendance screen opens on (RG-236). Clamped inside opening hours
	// (08:00–19:00) so it carries no conflict: running now when possible,
	// otherwise the day's nearest.
	hour := now.Hour() - 1
	if hour < 8 {
		hour = 8
	}
	if hour > 16 {
		hour = 16
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, time.Local)
	nowKey := "lesson-now-" + start.Format("2006-01-02")
	id := stableID(nowKey)
	exec(`INSERT INTO lesson (id, school_id, starts_at, ends_at)
	      VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`,
		id, stableID("school"), start, start.Add(3*time.Hour))
	exec(`INSERT INTO lesson_lesson_kind (lesson_id, lesson_kind_id)
	      VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, stableID("kind-conduite"))
	exec(`INSERT INTO lesson_resource (lesson_id, resource_id)
	      VALUES ($1, $2), ($1, $3) ON CONFLICT DO NOTHING`,
		id, stableID("res-ce01"), stableID("res-alhan"))
	for _, s := range []string{"std-chiffleau", "std-martin"} {
		exec(`INSERT INTO lesson_assignment (id, lesson_id, enrollment_id, assigned_by)
		      VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			stableID(nowKey+"-"+s), id, stableID(s+"-enr"), stableID("user-angelo"))
	}
}

// seedCurrentWeek plants a believable week of lessons around today, built
// to EXHIBIT one conflict of each kind the detector knows — that is what
// slice 1 demonstrates. Keys embed the Monday date: rerunning within the
// same week changes nothing, next week seeds a fresh one.
func seedCurrentWeek(exec func(sql string, args ...any)) {
	now := time.Now()
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).
		AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
	wk := monday.Format("2006-01-02")

	at := func(day, hour, min int) time.Time {
		return monday.AddDate(0, 0, day).Add(time.Duration(hour)*time.Hour + time.Duration(min)*time.Minute)
	}

	lesson := func(key string, start, end time.Time, kindKey string, resKeys, stdKeys []string, cancelReason string) {
		id := stableID("lesson-" + wk + "-" + key)
		exec(`INSERT INTO lesson (id, school_id, starts_at, ends_at)
		      VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`,
			id, stableID("school"), start, end)
		if cancelReason != "" {
			exec(`UPDATE lesson SET status = 'CANCELLED', cancel_reason = $2, cancelled_by = $3
			      WHERE id = $1 AND status = 'PLANNED'`,
				id, cancelReason, stableID("user-angelo"))
		}
		if kindKey != "" {
			exec(`INSERT INTO lesson_lesson_kind (lesson_id, lesson_kind_id)
			      VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, stableID(kindKey))
		}
		for _, r := range resKeys {
			exec(`INSERT INTO lesson_resource (lesson_id, resource_id)
			      VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, stableID(r))
		}
		for _, s := range stdKeys {
			exec(`INSERT INTO lesson_assignment (id, lesson_id, enrollment_id, assigned_by)
			      VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
				stableID("assign-"+wk+"-"+key+"-"+s), id, stableID(s+"-enr"), stableID("user-angelo"))
		}
	}

	// Monday: two clean lessons — the normal day.
	lesson("mon-am", at(0, 9, 0), at(0, 12, 0), "kind-conduite",
		[]string{"res-ce01", "res-alhan"}, []string{"std-chiffleau"}, "")
	lesson("mon-pm", at(0, 14, 0), at(0, 17, 0), "kind-conduite",
		[]string{"res-c02", "res-karim"}, []string{"std-martin"}, "")

	// Tuesday: CE-01 double-booked AND Jeanne in two places at once →
	// RESOURCE_BOOKED + STUDENT_BOOKED, both critical.
	lesson("tue-am", at(1, 9, 0), at(1, 12, 0), "kind-conduite",
		[]string{"res-ce01", "res-alhan"}, []string{"std-chiffleau"}, "")
	lesson("tue-clash", at(1, 10, 0), at(1, 13, 0), "kind-conduite",
		[]string{"res-ce01", "res-karim"}, []string{"std-chiffleau"}, "")

	// Wednesday: C-02 breaks down, return unknown (RG-65), but its lesson
	// KEEPS the assignment and shows why (RG-80) → RESOURCE_OFFLINE.
	exec(`INSERT INTO unavailability (id, school_id, resource_id, reason, starts_at, declared_by)
	      VALUES ($1, $2, $3, 'Panne — retour inconnu', $4, $5) ON CONFLICT (id) DO NOTHING`,
		stableID("unav-"+wk+"-c02"), stableID("school"), stableID("res-c02"),
		at(2, 0, 0), stableID("user-angelo"))
	lesson("wed-am", at(2, 9, 0), at(2, 11, 0), "kind-conduite",
		[]string{"res-c02", "res-karim"}, []string{"std-martin"}, "")

	// Thursday: Jane prepares D, CE-01 covers C and CE →
	// CATEGORY_MISMATCH, a warning that never removes anyone (RG-95).
	lesson("thu-am", at(3, 9, 0), at(3, 12, 0), "kind-conduite",
		[]string{"res-ce01", "res-alhan"}, []string{"std-rossi"}, "")

	// Friday 07:00: before opening (08:00) → OUTSIDE_HOURS, warning.
	lesson("fri-early", at(4, 7, 0), at(4, 9, 0), "kind-theorie",
		[]string{"res-salle", "res-karim"}, []string{"std-martin", "std-rossi"}, "")
	// Friday: a driving lesson with no vehicle → VEHICLE_MISSING (RG-149).
	lesson("fri-novehicle", at(4, 10, 0), at(4, 12, 0), "kind-conduite",
		[]string{"res-alhan"}, []string{"std-chiffleau"}, "")

	// Saturday: a cancelled lesson — greyed out, resources released.
	lesson("sat-cancelled", at(5, 9, 0), at(5, 11, 0), "kind-conduite",
		[]string{"res-ce01", "res-alhan"}, []string{"std-chiffleau"},
		"Météo — verglas")
}

// stableID derives the same UUID for the same key on every run — that is
// what makes the seed idempotent.
func stableID(key string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("crit-devseed:"+key))
}

// seedExamSession: next Tuesday in Meaux, CE-01 and Alhan committed. The
// 45-minute travel ties them up from 10:15 — poser une séance adjacente
// dans le planning fait apparaître le conflit avec sa note de trajet.
func seedExamSession(exec func(sql string, args ...any)) {
	now := time.Now()
	daysAhead := (int(time.Tuesday) - int(now.Weekday()) + 7) % 7
	if daysAhead == 0 {
		daysAhead = 7
	}
	day := now.AddDate(0, 0, daysAhead)
	start := time.Date(day.Year(), day.Month(), day.Day(), 11, 0, 0, 0, time.Local)
	key := "exam-" + start.Format("2006-01-02")
	id := stableID(key)
	exec(`INSERT INTO exam_session (id, school_id, exam_place_id, starts_at, ends_at, credit_allowance)
	      VALUES ($1, $2, $3, $4, $5, 8) ON CONFLICT (id) DO NOTHING`,
		id, stableID("school"), stableID("explace-meaux"), start, start.Add(6*time.Hour))
	exec(`INSERT INTO exam_session_resource (exam_session_id, resource_id)
	      VALUES ($1, $2), ($1, $3) ON CONFLICT DO NOTHING`,
		id, stableID("res-ce01"), stableID("res-alhan"))
}

// seedPastExamSession: a session already held, with every result state
// the screen must show — a pass (spent), a fail (spent), a still-
// committed booking past the date (derived "non renseigné", RG-112),
// and unengaged units forfeited (RG-39/40). Jane's off-road pass is
// what feeds "plateau obtenu le …" in the next session's proposals.
func seedPastExamSession(exec func(sql string, args ...any)) {
	now := time.Now()
	day := now.AddDate(0, 0, -18)
	start := time.Date(day.Year(), day.Month(), day.Day(), 9, 0, 0, 0, time.Local)
	id := stableID("exam-past-1")
	exec(`INSERT INTO exam_session (id, school_id, exam_place_id, starts_at, ends_at, credit_allowance)
	      VALUES ($1, $2, $3, $4, $5, 6) ON CONFLICT (id) DO NOTHING`,
		id, stableID("school"), stableID("explace-melun"), start, start.Add(5*time.Hour))
	exec(`INSERT INTO exam_session_resource (exam_session_id, resource_id)
	      VALUES ($1, $2), ($1, $3) ON CONFLICT DO NOTHING`,
		id, stableID("res-ce01"), stableID("res-alhan"))

	angelo := stableID("user-angelo")
	resultAt := start.Add(8 * time.Hour)
	book := func(key, enr, kind string, units int, status string, recorded bool) {
		var by, at any
		if recorded {
			by, at = angelo, resultAt
		}
		exec(`INSERT INTO exam_booking
		        (id, school_id, exam_session_id, enrollment_id, test_kind, status,
		         committed_units, booked_by, booked_at, result_by, result_at)
		      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		      ON CONFLICT (id) DO NOTHING`,
			stableID(key), stableID("school"), id, stableID(enr), kind, status,
			units, angelo, start.AddDate(0, 0, -20), by, at)
	}
	book("exbk-past-rossi", "std-rossi-enr", "OFFROAD", 1, "PASSED", true)
	book("exbk-past-martin", "std-martin-enr", "ONROAD", 2, "FAILED", true)
	book("exbk-past-chiffleau", "std-chiffleau-enr", "OFFROAD", 1, "COMMITTED", false)
}

// seedRequirements: the two sets per objective (RG-02), copies on the
// existing enrollments, and Jeanne's file matching the validated mockup —
// entry complete, exam missing 2 of 4.
func seedRequirements(exec func(sql string, args ...any)) {
	type tpl struct {
		key, label, set string
		instructorOK    bool
		validityMonths  int
	}
	templates := []tpl{
		{"req-identite", "Pièce d'identité vérifiée", "ENTRY", false, 0},
		{"req-medical", "Avis médical", "ENTRY", false, 24},
		{"req-financement-ok", "Financement accordé", "ENTRY", false, 0},
		{"req-dossier", "Dossier administratif complet", "EXAM", false, 0},
		{"req-neph", "NEPH obtenu", "EXAM", false, 0},
		{"req-etg", "Code (ETG) réussi", "EXAM", true, 0},
		{"req-solde", "Financement soldé", "EXAM", false, 0},
	}
	for _, obj := range []string{"C", "CE", "D"} {
		for _, t := range templates {
			var validity any
			if t.validityMonths > 0 {
				validity = t.validityMonths
			}
			exec(`INSERT INTO requirement_template
			        (id, school_id, objective_id, label, req_set, mandatory, instructor_may_validate, validity_months)
			      VALUES ($1, $2, $3, $4, $5, true, $6, $7) ON CONFLICT (id) DO NOTHING`,
				stableID(t.key+"-"+obj), stableID("school"), stableID("obj-"+obj),
				t.label, t.set, t.instructorOK, validity)
		}
	}

	// Copies for enrollments that predate the templates (idempotent).
	exec(`INSERT INTO enrollment_requirement
	        (id, school_id, enrollment_id, template_id, label, req_set, mandatory,
	         instructor_may_validate, validity_months)
	      SELECT gen_random_uuid(), e.school_id, e.id, rt.id, rt.label, rt.req_set,
	             rt.mandatory, rt.instructor_may_validate, rt.validity_months
	      FROM enrollment e
	      JOIN requirement_template rt ON rt.objective_id = e.objective_id AND rt.active
	      WHERE NOT EXISTS (SELECT 1 FROM enrollment_requirement er
	                        WHERE er.enrollment_id = e.id AND er.label = rt.label)`)

	// Jeanne, as on the mockup: entry set complete, exam set missing
	// Code (ETG) and Financement soldé.
	exec(`UPDATE enrollment_requirement SET
	        status = 'VALIDATED', validated_by = $2, validated_at = now(),
	        valid_until = CASE WHEN validity_months IS NOT NULL
	            THEN (now() + make_interval(months => validity_months))::date END
	      WHERE enrollment_id = $1
	        AND (req_set = 'ENTRY' OR label IN ('Dossier administratif complet', 'NEPH obtenu'))`,
		stableID("std-chiffleau-enr"), stableID("user-angelo"))
}
