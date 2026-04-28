package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
)

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode", "err", err)
	}
}

// Error writes a JSON error response: {"error": "message"}.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// Decode reads JSON from r.Body into v. Returns false and writes a 400 if it fails.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, constants.MaxRequestBodyBytes) // 10 MB limit
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}
