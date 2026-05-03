package api

import (
	"context"
	"net/http"

	"log/slog"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// dbCheckHandler returns an unauthenticated endpoint useful for temporary
// debugging of database connectivity. It pings the DB and runs a simple
// SELECT COUNT(*) FROM check_in_logs to exercise a query.
func dbCheckHandler(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var pingErr error
		if err := s.PingContext(ctx); err != nil {
			pingErr = err
			slog.Error("db ping failed", "err", err)
		}

		var count int
		var queryErr error
		// Try to use store interface method if available
		if c, ok := s.(interface {
			CountCheckInLogs(context.Context) (int, error)
		}); ok {
			if cnt, err := c.CountCheckInLogs(ctx); err != nil {
				queryErr = err
				slog.Error("db query failed", "err", err)
			} else {
				count = cnt
			}
		} else {
			// Fallback: unknown store implementation; attempt ping only
			slog.Error("store does not implement CountCheckInLogs; skipping query")
		}

		resp := map[string]interface{}{
			"success":     pingErr == nil && queryErr == nil,
			"ping_error":  nil,
			"query_error": nil,
			"table_count": nil,
		}
		if pingErr != nil {
			resp["ping_error"] = pingErr.Error()
		}
		if queryErr != nil {
			resp["query_error"] = queryErr.Error()
		}
		if queryErr == nil && pingErr == nil {
			resp["table_count"] = count
		}

		respond.JSON(w, http.StatusOK, resp)
	}
}
