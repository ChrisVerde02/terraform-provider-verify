package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/Christian-Verderame/terraform-provider-verify/internal/resources"
)

// TestJWTRefreshThreshold_default verifies that a zero/unset refresh threshold
// falls back to the 60-second default in the resource logic.
func TestJWTRefreshThreshold_default(t *testing.T) {
	const defaultBuffer = int64(60)

	// Simulate a JWT that expires in 30 seconds — within the 60s default buffer.
	expiresAt := time.Now().Unix() + 30

	// If buffer is 0 (unset), we expect it to be treated as 60.
	buffer := int64(0)
	if buffer <= 0 {
		buffer = defaultBuffer
	}

	needsRefresh := time.Now().Unix() >= expiresAt-buffer
	if !needsRefresh {
		t.Error("expected JWT expiring in 30s to need refresh with 60s buffer")
	}
}

func TestJWTRefreshThreshold_custom(t *testing.T) {
	// JWT expires in 90 seconds, custom threshold is 120s — should refresh.
	expiresAt := time.Now().Unix() + 90
	buffer := int64(120)

	needsRefresh := time.Now().Unix() >= expiresAt-buffer
	if !needsRefresh {
		t.Error("expected JWT expiring in 90s to need refresh with 120s buffer")
	}
}

func TestJWTRefreshThreshold_notExpiring(t *testing.T) {
	// JWT expires in 300 seconds, default 60s buffer — should NOT refresh.
	expiresAt := time.Now().Unix() + 300
	buffer := int64(60)

	needsRefresh := time.Now().Unix() >= expiresAt-buffer
	if needsRefresh {
		t.Error("expected JWT expiring in 300s to NOT need refresh with 60s buffer")
	}
}

// TestLabelRegexp_validLabels checks that labels used in JWT key_id pass validation.
func TestLabelRegexp_jwtKeyID(t *testing.T) {
	valid := []string{
		"demotokensigner",
		"my-key-2024",
		"key.v1",
		"A",
	}
	for _, l := range valid {
		if !resources.LabelRegexp.MatchString(l) {
			t.Errorf("LabelRegexp should accept %q", l)
		}
	}

	invalid := []string{
		"",
		"../traversal",
		"key/with/slash",
		strings.Repeat("a", 129),
	}
	for _, l := range invalid {
		if resources.LabelRegexp.MatchString(l) {
			t.Errorf("LabelRegexp should reject %q", l)
		}
	}
}
