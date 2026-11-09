package store

import (
	"context"
	"errors"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// Store-level sentinel errors let handlers map failures to stable HTTP
// statuses without leaking database details to clients.
var (
	ErrNotFound      = errors.New("store: not found")
	ErrForbidden     = errors.New("store: forbidden")
	ErrConflict      = errors.New("store: conflict")
	ErrAlreadyMember = errors.New("store: already a member")
	ErrInviteInvalid = errors.New("store: invalid invite")
	ErrInviteExpired = errors.New("store: invite expired")
	ErrInviteUsed    = errors.New("store: invite used")
	// ErrCorruptWorkoutData means a stored workouts row contains JSON that no
	// longer decodes. Reads surface this instead of silently returning an
	// empty exercise/metadata set, because a subsequent write would then
	// persist that emptiness over the real data.
	ErrCorruptWorkoutData = errors.New("store: stored workout data is corrupt")
)

// GroupMemberSummary is the raw per-member row used to build privacy-aware
// group summaries. It intentionally contains UserID for internal joins but is
// never serialized directly; handlers project it into models.MemberCard.
type GroupMemberSummary struct {
	UserID         string
	Name           string
	Role           string
	ShareClock     bool
	ShareStreak    bool
	ShareAdherence bool
	JoinedAt       string
	LoggedFood     bool
	LoggedWeight   bool
	LoggedWorkout  bool
	Streak         int
	AdherencePct   float64
}

// IntegrityFinding is one read-only anomaly check in an admin data-integrity
// report. Count is the total number of offending rows (or duplicate groups,
// where noted by the check), while Samples carries at most five evidence rows.
// Samples deliberately contain only benign fields — dates, numeric values and
// the household display name — never emails, password hashes or tokens.
type IntegrityFinding struct {
	Check   string           `json:"check"`
	Count   int              `json:"count"`
	Samples []map[string]any `json:"samples"`
}

// Store groups all data access sub-interfaces.
type Store interface {
	UserStore
	LogStore
	WorkoutStore
	TargetStore
	MealStore
	CheckinStore
	MeasurementStore
	AuthStore
	GroupStore
	IntegrityStore
	// For health check
	PingContext(ctx context.Context) error
}

// UserStore handles user and profile data.
type UserStore interface {
	FetchProfile(ctx context.Context, userID string) (models.Profile, error)
	UpsertProfile(ctx context.Context, p *models.Profile) error

	// auth / users
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, id, username, email, passwordHash string, isAdmin, pendingApproval bool, createdAt, updatedAt string) error
	FindUserByLogin(ctx context.Context, login string) (id, username, passwordHash string, isAdmin bool, err error)
	GetUserByID(ctx context.Context, userID string) (models.User, error)
	GetUserPasswordHash(ctx context.Context, userID string) (string, error)

	// registration gate
	AddSetting(ctx context.Context, key, value string) error
	GetSetting(ctx context.Context, key string) (string, error)
	CreateRegistrationInvite(ctx context.Context, inv *models.RegistrationInvite) error
	ListRegistrationInvites(ctx context.Context) ([]models.RegistrationInvite, error)
	RevokeRegistrationInvite(ctx context.Context, code string) error
	ConsumeRegistrationInvite(ctx context.Context, code, userID, now string) error
	// CompensateRegistrationInvite releases a code burned for userID when the
	// account creation that followed it failed. The UPDATE is scoped to the
	// exact attempt (used_by = userID) so a concurrent winner's burn is never
	// undone.
	CompensateRegistrationInvite(ctx context.Context, code, userID string) error
	ListPendingUsers(ctx context.Context) ([]models.User, error)
	IsUserPendingApproval(ctx context.Context, userID string) (bool, error)
	SetUserPendingApproval(ctx context.Context, userID string, pending bool) error

	// password management
	UpdateUserPassword(ctx context.Context, userID, passwordHash string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error

	// convenience lookups (added for compatibility)
	FindUserByUsername(ctx context.Context, username string) (models.User, error)
	FindUserByID(ctx context.Context, userID string) (models.User, error)

	// refresh tokens (kept here for historical reasons)
	SaveRefreshToken(ctx context.Context, id, userID, tokenHash, expiresAt, createdAt string) error
	FindRefreshToken(ctx context.Context, tokenHash string) (userID string, isAdmin bool, expiresAt string, err error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error

	// admin
	ListUsers(ctx context.Context) ([]models.User, error)
	DeleteUser(ctx context.Context, userID string) error
	PromoteUser(ctx context.Context, userID, now string) error
	DemoteUser(ctx context.Context, userID, now string) error
	CountAdmins(ctx context.Context) (int, error)
}

// LogStore handles nutrition and biometric logs.
type LogStore interface {
	FetchNutritionLogs(ctx context.Context, userID, since string) ([]models.NutritionLog, error)
	FetchNutritionLogsRange(ctx context.Context, userID, from, to string) ([]models.NutritionLog, error)
	GetNutritionLog(ctx context.Context, userID, date string) (models.NutritionLog, error)
	CreateNutritionLog(ctx context.Context, n *models.NutritionLog) error
	UpdateNutritionLog(ctx context.Context, n *models.NutritionLog) error
	DeleteNutritionLog(ctx context.Context, userID, date string) error

	FetchBiometricLogs(ctx context.Context, userID, since string) ([]models.BiometricLog, error)
	FetchBiometricLogsRange(ctx context.Context, userID, from, to string) ([]models.BiometricLog, error)
	GetBiometricLog(ctx context.Context, userID, date string) (models.BiometricLog, error)
	CreateBiometricLog(ctx context.Context, b *models.BiometricLog) error
	UpdateBiometricLog(ctx context.Context, b *models.BiometricLog) error
	DeleteBiometricLog(ctx context.Context, userID, date string) error

	FetchLatestWeight(ctx context.Context, userID string) (float64, error)
	FetchBodyMeasurements(ctx context.Context, userID string) (neckCm, waistCm, hipsCm float64, err error)
	FetchBodyMeasurementsRangeMap(ctx context.Context, userID, from, to string) (map[string]float64, error)
	FetchLastCheckin(ctx context.Context, userID string) (lastCheckinDate string, err error)
	GetLastCheckin(ctx context.Context, userID string) (models.CheckInLog, error)
	FetchMeasurementsRange(ctx context.Context, userID, from, to string) ([]models.BodyMeasurement, error)
}

// CheckinStore handles weekly check-in events.
type CheckinStore interface {
	CreateCheckinLog(ctx context.Context, c *models.CheckInLog) error
	// RecordCheckinTx persists the check-in row, an optional pre-change target
	// snapshot and an optional target update in a single transaction. Pass nil
	// for snap/targets to skip that write; a nil targets leaves stored targets
	// untouched (the blocked-update path).
	RecordCheckinTx(ctx context.Context, c *models.CheckInLog, targets *models.Targets, snap *models.TargetSnapshot) error
	FetchCheckinHistory(ctx context.Context, userID string, limit int) ([]models.CheckinHistoryEntry, error)
}

// MeasurementStore handles create/update/delete for body measurements.
type MeasurementStore interface {
	CreateMeasurement(ctx context.Context, m *models.BodyMeasurement) error
	UpdateMeasurement(ctx context.Context, m *models.BodyMeasurement) error
	DeleteMeasurement(ctx context.Context, userID, date string) error
}

// AuthStore groups auth-related operations (alias to existing methods).
type AuthStore interface {
	SaveRefreshToken(ctx context.Context, id, userID, tokenHash, expiresAt, createdAt string) error
	FindRefreshToken(ctx context.Context, tokenHash string) (userID string, isAdmin bool, expiresAt string, err error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
	DeleteExpiredRefreshTokens(ctx context.Context, userID string) error
	UpdateUserPassword(ctx context.Context, userID, passwordHash string) error
	PromoteUser(ctx context.Context, userID, now string) error
	DemoteUser(ctx context.Context, userID, now string) error
	DeleteUser(ctx context.Context, userID string) error
}

// WorkoutStore handles workout data.
type WorkoutStore interface {
	FetchWorkouts(ctx context.Context, userID, since string) ([]models.WorkoutEntry, error)
	FetchWorkoutsRange(ctx context.Context, userID, from, to string) ([]models.WorkoutEntry, error)
	// SumWorkoutCaloriesForDate returns the total calories burned by all
	// workouts on `date`, or 0 when none exist.
	SumWorkoutCaloriesForDate(ctx context.Context, userID, date string) (float64, error)
	GetWorkout(ctx context.Context, userID, date, slot string) (models.WorkoutEntry, error)
	UpsertWorkout(ctx context.Context, w *models.WorkoutEntry) error
	UpdateWorkout(ctx context.Context, w *models.WorkoutEntry) (models.WorkoutEntry, error)
	DeleteWorkout(ctx context.Context, userID, date, slot string) error
	FetchGripPB(ctx context.Context, userID, today string) (float64, error)
}

// TargetStore handles target data.
type TargetStore interface {
	FetchTargets(ctx context.Context, userID string) (models.Targets, error)
	UpsertTargets(ctx context.Context, t *models.Targets) error
	// RecordTargetUpdateTx upserts the target row and, when snap is non-nil,
	// inserts the pre-change snapshot in one transaction so history can never
	// disagree with the live row.
	RecordTargetUpdateTx(ctx context.Context, t *models.Targets, snap *models.TargetSnapshot) error
	CreateTargetSnapshot(ctx context.Context, s *models.TargetSnapshot) error
	FetchTargetHistory(ctx context.Context, userID string) ([]models.TargetSnapshot, error)
}

// MealStore handles saved meals and templates
type MealStore interface {
	FetchSavedMeals(ctx context.Context, userID string) ([]models.SavedMeal, error)
	CreateSavedMeal(ctx context.Context, m *models.SavedMeal) error
	DeleteSavedMeal(ctx context.Context, userID, mealID string) error

	FetchMealTemplates(ctx context.Context, userID string) ([]models.MealTemplate, error)
	CreateMealTemplate(ctx context.Context, t *models.MealTemplate) error
	DeleteMealTemplate(ctx context.Context, userID, templateID string) error

	// Group-scoped template sharing. Sharing is only allowed by the template
	// owner; unsharing is allowed by the template owner or the group owner.
	ShareTemplateToGroup(ctx context.Context, userID, templateID, groupID string) error
	UnshareTemplate(ctx context.Context, actorID, templateID string) error
}

// IntegrityStore exposes the admin-only, read-only anomaly scan. It never
// mutates data. A failing individual check is reported in the second return
// value so one broken query cannot suppress the rest of the report.
type IntegrityStore interface {
	IntegrityReport(ctx context.Context) ([]IntegrityFinding, []string)
}

// GroupStore handles groups, memberships, invites and group summaries.
type GroupStore interface {
	CreateGroup(ctx context.Context, g *models.Group) error
	ListGroupsForUser(ctx context.Context, userID string) ([]models.Group, error)
	GetGroup(ctx context.Context, groupID string) (models.Group, error)
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	MemberRole(ctx context.Context, groupID, userID string) (string, error)
	AddMember(ctx context.Context, groupID, userID, role, joinedAt string) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	LeaveGroup(ctx context.Context, groupID, userID string) error
	DeleteGroup(ctx context.Context, groupID string) error
	UpdateMySharing(ctx context.Context, groupID, userID string, shareClock, shareStreak, shareAdherence *bool) error
	CreateInvite(ctx context.Context, inv *models.GroupInvite) error
	ConsumeInvite(ctx context.Context, code, userID, now string) (string, error)
	GroupSummarySource(ctx context.Context, groupID, today string) ([]GroupMemberSummary, error)
}
