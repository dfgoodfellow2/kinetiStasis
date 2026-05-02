package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/export"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

type ExportHandler struct{ s store.Store }

func NewExportHandler(s store.Store) *ExportHandler { return &ExportHandler{s: s} }

func parseDateRange(r *http.Request) (from, to string) {
	to = r.URL.Query().Get("to")
	from = r.URL.Query().Get("from")
	today := time.Now().UTC().Format(constants.DateFormat)
	if to == "" {
		to = today
	}
	if from == "" {
		from = time.Now().UTC().AddDate(0, 0, -constants.DefaultExportLookbackDays).Format(constants.DateFormat)
	}
	return from, to
}

func writeExport(w http.ResponseWriter, r *http.Request, content, filename, mimeType string) {
	if r.Header.Get("Accept") == "application/json" {
		respond.JSON(w, 200, map[string]any{"content": content})
		return
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(200)
	if _, err := w.Write([]byte(content)); err != nil {
		// best-effort log — do not treat as fatal for response flow
		// use structured logger if available
		// fallback to standard library printing
		// but avoid importing log/slog here; keep simple
		return
	}
}

// GET /v1/export/nutrition
func (h *ExportHandler) Nutrition(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	from, to := parseDateRange(r)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}
	logs, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, from)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
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
	from, to := parseDateRange(r)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "md"
	}
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err != nil {
		profile.Units = "metric"
	}
	logs, err := h.s.FetchWorkouts(r.Context(), claims.UserID, from)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
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
	from, to := parseDateRange(r)
	profile, err := h.s.FetchProfile(r.Context(), claims.UserID)
	if err != nil {
		profile.Units = "metric"
	}
	nut, err := h.s.FetchNutritionLogs(r.Context(), claims.UserID, from)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	bio, err := h.s.FetchBiometricLogs(r.Context(), claims.UserID, from)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	// body_measurements table stores circumference measurements (waist etc.)
	// biometric_logs may not include waist; fetch body_measurements in range and merge waist values.
	db := h.s.DB()
	bmRows, err := db.QueryContext(r.Context(), `SELECT date, COALESCE(waist_cm,0) FROM body_measurements WHERE user_id = ? AND date >= ? AND date <= ?`, claims.UserID, from, to)
	if err == nil {
		defer bmRows.Close()
		bmMap := make(map[string]float64)
		for bmRows.Next() {
			var d string
			var waist float64
			if err := bmRows.Scan(&d, &waist); err == nil {
				if waist > 0 {
					bmMap[d] = waist
				}
			}
		}
		if len(bmMap) > 0 {
			// merge into bio slice: if biometric entry exists for date, set WaistCm if zero; otherwise create new biometric log entries for dates with only body measurements
			bioMap := make(map[string]*models.BiometricLog)
			for i := range bio {
				bioMap[bio[i].Date] = &bio[i]
			}
			for d, w := range bmMap {
				if b, ok := bioMap[d]; ok {
					if b.WaistCm == 0 {
						b.WaistCm = w
					}
				} else {
					// create a minimal BiometricLog for this date
					bio = append(bio, models.BiometricLog{Date: d, WaistCm: w})
				}
			}
		}
	}
	workouts, err := h.s.FetchWorkouts(r.Context(), claims.UserID, from)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	targets, err := h.s.FetchTargets(r.Context(), claims.UserID)
	if err != nil {
		targets = models.Targets{}
	}
	content := export.CombinedMarkdown(nut, bio, workouts, targets, from, to, profile.Units)
	filename := fmt.Sprintf("combined-%s-%s.md", from, to)
	writeExport(w, r, content, filename, "text/markdown")
}
