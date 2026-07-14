package automation

import (
	"encoding/json"
	"testing"

	"trade-backend-go/internal/automation/types"
)

// TestPersistedAutomation_LotPlanRoundTrip verifies the multi-order plan state
// survives a marshal/unmarshal cycle (crash-recovery persistence) and that
// RestoreAutomation preserves CurrentLotIndex and OrderPlan (restart mid-plan).
func TestPersistedAutomation_LotPlanRoundTrip(t *testing.T) {
	persisted := &PersistedAutomation{
		ConfigID:        "cfg-1",
		Status:          types.StatusMonitoring,
		OrderPlan:       []int{2, 2, 2, 1},
		CurrentLotIndex: 2,
		LockedStrikes: &types.StrikeSelection{
			ShortStrike: 100,
			LongStrike:  80,
			OptionType:  "put",
		},
		CurrentOrder: &types.PlacedOrder{OrderID: "ord-3"},
	}

	data, err := json.Marshal(persisted)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded PersistedAutomation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(decoded.OrderPlan) != 4 {
		t.Fatalf("expected OrderPlan len 4 after round-trip, got %d", len(decoded.OrderPlan))
	}
	if decoded.CurrentLotIndex != 2 {
		t.Errorf("expected CurrentLotIndex 2 after round-trip, got %d", decoded.CurrentLotIndex)
	}
	if decoded.LockedStrikes == nil || decoded.LockedStrikes.ShortStrike != 100 {
		t.Errorf("expected locked strikes to survive round-trip, got %+v", decoded.LockedStrikes)
	}
}

// TestRestoreAutomation_PreservesLotPlan verifies restore keeps the plan state so
// a mid-plan automation resumes on the correct lot after a server restart.
func TestRestoreAutomation_PreservesLotPlan(t *testing.T) {
	persisted := &PersistedAutomation{
		ConfigID:        "cfg-1",
		Status:          types.StatusMonitoring,
		OrderPlan:       []int{2, 2, 2},
		CurrentLotIndex: 1,
		CurrentOrder:    &types.PlacedOrder{OrderID: "ord-2"},
		LockedStrikes:   &types.StrikeSelection{ShortStrike: 50},
	}
	config := &types.AutomationConfig{ID: "cfg-1"}

	active := RestoreAutomation(persisted, config)

	if len(active.OrderPlan) != 3 {
		t.Fatalf("expected restored OrderPlan len 3, got %d", len(active.OrderPlan))
	}
	if active.CurrentLotIndex != 1 {
		t.Errorf("expected restored CurrentLotIndex 1 (must not be zeroed), got %d", active.CurrentLotIndex)
	}
	if active.LockedStrikes == nil || active.LockedStrikes.ShortStrike != 50 {
		t.Errorf("expected locked strikes preserved on restore, got %+v", active.LockedStrikes)
	}
	if active.Status != types.StatusMonitoring {
		t.Errorf("expected monitoring status preserved, got %s", active.Status)
	}
}

// TestRestoreAutomation_TradingRevertsButKeepsPlan verifies that when the crash
// occurred during the (transient) trading state, restore reverts to evaluating
// (existing behavior) yet the plan and lot index are preserved so re-evaluation
// re-enters trading at the current lot.
func TestRestoreAutomation_TradingRevertsButKeepsPlan(t *testing.T) {
	persisted := &PersistedAutomation{
		ConfigID:        "cfg-1",
		Status:          types.StatusTrading,
		OrderPlan:       []int{2, 2, 2},
		CurrentLotIndex: 2,
	}
	config := &types.AutomationConfig{ID: "cfg-1"}

	active := RestoreAutomation(persisted, config)

	if active.Status != types.StatusEvaluating {
		t.Errorf("expected trading to revert to evaluating on restore, got %s", active.Status)
	}
	if active.CurrentLotIndex != 2 {
		t.Errorf("expected CurrentLotIndex preserved through trading-revert, got %d", active.CurrentLotIndex)
	}
	if len(active.OrderPlan) != 3 {
		t.Errorf("expected OrderPlan preserved through trading-revert, got %v", active.OrderPlan)
	}
}
