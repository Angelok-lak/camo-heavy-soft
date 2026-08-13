package exams

// F-33: the seat request, calibrated on the CENTRE'S REAL FILES
// ("TABLEAU BE C CE <MOIS>"): rows are the WEEKS of the target month,
// columns are three category GROUPS — BE · Isolés C/D/C1/D1 · Ensemble
// véhicules CE C1E DE D1E — counted in exam units. Sent before the 5th
// of M-2 (Q-139 settled by the files; the day stays a parameter).
//
// The suggestion derives one line per week: units (off-road 1, on-road
// 2) for every student whose target date falls in that week, with the
// names behind each number. The office's figures always win.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"github.com/angelok-lak/camo-heavy-soft/internal/auth"
	"github.com/angelok-lak/camo-heavy-soft/internal/events"
)

func (h *Handler) seatRequestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/seat-requests/{month}/suggestion", h.seatRequestSuggestion)
	mux.HandleFunc("POST /api/seat-requests/{month}/generate", h.generateSeatRequest)
	mux.HandleFunc("GET /api/seat-requests/{month}/file", h.seatRequestFile)
}

// WeekLine mirrors one row of the real table.
type WeekLine struct {
	Week      string `json:"week"`  // "Sem 36"
	Range     string `json:"range"` // "1er au 4 septembre"
	BE        int    `json:"be"`
	Isoles    int    `json:"isoles"`
	Ensembles int    `json:"ensembles"`
	// The why, suggestion only: names per group with their test.
	Students []string `json:"students,omitempty"`
}

func groupOf(label string) string {
	switch label {
	case "BE":
		return "BE"
	case "CE", "C1E", "DE", "D1E":
		return "ENSEMBLES"
	default: // C, D, C1, D1…
		return "ISOLES"
	}
}

// weeksOf lists the Monday-to-Friday spans touching the month.
func weeksOf(month time.Time) []struct {
	Start, End time.Time
	Label      string
} {
	out := []struct {
		Start, End time.Time
		Label      string
	}{}
	monthEnd := month.AddDate(0, 1, 0)
	// Back to the Monday of the week containing the 1st.
	d := month.AddDate(0, 0, -((int(month.Weekday()) + 6) % 7))
	for d.Before(monthEnd) {
		friday := d.AddDate(0, 0, 4)
		start, end := d, friday
		if start.Before(month) {
			start = month
		}
		if end.After(monthEnd.AddDate(0, 0, -1)) {
			end = monthEnd.AddDate(0, 0, -1)
		}
		if !start.After(end) {
			_, wk := d.ISOWeek()
			out = append(out, struct {
				Start, End time.Time
				Label      string
			}{start, end, fmt.Sprintf("Sem %d", wk)})
		}
		d = d.AddDate(0, 0, 7)
	}
	return out
}

func frMonthName(t time.Time) string {
	months := []string{"janvier", "février", "mars", "avril", "mai", "juin",
		"juillet", "août", "septembre", "octobre", "novembre", "décembre"}
	return months[int(t.Month())-1]
}

func rangeLabel(start, end time.Time) string {
	day := func(t time.Time) string {
		if t.Day() == 1 {
			return "1er"
		}
		return fmt.Sprintf("%d", t.Day())
	}
	return day(start) + " au " + day(end) + " " + frMonthName(end)
}

// suggestionFor fills the week grid from the target dates: off-road = 1
// unit, on-road = 2 (the credit scale), named students beside each row.
func (h *Handler) suggestionFor(r *http.Request, schoolID uuid.UUID, month time.Time) ([]WeekLine, error) {
	weeks := weeksOf(month)
	lines := make([]WeekLine, len(weeks))
	for i, w := range weeks {
		lines[i] = WeekLine{Week: w.Label, Range: rangeLabel(w.Start, w.End)}
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT o.label, k.kind, k.target, p.last_name || ' ' || p.first_names
		FROM enrollment e
		JOIN objective o ON o.id = e.objective_id
		JOIN person p ON p.id = e.person_id
		CROSS JOIN LATERAL (VALUES
		    ('OFFROAD', e.offroad_target_date),
		    ('ONROAD', e.onroad_target_date)
		) AS k(kind, target)
		WHERE e.school_id = $1 AND e.life_status = 'ACTIVE'
		  AND k.target >= $2::date AND k.target < ($2::date + interval '1 month')
		ORDER BY k.target`, schoolID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var label, kind, name string
		var target time.Time
		if err := rows.Scan(&label, &kind, &target, &name); err != nil {
			return nil, err
		}
		units := 1
		test := "plateau"
		if kind == "ONROAD" {
			units = 2
			test = "circulation"
		}
		for i, w := range weeksOf(month) {
			if !target.Before(w.Start) && !target.After(w.End.AddDate(0, 0, 1)) {
				switch groupOf(label) {
				case "BE":
					lines[i].BE += units
				case "ENSEMBLES":
					lines[i].Ensembles += units
				default:
					lines[i].Isoles += units
				}
				lines[i].Students = append(lines[i].Students,
					fmt.Sprintf("%s (%s %s, %d u.)", name, label, test, units))
				break
			}
		}
	}
	return lines, rows.Err()
}

func (h *Handler) deadlineDay(r *http.Request, schoolID uuid.UUID) (int, error) {
	var day int
	err := h.pool.QueryRow(r.Context(),
		`SELECT seat_request_deadline_day FROM school WHERE id = $1`, schoolID).Scan(&day)
	return day, err
}

// DeadlineFor exposes the send-by date of a target month: day D of M-2,
// as printed on the real files ("à envoyer avant le 5 juin" for August).
func DeadlineFor(month time.Time, day int) time.Time {
	return time.Date(month.Year(), month.Month(), day, 23, 59, 0, 0, month.Location()).
		AddDate(0, -2, 0)
}

func parseMonth(raw string) (time.Time, error) {
	return time.ParseInLocation("2006-01", raw, time.Local)
}

func (h *Handler) seatRequestSuggestion(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	month, err := parseMonth(r.PathValue("month"))
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "month must be YYYY-MM")
		return
	}
	day, err := h.deadlineDay(r, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	suggested, err := h.suggestionFor(r, id.SchoolID, month)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	var genAt *time.Time
	var stored []byte
	var storedLines []WeekLine
	err = h.pool.QueryRow(r.Context(), `
		SELECT generated_at, lines FROM seat_request
		WHERE school_id = $1 AND target_month = $2`, id.SchoolID, month).Scan(&genAt, &stored)
	if err == nil {
		_ = json.Unmarshal(stored, &storedLines)
	}
	deadline := DeadlineFor(month, day)
	reply(w, http.StatusOK, map[string]any{
		"deadline":        deadline,
		"deadline_passed": time.Now().After(deadline),
		"suggested":       suggested,
		"generated_at":    genAt,
		"generated_lines": storedLines,
	})
}

func (h *Handler) generateSeatRequest(w http.ResponseWriter, r *http.Request) {
	id, ok := requireManager(w, r)
	if !ok {
		return
	}
	month, err := parseMonth(r.PathValue("month"))
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "month must be YYYY-MM")
		return
	}
	// Past the send-by date the request can no longer be submitted:
	// generating the file would produce a dead document. Not a business
	// rule softened into an alert (D-04) — the action itself is moot
	// (arbitrage Angelo, août 2026).
	day, err := h.deadlineDay(r, id.SchoolID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deadline := DeadlineFor(month, day); time.Now().After(deadline) {
		fail(w, http.StatusUnprocessableEntity,
			"l'échéance d'envoi du "+deadline.Format("02/01/2006")+" est dépassée pour ce mois")
		return
	}

	// The office's numbers rule; the suggestion is only the fallback.
	var body struct {
		Lines []WeekLine `json:"lines"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	needed := body.Lines
	if len(needed) == 0 {
		needed, err = h.suggestionFor(r, id.SchoolID, month)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	for i := range needed {
		needed[i].Students = nil // the trace stores numbers, not names
	}
	lines, err := json.Marshal(needed)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	newID, err := uuid.NewV7()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO seat_request (id, school_id, target_month, lines, generated_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (school_id, target_month)
		DO UPDATE SET lines = $4, generated_by = $5, generated_at = now()`,
		newID, id.SchoolID, month, lines, id.UserID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = events.Emit(r.Context(), h.pool, id.SchoolID, "seat_request.generated", "seat_request",
		newID, needed, id.UserID)
	reply(w, http.StatusOK, map[string]any{"lines": needed})
}

// seatRequestFile: the real table's columns, as CSV Excel opens as-is.
func (h *Handler) seatRequestFile(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		fail(w, http.StatusUnauthorized, "no identity")
		return
	}
	month, err := parseMonth(r.PathValue("month"))
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "month must be YYYY-MM")
		return
	}
	var raw []byte
	err = h.pool.QueryRow(r.Context(), `
		SELECT lines FROM seat_request WHERE school_id = $1 AND target_month = $2`,
		id.SchoolID, month).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, http.StatusNotFound, "aucune demande générée pour ce mois")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	var needed []WeekLine
	if err := json.Unmarshal(raw, &needed); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	day, _ := h.deadlineDay(r, id.SchoolID)
	deadline := DeadlineFor(month, day)

	buf, err := officialSeatRequestFile(month, deadline, needed)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"TABLEAU BE C CE %s %d.xlsx\"",
			strings.ToUpper(frMonthName(month)), month.Year()))
	_, _ = w.Write(buf)
}

// officialSeatRequestFile reproduces the layout of the real table the
// exam authority expects (« TABLEAU BE C CE AOUT 2026.xlsx », Q-131):
// blue centred title, red send-by line, bordered « Nombre d'examens »
// header over BE / Isolés / Ensembles, one row per week with the dates
// in red on the right, « En Unités » in green.
func officialSeatRequestFile(month, deadline time.Time, needed []WeekLine) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Sheet1"

	border := []excelize.Border{
		{Type: "left", Style: 2, Color: "000000"},
		{Type: "right", Style: 2, Color: "000000"},
		{Type: "top", Style: 2, Color: "000000"},
		{Type: "bottom", Style: 2, Color: "000000"},
	}
	center := excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}
	titleSt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "1F4FD8"},
		Alignment: &center, Border: border,
	})
	deadlineSt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "C00000"},
		Alignment: &center,
	})
	headSt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "1F4FD8"},
		Alignment: &center, Border: border,
	})
	unitsSt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "00A650"},
		Alignment: &center, Border: border,
	})
	weekSt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11},
		Alignment: &center, Border: border,
	})
	numSt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 12},
		Alignment: &center, Border: border,
	})
	datesSt, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Italic: true, Size: 12, Color: "C00000"},
		Alignment: &center, Border: border,
	})

	widths := map[string]float64{"A": 14, "B": 12, "C": 18, "D": 24, "E": 20}
	for col, wd := range widths {
		_ = f.SetColWidth(sheet, col, col, wd)
	}

	// Title and send-by line.
	_ = f.MergeCell(sheet, "A1", "E1")
	_ = f.SetCellValue(sheet, "A1",
		fmt.Sprintf("AUTO ECOLE %s %d", strings.ToUpper(frMonthName(month)), month.Year()))
	_ = f.SetCellStyle(sheet, "A1", "E1", titleSt)
	_ = f.SetRowHeight(sheet, 1, 28)
	_ = f.MergeCell(sheet, "A2", "E2")
	_ = f.SetCellValue(sheet, "A2",
		fmt.Sprintf("à envoyer avant le %d %s", deadline.Day(), frMonthName(deadline)))
	_ = f.SetCellStyle(sheet, "A2", "E2", deadlineSt)

	// « Nombre d'examens » over the three count columns.
	_ = f.MergeCell(sheet, "B3", "D3")
	_ = f.SetCellValue(sheet, "B3", "Nombre d'examens")
	_ = f.SetCellStyle(sheet, "A3", "E3", headSt)
	_ = f.SetCellValue(sheet, "B4", "BE")
	_ = f.SetCellValue(sheet, "C4", "Isolés C/D/C1/D1")
	_ = f.SetCellValue(sheet, "D4", "ENSEMBLE VEHICULES CE C1E DE D1E")
	_ = f.SetCellStyle(sheet, "A4", "D4", headSt)
	_ = f.SetCellValue(sheet, "E4", "En Unités")
	_ = f.SetCellStyle(sheet, "E4", "E4", unitsSt)
	_ = f.SetRowHeight(sheet, 4, 30)

	// One row per week: Sem on the left, counts, dates in red.
	for i, l := range needed {
		row := 5 + i
		rs := func(col string) string { return fmt.Sprintf("%s%d", col, row) }
		_ = f.SetCellValue(sheet, rs("A"), l.Week)
		_ = f.SetCellStyle(sheet, rs("A"), rs("A"), weekSt)
		if l.BE > 0 {
			_ = f.SetCellValue(sheet, rs("B"), l.BE)
		}
		if l.Isoles > 0 {
			_ = f.SetCellValue(sheet, rs("C"), l.Isoles)
		}
		if l.Ensembles > 0 {
			_ = f.SetCellValue(sheet, rs("D"), l.Ensembles)
		}
		_ = f.SetCellStyle(sheet, rs("B"), rs("D"), numSt)
		_ = f.SetCellValue(sheet, rs("E"), l.Range)
		_ = f.SetCellStyle(sheet, rs("E"), rs("E"), datesSt)
		_ = f.SetRowHeight(sheet, row, 34)
	}

	var out bytes.Buffer
	if err := f.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
