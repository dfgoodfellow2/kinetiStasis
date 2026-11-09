package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/export"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/validate"
)

type ExportHandler struct{ s store.Store }

func NewExportHandler(s store.Store) *ExportHandler { return &ExportHandler{s: s} }

// maxExportSpanDays bounds the export window. It keeps both the in-memory
// render and the SQL scan bounded even when a caller supplies a huge range.
const maxExportSpanDays = 366

// parseDateRange validates and resolves the export window.
//
// Defaults use the server's LOCAL calendar day, matching requestDate and
// daysSinceLocalDate elsewhere in the handlers package. Date-only exports must
// follow the same day boundary as the rest of the app; using UTC here would
// shift "today" (and therefore the default window) around local midnight.
//
// Both bounds are validated strictly as YYYY-MM-DD, from must not be after to,
// and the inclusive span may not exceed maxExportSpanDays.
func parseDateRange(r *http.Request) (from, to string, err error) {
	q := r.URL.Query()
	from = q.Get("from")
	to = q.Get("to")

	now := time.Now()
	if to == "" {
		to = now.Format(constants.DateFormat)
	}
	if from == "" {
		from = now.AddDate(0, 0, -constants.DefaultExportLookbackDays).Format(constants.DateFormat)
	}

	if validate.Date(from) != nil || validate.Date(to) != nil {
		return "", "", fmt.Errorf("invalid date range")
	}
	fromDate, err := time.Parse(constants.DateFormat, from)
	if err != nil {
		return "", "", fmt.Errorf("invalid date range")
	}
	toDate, err := time.Parse(constants.DateFormat, to)
	if err != nil {
		return "", "", fmt.Errorf("invalid date range")
	}
	if fromDate.After(toDate) {
		return "", "", fmt.Errorf("invalid date range")
	}
	if toDate.Sub(fromDate) > maxExportSpanDays*24*time.Hour {
		return "", "", fmt.Errorf("invalid date range")
	}
	return from, to, nil
}

// sanitizeFilename reduces a generated filename to a conservative allowlist so
// header interpolation can never inject quotes, newlines, path separators or
// query characters. The result is capped to keep response headers sane.
func sanitizeFilename(name string) string {
	const maxLen = 120
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	if out == "" {
		out = "export"
	}
	return out
}

func writeExport(w http.ResponseWriter, r *http.Request, content, filename, mimeType string) {
	if r.Header.Get("Accept") == "application/json" {
		respond.JSON(w, 200, map[string]any{"content": content})
		return
	}
	filename = sanitizeFilename(filename)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(content)); err != nil {
		slog.Error("write export response", "err", err, "filename", filename)
	}
}

// GET /v1/export/nutrition
func (h *ExportHandler) Nutrition(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	from, to, err := parseDateRange(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}
	logs, err := h.s.FetchNutritionLogsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		slog.Error("fetch nutrition logs for export", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	targets, err := h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil {
		targets = models.Targets{}
	}
	var content, filename, mime string
	if format == "csv" {
		content = export.NutritionCSV(logs, from, to)
		filename = fmt.Sprintf("nutrition-%s-%s.csv", from, to)
		mime = "text/csv"
	} else {
		content = export.NutritionMarkdown(logs, targets, from, to)
		filename = fmt.Sprintf("nutrition-%s-%s.md", from, to)
		mime = "text/markdown"
	}
	writeExport(w, r, content, filename, mime)
}

// GET /v1/export/workouts
func (h *ExportHandler) Workouts(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	from, to, err := parseDateRange(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err != nil {
		profile.Units = "metric"
	}
	logs, err := h.s.FetchWorkoutsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		slog.Error("fetch workouts for export", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	var content, filename, mime string
	if format == "csv" {
		content = export.WorkoutsCSV(logs, from, to, profile.Units)
		filename = fmt.Sprintf("workouts-%s-%s.csv", from, to)
		mime = "text/csv"
	} else {
		content = export.WorkoutsMarkdown(logs, from, to, profile.Units)
		filename = fmt.Sprintf("workouts-%s-%s.md", from, to)
		mime = "text/markdown"
	}
	writeExport(w, r, content, filename, mime)
}

// GET /v1/export/combined
func (h *ExportHandler) Combined(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	from, to, err := parseDateRange(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err != nil {
		profile.Units = "metric"
	}
	nut, err := h.s.FetchNutritionLogsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		slog.Error("fetch nutrition logs for combined export", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	bio, err := h.s.FetchBiometricLogsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		slog.Error("fetch biometric logs for combined export", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	// body_measurements table stores circumference measurements (waist etc.)
	// biometric_logs may not include waist; fetch body_measurements in range and merge waist values.
	//
	// Merge existing rows first, then append measurement-only dates in sorted
	// order. Capturing &bio[i] before appending is unsafe: append can grow the
	// slice and leave the map's pointers dangling, silently dropping the waist
	// value on whichever dates were processed before the reallocation.
	if bmMap, err := h.s.FetchBodyMeasurementsRangeMap(r.Context(), claims.UserID, from, to); err == nil && len(bmMap) > 0 {
		seen := make(map[string]struct{}, len(bio))
		for i := range bio {
			seen[bio[i].Date] = struct{}{}
			if bio[i].WaistCm == 0 {
				if waist, ok := bmMap[bio[i].Date]; ok {
					bio[i].WaistCm = waist
				}
			}
		}
		extra := make([]string, 0, len(bmMap))
		for date := range bmMap {
			if _, ok := seen[date]; !ok {
				extra = append(extra, date)
			}
		}
		sort.Strings(extra)
		for _, date := range extra {
			bio = append(bio, models.BiometricLog{Date: date, WaistCm: bmMap[date]})
		}
	}
	workouts, err := h.s.FetchWorkoutsRange(r.Context(), claims.UserID, from, to)
	if err != nil {
		slog.Error("fetch workouts for combined export", "err", err)
		respond.Error(w, http.StatusInternalServerError, "database error: "+err.Error())
		return
	}
	targets, err := h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil {
		targets = models.Targets{}
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}
	var content, filename, mime string
	if format == "csv" {
		content = export.CombinedCSV(nut, bio, workouts, from, to, profile.Units)
		filename = fmt.Sprintf("combined-%s-%s.csv", from, to)
		mime = "text/csv"
	} else {
		content = export.CombinedMarkdown(nut, bio, workouts, targets, from, to, profile.Units)
		filename = fmt.Sprintf("combined-%s-%s.md", from, to)
		mime = "text/markdown"
	}
	writeExport(w, r, content, filename, mime)
}
