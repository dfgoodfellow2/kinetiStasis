package models

import (
	"testing"
	"time"
)

// dateLayout is the canonical birthdate format ("YYYY-MM-DD").
const dateLayout = "2006-01-02"

// TestAgeFromBirthdate covers the exported wrapper plus the deterministic core
// (ageFromBirthdateAt) so cases that depend on "today" are stable.
//
// Documented behavior for future birthdates: AgeFromBirthdate returns 0 (not a
// negative age), since a future birthdate is nonsensical.
func TestAgeFromBirthdate(t *testing.T) {
	// Fixed reference "today" for deterministic expectations.
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		birthdate string
		now       time.Time
		want      int
	}{
		{
			name:      "birthday today (exact)",
			birthdate: "1996-10-21",
			now:       now,
			want:      30,
		},
		{
			name:      "day before birthday",
			birthdate: "1996-10-20",
			now:       now,
			want:      30,
		},
		{
			name:      "day after birthday",
			birthdate: "1996-10-22",
			now:       now,
			want:      29,
		},
		{
			// Born Feb 29 in a leap year; in a non-leap year the birthday is
			// treated as passed once the calendar passes February.
			name:      "Feb 29 birth in non-leap year (after February)",
			birthdate: "2000-02-29",
			now:       time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC),
			want:      26,
		},
		{
			name:      "Feb 29 birth in non-leap year (before February)",
			birthdate: "2000-02-29",
			now:       time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			want:      25,
		},
		{
			name:      "empty birthdate",
			birthdate: "",
			now:       now,
			want:      0,
		},
		{
			name:      "invalid month",
			birthdate: "2020-13-01",
			now:       now,
			want:      0,
		},
		{
			name:      "future date",
			birthdate: "2026-10-22",
			now:       now,
			want:      0,
		},
		{
			name:      "far future date",
			birthdate: "2999-01-01",
			now:       now,
			want:      0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ageFromBirthdateAt(tc.birthdate, tc.now); got != tc.want {
				t.Errorf("ageFromBirthdateAt(%q): got %d want %d", tc.birthdate, got, tc.want)
			}
		})
	}

	// Exported wrapper sanity checks (time.Now()-relative).
	if got := AgeFromBirthdate(""); got != 0 {
		t.Errorf("AgeFromBirthdate(\"\"): got %d want 0", got)
	}
	if got := AgeFromBirthdate("not-a-date"); got != 0 {
		t.Errorf("AgeFromBirthdate(\"not-a-date\"): got %d want 0", got)
	}
	if got := AgeFromBirthdate(time.Now().AddDate(0, 0, 1).Format(dateLayout)); got != 0 {
		t.Errorf("AgeFromBirthdate(future): got %d want 0", got)
	}
	if got := AgeFromBirthdate(time.Now().AddDate(-30, 0, 0).Format(dateLayout)); got != 30 {
		t.Errorf("AgeFromBirthdate(-30y): got %d want 30", got)
	}
}
