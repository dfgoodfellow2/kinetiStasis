package readiness

import (
	"math"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/constants"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/metrics"
)

// boltAnalysis holds EMA-based stats for BOLT score over 14 days.
type boltAnalysis struct {
	HasSufficientData bool
	CurrentBolt       float64
	EMA               float64
	StandardDeviation float64
	ZScore            float64
}

func analyzeBolt(logs []models.BiometricLog, alpha float64) boltAnalysis {
	var a boltAnalysis
	start := len(logs) - 14
	if start < 0 {
		start = 0
	}
	var vals []float64
	for _, b := range logs[start:] {
		if b.BoltScore > 0 {
			vals = append(vals, b.BoltScore)
		}
	}
	if len(vals) < 2 {
		return a
	}
	a.HasSufficientData = true
	a.CurrentBolt = vals[len(vals)-1]
	a.EMA = metrics.EMA(vals, alpha)
	a.StandardDeviation = metrics.StandardDeviation(vals)
	if a.StandardDeviation > 0 {
		a.ZScore = metrics.ZScore(a.CurrentBolt, a.EMA, a.StandardDeviation)
	}
	return a
}

// IdentifySynergy returns a coaching message based on Grip/BOLT z-score discordance.
// Returns empty string when no synergy pattern is detected.
func IdentifySynergy(zg, zb float64) string {
	// Grip High + BOLT Low: systemic strain despite neuromuscular readiness
	if zg >= 0 && zb <= -1.0 {
		return "Stress Overload: Systemic strain detected. Keep intensity high but reduce total volume. Prioritize recovery."
	}
	// Grip Low + BOLT High: neuromuscular fatigue despite respiratory readiness
	if zg <= -1.0 && zb >= 0 {
		return "Recovery Hangover: Neuromuscular fatigue detected. Avoid heavy loading; favor light technical work or mobility."
	}
	// Both Low: mandatory deload
	if zg <= -1.0 && zb <= -1.0 {
		return "Systemic Fatigue: Mandatory deload or rest recommended. Prioritize sleep and stress management."
	}
	return ""
}

// ComputeReadiness computes CNS readiness using weighted Z-scores from grip and BOLT,
// with sleep and subjective feel as minor modifiers.
// This is a port of the v1 algorithm adapted for v2's BiometricLog type.
func ComputeReadiness(biometrics []models.BiometricLog, profile models.Profile) models.ReadinessResult {
	gripWeight := profile.GripWeight
	if gripWeight <= 0 || gripWeight > 1 {
		gripWeight = 0.5
	}
	boltWeight := 1.0 - gripWeight

	bolt := analyzeBolt(biometrics, constants.ReadinessEMAAlpha)

	// Grip: 14-day window
	start := len(biometrics) - 14
	if start < 0 {
		start = 0
	}
	var gripVals []float64
	for _, b := range biometrics[start:] {
		if b.GripKg > 0 {
			gripVals = append(gripVals, b.GripKg)
		}
	}

	var gripEMA, gripSD, gripZ float64
	hasGrip := len(gripVals) >= 2
	if hasGrip {
		gripEMA = metrics.EMA(gripVals, constants.ReadinessEMAAlpha)
		gripSD = metrics.StandardDeviation(gripVals)
		currentGrip := gripVals[len(gripVals)-1]
		if gripSD > 0 {
			gripZ = metrics.ZScore(currentGrip, gripEMA, gripSD)
		}
	}

	// Build notes for sub-threshold warnings
	var notes []string
	if gripZ <= -1.0 {
		notes = append(notes, "Grip below baseline")
	}
	if bolt.ZScore <= -1.0 {
		notes = append(notes, "BOLT below baseline")
	}

	makeResult := func(level, message string, rz float64) models.ReadinessResult {
		// Derive legacy score/category from Rz
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
			Level:    level,
			Message:  message,
			Rz:       rz,
			GripZ:    gripZ,
			BoltZ:    bolt.ZScore,
			Notes:    notes,
			Score:    score,
			Category: category,
		}
	}

	// Not enough data yet
	if !hasGrip && !bolt.HasSufficientData {
		return makeResult("green", "Learning...", 0)
	}

	// Check for severe signals (|z| >= 2.0) — immediate red
	if hasGrip && bolt.HasSufficientData {
		if math.Abs(gripZ) >= 2.0 || math.Abs(bolt.ZScore) >= 2.0 {
			msg := "Rest needed"
			if math.Abs(bolt.ZScore) > math.Abs(gripZ) {
				msg = "Respiratory stress detected — consider breathing/rest"
			} else if math.Abs(gripZ) > math.Abs(bolt.ZScore) {
				msg = "Neuromuscular fatigue detected — consider reduced mechanical load"
			}
			return makeResult("red", msg, 0)
		}
	} else if hasGrip && math.Abs(gripZ) >= 2.0 {
		return makeResult("red", "Neuromuscular fatigue detected — consider reduced mechanical load", 0)
	} else if bolt.HasSufficientData && math.Abs(bolt.ZScore) >= 2.0 {
		return makeResult("red", "Respiratory stress detected — consider breathing/rest", 0)
	}

	// Compute combined Rz
	Rz := 0.0
	weightSum := 0.0
	if hasGrip {
		Rz += gripZ * gripWeight
		weightSum += gripWeight
	}
	if bolt.HasSufficientData {
		Rz += bolt.ZScore * boltWeight
		weightSum += boltWeight
	}
	if weightSum > 0 {
		Rz /= weightSum
	}

	// Single-signal path
	if !hasGrip || !bolt.HasSufficientData {
		singleZ := gripZ
		sigName := "Neuromuscular"
		if !hasGrip {
			singleZ = bolt.ZScore
			sigName = "Respiratory"
		}
		if math.Abs(singleZ) >= 1.0 && singleZ < 0 {
			return makeResult("yellow", sigName+" signal mild — consider easy day", Rz)
		}
		return makeResult("green", "Ready", Rz)
	}

	// Both signals available — check Rz thresholds and synergy
	if math.Abs(Rz) >= 1.0 && Rz < 0 {
		synergy := IdentifySynergy(gripZ, bolt.ZScore)
		if synergy != "" {
			return makeResult("yellow", synergy, Rz)
		}
		return makeResult("yellow", "Mildly fatigued", Rz)
	}

	// Check synergy even at neutral Rz (early warning)
	if synergy := IdentifySynergy(gripZ, bolt.ZScore); synergy != "" {
		return makeResult("yellow", synergy, Rz)
	}

	return makeResult("green", "Ready", Rz)
}

// ComputeReadinessVelocity computes the 7-day rolling average Rz vs the prior
// 7-day average and returns (currentAvgRz, yesterdayRz, delta, arrow).
// For each of the last 14 calendar days a cumulative window of all logs up to
// and including that day is passed to ComputeReadiness so the per-day Rz
// reflects the same EMA/SD context as the live dashboard.
func ComputeReadinessVelocity(biometrics []models.BiometricLog, profile models.Profile) (currAvg, yesterdayRz, delta float64, arrow string) {
	if len(biometrics) == 0 {
		return 0, 0, 0, "→"
	}

	today := time.Now()

	// Build per-day Rz for the last 14 calendar days (index 0 = 13 days ago, 13 = today)
	rzSeries := make([]float64, 14)
	for i, daysAgo := range [14]int{13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0} {
		targetDate := today.AddDate(0, 0, -daysAgo).Format("2006-01-02")
		var window []models.BiometricLog
		for _, b := range biometrics {
			if b.Date <= targetDate {
				window = append(window, b)
			}
		}
		if len(window) == 0 {
			rzSeries[i] = 0
			continue
		}
		res := ComputeReadiness(window, profile)
		rzSeries[i] = res.Rz
	}

	// Indices 0–6 → previous 7 days; indices 7–13 → current 7 days
	prevSum, currSum := 0.0, 0.0
	for i := 0; i < 7; i++ {
		prevSum += rzSeries[i]
		currSum += rzSeries[i+7]
	}
	prevAvg := prevSum / 7.0
	currAvg = currSum / 7.0
	delta = currAvg - prevAvg
	yesterdayRz = rzSeries[12]

	arrow = "→"
	if delta > 0.05 {
		arrow = "↑"
	} else if delta < -0.05 {
		arrow = "↓"
	}
	return
}
