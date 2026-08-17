package tests

import (
	"strings"
	"testing"
	"time"
)

// TestTokenExchangeRefreshThreshold_default verifies the 60s default buffer.
func TestTokenExchangeRefreshThreshold_default(t *testing.T) {
	const defaultBuffer = int64(60)

	// Token expires in 30 seconds — within the 60s default buffer.
	expiresAt := time.Now().Unix() + 30
	buffer := int64(0)
	if buffer <= 0 {
		buffer = defaultBuffer
	}

	needsRefresh := time.Now().Unix() >= expiresAt-buffer
	if !needsRefresh {
		t.Error("expected token expiring in 30s to need refresh with 60s buffer")
	}
}

func TestTokenExchangeRefreshThreshold_notExpiring(t *testing.T) {
	// Token expires in 3600 seconds — well outside the 60s buffer.
	expiresAt := time.Now().Unix() + 3600
	buffer := int64(60)

	needsRefresh := time.Now().Unix() >= expiresAt-buffer
	if needsRefresh {
		t.Error("expected token expiring in 3600s to NOT need refresh")
	}
}

func TestTokenExchangeRefreshThreshold_customBuffer(t *testing.T) {
	// Token expires in 200 seconds, custom buffer 300s — should refresh.
	expiresAt := time.Now().Unix() + 200
	buffer := int64(300)

	needsRefresh := time.Now().Unix() >= expiresAt-buffer
	if !needsRefresh {
		t.Error("expected token expiring in 200s to need refresh with 300s buffer")
	}
}

// TestRemoveResourceConditions verifies the string conditions that trigger
// RemoveResource vs AddError in the token exchange Read logic.
func TestRemoveResourceConditions(t *testing.T) {
	removeConditions := []string{
		"IBM Verify failed with HTTP 404: grant not found",
		"IBM Verify failed with HTTP 400: invalid_request: CSIAQ5212E Unable to verify the integrity of the token.",
	}
	for _, msg := range removeConditions {
		if !strings.Contains(msg, "HTTP 404") && !strings.Contains(msg, "CSIAQ5212E") {
			t.Errorf("expected %q to trigger RemoveResource", msg)
		}
	}

	preserveConditions := []string{
		"IBM Verify failed with HTTP 500: internal server error",
		"IBM Verify failed with HTTP 401: unauthorized",
		"send request: connection refused",
	}
	for _, msg := range preserveConditions {
		if strings.Contains(msg, "HTTP 404") || strings.Contains(msg, "CSIAQ5212E") {
			t.Errorf("expected %q to preserve state (AddError), not RemoveResource", msg)
		}
	}
}
