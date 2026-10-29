package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// TestGroupsRoundTrip exercises migration 010 end to end: groups, members,
// single-use invites, sharing toggles, summary source rows, ownership transfer
// and group-scoped template sharing.
func TestGroupsRoundTrip(t *testing.T) {
	ctx := context.Background()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, u := range []struct{ id, name string }{
		{"u1", "Alice"},
		{"u2", "Bob"},
		{"u3", "Cara"},
		{"u4", "Dana"},
	} {
		if _, err := d.ExecContext(ctx,
			`INSERT INTO users (id,username,email,password,created_at,updated_at) VALUES (?,?,?,'x','now','now')`,
			u.id, u.id, u.id+"@example.com",
		); err != nil {
			t.Fatal(err)
		}
		if _, err := d.ExecContext(ctx,
			`INSERT INTO profiles (user_id,name,updated_at) VALUES (?,?,'now')`, u.id, u.name,
		); err != nil {
			t.Fatal(err)
		}
	}

	s := store.NewSQLiteStore(d)
	const today = "2026-10-20"

	g := &models.Group{
		ID:        "g1",
		Name:      "Test Group",
		CreatedBy: "u1",
		CreatedAt: "2026-10-01T00:00:00Z",
		UpdatedAt: "2026-10-01T00:00:00Z",
	}
	if err := s.CreateGroup(ctx, g); err != nil {
		t.Fatalf("create group: %v", err)
	}
	// Owner membership is created implicitly.
	if member, err := s.IsMember(ctx, "g1", "u1"); err != nil || !member {
		t.Fatalf("owner membership = %v, err %v", member, err)
	}
	if err := s.AddMember(ctx, "g1", "u2", "member", "2026-10-02T00:00:00Z"); err != nil {
		t.Fatalf("add u2: %v", err)
	}
	if err := s.AddMember(ctx, "g1", "u3", "member", "2026-10-03T00:00:00Z"); err != nil {
		t.Fatalf("add u3: %v", err)
	}
	if err := s.AddMember(ctx, "g1", "u3", "member", "2026-10-03T00:00:00Z"); !errors.Is(err, store.ErrAlreadyMember) {
		t.Fatalf("duplicate add err = %v, want ErrAlreadyMember", err)
	}

	groups, err := s.ListGroupsForUser(ctx, "u2")
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 1 || groups[0].MyRole != "member" || groups[0].MemberCount != 3 {
		t.Fatalf("unexpected groups for u2: %+v", groups)
	}

	// Invites: invalid, expired, successful, then reused.
	if _, err := s.ConsumeInvite(ctx, "NOPE", "u4", "2026-10-10T00:00:00Z"); !errors.Is(err, store.ErrInviteInvalid) {
		t.Fatalf("invalid invite err = %v", err)
	}
	expired := &models.GroupInvite{Code: "EXPIRED234", GroupID: "g1", CreatedBy: "u1", ExpiresAt: "2026-10-05T00:00:00Z", CreatedAt: "2026-10-01T00:00:00Z"}
	if err := s.CreateInvite(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConsumeInvite(ctx, "EXPIRED234", "u4", "2026-10-10T00:00:00Z"); !errors.Is(err, store.ErrInviteExpired) {
		t.Fatalf("expired invite err = %v", err)
	}

	inv := &models.GroupInvite{Code: "ABC2345678", GroupID: "g1", CreatedBy: "u1", ExpiresAt: "2026-11-01T00:00:00Z", CreatedAt: "2026-10-01T00:00:00Z"}
	if err := s.CreateInvite(ctx, inv); err != nil {
		t.Fatal(err)
	}
	joinedGroup, err := s.ConsumeInvite(ctx, inv.Code, "u4", "2026-10-10T00:00:00Z")
	if err != nil || joinedGroup != "g1" {
		t.Fatalf("consume invite = %q, err %v", joinedGroup, err)
	}
	if _, err := s.ConsumeInvite(ctx, inv.Code, "u4", "2026-10-11T00:00:00Z"); !errors.Is(err, store.ErrInviteUsed) {
		t.Fatalf("reused invite err = %v", err)
	}
	if _, err := s.ConsumeInvite(ctx, inv.Code, "u2", "2026-10-11T00:00:00Z"); !errors.Is(err, store.ErrInviteUsed) {
		t.Fatalf("used invite for existing member err = %v", err)
	}

	// Sharing toggles: partial update leaves untouched fields alone.
	off := false
	on := true
	if err := s.UpdateMySharing(ctx, "g1", "u2", &off, &off, &off); err != nil {
		t.Fatalf("update sharing: %v", err)
	}
	if err := s.UpdateMySharing(ctx, "g1", "u2", nil, &on, nil); err != nil {
		t.Fatalf("partial sharing update: %v", err)
	}

	// Summary source includes names, toggles and today's logging signals.
	if err := s.CreateNutritionLog(ctx, &models.NutritionLog{
		ID: "n1", UserID: "u3", Date: today, Calories: 1800, ProteinG: 120, UpdatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.GroupSummarySource(ctx, "g1", today)
	if err != nil {
		t.Fatalf("summary source: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("summary rows = %d, want 4", len(rows))
	}
	byUser := map[string]store.GroupMemberSummary{}
	for _, row := range rows {
		byUser[row.UserID] = row
	}
	if got := byUser["u1"]; got.Name != "Alice" || got.Role != "owner" || !got.ShareClock {
		t.Fatalf("unexpected owner row: %+v", got)
	}
	if got := byUser["u2"]; got.Name != "Bob" || got.ShareClock || !got.ShareStreak || got.ShareAdherence {
		t.Fatalf("unexpected u2 toggles: %+v", got)
	}
	if got := byUser["u3"]; !got.LoggedFood || got.Streak != 1 || got.AdherencePct <= 0 {
		t.Fatalf("unexpected u3 signals: %+v", got)
	}

	// Template sharing: owner only; visible to group members with sharedBy.
	tpl := &models.MealTemplate{ID: "t1", UserID: "u1", Name: "Breakfast", Meals: []models.SavedMeal{}, CreatedAt: "now", UpdatedAt: "now"}
	if err := s.CreateMealTemplate(ctx, tpl); err != nil {
		t.Fatal(err)
	}
	if err := s.ShareTemplateToGroup(ctx, "u2", "t1", "g1"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("non-owner share err = %v, want ErrForbidden", err)
	}
	if err := s.ShareTemplateToGroup(ctx, "u1", "t1", "g1"); err != nil {
		t.Fatalf("share template: %v", err)
	}
	shared, err := s.FetchMealTemplates(ctx, "u2")
	if err != nil {
		t.Fatal(err)
	}
	if len(shared) != 1 || shared[0].ID != "t1" || shared[0].GroupID != "g1" || shared[0].SharedBy != "Alice" {
		t.Fatalf("unexpected shared templates: %+v", shared)
	}
	if err := s.UnshareTemplate(ctx, "u3", "t1"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("non-owner unshare err = %v, want ErrForbidden", err)
	}
	if err := s.UnshareTemplate(ctx, "u1", "t1"); err != nil {
		t.Fatalf("owner unshare: %v", err)
	}

	// Owner leaves: ownership transfers to the longest-standing member (u2).
	if err := s.LeaveGroup(ctx, "g1", "u1"); err != nil {
		t.Fatalf("owner leave: %v", err)
	}
	if _, err := s.MemberRole(ctx, "g1", "u1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("departed owner role err = %v, want ErrNotFound", err)
	}
	role, err := s.MemberRole(ctx, "g1", "u2")
	if err != nil || role != "owner" {
		t.Fatalf("u2 role = %q, err %v, want owner", role, err)
	}
	gAfter, err := s.GetGroup(ctx, "g1")
	if err != nil || gAfter.CreatedBy != "u2" {
		t.Fatalf("group after transfer = %+v, err %v", gAfter, err)
	}

	// The new group owner can unshare another member's template.
	tpl3 := &models.MealTemplate{ID: "t3", UserID: "u3", Name: "Lunch", Meals: []models.SavedMeal{}, CreatedAt: "now", UpdatedAt: "now"}
	if err := s.CreateMealTemplate(ctx, tpl3); err != nil {
		t.Fatal(err)
	}
	if err := s.ShareTemplateToGroup(ctx, "u3", "t3", "g1"); err != nil {
		t.Fatalf("u3 share: %v", err)
	}
	if err := s.UnshareTemplate(ctx, "u2", "t3"); err != nil {
		t.Fatalf("group owner unshare: %v", err)
	}

	// Deleting the group unshares any remaining templates and removes members.
	if err := s.ShareTemplateToGroup(ctx, "u4", "t3", "g1"); !errors.Is(err, store.ErrForbidden) {
		// u4 neither owns t3 nor can share it; ensure no accidental share.
		t.Fatalf("u4 share err = %v, want ErrForbidden", err)
	}
	if err := s.DeleteGroup(ctx, "g1"); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	if _, err := s.GetGroup(ctx, "g1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("group after delete err = %v, want ErrNotFound", err)
	}
	if member, err := s.IsMember(ctx, "g1", "u2"); err != nil || member {
		t.Fatalf("membership after delete = %v, err %v", member, err)
	}

	// JSON shape: a member with all toggles off must not leak gated fields or
	// identity fields.
	card := models.MemberCard{Name: "Bob", Role: "member"}
	raw, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"shared", "presence", "streak", "adherencePct", "userId", "email"} {
		if strings.Contains(string(raw), absent) {
			t.Fatalf("card JSON unexpectedly contains %q: %s", absent, raw)
		}
	}
	streak := 3
	card.Shared = &models.SharedFlags{Clock: true, Streak: true, Adherence: true}
	card.Presence = &models.PresenceFlags{LoggedFood: true}
	card.Streak = &streak
	raw, _ = json.Marshal(card)
	for _, want := range []string{`"shared":{"clock":true,"streak":true,"adherence":true}`, `"presence":{"loggedFood":true,"loggedWeight":false,"loggedWorkout":false}`, `"streak":3`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("card JSON missing %s: %s", want, raw)
		}
	}
	t.Logf("card json: %s", raw)
}
