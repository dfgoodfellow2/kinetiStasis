package readiness

import (
	"math"
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

// ComputeReadiness computes CNS readiness using weighted Z-scores from grip and BOLT,
// with sleep and subjective feel as minor modifiers.
func ComputeReadiness(biometrics []models.BiometricLog, profile models.Profile) models.ReadinessResult {
	gripWeight := profile.GripWeight
	if gripWeight <= 0 || gripWeight > 1 {
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

	makeResult := func(level, message string, rz float64) models.ReadinessResult {
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
			GripZ:    grip.ZScore,
			BoltZ:    bolt.ZScore,
			GripSD:   grip.StandardDeviation,
			GripEMA:  grip.EMA,
			BoltSD:   bolt.StandardDeviation,
			BoltEMA:  bolt.EMA,
			Notes:    notes,
			Score:    score,
			Category: category,
		}
	}

	if !grip.HasSufficientData && !bolt.HasSufficientData {
		return makeResult("green", "Learning...", 0)
	}

	if grip.HasSufficientData && bolt.HasSufficientData {
		if math.Abs(grip.ZScore) >= 2.0 || math.Abs(bolt.ZScore) >= 2.0 {
			msg := "Rest needed"
			if math.Abs(bolt.ZScore) > math.Abs(grip.ZScore) {
				msg = "Respiratory stress detected — consider breathing/rest"
			} else if math.Abs(grip.ZScore) > math.Abs(bolt.ZScore) {
				msg = "Neuromuscular fatigue detected — consider reduced mechanical load"
			}
			return makeResult("red", msg, 0)
		}
	} else if grip.HasSufficientData && math.Abs(grip.ZScore) >= 2.0 {
		return makeResult("red", "Neuromuscular fatigue detected — consider reduced mechanical load", 0)
	} else if bolt.HasSufficientData && math.Abs(bolt.ZScore) >= 2.0 {
		return makeResult("red", "Respiratory stress detected — consider breathing/rest", 0)
	}

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

	if !grip.HasSufficientData || !bolt.HasSufficientData {
		singleZ := grip.ZScore
		sigName := "Neuromuscular"
		if !grip.HasSufficientData {
			singleZ = bolt.ZScore
			sigName = "Respiratory"
		}
		if math.Abs(singleZ) >= 1.0 && singleZ < 0 {
			return makeResult("yellow", sigName+" signal mild — consider easy day", Rz)
		}
		return makeResult("green", "Ready", Rz)
	}

	if math.Abs(Rz) >= 1.0 && Rz < 0 {
		synergy := IdentifySynergy(grip.ZScore, bolt.ZScore)
		if synergy != "" {
			return makeResult("yellow", synergy, Rz)
		}
		return makeResult("yellow", "Mildly fatigued", Rz)
	}

	if synergy := IdentifySynergy(grip.ZScore, bolt.ZScore); synergy != "" {
		return makeResult("yellow", synergy, Rz)
	}

	return makeResult("green", "Ready", Rz)
}

// ComputeReadinessVelocity computes the 7-day rolling average Rz vs the prior
// 7-day average and returns (currentAvgRz, yesterdayRz, delta, arrow).
func ComputeReadinessVelocity(biometrics []models.BiometricLog, profile models.Profile) (currAvg, yesterdayRz, delta float64, arrow string) {
	if len(biometrics) == 0 {
		return 0, 0, 0, "→"
	}

	today := time.Now()

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
