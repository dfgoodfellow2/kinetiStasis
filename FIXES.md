STATUS

All fixes tracked below. Mark each as done when completed.

- [ ] FIX-001: Create internal/constants/constants.go
- [ ] FIX-002: Fix swallowed errors
- [ ] FIX-003: Refactor duplicated Gemini call logic
- [ ] FIX-004: Fix export field names + unit-aware output
- [ ] FIX-005: fetchWorkouts must select metadata_json
- [ ] FIX-006: Stop double-counting sex adjustment in BMR
- [ ] FIX-007: Remove duplicate respond package (internal/api/respond.go)
- [ ] FIX-008: Fix ratelimit ticker goroutine leak
- [ ] FIX-009: Remove dead code: computeEMASeries

OVERVIEW

This file is a working reference for an AI coding agent. Each section below (FIX-001 ... FIX-009)
contains precise change instructions, code snippets to add or replace, exact file paths, imports
required, and tests/verification steps. Each section is intentionally self-contained so the agent
can make the change without re-reading the entire repository.

------------------------------------------------------------

FIX-001: Create internal/constants/constants.go
------------------------------------------------------------
Goal: Centralize project-wide constants so handlers/services don't duplicate them.

Create file: internal/constants/constants.go

Contents (exact):

package constants

import "time"

const (
    DefaultTDEELookbackDays      = 90
    DefaultReadinessLookbackDays = 30
    DefaultExportLookbackDays    = 30
    MinCalorieFloor              = 1200.0
    ReadinessEMAAlpha            = 0.3
    DateFormat                   = "2006-01-02"
)

// MaxRequestBodyBytes is intentionally an int64 for consistent use with request length checks
const MaxRequestBodyBytes int64 = 10 << 20 // 10 MiB

// TimeFormat uses time.RFC3339 to remain consistent across handlers
var TimeFormat = time.RFC3339

Notes:
- Use this package by importing github.com/dfgoodfellow2/diet-tracker/v2/internal/constants
- Update all the files listed below to use these constants instead of hard-coded values:
  - internal/handlers/calculations.go (lines referencing 90 and 30 lookbacks and DateFormat)
  - internal/handlers/profile.go
  - internal/handlers/export.go
  - internal/services/calculator/adaptive.go (MinCalorieFloor)
  - internal/services/readiness/readiness.go (ReadinessEMAAlpha)
  - internal/respond/respond.go (MaxRequestBodyBytes)

Replacement pattern examples (exact snippets to apply):

// OLD: DateFormat used as string literal elsewhere
// "2006-01-02"

// NEW: use constants.DateFormat
import "github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
...
dateStr := t.Format(constants.DateFormat)

// OLD: MaxRequestBodyBytes used directly as literal
// NEW:
var max int64 = constants.MaxRequestBodyBytes

Verification:
- Run `go build ./...` to ensure no missing symbols.

------------------------------------------------------------

FIX-002: Fix swallowed errors
------------------------------------------------------------
Goal: Do not ignore DB/scan errors. Log and (when appropriate) return 500.

Files and exact changes:

1) internal/handlers/auth.go
- Replace the current profile INSERT which ignores error with the following pattern (lines ~79):

// CURRENT (do not leave):
// _, _ = h.db.ExecContext(r.Context(),
//     `INSERT INTO profiles (user_id, updated_at) VALUES (?, ?)`,
//     userID, now,
// )

// REPLACE WITH:
if _, err := h.db.ExecContext(r.Context(),
    `INSERT INTO profiles (user_id, updated_at) VALUES (?, ?)`,
    userID, now,
); err != nil {
    // non-fatal during registration; log so we can debug later
    slog.Error("create profile", "user_id", userID, "err", err)
}

Ensure import: add or keep
import "log/slog"

2) internal/handlers/profile.go (around line ~96)
- Replace the silent upsert of targets with:

if _, err := h.db.ExecContext(r.Context(), `INSERT INTO targets ...`, /* args */); err != nil {
    slog.Warn("auto-compute targets upsert failed", "user_id", claims.UserID, "err", err)
}

Note: keep the operation non-fatal so user still gets profile saved. If the existing code uses a dynamic SQL string, keep args identical; only capture and log the error.

3) internal/handlers/nutrition.go (around line ~163)
- Previously the code re-fetched the row and ignored the scan error. Replace with:

if err := h.db.QueryRowContext(ctx, `SELECT ... FROM ... WHERE id = ?`, id).Scan(&out.Field1, &out.Field2 /* etc */); err != nil {
    slog.Error("re-fetch nutrition after update", "id", id, "err", err)
    respond.Error(w, http.StatusInternalServerError, "internal error")
    return
}
respond.JSON(w, http.StatusOK, out)

4) internal/handlers/biometric.go (around line ~110)
- Same as nutrition.go: check the error returned by Scan, log, and return 500 via respond.Error when non-nil.

5) internal/handlers/workouts.go (around line ~167)
- Replace:
// _ = scanWorkout(row, &out)
// respond.JSON(w, http.StatusOK, out)

// WITH:
if err := scanWorkout(row, &out); err != nil {
    slog.Error("scan workout failed", "err", err)
    respond.Error(w, http.StatusInternalServerError, "internal error")
    return
}
respond.JSON(w, http.StatusOK, out)

Notes:
- Add slog import where missing: import "log/slog"
- Use existing respond package (internal/respond/respond.go). If the file currently imports a router-local respond, see FIX-007.

Verification:
- Unit tests / integration tests that simulate DB failures should now surface 500 responses instead of returning zeroed payloads.

------------------------------------------------------------

FIX-003: Refactor duplicated Gemini call logic
------------------------------------------------------------
Goal: Remove duplication between ParseMeal and ParseWorkout in internal/services/gemini/gemini.go

Changes to make (within package gemini):

1) Add helper function callGemini

// callGemini creates a genai client, runs GenerateContent for the supplied prompt,
// collects textual parts and returns a single concatenated string. Implements one retry
// on 429/503 (sleep 2s then retry once). Returns the raw response (possibly multi-part).
func callGemini(ctx context.Context, apiKey, prompt string) (string, error) {
    client := genai.NewClient(apiKey)
    model := genai.Model{ID: "gemini-1.0"} // adapt to whatever the project used

    // wrapper to perform a single call
    do := func() (string, int, error) {
        resp, err := client.GenerateContent(ctx, genai.GenerateContentParams{
            Model: model,
            Input: prompt,
        })
        if err != nil {
            return "", 0, err
        }
        var full string
        for _, part := range resp.Candidates {
            full += part.Content[0].Text
        }
        // derive a status code if available in error/response; return 0 otherwise
        return full, 0, nil
    }

    out, status, err := do()
    if err != nil && (status == 429 || status == 503) {
        time.Sleep(2 * time.Second)
        out, status, err = do()
    }
    if err != nil {
        return "", err
    }
    return out, nil
}

2) Add helper function cleanAndExtractJSON

// cleanAndExtractJSON strips markdown code fences (```json or ```)
// and returns the first {...} JSON object by counting braces. If no
// braces are found, it returns the trimmed input.
func cleanAndExtractJSON(s string) string {
    s = strings.TrimSpace(s)
    if strings.HasPrefix(s, "```json") {
        s = strings.TrimPrefix(s, "```json")
    }
    if strings.HasPrefix(s, "```") {
        s = strings.TrimPrefix(s, "```")
    }
    s = strings.TrimSpace(s)

    // find first balanced brace object
    start := -1
    depth := 0
    for i, r := range s {
        if r == '{' {
            if start == -1 {
                start = i
            }
            depth++
        }
        if r == '}' {
            depth--
            if depth == 0 && start != -1 {
                return strings.TrimSpace(s[start : i+1])
            }
        }
    }
    return strings.TrimSpace(s)
}

3) Replace in ParseMeal and ParseWorkout the duplicated sections with calls to these two helpers:

raw, err := callGemini(ctx, apiKey, prompt)
if err != nil {
    return nil, err
}
clean := cleanAndExtractJSON(raw)
// then unmarshal clean into the expected struct

Notes:
- Ensure imports include time and strings as needed.
- Keep the public ParseMeal/ParseWorkout signatures unchanged; they should now call the helpers.

Verification:
- Build and run unit tests for the gemini package. Compare outputs from sample prompts to ensure identical behavior.

------------------------------------------------------------

FIX-004: Fix internal/services/export/export.go — field name errors + unit-aware output
------------------------------------------------------------
Goal: Correct non-existent field usages and make export output respect profile.Units.

Summary of concrete changes:

1) Replace incorrect field names:
- Replace bio.WeightLbs -> bio.WeightKg
- Replace s.LoadLbs -> s.LoadKg

2) Change function signatures to accept a units string:
- CombinedMarkdown(logs, biometrics, workouts, targets, from, to string) -> add units string
- WorkoutsMarkdown(workouts, from, to string) -> add units string
- WorkoutsCSV(workouts, from, to string) -> add units string

3) Use internal/services/units converters when units == "imperial".
Import:
import "github.com/dfgoodfellow2/diet-tracker/v2/internal/services/units"

Suggested helper functions inside export.go (exact style optional):

func displayWeight(weightKg float64, units string) (float64, string) {
    if units == "imperial" {
        return weightKg * units.KgToLbs, "lbs"
    }
    return weightKg, "kg"
}

func displayLoad(loadKg float64, units string) (float64, string) {
    if units == "imperial" {
        return loadKg * units.KgToLbs, "lbs"
    }
    return loadKg, "kg"
}

func displayWaist(waistCm float64, units string) (float64, string) {
    if units == "imperial" {
        return waistCm * units.CmToInch, "in"
    }
    return waistCm, "cm"
}

4) Update usage examples inside CombinedMarkdown and WorkoutsMarkdown/CSV to call these helpers and use the returned value and label. For example:

// when formatting a biometric
val, unitLabel := displayWeight(bio.WeightKg, units)
fmt.Sprintf("Weight: %.1f %s", val, unitLabel)

5) Update callers in internal/handlers/export.go

Handler must:
- Fetch profile using existing fetchProfile(ctx, db, userID) (from internal/handlers/calculations.go)
- Extract profile.Units
- Call export.CombinedMarkdown(..., profile.Units) or export.WorkoutsCSV(..., profile.Units)

Example handler snippet (replace appropriate section):

profile, err := fetchProfile(ctx, h.db, claims.UserID)
if err != nil {
    slog.Error("fetch profile for export", "user_id", claims.UserID, "err", err)
    respond.Error(w, http.StatusInternalServerError, "internal error")
    return
}
md := export.CombinedMarkdown(logs, biometrics, workouts, targets, from, to, profile.Units)

Notes:
- The units conversion constants are in internal/services/units/convert.go; import the package path above.
- Ensure the CSV header for load reflects the unit label ("Load (kg)" or "Load (lbs)").

Verification:
- Exporting with a profile that has Units == "imperial" should show lbs/in values; Units == "metric" should show kg/cm.
- Fix the compile errors caused by the earlier invalid field names.

------------------------------------------------------------

FIX-005: Fix internal/handlers/calculations.go — fetchWorkouts missing metadata_json
------------------------------------------------------------
Goal: Ensure fetchWorkouts selects and decodes metadata_json into WorkoutEntry.Metadata

File: internal/handlers/calculations.go

Change the SELECT column list to include metadata_json (14 columns instead of 13). Use this exact SELECT order in the rows.Scan call.

Replace the rows.Scan and unmarshalling block with the following (exact):

var exercisesJSON, metadataJSON string
if err := rows.Scan(&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes,
    &w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity,
    &exercisesJSON, &metadataJSON, &w.UpdatedAt); err != nil {
    return nil, err
}
if exercisesJSON != "" {
    _ = json.Unmarshal([]byte(exercisesJSON), &w.Exercises)
}
if metadataJSON != "" {
    _ = json.Unmarshal([]byte(metadataJSON), &w.Metadata)
}

Also update the original SQL query string to select metadata_json before updated_at:

..., exercises_json, metadata_json, updated_at

Verification:
- Build and run any analytics/dashboard code that reads WorkoutEntry.Metadata; it should now be populated when metadata_json exists in DB rows.

------------------------------------------------------------

FIX-006: Fix AdjustBMRForSex double-counting in internal/services/nutrition/macros.go
------------------------------------------------------------
Goal: Prevent applying an extra sex-based modifier on top of the Mifflin estimation.

Change (exact replacement recommended) in ComputeBMR function:

// ORIGINAL (problem):
// bmr := EstimateBMRMifflin(...)
// bmr = AdjustBMRForSex(bmr, p.Sex)

// NEW (exact):
func ComputeBMR(p models.Profile, weightKg float64) float64 {
    isMale := p.Sex == "male"
    if p.BfPct > 0 {
        bmr := EstimateBMRKatchMcArdle(weightKg, p.BfPct)
        if bmr <= 0 {
            return EstimateBMRMifflin(weightKg, p.HeightCm, p.Age, isMale)
        }
        return bmr
    }
    return EstimateBMRMifflin(weightKg, p.HeightCm, p.Age, isMale)
}

Additional guidance:
- If AdjustBMRForSex is not used by any other code after this change, remove it or mark it deprecated. Run `grep -R "AdjustBMRForSex" -n .` to confirm.

Verification:
- ComputeBMR for female profiles should now match expected Mifflin outputs and not include an extra ~5% reduction.

------------------------------------------------------------

FIX-007: Merge duplicate respond packages
------------------------------------------------------------
Goal: Remove duplicate file internal/api/respond.go and ensure all code imports internal/respond

1) Delete file: internal/api/respond.go

2) Update internal/api/router.go
- Add import for respond:
import "github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"

- Replace any direct calls to JSON(...) or Error(...) (which were previously in package api) with respond.JSON(...) and respond.Error(...). If router.go did not call these functions directly, just adding the import is OK.

Notes:
- The canonical respond package lives at internal/respond/respond.go (package respond). All handlers already import internal/respond — confirm and update any remaining references.

Verification:
- go build ./... should succeed. Running grep for duplicate respond.go should show only internal/respond/respond.go.

------------------------------------------------------------

FIX-008: Fix ratelimit goroutine leak in internal/middleware/ratelimit.go
------------------------------------------------------------
Goal: Allow the cleanup goroutine to stop, preventing a goroutine/ticker leak.

Changes to make:

1) Add stop channel to type rateLimitStore (file internal/middleware/ratelimit.go):

type rateLimitStore struct {
    mu   sync.Mutex
    data map[string]rateLimitEntry
    stop chan struct{}
}

2) When creating the store, initialize stop and start the goroutine that respects it. If there is a constructor newRateLimitStore, change it to:

func newRateLimitStore() *rateLimitStore {
    s := &rateLimitStore{
        data: make(map[string]rateLimitEntry),
        stop: make(chan struct{}),
    }

    go func() {
        ticker := time.NewTicker(2 * time.Minute)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                // cleanup logic (unchanged)
            case <-s.stop:
                return
            }
        }
    }()
    return s
}

3) Provide a Close or Stop method on rateLimitStore to signal shutdown:

func (s *rateLimitStore) Close() {
    close(s.stop)
}

4) Ensure the application shutdown path (if any) calls Close on the store. If no explicit shutdown path exists, adding Close is still valuable for future tests and prevents leaks when tests create/destroy stores.

Verification:
- No running goroutine/ticker remains after calling Close() in tests. Use `pprof` or runtime.NumGoroutine() in tests to validate.

------------------------------------------------------------

FIX-009: Remove dead code
------------------------------------------------------------
Goal: Remove an unused internal helper and document TUI stub and empty dirs.

1) internal/services/metrics/summary.go
- Remove the unexported function computeEMASeries entirely. Before deletion, verify it's unused:
  grep -R "computeEMASeries" -n .
- After deletion, build to ensure nothing breaks.

2) ui/tui/screens/workout_history_stub.go
- Do NOT delete. This file is an intentional stub to satisfy references in the TUI. Add or keep a short comment at top indicating the file is a deliberate stub.

3) internal/db/generated/ and internal/db/queries/
- These directories contain only .gitkeep and are from an abandoned sqlc integration. Do NOT delete them now. Documented here for future cleanup.

Verification:
- go build ./... should continue to succeed. Grep should not find computeEMASeries any more.

------------------------------------------------------------

COMMON VERIFICATION STEPS (after finishing all changes):

1) go vet ./...
2) go build ./...
3) Run existing unit tests (if any): go test ./... and fix failing tests due to intended behavior changes
4) Manual smoke tests:
   - Log in / register path for FIX-002 ensure it doesn't fail on non-fatal profile insert errors
   - Export endpoint (FIX-004) for a profile with Units="imperial" to confirm conversion
   - Workout fetching for a user with metadata_json present to verify FIX-005

------------------------------------------------------------

CONTACT / NOTES

If any change requires further clarification (for example, exact SQL strings in profile upsert, or which model ID to use when instantiating the Gemini API model), ask before making assumptions. This document intentionally avoids guessing those small implementation details.

Keep each commit focused: one fix per commit. When committing, include file list in the commit message (e.g., "FIX-004: export unit-aware output; update handlers and export service").
