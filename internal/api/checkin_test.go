package api_test

// Check-in policy tests. They drive the real HTTP stack so the Preview/Create
// gates, persistence and macro recomputation are exercised together. The matrix
// pinned here is:
//
//   0 weigh-ins  -> cannot check in; POST rejects and targets are untouched
//   3 weigh-ins  -> can check in, but < 7 weight days blocks target updates;
//                   the check-in row is still recorded with unchanged targets
//   8 weigh-ins  -> full energy-balance path; calories AND macros update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/nutrition"
)

func shiftDate(base string, delta int) string {
	d, _ := time.Parse("2006-01-02", base)
	return d.AddDate(0, 0, delta).Format("2006-01-02")
}

// doFromIP sends a request labelled with a distinct X-Forwarded-For address.
// The full-path test seeds many days of logs in a tight loop; the standard
// limiter (100 req/min, burst 20) would otherwise reject the burst from one
// client. Giving each seed request its own address keeps the limiter's
// production settings intact while still exercising the real HTTP stack.
func doFromIP(t *testing.T, h *harness, ip, method, path string, payload any, wantStatus int) []byte {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, h.server.URL+path, body)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Forwarded-For", ip)

	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, path, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d, body = %s", method, path, resp.StatusCode, wantStatus, data)
	}
	return data
}

func seedCheckinProfile(t *testing.T, h *harness) {
	t.Helper()
	h.do(t, http.MethodPut, "/v1/profile", map[string]any{
		"sex":               "male",
		"age":               35,
		"heightCm":          180,
		"activity":          "moderately_active",
		"exerciseFreq":      3,
		"goal":              "maintenance",
		"isLifter":          true,
		"bfPct":             20,
		"tdeeLookbackDays":  90,
		"macroDistribution": "balanced",
	}, http.StatusOK)
}

func seedCheckinTargets(t *testing.T, h *harness, calories float64) {
	t.Helper()
	h.do(t, http.MethodPut, "/v1/targets", map[string]any{
		"calories": calories,
		"proteinG": 150,
		"carbsG":   200,
		"fatG":     70,
		"fiberG":   30,
		"waterMl":  2500,
	}, http.StatusOK)
}

func postWeight(t *testing.T, h *harness, date string, kg float64) {
	t.Helper()
	h.do(t, http.MethodPost, "/v1/biometrics", map[string]any{
		"date":     date,
		"weightKg": kg,
	}, http.StatusOK)
}

func postSleepOnly(t *testing.T, h *harness, date string) {
	t.Helper()
	h.do(t, http.MethodPost, "/v1/biometrics", map[string]any{
		"date":       date,
		"sleepHours": 8,
	}, http.StatusOK)
}

// TestCheckinZeroWeighInsBlocksCheckin reproduces the production bug where
// sleep-only biometric rows unlocked the check-in gate. The new gate counts
// weight entries, so this must reject both Preview and POST.
func TestCheckinZeroWeighInsBlocksCheckin(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	// Three biometric ROWS but zero WEIGH-INS: the old len(bioLogs) gate would
	// have allowed this.
	for i := -2; i <= 0; i++ {
		postSleepOnly(t, h, shiftDate(testDate, i))
	}

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if preview.CanCheckIn {
		t.Fatal("CanCheckIn = true with zero weigh-ins; sleep rows must not unlock check-in")
	}
	if !preview.TargetUpdateBlocked {
		t.Error("TargetUpdateBlocked = false, want true")
	}
	if preview.Warning == "" {
		t.Error("Warning is empty, want tell-user-to-log-weight message")
	}
	if preview.HasWeighinData {
		t.Error("HasWeighinData = true, want false with zero weigh-ins")
	}
	if preview.RecommendedCalories != 2000 {
		t.Errorf("RecommendedCalories = %d, want current target 2000", preview.RecommendedCalories)
	}
	if preview.WeightStart != 0 || preview.WeightEnd != 0 || preview.WeightChange != 0 {
		t.Errorf("weight markers = (%v,%v,%v), want zeros", preview.WeightStart, preview.WeightEnd, preview.WeightChange)
	}

	// POST must reject and leave targets untouched.
	h.do(t, http.MethodPost, "/v1/checkin", map[string]any{"caloriesAfter": 2500}, http.StatusBadRequest)

	targets := decodeJSON[models.Targets](t, h.do(t, http.MethodGet, "/v1/targets", nil, http.StatusOK))
	if targets.Calories != 2000 {
		t.Fatalf("targets after rejected check-in = %v, want unchanged 2000", targets.Calories)
	}

	history := decodeJSON[[]models.CheckinHistoryEntry](t, h.do(t, http.MethodGet, "/v1/checkin/history", nil, http.StatusOK))
	if len(history) != 0 {
		t.Fatalf("check-in history rows = %d, want 0 (rejected check-in must not persist)", len(history))
	}
}

// TestCheckinTwoWeighInsRemainGated documents the weigh-in minimum: the
// production gate requires three weight-bearing days before any check-in.
func TestCheckinTwoWeighInsRemainGated(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	postWeight(t, h, shiftDate(testDate, -1), 82.0)
	postWeight(t, h, testDate, 82.0)

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if preview.CanCheckIn {
		t.Fatal("CanCheckIn = true with 2 weigh-ins, want false (need 3)")
	}
	// Weight data exists, but Preview suppresses the display markers while the
	// check-in gate is closed: the UI must not imply usable history.
	if !preview.HasWeighinData {
		t.Error("HasWeighinData = false, want true with 2 weigh-ins")
	}
	if preview.WeightStart != 0 || preview.WeightEnd != 0 || preview.WeightChange != 0 {
		t.Errorf("weight markers = (%v,%v,%v), want zeros while check-in is blocked",
			preview.WeightStart, preview.WeightEnd, preview.WeightChange)
	}
}

// TestCheckinInsufficientWeightHistoryRecordsButDoesNotUpdate is the middle
// policy case: enough weigh-ins to check in, but fewer than seven weight days,
// so no target update is allowed. The check-in row must still be recorded.
func TestCheckinInsufficientWeightHistoryRecordsButDoesNotUpdate(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	// Three distinct weight days: passes the check-in gate, fails the 7-day
	// weight-reliability gate.
	for i := -2; i <= 0; i++ {
		postWeight(t, h, shiftDate(testDate, i), 82.0)
	}

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if !preview.CanCheckIn {
		t.Fatal("CanCheckIn = false with 3 weigh-ins, want true")
	}
	if preview.WeightReliable {
		t.Error("WeightReliable = true with 3 weight days, want false")
	}
	if !preview.TargetUpdateBlocked {
		t.Error("TargetUpdateBlocked = false, want true with <7 weight days")
	}
	if preview.RecommendedCalories != 2000 {
		t.Errorf("RecommendedCalories = %d, want current target 2000", preview.RecommendedCalories)
	}
	if preview.CalorieAdjustment != 0 {
		t.Errorf("CalorieAdjustment = %v, want 0", preview.CalorieAdjustment)
	}

	// Client tries to force a raise; the policy must override it.
	created := decodeJSON[models.CheckInLog](t, h.do(t, http.MethodPost, "/v1/checkin", map[string]any{"caloriesAfter": 2600}, http.StatusCreated))
	if created.CaloriesAfter != 2000 {
		t.Errorf("persisted CaloriesAfter = %d, want current target 2000 (no adjustment)", created.CaloriesAfter)
	}

	targets := decodeJSON[models.Targets](t, h.do(t, http.MethodGet, "/v1/targets", nil, http.StatusOK))
	if targets.Calories != 2000 {
		t.Fatalf("targets after blocked update = %v, want unchanged 2000", targets.Calories)
	}
	if targets.ProteinG != 150 || targets.CarbsG != 200 || targets.FatG != 70 {
		t.Fatalf("macros changed despite blocked update: protein=%v carbs=%v fat=%v", targets.ProteinG, targets.CarbsG, targets.FatG)
	}

	history := decodeJSON[[]models.CheckinHistoryEntry](t, h.do(t, http.MethodGet, "/v1/checkin/history", nil, http.StatusOK))
	if len(history) != 1 {
		t.Fatalf("check-in history rows = %d, want 1 (blocked check-in still recorded)", len(history))
	}
	if history[0].ResultingCalories != 2000 {
		t.Errorf("history resultingCalories = %d, want 2000", history[0].ResultingCalories)
	}
}

// TestCheckinFullPathUpdatesCaloriesAndMacros exercises the reliable-weight
// path: the calorie target moves and the complete macro row is recomputed from
// the FINAL calories so macros can never disagree with the stored energy target.
func TestCheckinFullPathUpdatesCaloriesAndMacros(t *testing.T) {
	h := newHarness(t)
	seedCheckinProfile(t, h)
	seedCheckinTargets(t, h, 2000)

	// Eight weight days spanning seven calendar days: reliability gate passes.
	for i := -7; i <= 0; i++ {
		doFromIP(t, h, fmt.Sprintf("198.51.100.%d", i+8), http.MethodPost, "/v1/biometrics", map[string]any{
			"date":     shiftDate(testDate, i),
			"weightKg": 82.0,
		}, http.StatusOK)
	}
	// Constant intake so observed expenditure equals 2900 at zero weight change.
	for i := -7; i <= 0; i++ {
		doFromIP(t, h, fmt.Sprintf("203.0.113.%d", i+8), http.MethodPost, "/v1/nutrition/logs", map[string]any{
			"date":     shiftDate(testDate, i),
			"calories": 2900,
		}, http.StatusCreated)
	}

	preview := decodeJSON[models.CheckinPreview](t, h.do(t, http.MethodGet, "/v1/checkin", nil, http.StatusOK))
	if !preview.CanCheckIn {
		t.Fatal("CanCheckIn = false, want true")
	}
	if !preview.WeightReliable {
		t.Fatal("WeightReliable = false, want true with 8 weigh-ins over 7 days")
	}
	if preview.TargetUpdateBlocked {
		t.Fatal("TargetUpdateBlocked = true, want false on the full path")
	}
	if preview.RecommendedCalories != 2500 {
		t.Fatalf("RecommendedCalories = %d, want clamp(2000 + 500) = 2500", preview.RecommendedCalories)
	}
	if math.Abs(preview.ObservedTDEE-2900) > 25 {
		t.Fatalf("ObservedTDEE = %v, want ~2900 (flat weight)", preview.ObservedTDEE)
	}

	h.do(t, http.MethodPost, "/v1/checkin", map[string]any{"caloriesAfter": preview.RecommendedCalories}, http.StatusCreated)

	targets := decodeJSON[models.Targets](t, h.do(t, http.MethodGet, "/v1/targets", nil, http.StatusOK))
	if targets.Calories != 2500 {
		t.Fatalf("targets calories = %v, want 2500", targets.Calories)
	}

	profile := models.Profile{
		Sex:               "male",
		Age:               35,
		HeightCm:          180,
		Activity:          "moderately_active",
		ExerciseFreq:      3,
		Goal:              "maintenance",
		IsLifter:          true,
		BfPct:             20,
		MacroDistribution: "balanced",
	}
	want := nutrition.ComputeMacros(profile, targets.Calories, 82.0)
	if targets.ProteinG != want.ProteinG || targets.CarbsG != want.CarbsG || targets.FatG != want.FatG {
		t.Fatalf("macros = (p %v, c %v, f %v), want consistent with %v kcal: (p %v, c %v, f %v)",
			targets.ProteinG, targets.CarbsG, targets.FatG, targets.Calories,
			want.ProteinG, want.CarbsG, want.FatG)
	}
	if targets.FiberG != want.FiberG || targets.WaterMl != want.WaterMl {
		t.Fatalf("fiber/water = (%v,%v), want (%v,%v)", targets.FiberG, targets.WaterMl, want.FiberG, want.WaterMl)
	}

	history := decodeJSON[[]models.CheckinHistoryEntry](t, h.do(t, http.MethodGet, "/v1/checkin/history", nil, http.StatusOK))
	if len(history) != 1 {
		t.Fatalf("check-in history rows = %d, want 1", len(history))
	}
	if history[0].ResultingCalories != 2500 {
		t.Fatalf("history resultingCalories = %d, want 2500; entry=%+v", history[0].ResultingCalories, history[0])
	}
}
