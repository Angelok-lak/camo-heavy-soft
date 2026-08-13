package planning

// F-10, slice 2: suggest students for a lesson, ranked by how late they
// run on projected hours (RG-153). The slice-1 name search grew into the
// real thing the moment attendance made the counters live.
//
// Two rules bound the list:
//   - Exclusions stay minimal (RG-154): closed enrollments and students
//     already placed. Everyone else appears — a student with no gap is
//     still placeable, just ranked lower (RG-95: signal, never remove).
//   - Every rank is EXPLAINED (RG-97): gap in hours, days left,
//     projected against threshold. A bare ordering would be an oracle.

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
)

type SuggestedStudent struct {
	EnrollmentID uuid.UUID  `json:"enrollment_id"`
	StudentName  string     `json:"student_name"`
	Objective    string     `json:"objective"`
	Category     string     `json:"category"`
	// The explanation of the rank (RG-97). Zero-gap students carry
	// GapHours 0 and sort after the late ones.
	GapHours   float64    `json:"gap_hours"`
	Target     string     `json:"target"` // which date drives the gap, "" if none
	TargetDate *time.Time `json:"target_date"`
	DaysLeft   *int       `json:"days_left"`
	Projected  *float64   `json:"projected_hours"`
	Threshold  *float64   `json:"threshold_hours"`
}

func (h *Handler) suggestStudents(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	lessonID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "unknown lesson")
		return
	}
	var exists bool
	err = h.store.pool.QueryRow(r.Context(), `
		SELECT EXISTS (SELECT 1 FROM lesson WHERE school_id = $1 AND id = $2)`,
		id.SchoolID, lessonID).Scan(&exists)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		fail(w, http.StatusNotFound, "unknown lesson")
		return
	}

	// The worst of the two gaps drives the rank; nearest date breaks
	// ties; alphabetical order settles the gap-free tail.
	rows, err := h.store.pool.Query(r.Context(), `
		SELECT eh.enrollment_id, p.last_name || ' ' || p.first_names, o.label, o.label,
		       g.target, g.target_date, g.projected, g.threshold,
		       GREATEST(g.threshold - g.projected, 0) AS gap
		FROM enrollment_hours eh
		JOIN enrollment e ON e.id = eh.enrollment_id AND e.life_status = 'ACTIVE'
		JOIN person p ON p.id = e.person_id AND p.status = 'ACTIVE'
		JOIN objective o ON o.id = e.objective_id
		LEFT JOIN LATERAL (
			SELECT v.target, v.target_date, v.projected, v.threshold
			FROM (VALUES
			    ('OFFROAD', eh.offroad_target_date, eh.projected_offroad_hours, eh.hours_before_offroad),
			    ('ONROAD',  eh.onroad_target_date,  eh.projected_onroad_hours,  eh.total_hours)
			) AS v(target, target_date, projected, threshold)
			WHERE v.target_date IS NOT NULL AND v.projected IS NOT NULL
			ORDER BY (v.threshold - v.projected) DESC
			LIMIT 1
		) g ON true
		WHERE eh.school_id = $1
		  AND NOT EXISTS (SELECT 1 FROM lesson_assignment la
		                  WHERE la.lesson_id = $2 AND la.enrollment_id = eh.enrollment_id)
		ORDER BY GREATEST(g.threshold - g.projected, 0) DESC NULLS LAST,
		         g.target_date ASC NULLS LAST,
		         p.last_name, p.first_names`,
		id.SchoolID, lessonID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	out := []SuggestedStudent{}
	for rows.Next() {
		var s SuggestedStudent
		var target *string
		var gap *float64
		if err := rows.Scan(&s.EnrollmentID, &s.StudentName, &s.Objective, &s.Category,
			&target, &s.TargetDate, &s.Projected, &s.Threshold, &gap); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		if target != nil {
			s.Target = *target
		}
		if gap != nil {
			s.GapHours = *gap
		}
		if s.TargetDate != nil {
			d := int(time.Until(*s.TargetDate).Hours() / 24)
			s.DaysLeft = &d
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	reply(w, http.StatusOK, out)
}
