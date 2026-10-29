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
	// For health check
	PingContext(ctx context.Context) error
}

// UserStore handles user and profile data.
type UserStore interface {
	FetchProfile(ctx context.Context, userID string) (models.Profile, error)
	UpsertProfile(ctx context.Context, p *models.Profile) error

	// auth / users
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, id, username, email, passwordHash string, isAdmin bool, createdAt, updatedAt string) error
	FindUserByLogin(ctx context.Context, login string) (id, username, passwordHash string, isAdmin bool, err error)
	GetUserByID(ctx context.Context, userID string) (models.User, error)

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
	PromoteUser(ctx context.Context, userID, now string) error
	DemoteUser(ctx context.Context, userID, now string) error
	DeleteUser(ctx context.Context, userID string) error
}

// WorkoutStore handles workout data.
type WorkoutStore interface {
	FetchWorkouts(ctx context.Context, userID, since string) ([]models.WorkoutEntry, error)
	FetchWorkoutsRange(ctx context.Context, userID, from, to string) ([]models.WorkoutEntry, error)
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
