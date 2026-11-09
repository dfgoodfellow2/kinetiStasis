package api_test

// This suite pins the data-integrity semantics repaired in the Wave 1 data
// integrity work. It deliberately goes through the real HTTP stack
// (api.NewRouter + httptest) rather than calling handlers or store methods
// directly, so route wiring, cookie auth, decoding, validation and persistence
// are all exercised together.

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/api"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// testDate is fixed so assertions never depend on the wall clock. Each test
// gets a fresh database, so reusing the date is safe.
const testDate = "2026-10-20"

type harness struct {
	server *httptest.Server
	client *http.Client
	db     *sql.DB
}

// newHarness builds a migrated temp-file SQLite database and a real router,
// then registers the first user (auto-admin) and keeps both auth cookies in
// the client's jar for all protected calls.
func newHarness(t *testing.T) *harness {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	cfg := &config.Config{
		Port:      "0",
		Env:       "test",
		DBPath:    dbPath,
		JWTSecret: []byte("testsecret"),
	}
	srv := httptest.NewServer(api.NewRouter(cfg, store.NewSQLiteStore(database), nil))

	jar, err := cookiejar.New(nil)
	if err != nil {
		srv.Close()
		database.Close()
		t.Fatalf("create cookie jar: %v", err)
	}

	h := &harness{
		server: srv,
		client: &http.Client{Jar: jar},
		db:     database,
	}
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})

	h.do(t, http.MethodPost, "/v1/auth/register", map[string]any{
		"username": "integrity",
		"email":    "integrity@example.com",
		"password": "supersecret123",
	}, http.StatusCreated)

	return h
}

// do sends a JSON request and returns the response body, failing the test if
// the status code is not the expected one.
func (h *harness) do(t *testing.T, method, path string, payload any, wantStatus int) []byte {
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

func decodeJSON[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode response %s: %v", data, err)
	}
	return v
}

// 1. PUT is a full replace. The Wave 1 bug routed edits through the additive
// POST semantics, turning a 500 kcal day into 1100 kcal after editing to 600.
func TestNutritionPUTReplacesRatherThanAdds(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/nutrition/logs", map[string]any{
		"date":     testDate,
		"calories": 500,
		"proteinG": 40,
	}, http.StatusCreated)

	h.do(t, http.MethodPut, "/v1/nutrition/logs/"+testDate, map[string]any{
		"calories": 600,
		"proteinG": 50,
	}, http.StatusOK)

	got := decodeJSON[models.NutritionLog](t, h.do(t, http.MethodGet, "/v1/nutrition/logs/"+testDate, nil, http.StatusOK))
	if got.Calories != 600 {
		t.Fatalf("calories after PUT = %v, want 600 (1100 indicates additive merge regression)", got.Calories)
	}
	if got.ProteinG != 50 {
		t.Fatalf("proteinG after PUT = %v, want 50", got.ProteinG)
	}
}

// 2. POST remains additive: the documented quick-add semantics must survive
// the PUT fix.
func TestNutritionPOSTStillAdditivelyMerges(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/nutrition/logs", map[string]any{
		"date":     testDate,
		"calories": 500,
		"proteinG": 40,
	}, http.StatusCreated)

	h.do(t, http.MethodPost, "/v1/nutrition/logs", map[string]any{
		"date":     testDate,
		"calories": 250,
		"proteinG": 20,
	}, http.StatusOK)

	got := decodeJSON[models.NutritionLog](t, h.do(t, http.MethodGet, "/v1/nutrition/logs/"+testDate, nil, http.StatusOK))
	if got.Calories != 750 {
		t.Fatalf("calories after second POST = %v, want 750 (additive merge)", got.Calories)
	}
	if got.ProteinG != 60 {
		t.Fatalf("proteinG after second POST = %v, want 60 (additive merge)", got.ProteinG)
	}
}

// 3. A partial biometrics POST (e.g. grip only at night) must not wipe the
// weight captured in the morning.
func TestBiometricPOSTBlankFieldsPreserveSameDayValues(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/biometrics", map[string]any{
		"date":     testDate,
		"weightKg": 80.2,
	}, http.StatusOK)

	h.do(t, http.MethodPost, "/v1/biometrics", map[string]any{
		"date":   testDate,
		"gripKg": 55,
	}, http.StatusOK)

	got := decodeJSON[models.BiometricLog](t, h.do(t, http.MethodGet, "/v1/biometrics/"+testDate, nil, http.StatusOK))
	if got.WeightKg != 80.2 {
		t.Fatalf("weightKg after grip-only POST = %v, want 80.2 (must be preserved)", got.WeightKg)
	}
	if got.GripKg != 55 {
		t.Fatalf("gripKg after grip-only POST = %v, want 55", got.GripKg)
	}
}

// 4. Measurements have an intentional asymmetry: POST merges (preserve on
// zero) while PUT is a true replace that clears omitted/blanked fields.
func TestMeasurementPOSTPreservesButPUTReplaces(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"date":    testDate,
		"waistCm": 90,
		"neckCm":  40,
	}, http.StatusOK)

	// Partial POST: neck is zero in the payload and must survive.
	h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"date":    testDate,
		"waistCm": 91,
	}, http.StatusOK)

	listPath := "/v1/measurements?from=" + testDate + "&to=" + testDate
	afterPost := decodeJSON[[]models.BodyMeasurement](t, h.do(t, http.MethodGet, listPath, nil, http.StatusOK))
	if len(afterPost) != 1 {
		t.Fatalf("measurements after POST = %d rows, want 1", len(afterPost))
	}
	if afterPost[0].WaistCm != 91 || afterPost[0].NeckCm != 40 {
		t.Fatalf("after partial POST: waist=%v neck=%v, want waist=91 neck=40 (preserve on zero)", afterPost[0].WaistCm, afterPost[0].NeckCm)
	}

	// True replace: the omitted neck must now be cleared.
	h.do(t, http.MethodPut, "/v1/measurements/"+testDate, map[string]any{
		"waistCm": 92,
	}, http.StatusOK)

	afterPut := decodeJSON[[]models.BodyMeasurement](t, h.do(t, http.MethodGet, listPath, nil, http.StatusOK))
	if len(afterPut) != 1 {
		t.Fatalf("measurements after PUT = %d rows, want 1", len(afterPut))
	}
	if afterPut[0].WaistCm != 92 {
		t.Fatalf("waist after PUT = %v, want 92", afterPut[0].WaistCm)
	}
	if afterPut[0].NeckCm != 0 {
		t.Fatalf("neck after PUT = %v, want 0 (PUT is a true replace)", afterPut[0].NeckCm)
	}
}

// 5. A water-only target edit must be detected and snapshot the water column.
// target_history stores the superseded row (see ui/web/src/pages/Targets.svelte),
// so the newest snapshot carries the water value that was live before the edit;
// the current GET reflects the new value. Before the Wave 1 fix, water-only
// edits were invisible to change detection and the snapshot row either was not
// created or had water_ml = 0.
func TestTargetsWaterOnlyChangeSnapshotsWater(t *testing.T) {
	h := newHarness(t)

	base := map[string]any{
		"calories": 2000,
		"proteinG": 150,
		"carbsG":   200,
		"fatG":     70,
		"fiberG":   30,
		"waterMl":  2500,
	}
	h.do(t, http.MethodPut, "/v1/targets", base, http.StatusOK)

	waterOnly := map[string]any{
		"calories": 2000,
		"proteinG": 150,
		"carbsG":   200,
		"fatG":     70,
		"fiberG":   30,
		"waterMl":  3000,
	}
	h.do(t, http.MethodPut, "/v1/targets", waterOnly, http.StatusOK)

	current := decodeJSON[models.Targets](t, h.do(t, http.MethodGet, "/v1/targets", nil, http.StatusOK))
	if current.WaterMl != 3000 {
		t.Fatalf("current waterMl = %v, want 3000", current.WaterMl)
	}

	history := decodeJSON[[]models.TargetSnapshot](t, h.do(t, http.MethodGet, "/v1/targets/history", nil, http.StatusOK))
	if len(history) != 1 {
		t.Fatalf("target history rows = %d, want 1 (water-only edit must snapshot)", len(history))
	}
	if history[0].WaterMl != 2500 {
		t.Fatalf("snapshot waterMl = %v, want 2500 (superseded water value must round-trip, not be zero)", history[0].WaterMl)
	}
}

// 6. Validation must reject non-physical or malformed input at the HTTP edge.
func TestValidationRejectsInvalidPayloads(t *testing.T) {
	h := newHarness(t)

	cases := []struct {
		name    string
		method  string
		path    string
		payload map[string]any
	}{
		{
			name:    "nutrition negative calories",
			method:  http.MethodPost,
			path:    "/v1/nutrition/logs",
			payload: map[string]any{"date": testDate, "calories": -1},
		},
		{
			name:    "nutrition malformed date",
			method:  http.MethodPost,
			path:    "/v1/nutrition/logs",
			payload: map[string]any{"date": "2026-1-2", "calories": 100},
		},
		{
			name:    "workout negative duration",
			method:  http.MethodPost,
			path:    "/v1/workouts",
			payload: map[string]any{"date": testDate, "slot": "am", "durationMin": -5},
		},
		{
			name:    "targets negative protein",
			method:  http.MethodPut,
			path:    "/v1/targets",
			payload: map[string]any{"calories": 2000, "proteinG": -5},
		},
		{
			name:    "parse meal without text or image",
			method:  http.MethodPost,
			path:    "/v1/parse/meal",
			payload: map[string]any{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h.do(t, tc.method, tc.path, tc.payload, http.StatusBadRequest)
		})
	}
}

// --- Admin data-integrity report -------------------------------------------
//
// These tests exercise GET /v1/admin/integrity end to end. Anomalies are seeded
// through the public API where the handlers allow the value, and through raw
// SQL where a past bug could write something the current validators reject
// (NULL leftovers, a 300 cm circumference).

type integrityFinding struct {
	Check   string           `json:"check"`
	Count   int              `json:"count"`
	Samples []map[string]any `json:"samples"`
}

type integrityReport struct {
	GeneratedAt string             `json:"generatedAt"`
	Results     []integrityFinding `json:"results"`
	Errors      []string           `json:"errors"`
}

func findIntegrityFinding(t *testing.T, report integrityReport, check string) integrityFinding {
	t.Helper()
	for _, finding := range report.Results {
		if finding.Check == check {
			return finding
		}
	}
	t.Fatalf("integrity report is missing check %q (got %d results)", check, len(report.Results))
	return integrityFinding{}
}

func adminUserID(t *testing.T, h *harness) string {
	t.Helper()
	var id string
	if err := h.db.QueryRow(`SELECT id FROM users ORDER BY created_at ASC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("read admin user id: %v", err)
	}
	return id
}

func TestIntegrityReportFlagsAnomalies(t *testing.T) {
	h := newHarness(t)
	userID := adminUserID(t, h)

	// A doubled day: 4600 kcal stored while the macros imply ~2210 kcal, and
	// the same meal line twice in the notes.
	h.do(t, http.MethodPost, "/v1/nutrition/logs", map[string]any{
		"date":      testDate,
		"calories":  4600,
		"proteinG":  150,
		"carbsG":    200,
		"fatG":      90,
		"mealNotes": "Chicken and rice bowl\nChicken and rice bowl",
	}, http.StatusCreated)

	// A NULL leftover that migrations 013/014 would have normalised. This row's
	// macros match its calories so it does not add a second mismatch finding.
	if _, err := h.db.Exec(`
        INSERT INTO nutrition_logs
          (id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		"null-leftover", userID, "2026-10-10", 2000, 200, 150, 66.7, 30, 2000, nil,
		"2026-10-10T00:00:00.000000000Z"); err != nil {
		t.Fatalf("seed NULL leftover: %v", err)
	}

	// A unit-corrupted circumference the current validator (max 250 cm) would
	// reject, so it must be seeded directly.
	if _, err := h.db.Exec(`
        INSERT INTO body_measurements
          (id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"bad-measurement", userID, "2026-10-18", 40, 100, 300, 100, 60, 35, "",
		"2026-10-18T00:00:00.000000000Z", 120, 40); err != nil {
		t.Fatalf("seed implausible measurement: %v", err)
	}

	report := decodeJSON[integrityReport](t, h.do(t, http.MethodGet, "/v1/admin/integrity", nil, http.StatusOK))

	if len(report.Errors) != 0 {
		t.Fatalf("integrity report errors = %v, want none", report.Errors)
	}
	if report.GeneratedAt == "" {
		t.Fatal("generatedAt is empty")
	}
	if len(report.Results) != 10 {
		t.Fatalf("integrity results = %d checks, want 10", len(report.Results))
	}

	cases := []struct {
		check string
		want  int
	}{
		{"nutrition_macro_mismatch", 1},
		{"nutrition_doubled_notes", 1},
		{"nutrition_implausible", 0},
		{"null_leftovers", 1},
		{"measurement_implausible", 1},
		{"biometric_implausible", 0},
	}
	for _, tc := range cases {
		finding := findIntegrityFinding(t, report, tc.check)
		if finding.Count != tc.want {
			t.Errorf("%s count = %d, want %d (samples=%v)", tc.check, finding.Count, tc.want, finding.Samples)
		}
		if len(finding.Samples) > 5 {
			t.Errorf("%s returned %d samples, want <= 5", tc.check, len(finding.Samples))
		}
	}

	// Evidence must identify the member and date without leaking credentials.
	macro := findIntegrityFinding(t, report, "nutrition_macro_mismatch")
	if len(macro.Samples) != 1 {
		t.Fatalf("macro samples = %d, want 1", len(macro.Samples))
	}
	if macro.Samples[0]["username"] != "integrity" {
		t.Errorf("macro sample username = %v, want integrity", macro.Samples[0]["username"])
	}
	if macro.Samples[0]["date"] != testDate {
		t.Errorf("macro sample date = %v, want %s", macro.Samples[0]["date"], testDate)
	}
	for _, forbidden := range []string{"email", "password", "token", "hash"} {
		if _, ok := macro.Samples[0][forbidden]; ok {
			t.Errorf("macro sample leaked %q column", forbidden)
		}
	}
}

func TestIntegrityReportCleanData(t *testing.T) {
	h := newHarness(t)

	// A sane day: calories agree with macros within tolerance.
	h.do(t, http.MethodPost, "/v1/nutrition/logs", map[string]any{
		"date":     testDate,
		"calories": 2000,
		"proteinG": 150,
		"carbsG":   200,
		"fatG":     66.7,
		"fiberG":   30,
		"waterMl":  2000,
	}, http.StatusCreated)

	h.do(t, http.MethodPost, "/v1/measurements", map[string]any{
		"date":    testDate,
		"waistCm": 90,
		"neckCm":  40,
	}, http.StatusOK)

	h.do(t, http.MethodPost, "/v1/biometrics", map[string]any{
		"date":       testDate,
		"weightKg":   80,
		"gripKg":     50,
		"boltScore":  30,
		"sleepHours": 7.5,
	}, http.StatusOK)

	report := decodeJSON[integrityReport](t, h.do(t, http.MethodGet, "/v1/admin/integrity", nil, http.StatusOK))
	if len(report.Errors) != 0 {
		t.Fatalf("integrity report errors = %v, want none", report.Errors)
	}

	for _, check := range []string{
		"nutrition_macro_mismatch",
		"nutrition_doubled_notes",
		"nutrition_implausible",
		"measurement_implausible",
		"biometric_implausible",
		"null_leftovers",
		"target_history_implausible",
		"checkin_raise_era",
		"duplicate_same_day_snapshots",
	} {
		finding := findIntegrityFinding(t, report, check)
		if finding.Count != 0 {
			t.Errorf("%s count = %d on clean data, want 0 (samples=%v)", check, finding.Count, finding.Samples)
		}
	}
}
