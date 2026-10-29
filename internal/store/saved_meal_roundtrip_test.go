package store_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// TestSavedMealNotesRoundTrip exercises migration 009: notes must survive both
// the saved-meal table and the meals_json blob used by meal templates.
func TestSavedMealNotesRoundTrip(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.ExecContext(ctx, `INSERT INTO users (id,username,email,password,created_at,updated_at) VALUES ('u1','u1','u1@example.com','x','now','now')`); err != nil {
		t.Fatal(err)
	}
	s := store.NewSQLiteStore(d)

	in := &models.SavedMeal{
		ID:        "m1",
		UserID:    "u1",
		Name:      "Overnight oats",
		Calories:  420,
		ProteinG:  24,
		CarbsG:    58,
		FatG:      11,
		FiberG:    8,
		Notes:     "Oats, whey, blueberries",
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if err := s.CreateSavedMeal(ctx, in); err != nil {
		t.Fatal(err)
	}

	meals, err := s.FetchSavedMeals(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(meals) != 1 {
		t.Fatalf("saved meals length = %d, want 1", len(meals))
	}
	got := meals[0]
	if got.Notes != in.Notes || got.Calories != in.Calories || got.ProteinG != in.ProteinG {
		t.Fatalf("unexpected saved meal: %+v", got)
	}

	// Legacy rows (empty notes) must still round-trip cleanly.
	legacy := &models.SavedMeal{ID: "m2", UserID: "u1", Name: "Legacy", CreatedAt: "now", UpdatedAt: "now"}
	if err := s.CreateSavedMeal(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	meals, err = s.FetchSavedMeals(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(meals) != 2 {
		t.Fatalf("saved meals length = %d, want 2", len(meals))
	}
	for _, m := range meals {
		if m.ID == "m2" && m.Notes != "" {
			t.Fatalf("legacy notes = %q, want empty", m.Notes)
		}
	}

	// Templates embed SavedMeal values in meals_json, so notes must persist
	// through the JSON blob too.
	tpl := &models.MealTemplate{
		ID:        "t1",
		UserID:    "u1",
		Name:      "Breakfast",
		Meals:     []models.SavedMeal{*in},
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if err := s.CreateMealTemplate(ctx, tpl); err != nil {
		t.Fatal(err)
	}
	templates, err := s.FetchMealTemplates(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 1 || len(templates[0].Meals) != 1 {
		t.Fatalf("unexpected templates: %+v", templates)
	}
	if templates[0].Meals[0].Notes != in.Notes {
		t.Fatalf("template meal notes = %q, want %q", templates[0].Meals[0].Notes, in.Notes)
	}

	b, _ := json.Marshal(got)
	for _, want := range []string{`"name":"Overnight oats"`, `"notes":"Oats, whey, blueberries"`, `"proteinG":24`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("json missing %s: %s", want, b)
		}
	}
	t.Logf("json: %s", b)
}
