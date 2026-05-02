package store

import (
	"context"
	"database/sql"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// Store groups all data access sub-interfaces.
type Store interface {
	UserStore
	LogStore
	WorkoutStore
	TargetStore
	DB() *sql.DB
}

// UserStore handles user and profile data.
type UserStore interface {
	FetchProfile(ctx context.Context, userID string) (models.Profile, error)
	// Add other user-related methods as needed
}

// LogStore handles nutrition and biometric logs.
type LogStore interface {
	FetchNutritionLogs(ctx context.Context, userID, since string) ([]models.NutritionLog, error)
	FetchBiometricLogs(ctx context.Context, userID, since string) ([]models.BiometricLog, error)
	FetchLatestWeight(ctx context.Context, userID string) (float64, error)
	FetchBodyMeasurements(ctx context.Context, userID string) (neckCm, waistCm, hipsCm float64, err error)
	FetchLastCheckin(ctx context.Context, userID string) (lastCheckinDate string, err error)
}

// WorkoutStore handles workout data.
type WorkoutStore interface {
	FetchWorkouts(ctx context.Context, userID, since string) ([]models.WorkoutEntry, error)
	// Add Create, Update, Delete as needed
	FetchGripPB(ctx context.Context, userID, today string) (float64, error)
}

// TargetStore handles target data.
type TargetStore interface {
	FetchTargets(ctx context.Context, userID string) (models.Targets, error)
	// Add Update as needed
}
