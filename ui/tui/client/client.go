package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
)

var ErrUnauthorized = errors.New("unauthorized")

type Client struct {
	http       *http.Client
	baseURL    string
	jar        *cookiejar.Jar
	cookieFile string
}

type cookieRecord struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func New(baseURL, sessionDir string) *Client {
	_ = os.MkdirAll(sessionDir, 0o700)
	jar, _ := cookiejar.New(nil)
	c := &Client{
		baseURL:    baseURL,
		jar:        jar,
		cookieFile: filepath.Join(sessionDir, "cookies.json"),
		http:       &http.Client{Jar: jar},
	}
	_ = c.loadCookies()
	return c
}

func (c *Client) saveCookies() error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	cookies := c.jar.Cookies(u)
	var out []cookieRecord
	for _, ck := range cookies {
		if ck.Name == "access_token" || ck.Name == "refresh_token" {
			out = append(out, cookieRecord{Name: ck.Name, Value: ck.Value})
		}
	}
	if len(out) == 0 {
		// remove file if exists
		_ = os.Remove(c.cookieFile)
		return nil
	}
	data, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.cookieFile, data, 0o600); err != nil {
		return err
	}
	return nil
}

func (c *Client) loadCookies() error {
	data, err := os.ReadFile(c.cookieFile)
	if err != nil {
		return nil
	}
	var recs []cookieRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	var cookies []*http.Cookie
	for _, r := range recs {
		cookies = append(cookies, &http.Cookie{Name: r.Name, Value: r.Value, Path: "/"})
	}
	c.jar.SetCookies(u, cookies)
	return nil
}

func (c *Client) ClearSession() error {
	return os.Remove(c.cookieFile)
}

func (c *Client) do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		// try refresh
		rr, rerr := c.http.Post(c.baseURL+"/v1/auth/refresh", "application/json", nil)
		if rerr != nil || rr.StatusCode != http.StatusOK {
			if rr != nil {
				rr.Body.Close()
			}
			return nil, ErrUnauthorized
		}
		rr.Body.Close()
		_ = c.saveCookies()
		// retry original
		var retryReader io.Reader
		if body != nil {
			data, _ := json.Marshal(body)
			retryReader = bytes.NewReader(data)
		}
		req2, _ := http.NewRequest(method, c.baseURL+path, retryReader)
		if body != nil {
			req2.Header.Set("Content-Type", "application/json")
		}
		return c.http.Do(req2)
	}
	_ = c.saveCookies()
	return resp, nil
}

func decode(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var er struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&er)
		if er.Error != "" {
			return fmt.Errorf("%s", er.Error)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Data types

type NutritionLog struct {
	ID        string  `json:"id"`
	Date      string  `json:"date"`
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	WaterMl   float64 `json:"water_ml"`
	MealNotes string  `json:"meal_notes"`
}

type BiometricLog struct {
	Date           string  `json:"date"`
	WeightKg       float64 `json:"weight_kg"`
	WaistCm        float64 `json:"waist_cm"`
	GripKg         float64 `json:"grip_kg"`
	BoltScore      float64 `json:"bolt_score"`
	SleepHours     float64 `json:"sleep_hours"`
	SleepQuality   float64 `json:"sleep_quality"`
	SubjectiveFeel int     `json:"subjective_feel"`
	Notes          string  `json:"notes"`
}

type ExerciseSet struct {
	Reps        int     `json:"reps"`
	LoadKg      float64 `json:"load_kg"`
	TUTSeconds  float64 `json:"tut_seconds"`
	RestSeconds float64 `json:"rest_seconds"`
}

type ExerciseEntry struct {
	Name        string        `json:"name"`
	Category    string        `json:"category"`
	Sets        []ExerciseSet `json:"sets"`
	Notes       string        `json:"notes"`
	DistanceKm  float64       `json:"distance_km,omitempty"`
	ElevationM  float64       `json:"elevation_m,omitempty"`
	Pace        string        `json:"pace,omitempty"`
	RPE         float64       `json:"rpe,omitempty"`
	LoadRaw     string        `json:"load_raw,omitempty"`
	DurationRaw string        `json:"duration_raw,omitempty"`
	Tempo       string        `json:"tempo,omitempty"`
}

type WorkoutMetadata struct {
	Type    string   `json:"type"`
	Style   string   `json:"style"`
	Surface string   `json:"surface"`
	Focus   []string `json:"focus"`
	RPE     float64  `json:"rpe"`
	AvgHR   int      `json:"avg_hr"`
	MaxHR   int      `json:"max_hr"`
}

type WorkoutEntry struct {
	ID             string          `json:"id"`
	Date           string          `json:"date"`
	Slot           string          `json:"slot"`
	Title          string          `json:"title"`
	DurationMin    float64         `json:"duration_min"`
	CaloriesBurned float64         `json:"calories_burned"`
	MWV            float64         `json:"mwv"`
	NDS            float64         `json:"nds"`
	SessionDensity float64         `json:"session_density"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Metadata       WorkoutMetadata `json:"metadata"`
	RawNotes       string          `json:"raw_notes"`
}

type Profile struct {
	Name             string  `json:"name"`
	Age              int     `json:"age"`
	Sex              string  `json:"sex"`
	HeightCm         float64 `json:"height_cm"`
	Activity         string  `json:"activity"`
	ExerciseFreq     int     `json:"exercise_freq"`
	RunningKm        float64 `json:"running_km"`
	IsLifter         bool    `json:"is_lifter"`
	Goal             string  `json:"goal"`
	PrioritizeCarbs  bool    `json:"prioritize_carbs"`
	BfPct            float64 `json:"bf_pct"`
	HRRest           int     `json:"hr_rest"`
	HRMax            int     `json:"hr_max"`
	GripWeight       float64 `json:"grip_weight"`
	TDEELookbackDays int     `json:"tdee_lookback_days"`
	SleepQualityMax  float64 `json:"sleep_quality_max"`
	Units            string  `json:"units"`
}

type Targets struct {
	Calories        float64 `json:"calories"`
	ProteinG        float64 `json:"protein_g"`
	CarbsG          float64 `json:"carbs_g"`
	FatG            float64 `json:"fat_g"`
	FiberG          float64 `json:"fiber_g"`
	WaterMl         float64 `json:"water_ml"`
	EatBackExercise bool    `json:"eat_back_exercise"`
}

type BodyMeasurement struct {
	Date    string  `json:"date"`
	NeckCm  float64 `json:"neck_cm"`
	ChestCm float64 `json:"chest_cm"`
	WaistCm float64 `json:"waist_cm"`
	HipsCm  float64 `json:"hips_cm"`
	ThighCm float64 `json:"thigh_cm"`
	BicepCm float64 `json:"bicep_cm"`
	Notes   string  `json:"notes"`
}

type ParsedMeal struct {
	Calories  float64 `json:"calories"`
	ProteinG  float64 `json:"protein_g"`
	CarbsG    float64 `json:"carbs_g"`
	FatG      float64 `json:"fat_g"`
	FiberG    float64 `json:"fiber_g"`
	WaterMl   float64 `json:"water_ml"`
	MealNotes string  `json:"meal_notes"`
}

type ParsedWorkout struct {
	Title          string          `json:"title"`
	Slot           string          `json:"slot"`
	DurationMin    float64         `json:"duration_min"`
	CaloriesBurned float64         `json:"calories_burned"`
	Exercises      []ExerciseEntry `json:"exercises"`
	Notes          string          `json:"notes"`
	Type           string          `json:"type"`
	Style          string          `json:"style"`
	RPE            float64         `json:"rpe"`
}

type BodyFatResult struct {
	Method     string  `json:"method"`
	BfPct      float64 `json:"bf_pct"`
	LeanMassKg float64 `json:"lean_mass_kg"`
	FatMassKg  float64 `json:"fat_mass_kg"`
}

type TDEEResult struct {
	EstimatedTDEE float64 `json:"estimated_tdee"`
	ObservedTDEE  float64 `json:"observed_tdee"`
	Confidence    string  `json:"confidence"`
	DaysOfData    int     `json:"days_of_data"`
	Method        string  `json:"method"`
}

type ReadinessResult struct {
	Level         string   `json:"level"`
	Message       string   `json:"message"`
	Score         float64  `json:"score"`
	VelocityTrend string   `json:"velocity_trend"`
	VelocityDelta float64  `json:"velocity_delta"`
	GripZ         float64  `json:"grip_z"`
	BoltZ         float64  `json:"bolt_z"`
	Notes         []string `json:"notes"`
}

type WeeklyStats struct {
	AvgCalories   float64 `json:"avg_calories"`
	AvgProteinG   float64 `json:"avg_protein_g"`
	TotalWorkouts int     `json:"total_workouts"`
	TotalMWV      float64 `json:"total_mwv"`
	AvgSleepHours float64 `json:"avg_sleep_hours"`
	AvgWeightKg   float64 `json:"avg_weight_kg"`
}

type TodaySummary struct {
	Date         string       `json:"date"`
	Consumed     NutritionLog `json:"consumed"`
	Targets      Targets      `json:"targets"`
	CaloriesLeft float64      `json:"calories_left"`
	ProteinLeft  float64      `json:"protein_left"`
	ProgressPct  float64      `json:"progress_pct"`
}

type WeightPoint struct {
	Date     string  `json:"date"`
	WeightKg float64 `json:"weight_kg"`
}

type DashboardData struct {
	Today        TodaySummary    `json:"today"`
	TDEE         TDEEResult      `json:"tdee"`
	Readiness    ReadinessResult `json:"readiness"`
	WeeklyStats  WeeklyStats     `json:"weekly_stats"`
	WeightTrend  []WeightPoint   `json:"weight_trend"`
	TodayBio     *BiometricLog   `json:"today_bio"`
	WorkoutToday bool            `json:"workout_today"`
}

// --- Methods ---

func (c *Client) Login(username, password string) error {
	resp, err := c.do("POST", "/v1/auth/login", map[string]string{"login": username, "password": password})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Register(username, email, password string) error {
	resp, err := c.do("POST", "/v1/auth/register", map[string]string{"username": username, "email": email, "password": password})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Logout() error {
	resp, err := c.do("POST", "/v1/auth/logout", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	_ = c.ClearSession()
	return nil
}

func (c *Client) GetDashboard() (DashboardData, error) {
	var out DashboardData
	resp, err := c.do("GET", "/v1/dashboard", nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) ListNutritionLogs(from, to string) ([]NutritionLog, error) {
	resp, err := c.do("GET", "/v1/nutrition/logs?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []NutritionLog
	return out, decode(resp, &out)
}

func (c *Client) PostNutritionLog(log NutritionLog) error {
	resp, err := c.do("POST", "/v1/nutrition/logs", log)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ParseMeal(text string) (ParsedMeal, error) {
	var out ParsedMeal
	resp, err := c.do("POST", "/v1/parse/meal", map[string]string{"text": text})
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) PostBiometric(b BiometricLog) error {
	resp, err := c.do("POST", "/v1/biometrics", b)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListBiometrics(from, to string) ([]BiometricLog, error) {
	resp, err := c.do("GET", "/v1/biometrics?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []BiometricLog
	return out, decode(resp, &out)
}

func (c *Client) PostWorkout(w WorkoutEntry) error {
	resp, err := c.do("POST", "/v1/workouts", w)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListWorkouts(from, to string) ([]WorkoutEntry, error) {
	resp, err := c.do("GET", "/v1/workouts?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []WorkoutEntry
	return out, decode(resp, &out)
}

func (c *Client) GetProfile() (Profile, error) {
	var out Profile
	resp, err := c.do("GET", "/v1/profile", nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) UpdateProfile(p Profile) error {
	resp, err := c.do("PUT", "/v1/profile", p)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) PostMeasurement(m BodyMeasurement) error {
	resp, err := c.do("POST", "/v1/measurements", m)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListMeasurements(from, to string) ([]BodyMeasurement, error) {
	resp, err := c.do("GET", "/v1/measurements?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []BodyMeasurement
	return out, decode(resp, &out)
}

func (c *Client) ExportContent(kind, from, to, format string) (string, error) {
	// build URL with query params
	u := c.baseURL + "/v1/export/" + kind + "?from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to) + "&format=" + url.QueryEscape(format)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	var out struct {
		Content string `json:"content"`
	}
	if err := decode(resp, &out); err != nil {
		return "", err
	}
	return out.Content, nil
}

func (c *Client) GetBodyFat(method string) (BodyFatResult, error) {
	var out BodyFatResult
	resp, err := c.do("GET", "/v1/calc/bodyfat?method="+url.QueryEscape(method), nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) GetTargets() (Targets, error) {
	var out Targets
	resp, err := c.do("GET", "/v1/targets", nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) PutTargets(t Targets) error {
	resp, err := c.do("PUT", "/v1/targets", t)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetTDEE(days int) (TDEEResult, error) {
	var out TDEEResult
	path := "/v1/calc/tdee"
	if days > 0 {
		path += fmt.Sprintf("?days=%d", days)
	}
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) ParseWorkout(text, format string) (ParsedWorkout, error) {
	var out ParsedWorkout
	resp, err := c.do("POST", "/v1/parse/workout", map[string]string{"text": text, "format": format})
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) PutNutritionLog(date string, log NutritionLog) error {
	resp, err := c.do("PUT", "/v1/nutrition/logs/"+url.PathEscape(date), log)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) DeleteNutritionLog(date string) error {
	resp, err := c.do("DELETE", "/v1/nutrition/logs/"+url.PathEscape(date), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) DeleteWorkout(date, slot string) error {
	resp, err := c.do("DELETE", "/v1/workouts/"+url.PathEscape(date)+"/"+url.PathEscape(slot), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
