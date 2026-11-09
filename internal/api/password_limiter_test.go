package api_test

// The password endpoint protects a current-password check, so it has its own
// stricter limiter. The burst is 5: the sixth immediate attempt is rejected
// before the handler runs.

import (
	"net/http"
	"testing"
)

func TestPasswordEndpointRateLimited(t *testing.T) {
	h := newHarness(t)
	carol := newAuthedClient(t, h, "carol")

	for i := 0; i < 5; i++ {
		doAs(t, h, carol, http.MethodPost, "/v1/auth/password", map[string]any{
			"currentPassword": "wrong-password",
			"newPassword":     "newsecret123",
		}, http.StatusUnauthorized)
	}

	doAs(t, h, carol, http.MethodPost, "/v1/auth/password", map[string]any{
		"currentPassword": "wrong-password",
		"newPassword":     "newsecret123",
	}, http.StatusTooManyRequests)
}
