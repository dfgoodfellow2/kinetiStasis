package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/models"
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

// Backwards-compatible type aliases pointing to the models package.
// Other packages may still reference client.<Type>, so keep aliases here.
type NutritionLog = models.NutritionLog
type BiometricLog = models.BiometricLog
type ExerciseSet = models.ExerciseSet
type ExerciseEntry = models.ExerciseEntry
type WorkoutMetadata = models.WorkoutMetadata
type WorkoutEntry = models.WorkoutEntry
type Profile = models.Profile
type Targets = models.Targets
type BodyMeasurement = models.BodyMeasurement
type ParsedMeal = models.ParsedMeal
type ParsedWorkout = models.ParsedWorkout
type BodyFatResult = models.BodyFatResult
type TDEEResult = models.TDEEResult
type ReadinessResult = models.ReadinessResult
type WeeklyStats = models.WeeklyStats
type TodaySummary = models.TodaySummary
type WeightPoint = models.WeightPoint
type DashboardData = models.DashboardData

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

// checkHTTPResponse checks for HTTP error status and returns a formatted error.
// Returns nil if status is < 400.
func checkHTTPResponse(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		return checkHTTPResponse(resp)
	}
	return nil
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
		return checkHTTPResponse(resp)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Domain models moved to ui/tui/models

// --- Methods ---

func (c *Client) Login(username, password string) error {
	resp, err := c.do("POST", "/v1/auth/login", map[string]string{"login": username, "password": password})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return checkHTTPResponse(resp)
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
		return checkHTTPResponse(resp)
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

func (c *Client) GetDashboard() (models.DashboardData, error) {
	var out models.DashboardData
	resp, err := c.do("GET", "/v1/dashboard", nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) ListNutritionLogs(from, to string) ([]models.NutritionLog, error) {
	resp, err := c.do("GET", "/v1/nutrition/logs?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []models.NutritionLog
	return out, decode(resp, &out)
}

func (c *Client) PostNutritionLog(log models.NutritionLog) error {
	resp, err := c.do("POST", "/v1/nutrition/logs", log)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) ParseMeal(text string) (models.ParsedMeal, error) {
	var out models.ParsedMeal
	resp, err := c.do("POST", "/v1/parse/meal", map[string]string{"text": text})
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) PostBiometric(b models.BiometricLog) error {
	resp, err := c.do("POST", "/v1/biometrics", b)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) ListBiometrics(from, to string) ([]models.BiometricLog, error) {
	resp, err := c.do("GET", "/v1/biometrics?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []models.BiometricLog
	return out, decode(resp, &out)
}

func (c *Client) PostWorkout(w models.WorkoutEntry) error {
	resp, err := c.do("POST", "/v1/workouts", w)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) ListWorkouts(from, to string) ([]models.WorkoutEntry, error) {
	resp, err := c.do("GET", "/v1/workouts?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []models.WorkoutEntry
	return out, decode(resp, &out)
}

func (c *Client) GetProfile() (models.Profile, error) {
	var out models.Profile
	resp, err := c.do("GET", "/v1/profile", nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) UpdateProfile(p models.Profile) error {
	resp, err := c.do("PUT", "/v1/profile", p)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) PostMeasurement(m models.BodyMeasurement) error {
	resp, err := c.do("POST", "/v1/measurements", m)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) ListMeasurements(from, to string) ([]models.BodyMeasurement, error) {
	resp, err := c.do("GET", "/v1/measurements?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil)
	if err != nil {
		return nil, err
	}
	var out []models.BodyMeasurement
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

func (c *Client) GetBodyFat(method string) (models.BodyFatResult, error) {
	var out models.BodyFatResult
	resp, err := c.do("GET", "/v1/calc/bodyfat?method="+url.QueryEscape(method), nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) GetTargets() (models.Targets, error) {
	var out models.Targets
	resp, err := c.do("GET", "/v1/targets", nil)
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) PutTargets(t models.Targets) error {
	resp, err := c.do("PUT", "/v1/targets", t)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetTDEE(days int) (models.TDEEResult, error) {
	var out models.TDEEResult
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

func (c *Client) ParseWorkout(text, format string) (models.ParsedWorkout, error) {
	var out models.ParsedWorkout
	resp, err := c.do("POST", "/v1/parse/workout", map[string]string{"text": text, "format": format})
	if err != nil {
		return out, err
	}
	return out, decode(resp, &out)
}

func (c *Client) PutNutritionLog(date string, log models.NutritionLog) error {
	resp, err := c.do("PUT", "/v1/nutrition/logs/"+url.PathEscape(date), log)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) DeleteNutritionLog(date string) error {
	resp, err := c.do("DELETE", "/v1/nutrition/logs/"+url.PathEscape(date), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *Client) DeleteWorkout(date, slot string) error {
	resp, err := c.do("DELETE", "/v1/workouts/"+url.PathEscape(date)+"/"+url.PathEscape(slot), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if err := checkHTTPResponse(resp); err != nil {
		return err
	}
	return nil
}
