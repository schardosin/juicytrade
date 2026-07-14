package automation

import (
	"testing"

	"trade-backend-go/internal/automation/types"
)

// TestQA_DailyFailure_NoLastTradeDate_NewDayNeverResets is an ADVERSARIAL test
// for a GAP in the QA-1 fix.
//
// checkAndResetForNewDay guards with:
//
//	if !active.TradedToday || active.LastTradeDate == "" { return }
//
// The QA-1 fix clears the stale plan INSIDE that function. But three daily
// FAILURE paths in handleTradingState set TradedToday=true WITHOUT ever setting
// LastTradeDate:
//   - capital-resolution failure (engine.go ~702)
//   - order-placement failure    (engine.go ~860)
//   - strike-finding failure     (engine.go ~904)
//
// LastTradeDate is only ever assigned in the MONITORING state (~1009/1069/1123),
// which is reached only AFTER an order is placed. So if a daily multi-lot run
// fails on one of the three pre-order paths, LastTradeDate stays "".
//
// Consequence: on the next day, checkAndResetForNewDay returns early at the
// guard (LastTradeDate == ""). TradedToday is NEVER flipped back to false and
// the stale OrderPlan is NEVER cleared. The automation is stuck "waiting"
// forever and, if it ever did resume, would resume the stale mid-plan.
//
// This test reproduces the state left by a daily capital-resolution failure on
// the first trigger (LastTradeDate == "", TradedToday == true, plan present)
// and asserts the CORRECT behavior: a new day should reset it.
func TestQA_DailyFailure_NoLastTradeDate_NewDayNeverResets(t *testing.T) {
	e := &Engine{} // only uses e.mu

	// State exactly as left by the daily capital-resolution failure path
	// (engine.go ~701-704): TradedToday=true, Status=waiting, plan intact,
	// LastTradeDate NEVER set (still zero value) because no order ever reached
	// the monitoring state where LastTradeDate is assigned.
	active := &types.ActiveAutomation{
		Config: &types.AutomationConfig{
			ID:         "cfg-daily-capfail",
			Recurrence: types.RecurrenceDaily,
			TradeConfig: types.TradeConfiguration{
				LotSize:   2,
				LegsDrift: false,
			},
		},
		Status:          types.StatusWaiting,
		TradedToday:     true,
		LastTradeDate:   "", // <-- never set on the pre-order failure paths
		OrderPlan:       []int{2, 2, 2},
		CurrentLotIndex: 0,
		LockedStrikes:   &types.StrikeSelection{ShortStrike: 100, LongStrike: 80, OptionType: "put"},
	}

	e.checkAndResetForNewDay(active)

	// A new trading day MUST reset the run so a fresh trigger can fire.
	if active.TradedToday {
		t.Errorf("BUG: TradedToday still true after a new day because LastTradeDate was never "+
			"set on the daily capital-resolution failure path. checkAndResetForNewDay returns "+
			"early at the guard (LastTradeDate==\"\"), so the run is stuck 'waiting' forever. "+
			"Status=%s OrderPlan=%v", active.Status, active.OrderPlan)
	}
	if len(active.OrderPlan) != 0 {
		t.Errorf("BUG: stale OrderPlan %v survives forever (new-day reset never runs when "+
			"LastTradeDate is empty)", active.OrderPlan)
	}
}
