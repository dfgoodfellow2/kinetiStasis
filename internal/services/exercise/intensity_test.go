package exercise

import "testing"

// TestEstimate1RMCapsRepsAtTwelve pins the divergence guard: estimation uses at
// most 12 reps, so a 30-rep set must produce exactly the 12-rep estimate rather
// than a blowout extrapolated beyond the formulas' useful range.
func TestEstimate1RMCapsRepsAtTwelve(t *testing.T) {
	const weight = 100.0

	capped := Estimate1RM(weight, 12)
	// Epley at the cap: 100 × (1 + 12/30) = 140 kg.
	if want := 140.0; capped != want {
		t.Fatalf("Estimate1RM(100, 12) = %v, want %v", capped, want)
	}

	for _, reps := range []int{13, 20, 25, 30} {
		if got := Estimate1RM(weight, reps); got != capped {
			t.Fatalf("Estimate1RM(100, %d) = %v, want capped value %v", reps, got, capped)
		}
	}

	// The uncapped Epley formula at 30 reps would be 100 × (1 + 30/30) = 200 kg.
	if got := Estimate1RM(weight, 30); got == 200.0 {
		t.Fatalf("Estimate1RM(100, 30) = %v, want the 12-rep cap (divergent blowout)", got)
	}
}

// TestEstimate1RMLowRepsUnchanged confirms the cap does not alter the
// low-rep branches: 1 rep is the weight itself and Brzycki still applies below
// 10 reps.
func TestEstimate1RMLowRepsUnchanged(t *testing.T) {
	if got := Estimate1RM(100, 1); got != 100 {
		t.Fatalf("Estimate1RM(100, 1) = %v, want 100", got)
	}
	if got, want := Estimate1RM(100, 5), 100*(36.0/32.0); got != want {
		t.Fatalf("Estimate1RM(100, 5) = %v, want %v", got, want)
	}
}
