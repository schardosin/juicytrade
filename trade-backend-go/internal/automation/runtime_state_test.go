package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trade-backend-go/internal/automation/types"
)

// newTestRuntimeState builds a RuntimeStateStorage backed by a temp file.
func newTestRuntimeState(t *testing.T) *RuntimeStateStorage {
	t.Helper()
	return &RuntimeStateStorage{filePath: filepath.Join(t.TempDir(), "runtime.json")}
}

func TestRuntimeState_SaveLoadPreservesEffectiveCapitalSnapshot(t *testing.T) {
	r := newTestRuntimeState(t)
	at := time.Date(2026, 7, 12, 15, 30, 0, 0, time.UTC)

	active := &types.ActiveAutomation{
		Config:                  &types.AutomationConfig{ID: "cfg1"},
		Status:                  types.StatusMonitoring, // recoverable
		EffectiveCapital:        6000,
		EffectiveCapitalNetLiq:  10000,
		EffectiveCapitalPercent: 60,
		EffectiveCapitalAt:      &at,
	}

	if err := r.Save(map[string]*types.ActiveAutomation{"a1": active}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	state, err := r.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	p, ok := state.Automations["a1"]
	if !ok {
		t.Fatal("expected persisted automation a1")
	}
	if p.EffectiveCapital != 6000 {
		t.Errorf("expected EffectiveCapital 6000, got %v", p.EffectiveCapital)
	}
	if p.EffectiveCapitalNetLiq != 10000 {
		t.Errorf("expected EffectiveCapitalNetLiq 10000, got %v", p.EffectiveCapitalNetLiq)
	}
	if p.EffectiveCapitalPercent != 60 {
		t.Errorf("expected EffectiveCapitalPercent 60, got %v", p.EffectiveCapitalPercent)
	}
	if p.EffectiveCapitalAt == nil || !p.EffectiveCapitalAt.Equal(at) {
		t.Errorf("expected EffectiveCapitalAt %v, got %v", at, p.EffectiveCapitalAt)
	}
}

func TestRestoreAutomation_CopiesEffectiveCapitalSnapshot(t *testing.T) {
	at := time.Date(2026, 7, 12, 15, 30, 0, 0, time.UTC)
	persisted := &PersistedAutomation{
		ConfigID:                "cfg1",
		Status:                  types.StatusMonitoring,
		EffectiveCapital:        6000,
		EffectiveCapitalNetLiq:  10000,
		EffectiveCapitalPercent: 60,
		EffectiveCapitalAt:      &at,
	}
	config := &types.AutomationConfig{ID: "cfg1"}

	active := RestoreAutomation(persisted, config)

	if active.EffectiveCapital != 6000 {
		t.Errorf("expected EffectiveCapital 6000, got %v", active.EffectiveCapital)
	}
	if active.EffectiveCapitalNetLiq != 10000 {
		t.Errorf("expected EffectiveCapitalNetLiq 10000, got %v", active.EffectiveCapitalNetLiq)
	}
	if active.EffectiveCapitalPercent != 60 {
		t.Errorf("expected EffectiveCapitalPercent 60, got %v", active.EffectiveCapitalPercent)
	}
	if active.EffectiveCapitalAt == nil || !active.EffectiveCapitalAt.Equal(at) {
		t.Errorf("expected EffectiveCapitalAt %v, got %v", at, active.EffectiveCapitalAt)
	}
}

func TestRestoreAutomation_LegacyStateWithoutSnapshot(t *testing.T) {
	// Legacy persisted JSON has no effective_capital* fields; must restore with
	// zero values and not panic.
	raw := `{
      "config_id": "cfg1",
      "status": "monitoring",
      "started_at": "2026-07-12T15:00:00Z",
      "error_count": 0
    }`
	var persisted PersistedAutomation
	if err := json.Unmarshal([]byte(raw), &persisted); err != nil {
		t.Fatalf("unmarshal legacy state failed: %v", err)
	}
	if persisted.EffectiveCapital != 0 || persisted.EffectiveCapitalAt != nil {
		t.Errorf("expected zero-value snapshot for legacy state, got %+v", persisted)
	}

	active := RestoreAutomation(&persisted, &types.AutomationConfig{ID: "cfg1"})
	if active.EffectiveCapital != 0 {
		t.Errorf("expected EffectiveCapital 0 for legacy restore, got %v", active.EffectiveCapital)
	}
	if active.EffectiveCapitalAt != nil {
		t.Errorf("expected nil EffectiveCapitalAt for legacy restore, got %v", active.EffectiveCapitalAt)
	}
}

func TestRuntimeState_LoadFromLegacyFileNoSnapshot(t *testing.T) {
	// A full runtime-state file written before the feature; Load must succeed and
	// leave snapshot fields zero.
	r := newTestRuntimeState(t)
	legacy := `{
      "version": "1.0",
      "updated_at": "2026-07-12T15:00:00Z",
      "automations": {
        "a1": {"config_id":"cfg1","status":"waiting","started_at":"2026-07-12T15:00:00Z","error_count":0}
      }
    }`
	if err := os.WriteFile(r.filePath, []byte(legacy), 0644); err != nil {
		t.Fatalf("write legacy file failed: %v", err)
	}
	state, err := r.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	p := state.Automations["a1"]
	if p == nil {
		t.Fatal("expected automation a1 from legacy file")
	}
	if p.EffectiveCapital != 0 || p.EffectiveCapitalAt != nil {
		t.Errorf("expected zero snapshot from legacy file, got %+v", p)
	}
}
