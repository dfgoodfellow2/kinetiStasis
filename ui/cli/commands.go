package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func emit(res *Response, jsonOut, rawText bool) error {
	if err := res.Err(); err != nil {
		return err
	}
	return printBody(os.Stdout, res.Body, jsonOut, rawText)
}

func cmdSimpleGet(client *Client, name, path string, args []string, jsonOut bool) error {
	fs := newFlagSet(name)
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	res, err := client.Get(path, nil)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func cmdRangeGet(client *Client, name, path string, args []string, jsonOut bool) error {
	fs := newFlagSet(name)
	from := fs.String("from", todayUTC(), "start date (YYYY-MM-DD), default today UTC")
	to := fs.String("to", todayUTC(), "end date (YYYY-MM-DD), default today UTC")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	query, err := rangeQuery(*from, *to)
	if err != nil {
		return err
	}
	res, err := client.Get(path, query)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func rangeQuery(from, to string) (url.Values, error) {
	if err := validateDate(from); err != nil {
		return nil, fmt.Errorf("--from: %w", err)
	}
	if err := validateDate(to); err != nil {
		return nil, fmt.Errorf("--to: %w", err)
	}
	if from > to {
		return nil, fmt.Errorf("--from (%s) must be on or before --to (%s)", from, to)
	}
	query := url.Values{}
	query.Set("from", from)
	query.Set("to", to)
	return query, nil
}

func cmdDashboard(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("dashboard")
	date := fs.String("date", todayUTC(), "date (YYYY-MM-DD), default today UTC")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if err := validateDate(*date); err != nil {
		return fmt.Errorf("--date: %w", err)
	}
	query := url.Values{}
	query.Set("date", *date)

	res, err := client.Get("/v1/dashboard", query)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func cmdExport(client *Client, args []string, jsonOut bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: dietctl export <combined|nutrition|workouts> [--from --to] [--format md|csv]")
	}
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Print(helpText)
		return errHelp
	}
	kind := args[0]
	switch kind {
	case "combined", "nutrition", "workouts":
	default:
		return fmt.Errorf("unknown export kind %q; want combined, nutrition or workouts", kind)
	}

	fs := newFlagSet("export")
	from := fs.String("from", todayUTC(), "start date (YYYY-MM-DD), default today UTC")
	to := fs.String("to", todayUTC(), "end date (YYYY-MM-DD), default today UTC")
	format := fs.String("format", "md", "export format: md or csv")
	if err := parseArgs(fs, args[1:]); err != nil {
		return err
	}
	if *format != "md" && *format != "csv" {
		return fmt.Errorf("--format must be md or csv, got %q", *format)
	}

	query, err := rangeQuery(*from, *to)
	if err != nil {
		return err
	}
	query.Set("format", *format)

	headers := map[string]string(nil)
	if jsonOut {
		// The export endpoints return a JSON wrapper when this header is set.
		headers = map[string]string{"Accept": "application/json"}
	}
	res, err := client.Do(http.MethodGet, "/v1/export/"+kind, query, nil, headers)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, !jsonOut)
}

func cmdLogin(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("login")
	loginFlag := fs.String("login", "", "username or email (or DIET_LOGIN)")
	passwordFlag := fs.String("password", "", "password (or DIET_PASSWORD)")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	login := *loginFlag
	if login == "" {
		login = os.Getenv("DIET_LOGIN")
	}
	if login == "" {
		prompted, err := promptLogin()
		if err != nil {
			return fmt.Errorf("read login: %w", err)
		}
		login = prompted
	}
	if login == "" {
		return fmt.Errorf("login is required")
	}

	password := *passwordFlag
	if password == "" {
		password = os.Getenv("DIET_PASSWORD")
	}
	if password == "" {
		prompted, err := promptPassword()
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		password = prompted
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}

	res, err := client.Post("/v1/auth/login", nil, map[string]string{
		"login":    login,
		"password": password,
	})
	if err != nil {
		return err
	}
	if err := res.Err(); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if err := client.SaveCookies(); err != nil {
		return fmt.Errorf("login succeeded but the session could not be persisted: %w", err)
	}
	return printBody(os.Stdout, res.Body, jsonOut, false)
}

func cmdLogout(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("logout")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	res, err := client.Post("/v1/auth/logout", nil, nil)
	// Always drop the local session, even if the server call fails.
	_ = client.ClearSession()
	if err != nil {
		return err
	}
	if err := res.Err(); err != nil {
		return fmt.Errorf("logout request failed (local session cleared): %w", err)
	}
	return printBody(os.Stdout, res.Body, jsonOut, false)
}

func cmdWorkoutAdd(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("workout-add")
	file := fs.String("file", "", "path to a workout YAML file (required)")
	dateFlag := fs.String("date", "", "must be today UTC if supplied")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("--file is required")
	}
	date, err := resolveWriteDate(*dateFlag)
	if err != nil {
		return err
	}

	text, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read workout file: %w", err)
	}
	if strings.TrimSpace(string(text)) == "" {
		return fmt.Errorf("workout file is empty")
	}

	parseRes, err := client.Post("/v1/parse/workout", nil, map[string]string{
		"text":   string(text),
		"format": "yaml",
	})
	if err != nil {
		return err
	}
	if err := parseRes.Err(); err != nil {
		if parseRes.StatusCode == http.StatusUnprocessableEntity {
			return fmt.Errorf("workout YAML parse failed (422): %s", apiMessage(parseRes))
		}
		return fmt.Errorf("workout parse failed: %w", err)
	}

	var parsed parsedWorkout
	if err := json.Unmarshal(parseRes.Body, &parsed); err != nil {
		return fmt.Errorf("decode parsed workout: %w", err)
	}

	entry := mapParsedWorkout(parsed, date)
	summary, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workout summary: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Mapped workout JSON (about to POST /v1/workouts):\n%s\n", summary)

	res, err := client.Post("/v1/workouts", nil, entry)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func cmdBiometricAdd(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("biometric-add")
	gripKg := fs.Float64("grip-kg", 0, "grip strength in kg")
	boltScore := fs.Float64("bolt-score", 0, "BOLT score")
	weightKg := fs.Float64("weight-kg", 0, "body weight in kg")
	waistCm := fs.Float64("waist-cm", 0, "waist circumference in cm")
	sleepHours := fs.Float64("sleep-hours", 0, "sleep duration in hours")
	notes := fs.String("notes", "", "free-form notes")
	dateFlag := fs.String("date", "", "must be today UTC if supplied")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	date, err := resolveWriteDate(*dateFlag)
	if err != nil {
		return err
	}
	provided := providedFlags(fs)

	body := map[string]any{"date": date}
	if err := addFloat(body, "gripKg", *gripKg, provided["grip-kg"]); err != nil {
		return err
	}
	if err := addFloat(body, "boltScore", *boltScore, provided["bolt-score"]); err != nil {
		return err
	}
	if err := addFloat(body, "weightKg", *weightKg, provided["weight-kg"]); err != nil {
		return err
	}
	if err := addFloat(body, "waistCm", *waistCm, provided["waist-cm"]); err != nil {
		return err
	}
	if err := addFloat(body, "sleepHours", *sleepHours, provided["sleep-hours"]); err != nil {
		return err
	}
	if provided["notes"] && *notes != "" {
		body["notes"] = *notes
	}
	if len(body) == 1 {
		return fmt.Errorf("no biometric fields provided; supply at least one of --grip-kg, --bolt-score, --weight-kg, --waist-cm, --sleep-hours, --notes")
	}

	res, err := client.Post("/v1/biometrics", nil, body)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func cmdNutritionAdd(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("nutrition-add")
	kcal := fs.Float64("kcal", 0, "calories")
	protein := fs.Float64("protein-g", 0, "protein grams")
	carbs := fs.Float64("carbs-g", 0, "carbohydrate grams")
	fat := fs.Float64("fat-g", 0, "fat grams")
	fiber := fs.Float64("fiber-g", 0, "fiber grams")
	water := fs.Float64("water-ml", 0, "water millilitres")
	note := fs.String("note", "", "meal note")
	dateFlag := fs.String("date", "", "must be today UTC if supplied")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	date, err := resolveWriteDate(*dateFlag)
	if err != nil {
		return err
	}
	provided := providedFlags(fs)

	body := map[string]any{"date": date}
	if err := addFloat(body, "calories", *kcal, provided["kcal"]); err != nil {
		return err
	}
	if err := addFloat(body, "proteinG", *protein, provided["protein-g"]); err != nil {
		return err
	}
	if err := addFloat(body, "carbsG", *carbs, provided["carbs-g"]); err != nil {
		return err
	}
	if err := addFloat(body, "fatG", *fat, provided["fat-g"]); err != nil {
		return err
	}
	if err := addFloat(body, "fiberG", *fiber, provided["fiber-g"]); err != nil {
		return err
	}
	if err := addFloat(body, "waterMl", *water, provided["water-ml"]); err != nil {
		return err
	}
	if provided["note"] && *note != "" {
		body["mealNotes"] = *note
	}
	if len(body) == 1 {
		return fmt.Errorf("no nutrition fields provided; supply at least one macro, --kcal, --water-ml or --note")
	}

	res, err := client.Post("/v1/nutrition/logs", nil, body)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func cmdMeasurementAdd(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("measurement-add")
	neck := fs.Float64("neck-cm", 0, "neck circumference in cm")
	chest := fs.Float64("chest-cm", 0, "chest circumference in cm")
	waist := fs.Float64("waist-cm", 0, "waist circumference in cm")
	hips := fs.Float64("hips-cm", 0, "hip circumference in cm")
	thigh := fs.Float64("thigh-cm", 0, "thigh circumference in cm")
	bicep := fs.Float64("bicep-cm", 0, "bicep circumference in cm")
	shoulders := fs.Float64("shoulders-cm", 0, "shoulder circumference in cm")
	calves := fs.Float64("calves-cm", 0, "calf circumference in cm")
	notes := fs.String("note", "", "free-form notes")
	dateFlag := fs.String("date", "", "must be today UTC if supplied")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	date, err := resolveWriteDate(*dateFlag)
	if err != nil {
		return err
	}
	provided := providedFlags(fs)

	body := map[string]any{"date": date}
	fields := []struct {
		flag string
		key  string
		val  float64
	}{
		{"neck-cm", "neckCm", *neck},
		{"chest-cm", "chestCm", *chest},
		{"waist-cm", "waistCm", *waist},
		{"hips-cm", "hipsCm", *hips},
		{"thigh-cm", "thighCm", *thigh},
		{"bicep-cm", "bicepCm", *bicep},
		{"shoulders-cm", "shouldersCm", *shoulders},
		{"calves-cm", "calvesCm", *calves},
	}
	for _, field := range fields {
		if err := addFloat(body, field.key, field.val, provided[field.flag]); err != nil {
			return err
		}
	}
	if provided["note"] && *notes != "" {
		body["notes"] = *notes
	}
	if len(body) == 1 {
		return fmt.Errorf("no measurement fields provided; supply at least one circumference or --note")
	}

	res, err := client.Post("/v1/measurements", nil, body)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func cmdGripWeight(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("grip-weight")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: dietctl grip-weight <0.0-1.0>")
	}
	value, err := strconv.ParseFloat(fs.Arg(0), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("grip weight must be a number between 0.0 and 1.0")
	}
	if value < 0 || value > 1 {
		return fmt.Errorf("grip weight must be between 0.0 and 1.0, got %v", value)
	}

	// Read the complete profile first. A partial body would zero every field
	// this CLI does not know about, so the full object is sent back.
	current, err := client.Get("/v1/profile", nil)
	if err != nil {
		return err
	}
	if err := current.Err(); err != nil {
		return fmt.Errorf("fetch profile: %w", err)
	}
	var profile map[string]any
	if err := json.Unmarshal(current.Body, &profile); err != nil {
		return fmt.Errorf("decode profile: %w", err)
	}
	if profile == nil {
		profile = make(map[string]any)
	}
	profile["gripWeight"] = value

	res, err := client.Do(http.MethodPut, "/v1/profile", nil, profile, nil)
	if err != nil {
		return err
	}
	return emit(res, jsonOut, false)
}

func cmdCheckin(client *Client, args []string, jsonOut bool) error {
	fs := newFlagSet("checkin")
	weightBefore := fs.Float64("weight-before", 0, "weight before the check-in week")
	weightAfter := fs.Float64("weight-after", 0, "weight after the check-in week")
	caloriesBefore := fs.Int("calories-before", 0, "calorie target before the check-in")
	caloriesAfter := fs.Int("calories-after", 0, "calorie target after the check-in (WARNING: >0 updates targets)")
	reason := fs.String("reason", "", "reason for the adjustment")
	dateFlag := fs.String("date", "", "must be today UTC if supplied")
	if err := parseArgs(fs, args); err != nil {
		return err
	}
	date, err := resolveWriteDate(*dateFlag)
	if err != nil {
		return err
	}
	if math.IsNaN(*weightBefore) || math.IsInf(*weightBefore, 0) ||
		math.IsNaN(*weightAfter) || math.IsInf(*weightAfter, 0) {
		return fmt.Errorf("weights must be finite numbers")
	}

	body := map[string]any{
		"checkInDate":    date,
		"weightBefore":   *weightBefore,
		"weightAfter":    *weightAfter,
		"caloriesBefore": *caloriesBefore,
		"caloriesAfter":  *caloriesAfter,
		"reason":         *reason,
	}

	res, err := client.Post("/v1/checkin", nil, body)
	if err != nil {
		return err
	}
	if err := res.Err(); err != nil {
		return fmt.Errorf("check-in rejected: %w", err)
	}
	if *caloriesAfter > 0 {
		fmt.Fprintf(os.Stderr, "note: caloriesAfter=%d also updated your calorie targets server-side.\n", *caloriesAfter)
	}
	return printBody(os.Stdout, res.Body, jsonOut, false)
}

func providedFlags(fs *flag.FlagSet) map[string]bool {
	provided := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})
	return provided
}

// addFloat sets key only when the flag was provided with a non-zero value, so
// the server's non-zero merge semantics are preserved.
func addFloat(body map[string]any, key string, value float64, provided bool) error {
	if !provided || value == 0 {
		return nil
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be a finite number", key)
	}
	body[key] = value
	return nil
}
