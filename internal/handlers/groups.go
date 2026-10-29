package handlers

import (
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GroupsHandler implements group membership, invites, privacy-aware summaries
// and group-scoped template sharing.
type GroupsHandler struct{ s store.Store }

func NewGroupsHandler(s store.Store) *GroupsHandler { return &GroupsHandler{s: s} }

const (
	groupNameMaxLen = 64
	inviteCodeLen   = 10
	inviteTTL       = 7 * 24 * time.Hour
	// No 0/O/1/I so codes survive being read aloud or copied by hand.
	inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

func groupUserID(r *http.Request) string {
	if claims := auth.ClaimsFromCtx(r); claims != nil {
		return claims.UserID
	}
	return ""
}

// validGroupName normalizes surrounding whitespace and enforces 1-64 runes.
func validGroupName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > groupNameMaxLen {
		return "", false
	}
	return name, true
}

// generateInviteCode returns a cryptographically random code from the
// no-ambiguous alphabet.
func generateInviteCode() (string, error) {
	buf := make([]byte, inviteCodeLen)
	max := big.NewInt(int64(len(inviteAlphabet)))
	for i := range buf {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		buf[i] = inviteAlphabet[n.Int64()]
	}
	return string(buf), nil
}

// POST /v1/groups
func (h *GroupsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !respond.Decode(w, r, &in) {
		return
	}
	name, ok := validGroupName(in.Name)
	if !ok {
		respond.Error(w, http.StatusBadRequest, "group name must be 1-64 characters")
		return
	}

	userID := groupUserID(r)
	now := time.Now().UTC().Format(time.RFC3339)
	g := &models.Group{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.s.CreateGroup(r.Context(), g); err != nil {
		slog.Error("create group", "err", err, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to create group")
		return
	}
	g.MyRole = "owner"
	g.MemberCount = 1
	respond.JSON(w, http.StatusCreated, g)
}

// GET /v1/groups
func (h *GroupsHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := groupUserID(r)
	groups, err := h.s.ListGroupsForUser(r.Context(), userID)
	if err != nil {
		slog.Error("list groups", "err", err, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to list groups")
		return
	}
	if groups == nil {
		groups = []models.Group{}
	}
	respond.JSON(w, http.StatusOK, groups)
}

// POST /v1/groups/invites — owner only.
func (h *GroupsHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		GroupID string `json:"groupId"`
	}
	if !respond.Decode(w, r, &in) {
		return
	}
	groupID := strings.TrimSpace(in.GroupID)
	if groupID == "" {
		respond.Error(w, http.StatusBadRequest, "groupId is required")
		return
	}
	userID := groupUserID(r)
	if !h.requireOwner(w, r, groupID, userID) {
		return
	}

	code, err := generateInviteCode()
	if err != nil {
		slog.Error("generate invite code", "err", err)
		respond.Error(w, http.StatusInternalServerError, "unable to create invite")
		return
	}
	now := time.Now().UTC()
	inv := &models.GroupInvite{
		Code:      code,
		GroupID:   groupID,
		CreatedBy: userID,
		ExpiresAt: now.Add(inviteTTL).Format(time.RFC3339),
		CreatedAt: now.Format(time.RFC3339),
	}
	if err := h.s.CreateInvite(r.Context(), inv); err != nil {
		slog.Error("create invite", "err", err, "group_id", groupID, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to create invite")
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]string{
		"code":      inv.Code,
		"expiresAt": inv.ExpiresAt,
	})
}

// POST /v1/groups/join — strict rate limited at the router.
func (h *GroupsHandler) Join(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !respond.Decode(w, r, &in) {
		return
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		respond.Error(w, http.StatusBadRequest, "invite code is required")
		return
	}
	userID := groupUserID(r)
	now := time.Now().UTC().Format(time.RFC3339)
	groupID, err := h.s.ConsumeInvite(r.Context(), code, userID, now)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrInviteInvalid):
			respond.Error(w, http.StatusBadRequest, "invalid invite code")
		case errors.Is(err, store.ErrInviteExpired):
			respond.Error(w, http.StatusBadRequest, "invite code has expired")
		case errors.Is(err, store.ErrInviteUsed):
			respond.Error(w, http.StatusConflict, "invite code has already been used")
		case errors.Is(err, store.ErrAlreadyMember):
			respond.Error(w, http.StatusConflict, "already a member of this group")
		default:
			slog.Error("consume invite", "err", err, "user_id", userID)
			respond.Error(w, http.StatusInternalServerError, "unable to join group")
		}
		return
	}

	g, err := h.s.GetGroup(r.Context(), groupID)
	if err != nil {
		slog.Error("fetch joined group", "err", err, "group_id", groupID, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to load group")
		return
	}
	g.MyRole = "member"
	respond.JSON(w, http.StatusOK, g)
}

// GET /v1/groups/{id}/summary — members only; signal visibility is enforced
// server-side from each member's stored toggles.
func (h *GroupsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	userID := groupUserID(r)
	role, err := h.s.MemberRole(r.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "group not found")
			return
		}
		slog.Error("read group role", "err", err, "group_id", groupID, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to load group")
		return
	}
	g, err := h.s.GetGroup(r.Context(), groupID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "group not found")
			return
		}
		slog.Error("get group summary", "err", err, "group_id", groupID)
		respond.Error(w, http.StatusInternalServerError, "unable to load group")
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	rows, err := h.s.GroupSummarySource(r.Context(), groupID, today)
	if err != nil {
		slog.Error("group summary source", "err", err, "group_id", groupID)
		respond.Error(w, http.StatusInternalServerError, "unable to build summary")
		return
	}

	cards := make([]models.MemberCard, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			name = "Member"
		}
		card := models.MemberCard{Name: name, Role: row.Role}
		if row.ShareClock || row.ShareStreak || row.ShareAdherence {
			card.Shared = &models.SharedFlags{
				Clock:     row.ShareClock,
				Streak:    row.ShareStreak,
				Adherence: row.ShareAdherence,
			}
		}
		if row.ShareClock {
			card.Presence = &models.PresenceFlags{
				LoggedFood:    row.LoggedFood,
				LoggedWeight:  row.LoggedWeight,
				LoggedWorkout: row.LoggedWorkout,
			}
		}
		if row.ShareStreak {
			streak := row.Streak
			card.Streak = &streak
		}
		if row.ShareAdherence {
			pct := row.AdherencePct
			card.AdherencePct = &pct
		}
		cards = append(cards, card)
	}

	respond.JSON(w, http.StatusOK, models.GroupSummary{
		GroupID:   g.ID,
		GroupName: g.Name,
		MyRole:    role,
		Members:   cards,
	})
}

// PATCH /v1/groups/{id}/my-sharing
func (h *GroupsHandler) UpdateMySharing(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	var in struct {
		ShareClock     *bool `json:"shareClock"`
		ShareStreak    *bool `json:"shareStreak"`
		ShareAdherence *bool `json:"shareAdherence"`
	}
	if !respond.Decode(w, r, &in) {
		return
	}
	if in.ShareClock == nil && in.ShareStreak == nil && in.ShareAdherence == nil {
		respond.Error(w, http.StatusBadRequest, "at least one sharing toggle is required")
		return
	}
	userID := groupUserID(r)
	if err := h.s.UpdateMySharing(r.Context(), groupID, userID, in.ShareClock, in.ShareStreak, in.ShareAdherence); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "group membership not found")
			return
		}
		slog.Error("update sharing", "err", err, "group_id", groupID, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to update sharing")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE /v1/groups/{id}/members/{userId} — owner only, never self-removal.
func (h *GroupsHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	targetID := chi.URLParam(r, "userId")
	userID := groupUserID(r)
	if targetID == userID {
		respond.Error(w, http.StatusBadRequest, "use the leave endpoint to remove yourself")
		return
	}
	if !h.requireOwner(w, r, groupID, userID) {
		return
	}
	targetRole, err := h.s.MemberRole(r.Context(), groupID, targetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "member not found")
			return
		}
		slog.Error("read target member role", "err", err, "group_id", groupID, "target_id", targetID)
		respond.Error(w, http.StatusInternalServerError, "unable to remove member")
		return
	}
	if targetRole == "owner" {
		respond.Error(w, http.StatusForbidden, "the group owner cannot be removed")
		return
	}
	if err := h.s.RemoveMember(r.Context(), groupID, targetID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "member not found")
			return
		}
		slog.Error("remove member", "err", err, "group_id", groupID, "target_id", targetID)
		respond.Error(w, http.StatusInternalServerError, "unable to remove member")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// POST /v1/groups/{id}/leave
func (h *GroupsHandler) Leave(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	userID := groupUserID(r)
	if err := h.s.LeaveGroup(r.Context(), groupID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "group membership not found")
			return
		}
		slog.Error("leave group", "err", err, "group_id", groupID, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to leave group")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "left"})
}

// DELETE /v1/groups/{id} — owner only.
func (h *GroupsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	userID := groupUserID(r)
	if !h.requireOwner(w, r, groupID, userID) {
		return
	}
	if err := h.s.DeleteGroup(r.Context(), groupID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "group not found")
			return
		}
		slog.Error("delete group", "err", err, "group_id", groupID, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to delete group")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /v1/meals/templates/{id}/share — template owner only.
func (h *GroupsHandler) ShareTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := chi.URLParam(r, "id")
	var in struct {
		GroupID string `json:"groupId"`
	}
	if !respond.Decode(w, r, &in) {
		return
	}
	groupID := strings.TrimSpace(in.GroupID)
	if groupID == "" {
		respond.Error(w, http.StatusBadRequest, "groupId is required")
		return
	}
	userID := groupUserID(r)
	if err := h.s.ShareTemplateToGroup(r.Context(), userID, templateID, groupID); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			respond.Error(w, http.StatusNotFound, "template or group not found")
		case errors.Is(err, store.ErrForbidden):
			respond.Error(w, http.StatusForbidden, "only the template owner can share it with their group")
		default:
			slog.Error("share template", "err", err, "template_id", templateID, "group_id", groupID, "user_id", userID)
			respond.Error(w, http.StatusInternalServerError, "unable to share template")
		}
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "shared"})
}

// DELETE /v1/meals/templates/{id}/share — template owner or group owner.
func (h *GroupsHandler) UnshareTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := chi.URLParam(r, "id")
	userID := groupUserID(r)
	if err := h.s.UnshareTemplate(r.Context(), userID, templateID); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			respond.Error(w, http.StatusNotFound, "template not found")
		case errors.Is(err, store.ErrForbidden):
			respond.Error(w, http.StatusForbidden, "not allowed to unshare this template")
		default:
			slog.Error("unshare template", "err", err, "template_id", templateID, "user_id", userID)
			respond.Error(w, http.StatusInternalServerError, "unable to unshare template")
		}
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "unshared"})
}

// requireOwner writes a 403 unless userID owns groupID. Missing groups are
// reported as forbidden so non-members cannot probe group existence.
func (h *GroupsHandler) requireOwner(w http.ResponseWriter, r *http.Request, groupID, userID string) bool {
	role, err := h.s.MemberRole(r.Context(), groupID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respond.Error(w, http.StatusForbidden, "owner access required")
			return false
		}
		slog.Error("read group role", "err", err, "group_id", groupID, "user_id", userID)
		respond.Error(w, http.StatusInternalServerError, "unable to verify group access")
		return false
	}
	if role != "owner" {
		respond.Error(w, http.StatusForbidden, "owner access required")
		return false
	}
	return true
}
