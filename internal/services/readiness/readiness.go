package readiness

import (
	"fmt"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/metrics"
)

// SignalAnalysis holds EMA-based stats for a single readiness signal (grip or BOLT).
type SignalAnalysis struct {
	HasSufficientData bool
	CurrentValue      float64
	EMA               float64
	StandardDeviation float64
	ZScore            float64
}

// ReadinessThresholdZone defines a consecutive-day escalation zone.
type ReadinessThresholdZone struct {
	MinZ       float64 // inclusive lower bound for the NEGATIVE deviation (-MinZ)
	MaxZ       float64 // exclusive upper bound for the negative deviation (0 = no bound)
	DaysYellow int     // consecutive days to trigger yellow
	DaysRed    int     // consecutive days to trigger red
}

// DefaultThresholdZones are the standard readiness escalation rules.
// Inspired by Andy Galpin's HRV framework (2 SD >5 days → action),
// but graduated for faster response.
//
// Only NEGATIVE deviations escalate: readiness is a fatigue signal, so a run of
// days far ABOVE baseline is informational, never a reason to mandate rest.
var DefaultThresholdZones = []ReadinessThresholdZone{
	{MinZ: 2.0, MaxZ: 0, DaysYellow: 2, DaysRed: 3},   // Severe: Rz ≤ -2
	{MinZ: 1.0, MaxZ: 2.0, DaysYellow: 4, DaysRed: 5}, // Moderate: -2 < Rz ≤ -1
}

// analyzeSignal computes EMA, SD, and z-score for a set of values.
// Returns an empty result (HasSufficientData=false) if fewer than 2 values are provided.
func analyzeSignal(vals []float64, alpha float64) SignalAnalysis {
	var a SignalAnalysis
	if len(vals) < 2 {
		return a
	}
	a.HasSufficientData = true
	a.CurrentValue = vals[len(vals)-1]
	a.EMA = metrics.EMA(vals, alpha)
	a.StandardDeviation = metrics.StandardDeviation(vals)
	if a.StandardDeviation > 0 {
		a.ZScore = metrics.ZScore(a.CurrentValue, a.EMA, a.StandardDeviation)
	}
	return a
}

// extractValues pulls positive (>0) values for the given field extractor
// from the most recent 14 entries in the biometric logs.
func extractValues(logs []models.BiometricLog, getter func(models.BiometricLog) float64) []float64 {
	start := len(logs) - 14
	if start < 0 {
		start = 0
	}
	var vals []float64
	for _, b := range logs[start:] {
		if v := getter(b); v > 0 {
			vals = append(vals, v)
		}
	}
	return vals
}

// analyzeGrip computes SignalAnalysis for grip strength over a 14-day window.
func analyzeGrip(logs []models.BiometricLog, alpha float64) SignalAnalysis {
	vals := extractValues(logs, func(b models.BiometricLog) float64 { return b.GripKg })
	return analyzeSignal(vals, alpha)
}

// analyzeBolt computes SignalAnalysis for BOLT score over a 14-day window.
func analyzeBolt(logs []models.BiometricLog, alpha float64) SignalAnalysis {
	vals := extractValues(logs, func(b models.BiometricLog) float64 { return b.BoltScore })
	return analyzeSignal(vals, alpha)
}

// String returns a coaching message based on Grip/BOLT z-score discordance.
// Returns empty string when no synergy pattern is detected.
func IdentifySynergy(zg, zb float64) string {
	if zg >= 0 && zb <= -1.0 {
		return "Stress Overload: Systemic strain detected. Keep intensity high but reduce total volume. Prioritize recovery."
	}
	if zg <= -1.0 && zb >= 0 {
		return "Recovery Hangover: Neuromuscular fatigue detected. Avoid heavy loading; favor light technical work or mobility."
	}
	if zg <= -1.0 && zb <= -1.0 {
		return "Systemic Fatigue: Mandatory deload or rest recommended. Prioritize sleep and stress management."
	}
	return ""
}

// computeCombinedRz computes the weighted combined Rz from grip and BOLT signals.
func computeCombinedRz(logs []models.BiometricLog, gripWeight float64) (float64, bool, bool) {
	grip := analyzeGrip(logs, constants.ReadinessEMAAlpha)
	bolt := analyzeBolt(logs, constants.ReadinessEMAAlpha)

	Rz := 0.0
	weightSum := 0.0
	if grip.HasSufficientData {
		Rz += grip.ZScore * gripWeight
		weightSum += gripWeight
	}
	// boltWeight derived from gripWeight
	boltWeight := 1.0 - gripWeight
	if bolt.HasSufficientData {
		Rz += bolt.ZScore * boltWeight
		weightSum += boltWeight
	}
	if weightSum > 0 {
		Rz /= weightSum
	}
	return Rz, grip.HasSufficientData, bolt.HasSufficientData
}

// filterLogsUpTo returns all logs with Date <= the given date.
func filterLogsUpTo(logs []models.BiometricLog, date string) []models.BiometricLog {
	var filtered []models.BiometricLog
	for _, b := range logs {
		if b.Date <= date {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// computeRzHistory computes the combined Rz for each of the last `days` days.
// Days are ordered from oldest to newest (index 0 = days-1 days ago).
func computeRzHistory(logs []models.BiometricLog, profile models.Profile, days int, asOf string) []float64 {
	if len(logs) == 0 {
		return make([]float64, days)
	}

	gripWeight := profile.GripWeight
	if gripWeight <= 0 || gripWeight >= 1 {
		gripWeight = 0.5
	}

	today := parseAsOf(asOf)
	rzSeries := make([]float64, days)
	for i := 0; i < days; i++ {
		targetDate := today.AddDate(0, 0, -(days - 1 - i)).Format("2006-01-02")
		window := filterLogsUpTo(logs, targetDate)
		if len(window) == 0 {
			continue
		}
		Rz, _, _ := computeCombinedRz(window, gripWeight)
		rzSeries[i] = Rz
	}
	return rzSeries
}

// countDaysInRange counts trailing consecutive days (from most recent backward)
// where the combined Rz is at least minZ BELOW baseline, i.e. Rz ≤ -minZ. When
// maxZ is non-zero the deviation must also be less severe than -maxZ
// (-maxZ < Rz). Positive deviations never count: a good day above baseline is
// not fatigue.
//
// NOTE: A day with no biometric data produces Rz=0 (the zero value for a
// float64). Since 0 > -1.0, such a day does NOT match any zone and breaks the
// streak. This means a missed log day resets the consecutive-day count — by
// design, since we cannot confirm a deviation streak without data.
func countDaysInRange(rzHistory []float64, minZ, maxZ float64) int {
	count := 0
	for i := len(rzHistory) - 1; i >= 0; i-- {
		z := rzHistory[i]
		inRange := z <= -minZ && (maxZ == 0 || z > -maxZ)
		if inRange {
			count++
		} else {
			break
		}
	}
	return count
}

// parseAsOf converts an optional YYYY-MM-DD client date into a UTC time. An
// empty or malformed value falls back to the server's current UTC day so
// callers never silently lose a reference date.
func parseAsOf(asOf string) time.Time {
	if asOf != "" {
		if d, err := time.Parse("2006-01-02", asOf); err == nil {
			return d.UTC()
		}
	}
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// ComputeReadiness computes CNS readiness using weighted Z-scores from grip and BOLT,
// with consecutive-day threshold escalation informed by Andy Galpin's HRV framework.
// asOf (YYYY-MM-DD) is the client's reference date: logs after it are ignored
// and every rolling window is anchored to it. Empty falls back to server today.
func ComputeReadiness(biometrics []models.BiometricLog, profile models.Profile, asOf string) models.ReadinessResult {
	// Never let the future leak into a historical readiness view.
	biometrics = filterLogsUpTo(biometrics, parseAsOf(asOf).Format("2006-01-02"))
	gripWeight := profile.GripWeight
	if gripWeight <= 0 || gripWeight >= 1 {
		gripWeight = 0.5
	}
	boltWeight := 1.0 - gripWeight

	grip := analyzeGrip(biometrics, constants.ReadinessEMAAlpha)
	bolt := analyzeBolt(biometrics, constants.ReadinessEMAAlpha)

	var notes []string
	if grip.ZScore <= -1.0 {
		notes = append(notes, "Grip below baseline")
	}
	if bolt.ZScore <= -1.0 {
		notes = append(notes, "BOLT below baseline")
	}
	// Positive deviations are informational only. They are surfaced so the user
	// knows the signal is unusually high, but they never escalate readiness.
	if grip.ZScore >= 1.0 {
		notes = append(notes, "Grip above baseline (informational)")
	}
	if bolt.ZScore >= 1.0 {
		notes = append(notes, "BOLT above baseline (informational)")
	}

	makeResult := func(level, message string, rz float64, consecutiveDays int) models.ReadinessResult {
		score := 50 + (rz/3.0)*50
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		category := "low"
		switch {
		case score >= 85:
			category = "peak"
		case score >= 65:
			category = "high"
		case score >= 40:
			category = "moderate"
		}
		return models.ReadinessResult{
			Level:           level,
			Message:         message,
			Rz:              rz,
			GripZ:           grip.ZScore,
			BoltZ:           bolt.ZScore,
			GripSD:          grip.StandardDeviation,
			GripEMA:         grip.EMA,
			BoltSD:          bolt.StandardDeviation,
			BoltEMA:         bolt.EMA,
			Notes:           notes,
			Score:           score,
			Category:        category,
			ConsecutiveDays: consecutiveDays,
			// A readiness score is meaningful when at least one signal has the
			// two readings needed for EMA/SD. This is the inverse of the
			// "Learning..." path below.
			HasSufficientData: grip.HasSufficientData || bolt.HasSufficientData,
		}
	}

	if !grip.HasSufficientData && !bolt.HasSufficientData {
		return makeResult("green", "Learning...", 0, 0)
	}

	// Compute combined Rz for today
	Rz := 0.0
	weightSum := 0.0
	if grip.HasSufficientData {
		Rz += grip.ZScore * gripWeight
		weightSum += gripWeight
	}
	if bolt.HasSufficientData {
		Rz += bolt.ZScore * boltWeight
		weightSum += boltWeight
	}
	if weightSum > 0 {
		Rz /= weightSum
	}

	// Check consecutive-day thresholds
	rzHistory := computeRzHistory(biometrics, profile, 14, asOf) // 14-day history for streak detection

	level := "green"
	msg := "Ready"
	var consecutiveDays int

	// Check each zone, most severe first
	for _, zone := range DefaultThresholdZones {
		count := countDaysInRange(rzHistory, zone.MinZ, zone.MaxZ)
		if count >= zone.DaysRed {
			level = "red"
			if zone.MinZ >= 2.0 {
				msg = fmt.Sprintf("Severe deviation %d days — rest required", count)
			} else {
				msg = fmt.Sprintf("Moderate deviation %d days — recovery needed", count)
			}
			consecutiveDays = count
			notes = append(notes, fmt.Sprintf("Rz ≤ -%.0f for %d consecutive days", zone.MinZ, count))
			break // highest priority zone wins
		}
		if count >= zone.DaysYellow {
			level = "yellow"
			if zone.MinZ >= 2.0 {
				msg = fmt.Sprintf("Severe deviation %d days — monitor closely", count)
			} else {
				msg = fmt.Sprintf("Moderate deviation %d days — consider easy day", count)
			}
			consecutiveDays = count
			notes = append(notes, fmt.Sprintf("Rz ≤ -%.0f for %d consecutive days", zone.MinZ, count))
			break
		}
	}

	// Fallback: if no zone breached, check synergy patterns
	if level == "green" {
		if synergy := IdentifySynergy(grip.ZScore, bolt.ZScore); synergy != "" {
			return makeResult("yellow", synergy, Rz, 0)
		}
	}

	return makeResult(level, msg, Rz, consecutiveDays)
}

// hasReadingOn reports whether a usable biometric reading (grip or BOLT) was
// recorded on the given date. A day without one is a genuine gap, not a zero
// signal, and must not be averaged into a trend.
func hasReadingOn(logs []models.BiometricLog, date string) bool {
	for _, b := range logs {
		if b.Date == date && (b.GripKg > 0 || b.BoltScore > 0) {
			return true
		}
	}
	return false
}

// ComputeReadinessVelocity computes the 7-day rolling average Rz vs the prior
// 7-day average and returns (currentAvgRz, yesterdayRz, delta, arrow). asOf
// (YYYY-MM-DD) anchors both windows; empty falls back to server today.
//
// Only days that actually carry a biometric reading are averaged. Missing days
// are skipped rather than zero-filled: a zero-fill mixes "no measurement" with
// a real Rz of 0 (which means "at baseline") and drags the trend toward zero.
// When either window has no readings at all there is no comparable trend, so
// delta stays 0 and the arrow stays flat.
func ComputeReadinessVelocity(biometrics []models.BiometricLog, profile models.Profile, asOf string) (currAvg, yesterdayRz, delta float64, arrow string) {
	biometrics = filterLogsUpTo(biometrics, parseAsOf(asOf).Format("2006-01-02"))
	if len(biometrics) == 0 {
		return 0, 0, 0, "→"
	}

	today := parseAsOf(asOf)

	rzSeries := make([]float64, 14)
	hasReading := make([]bool, 14)
	for i, daysAgo := range [14]int{13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0} {
		targetDate := today.AddDate(0, 0, -daysAgo).Format("2006-01-02")
		if !hasReadingOn(biometrics, targetDate) {
			continue // gap: excluded from the average
		}
		window := filterLogsUpTo(biometrics, targetDate)
		res := ComputeReadiness(window, profile, targetDate)
		rzSeries[i] = res.Rz
		hasReading[i] = true
	}

	prevSum, currSum := 0.0, 0.0
	prevCount, currCount := 0, 0
	for i := 0; i < 7; i++ {
		if hasReading[i] {
			prevSum += rzSeries[i]
			prevCount++
		}
		if hasReading[i+7] {
			currSum += rzSeries[i+7]
			currCount++
		}
	}

	// Average only the days that were actually measured, guarding division by
	// zero when a whole window is empty.
	prevAvg := 0.0
	if prevCount > 0 {
		prevAvg = prevSum / float64(prevCount)
	}
	if currCount > 0 {
		currAvg = currSum / float64(currCount)
	}
	if prevCount == 0 || currCount == 0 {
		delta = 0 // no comparable trend in one of the windows
	} else {
		delta = currAvg - prevAvg
	}
	yesterdayRz = rzSeries[12]

	arrow = "→"
	if delta > 0.05 {
		arrow = "↑"
	} else if delta < -0.05 {
		arrow = "↓"
	}
	return
}
