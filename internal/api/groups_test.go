package api_test

// Groups API integration coverage: real router, real cookies, real SQLite.
// The suite pins the privacy contract (toggles gate summary fields), invite
// joining, group-scoped template visibility and ownership transfer on leave.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// newAuthedClient registers a fresh user on the harness server and returns a
// client whose cookie jar carries that user's auth cookies.
func newAuthedClient(t *testing.T, h *harness, username string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	c := &http.Client{Jar: jar}

	payload, _ := json.Marshal(map[string]any{
		"username": username,
		"email":    username + "@example.com",
		"password": "supersecret123",
	})
	resp, err := c.Post(h.server.URL+"/v1/auth/register", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s: status %d, body %s", username, resp.StatusCode, body)
	}
	return c
}

// doAs is the multi-client analogue of harness.do.
func doAs(t *testing.T, h *harness, c *http.Client, method, path string, payload any, wantStatus int) []byte {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %s %s: %v", method, path, err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, body)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s: %v", method, path, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d, body = %s", method, path, resp.StatusCode, wantStatus, data)
	}
	return data
}

func TestGroupsInviteSummarySharingAndOwnership(t *testing.T) {
	h := newHarness(t) // owner: "integrity"
	bob := newAuthedClient(t, h, "bob")

	created := decodeJSON[models.Group](t, h.do(t, http.MethodPost, "/v1/groups", map[string]any{"name": "Kitchen"}, http.StatusCreated))
	if created.ID == "" || created.MyRole != "owner" || created.MemberCount != 1 {
		t.Fatalf("unexpected created group: %+v", created)
	}
	groupID := created.ID

	// Owner-only invite creation.
	invite := decodeJSON[map[string]string](t, h.do(t, http.MethodPost, "/v1/groups/invites", map[string]any{"groupId": groupID}, http.StatusCreated))
	if len(invite["code"]) != 10 || invite["expiresAt"] == "" {
		t.Fatalf("unexpected invite: %+v", invite)
	}

	// Non-owner cannot mint invites.
	doAs(t, h, bob, http.MethodPost, "/v1/groups/invites", map[string]any{"groupId": groupID}, http.StatusForbidden)

	joined := decodeJSON[models.Group](t, doAs(t, h, bob, http.MethodPost, "/v1/groups/join", map[string]any{"code": invite["code"]}, http.StatusOK))
	if joined.ID != groupID || joined.MyRole != "member" {
		t.Fatalf("unexpected join response: %+v", joined)
	}
	// The same code is single use.
	doAs(t, h, bob, http.MethodPost, "/v1/groups/join", map[string]any{"code": invite["code"]}, http.StatusConflict)

	// Default sharing exposes streak and presence for both members.
	summary := decodeJSON[models.GroupSummary](t, h.do(t, http.MethodGet, "/v1/groups/"+groupID+"/summary", nil, http.StatusOK))
	if summary.MyRole != "owner" || len(summary.Members) != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Members[1].Streak == nil || summary.Members[1].Presence == nil || summary.Members[1].Shared == nil {
		t.Fatalf("default summary should share streak/presence/shared: %+v", summary.Members[1])
	}

	// Bob turns off streak and adherence. The owner must no longer see them,
	// while clock/presence remains.
	doAs(t, h, bob, http.MethodPatch, "/v1/groups/"+groupID+"/my-sharing", map[string]any{
		"shareStreak":    false,
		"shareAdherence": false,
	}, http.StatusOK)

	raw := h.do(t, http.MethodGet, "/v1/groups/"+groupID+"/summary", nil, http.StatusOK)
	var rawSummary struct {
		Members []map[string]json.RawMessage `json:"members"`
	}
	if err := json.Unmarshal(raw, &rawSummary); err != nil {
		t.Fatalf("decode raw summary: %v", err)
	}
	if len(rawSummary.Members) != 2 {
		t.Fatalf("raw summary members = %d", len(rawSummary.Members))
	}
	bobCard := rawSummary.Members[1]
	for _, absent := range []string{"streak", "adherencePct"} {
		if _, ok := bobCard[absent]; ok {
			t.Fatalf("bob summary leaked %q after toggle off: %s", absent, raw)
		}
	}
	for _, present := range []string{"shared", "presence"} {
		if _, ok := bobCard[present]; !ok {
			t.Fatalf("bob summary missing %q while clock sharing is on: %s", present, raw)
		}
	}
	// Identity must never be exposed on a card.
	for _, absent := range []string{"userId", "email"} {
		if _, ok := bobCard[absent]; ok {
			t.Fatalf("bob summary leaked identity field %q: %s", absent, raw)
		}
	}

	// Owner shares a template; it appears in Bob's merged template list with a
	// provenance badge.
	tpl := decodeJSON[models.MealTemplate](t, h.do(t, http.MethodPost, "/v1/meals/templates", map[string]any{
		"name":  "Group breakfast",
		"meals": []any{},
	}, http.StatusCreated))
	h.do(t, http.MethodPost, "/v1/meals/templates/"+tpl.ID+"/share", map[string]any{"groupId": groupID}, http.StatusOK)

	bobTemplates := decodeJSON[[]models.MealTemplate](t, doAs(t, h, bob, http.MethodGet, "/v1/meals/templates", nil, http.StatusOK))
	found := false
	for _, bt := range bobTemplates {
		if bt.ID == tpl.ID {
			found = true
			if bt.SharedBy == "" || bt.GroupID != groupID {
				t.Fatalf("shared template missing provenance: %+v", bt)
			}
		}
	}
	if !found {
		t.Fatalf("shared template %s not visible to bob: %+v", tpl.ID, bobTemplates)
	}

	// Owner leaves; Bob, the only remaining member, becomes owner.
	h.do(t, http.MethodPost, "/v1/groups/"+groupID+"/leave", nil, http.StatusOK)
	bobGroups := decodeJSON[[]models.Group](t, doAs(t, h, bob, http.MethodGet, "/v1/groups", nil, http.StatusOK))
	if len(bobGroups) != 1 || bobGroups[0].MyRole != "owner" {
		t.Fatalf("ownership did not transfer: %+v", bobGroups)
	}
	ownerGroups := decodeJSON[[]models.Group](t, h.do(t, http.MethodGet, "/v1/groups", nil, http.StatusOK))
	if len(ownerGroups) != 0 {
		t.Fatalf("departed owner still sees groups: %+v", ownerGroups)
	}
}
