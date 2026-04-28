package met

// LookupMET returns a best-effort MET value for a given exercise name.
// This is a minimal implementation used by the workout YAML parser when
// no explicit `met` value is provided. It returns 0 for unknown names.
func LookupMET(name string) float64 {
	if name == "" {
		return 0
	}
	// Very small heuristic mapping for common activities.
	switch {
	case containsIgnoreCase(name, "run") || containsIgnoreCase(name, "jog"):
		return 9.8
	case containsIgnoreCase(name, "walk"):
		return 3.5
	case containsIgnoreCase(name, "bike") || containsIgnoreCase(name, "cycling"):
		return 7.5
	case containsIgnoreCase(name, "kb") || containsIgnoreCase(name, "kettlebell"):
		return 6.0
	case containsIgnoreCase(name, "deadlift") || containsIgnoreCase(name, "squat") || containsIgnoreCase(name, "press"):
		return 6.0
	case containsIgnoreCase(name, "row"):
		return 7.0
	default:
		return 0
	}
}

func containsIgnoreCase(s, sub string) bool {
	// simple case-insensitive contains
	if len(s) < len(sub) {
		return false
	}
	ss := []rune(s)
	subr := []rune(sub)
	ls := len(ss)
	lsub := len(subr)
	for i := 0; i <= ls-lsub; i++ {
		match := true
		for j := 0; j < lsub; j++ {
			a := ss[i+j]
			b := subr[j]
			if a >= 'A' && a <= 'Z' {
				a = a + ('a' - 'A')
			}
			if b >= 'A' && b <= 'Z' {
				b = b + ('a' - 'A')
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
