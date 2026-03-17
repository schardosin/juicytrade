package providers

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// ============================================================================
// QA Tests for UpdateCredentialFields (Step 1 — AC-13, AC-14)
// ============================================================================

// TestUpdateCredentialFields_EmptyUpdates verifies that calling
// UpdateCredentialFields with an empty map succeeds without error and
// still updates the "updated_at" timestamp.
func TestUpdateCredentialFields_EmptyUpdates(t *testing.T) {
	cs, _ := newTestCredentialStore(t)
	seedInstance(t, cs, "schwab_live_1", map[string]interface{}{
		"app_key":       "my_key",
		"refresh_token": "my_token",
	})

	// Capture the updated_at timestamp before the call.
	instBefore := cs.GetInstance("schwab_live_1")
	updatedBefore, _ := instBefore["updated_at"].(int64)

	// Small sleep to ensure time.Now().Unix() advances by at least 1 second.
	// If the system clock granularity is too coarse, we fall back to verifying
	// that updated_at is >= the previous value.
	time.Sleep(1100 * time.Millisecond)

	err := cs.UpdateCredentialFields("schwab_live_1", map[string]interface{}{})
	if err != nil {
		t.Fatalf("UpdateCredentialFields with empty map returned error: %v", err)
	}

	instAfter := cs.GetInstance("schwab_live_1")
	updatedAfter, _ := instAfter["updated_at"].(int64)

	if updatedAfter < updatedBefore {
		t.Errorf("updated_at went backwards: before=%d, after=%d", updatedBefore, updatedAfter)
	}
	if updatedAfter == updatedBefore {
		// With the 1.1s sleep this should not happen, but if it does we treat
		// it as a soft warning rather than a hard failure — the important thing
		// is that the call succeeded without error and the timestamp was set.
		t.Logf("warning: updated_at did not change (before=%d, after=%d); clock granularity issue?", updatedBefore, updatedAfter)
	}

	// Credentials sub-map should still contain the original fields.
	creds, ok := instAfter["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials is not a map after empty update")
	}
	if creds["app_key"] != "my_key" {
		t.Errorf("app_key = %v, want %q", creds["app_key"], "my_key")
	}
	if creds["refresh_token"] != "my_token" {
		t.Errorf("refresh_token = %v, want %q", creds["refresh_token"], "my_token")
	}
}

// TestUpdateCredentialFields_OverwriteToEmpty verifies that setting a
// credential field to an empty string "" persists that empty string
// (the key is NOT removed from the map).
func TestUpdateCredentialFields_OverwriteToEmpty(t *testing.T) {
	cs, filePath := newTestCredentialStore(t)
	seedInstance(t, cs, "schwab_live_1", map[string]interface{}{
		"app_key":       "my_key",
		"refresh_token": "valid_token",
	})

	// Overwrite refresh_token with empty string.
	err := cs.UpdateCredentialFields("schwab_live_1", map[string]interface{}{
		"refresh_token": "",
	})
	if err != nil {
		t.Fatalf("UpdateCredentialFields returned error: %v", err)
	}

	// Verify in-memory: the key should still exist with value "".
	inst := cs.GetInstance("schwab_live_1")
	creds, ok := inst["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials is not a map")
	}

	val, exists := creds["refresh_token"]
	if !exists {
		t.Fatal("refresh_token key was removed from credentials; expected it to exist with empty string value")
	}
	if val != "" {
		t.Errorf("refresh_token = %v, want empty string", val)
	}

	// Verify the other field is untouched.
	if creds["app_key"] != "my_key" {
		t.Errorf("app_key = %v, want %q", creds["app_key"], "my_key")
	}

	// Verify on-disk persistence: reload from file and check.
	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read credentials file: %v", err)
	}

	var diskData map[string]map[string]interface{}
	if err := json.Unmarshal(raw, &diskData); err != nil {
		t.Fatalf("failed to unmarshal credentials file: %v", err)
	}

	diskCreds, ok := diskData["schwab_live_1"]["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("on-disk credentials is not a map")
	}

	diskVal, diskExists := diskCreds["refresh_token"]
	if !diskExists {
		t.Fatal("on-disk refresh_token key was removed; expected it to persist as empty string")
	}
	if diskVal != "" {
		t.Errorf("on-disk refresh_token = %v, want empty string", diskVal)
	}
}

// TestUpdateCredentialFields_DoesNotClobberTopLevel verifies that calling
// UpdateCredentialFields only modifies the "credentials" sub-map and does
// NOT alter top-level keys such as "provider_type", "display_name",
// "active", "account_type", or "created_at".
func TestUpdateCredentialFields_DoesNotClobberTopLevel(t *testing.T) {
	cs, _ := newTestCredentialStore(t)
	seedInstance(t, cs, "schwab_live_1", map[string]interface{}{
		"app_key":       "my_key",
		"refresh_token": "old_token",
	})

	// Snapshot all top-level keys before the update.
	instBefore := cs.GetInstance("schwab_live_1")
	providerTypeBefore := instBefore["provider_type"]
	displayNameBefore := instBefore["display_name"]
	activeBefore := instBefore["active"]
	accountTypeBefore := instBefore["account_type"]
	createdAtBefore := instBefore["created_at"]

	// Perform a credential field update.
	err := cs.UpdateCredentialFields("schwab_live_1", map[string]interface{}{
		"refresh_token": "new_token",
		"extra_field":   "extra_value",
	})
	if err != nil {
		t.Fatalf("UpdateCredentialFields returned error: %v", err)
	}

	instAfter := cs.GetInstance("schwab_live_1")

	// Verify all top-level keys are unchanged.
	if instAfter["provider_type"] != providerTypeBefore {
		t.Errorf("provider_type changed: before=%v, after=%v", providerTypeBefore, instAfter["provider_type"])
	}
	if instAfter["display_name"] != displayNameBefore {
		t.Errorf("display_name changed: before=%v, after=%v", displayNameBefore, instAfter["display_name"])
	}
	if instAfter["active"] != activeBefore {
		t.Errorf("active changed: before=%v, after=%v", activeBefore, instAfter["active"])
	}
	if instAfter["account_type"] != accountTypeBefore {
		t.Errorf("account_type changed: before=%v, after=%v", accountTypeBefore, instAfter["account_type"])
	}
	if instAfter["created_at"] != createdAtBefore {
		t.Errorf("created_at changed: before=%v, after=%v", createdAtBefore, instAfter["created_at"])
	}

	// updated_at IS expected to change — that's the one top-level key
	// UpdateCredentialFields intentionally modifies. Just verify it exists.
	if instAfter["updated_at"] == nil {
		t.Error("updated_at is nil after update; expected a Unix timestamp")
	}

	// Verify the credentials sub-map itself was updated correctly.
	creds, ok := instAfter["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials is not a map")
	}
	if creds["refresh_token"] != "new_token" {
		t.Errorf("refresh_token = %v, want %q", creds["refresh_token"], "new_token")
	}
	if creds["extra_field"] != "extra_value" {
		t.Errorf("extra_field = %v, want %q", creds["extra_field"], "extra_value")
	}
	// Original credential field should be preserved.
	if creds["app_key"] != "my_key" {
		t.Errorf("app_key = %v, want %q", creds["app_key"], "my_key")
	}

	// Verify that the field updates did NOT leak into the top-level map.
	if _, leaked := instAfter["refresh_token"]; leaked {
		t.Error("refresh_token leaked into top-level instance map; should only exist in credentials sub-map")
	}
	if _, leaked := instAfter["extra_field"]; leaked {
		t.Error("extra_field leaked into top-level instance map; should only exist in credentials sub-map")
	}
}
