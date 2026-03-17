package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// newTestCredentialStore creates a CredentialStore backed by a temp file.
// The file does not exist yet, so loadCredentials returns an empty map.
func newTestCredentialStore(t *testing.T) (*CredentialStore, string) {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "provider_credentials.json")
	cs := NewCredentialStoreWithFile(filePath)
	return cs, filePath
}

// seedInstance adds a test instance directly to the store and persists it.
func seedInstance(t *testing.T, cs *CredentialStore, instanceID string, creds map[string]interface{}) {
	t.Helper()
	ok := cs.AddInstance(instanceID, "schwab", "live", "Test Provider", creds)
	if !ok {
		t.Fatalf("failed to seed instance %s", instanceID)
	}
}

// TestUpdateCredentialFields_Success verifies that updating a single field
// in an existing instance's credentials changes only that field.
func TestUpdateCredentialFields_Success(t *testing.T) {
	cs, _ := newTestCredentialStore(t)
	seedInstance(t, cs, "schwab_live_1", map[string]interface{}{
		"app_key":       "my_key",
		"app_secret":    "my_secret",
		"refresh_token": "old_token",
	})

	err := cs.UpdateCredentialFields("schwab_live_1", map[string]interface{}{
		"refresh_token": "new_token",
	})
	if err != nil {
		t.Fatalf("UpdateCredentialFields returned error: %v", err)
	}

	inst := cs.GetInstance("schwab_live_1")
	creds, ok := inst["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials is not a map")
	}

	// The updated field should have the new value
	if creds["refresh_token"] != "new_token" {
		t.Errorf("refresh_token = %v, want %q", creds["refresh_token"], "new_token")
	}
	// Other fields should be preserved
	if creds["app_key"] != "my_key" {
		t.Errorf("app_key = %v, want %q", creds["app_key"], "my_key")
	}
	if creds["app_secret"] != "my_secret" {
		t.Errorf("app_secret = %v, want %q", creds["app_secret"], "my_secret")
	}
}

// TestUpdateCredentialFields_MultipleFields verifies that updating multiple
// fields at once applies all changes while preserving untouched fields.
func TestUpdateCredentialFields_MultipleFields(t *testing.T) {
	cs, _ := newTestCredentialStore(t)
	seedInstance(t, cs, "schwab_live_1", map[string]interface{}{
		"app_key":       "my_key",
		"app_secret":    "my_secret",
		"refresh_token": "old_refresh",
		"account_hash":  "old_hash",
	})

	err := cs.UpdateCredentialFields("schwab_live_1", map[string]interface{}{
		"refresh_token": "new_refresh",
		"account_hash":  "new_hash",
	})
	if err != nil {
		t.Fatalf("UpdateCredentialFields returned error: %v", err)
	}

	inst := cs.GetInstance("schwab_live_1")
	creds, ok := inst["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials is not a map")
	}

	// Both updated fields should reflect new values
	if creds["refresh_token"] != "new_refresh" {
		t.Errorf("refresh_token = %v, want %q", creds["refresh_token"], "new_refresh")
	}
	if creds["account_hash"] != "new_hash" {
		t.Errorf("account_hash = %v, want %q", creds["account_hash"], "new_hash")
	}
	// Untouched fields should be preserved
	if creds["app_key"] != "my_key" {
		t.Errorf("app_key = %v, want %q", creds["app_key"], "my_key")
	}
	if creds["app_secret"] != "my_secret" {
		t.Errorf("app_secret = %v, want %q", creds["app_secret"], "my_secret")
	}
}

// TestUpdateCredentialFields_NonExistentInstance verifies that calling
// UpdateCredentialFields with an unknown instance ID returns an error.
func TestUpdateCredentialFields_NonExistentInstance(t *testing.T) {
	cs, _ := newTestCredentialStore(t)

	err := cs.UpdateCredentialFields("does_not_exist", map[string]interface{}{
		"refresh_token": "value",
	})
	if err == nil {
		t.Fatal("expected error for non-existent instance, got nil")
	}
}

// TestUpdateCredentialFields_NilCredentials verifies that when an instance
// exists but has no credentials sub-map, UpdateCredentialFields creates
// the sub-map and inserts the fields.
func TestUpdateCredentialFields_NilCredentials(t *testing.T) {
	cs, _ := newTestCredentialStore(t)

	// Manually add an instance with nil credentials (bypass AddInstance to avoid
	// it automatically creating a credentials map).
	cs.data["schwab_no_creds"] = map[string]interface{}{
		"active":        true,
		"provider_type": "schwab",
		"account_type":  "live",
		"display_name":  "No Creds",
		// NOTE: no "credentials" key at all
	}
	// Persist so the store is in a consistent state
	if err := cs.saveCredentials(); err != nil {
		t.Fatalf("saveCredentials failed: %v", err)
	}

	err := cs.UpdateCredentialFields("schwab_no_creds", map[string]interface{}{
		"refresh_token": "brand_new_token",
	})
	if err != nil {
		t.Fatalf("UpdateCredentialFields returned error: %v", err)
	}

	inst := cs.GetInstance("schwab_no_creds")
	creds, ok := inst["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials sub-map was not created")
	}
	if creds["refresh_token"] != "brand_new_token" {
		t.Errorf("refresh_token = %v, want %q", creds["refresh_token"], "brand_new_token")
	}
}

// TestUpdateCredentialFields_Persistence verifies that after calling
// UpdateCredentialFields the change is persisted to the JSON file on disk
// and a freshly loaded store sees the update.
func TestUpdateCredentialFields_Persistence(t *testing.T) {
	cs, filePath := newTestCredentialStore(t)
	seedInstance(t, cs, "schwab_live_1", map[string]interface{}{
		"app_key":       "my_key",
		"refresh_token": "old_token",
	})

	err := cs.UpdateCredentialFields("schwab_live_1", map[string]interface{}{
		"refresh_token": "persisted_token",
	})
	if err != nil {
		t.Fatalf("UpdateCredentialFields returned error: %v", err)
	}

	// Read the file directly from disk and verify
	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read credentials file: %v", err)
	}

	var diskData map[string]map[string]interface{}
	if err := json.Unmarshal(raw, &diskData); err != nil {
		t.Fatalf("failed to unmarshal credentials file: %v", err)
	}

	instData, exists := diskData["schwab_live_1"]
	if !exists {
		t.Fatal("instance schwab_live_1 not found on disk")
	}

	creds, ok := instData["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials is not a map on disk")
	}

	if creds["refresh_token"] != "persisted_token" {
		t.Errorf("on-disk refresh_token = %v, want %q", creds["refresh_token"], "persisted_token")
	}
	if creds["app_key"] != "my_key" {
		t.Errorf("on-disk app_key = %v, want %q", creds["app_key"], "my_key")
	}

	// Also verify by creating a brand-new CredentialStore from the same file
	cs2 := NewCredentialStoreWithFile(filePath)
	inst2 := cs2.GetInstance("schwab_live_1")
	creds2, ok := inst2["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("credentials is not a map in reloaded store")
	}
	if creds2["refresh_token"] != "persisted_token" {
		t.Errorf("reloaded refresh_token = %v, want %q", creds2["refresh_token"], "persisted_token")
	}
}
