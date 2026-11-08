package handlers

// Cooldown day-boundary coverage. daysSinceLocalDateAt is the deterministic
// core of the check-in interval gate; it must count calendar days rather than
// elapsed 24-hour chunks, otherwise a late-evening check-in followed by an
// early-morning request 25 hours later would still report one day.

import (
	"testing"
	"time"
)

func TestDaysSinceLocalDateAtUsesCalendarDays(t *testing.T) {
	// 01:00 on 20 Oct, check-in dated 18 Oct: 25 elapsed hours but two local
	// calendar days. Elapsed-hours division would report 1.
	now := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)
	got, ok := daysSinceLocalDateAt(now, "2026-10-18")
	if !ok {
		t.Fatal("valid date rejected")
	}
	if got != 2 {
		t.Fatalf("days since 2026-10-18 at 01:00 on 2026-10-20 = %d, want 2 calendar days", got)
	}

	// The boundary the caller cares about: just before midnight is still the
	// same day, not the next one.
	late := time.Date(2026, 10, 20, 23, 59, 59, 0, time.UTC)
	if got, _ := daysSinceLocalDateAt(late, "2026-10-20"); got != 0 {
		t.Fatalf("same-day distance = %d, want 0", got)
	}
	if got, _ := daysSinceLocalDateAt(late, "2026-10-19"); got != 1 {
		t.Fatalf("previous-day distance = %d, want 1", got)
	}

	// A non-UTC location still projects both date components onto midnights,
	// so the answer is independent of the caller's offset.
	loc := time.FixedZone("UTC-7", -7*60*60)
	localNoon := time.Date(2026, 10, 20, 12, 0, 0, 0, loc)
	if got, _ := daysSinceLocalDateAt(localNoon, "2026-10-18"); got != 2 {
		t.Fatalf("offset-local distance = %d, want 2", got)
	}

	if _, ok := daysSinceLocalDateAt(now, "not-a-date"); ok {
		t.Fatal("malformed date accepted")
	}
}
