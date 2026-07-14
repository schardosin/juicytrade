package automation

import (
	"testing"
	"time"

	"trade-backend-go/internal/automation/types"
)

// qaMultiLotActive builds a running daily multi-lot automation that failed on a
// MIDDLE lot yesterday: the plan was built, lot 0 filled (CurrentLotIndex
// advanced to 1), then lot 1 exhausted its attempts and the engine marked
// TradedToday=true / Status=waiting WITHOUT clearing the plan.
func qaMultiLotActive(lastTradeDate string) *types.ActiveAutomation {
	return &types.ActiveAutomation{
		Config: &types.AutomationConfig{
			ID:         "cfg-daily-1",
			Recurrence: types.RecurrenceDaily,
			TradeConfig: types.TradeConfiguration{
				LotSize:   2,
				LegsDrift: false,
			},
		},
		Status:          types.StatusWaiting,
		TradedToday:     true,
		LastTradeDate:   lastTradeDate,
		OrderPlan:       []int{2, 2, 2},
		CurrentLotIndex: 1, // stuck mid-plan from the failed lot
		LockedStrikes:   &types.StrikeSelection{ShortStrike: 100, LongStrike: 80, OptionType: "put"},
	}
}

// TestQA_DailyNewDay_StalePlanNotCleared is an ADVERSARIAL reproduction test.
//
// Scenario (OD-2 / FR-7): a DAILY multi-lot automation fails on a middle lot.
// The engine's daily-failure branches set TradedToday=true and leave the run in
// "waiting". The successful last-lot path (handleMonitoringState "filled")
// explicitly clears OrderPlan/CurrentLotIndex/locked strikes so the NEXT trading
// day rebuilds a fresh plan. The failure paths do NOT.
//
// checkAndResetForNewDay is the single place a new trading day is detected. If
// it does not clear the plan, then on the next day handleTradingState sees
// planExists==true, SKIPS plan initialization, and resumes placing lot 1 of a
// STALE plan (only 4 of the 6 sized units) using yesterday's locked strikes and
// WITHOUT re-sizing capital for the new day.
//
// This test asserts the CORRECT behavior (plan cleared on a new day). It
// currently FAILS, documenting the bug. If the plan-clearing fix is added to
// checkAndResetForNewDay, this test will pass.
func TestQA_DailyNewDay_StalePlanNotCleared(t *testing.T) {
	e := &Engine{} // only uses e.mu; safe zero-value engine

	// Yesterday's date in NY so "today" is guaranteed different.
	ny, _ := time.LoadLocation("America/New_York")
	yesterday := time.Now().In(ny).AddDate(0, 0, -1).Format("2006-01-02")

	active := qaMultiLotActive(yesterday)

	e.checkAndResetForNewDay(active)

	// The new-day reset must flip TradedToday (this part works today).
	if active.TradedToday {
		t.Fatalf("checkAndResetForNewDay must reset TradedToday on a new day")
	}

	// BUG: the stale multi-lot plan must be cleared so the new day rebuilds a
	// fresh, correctly-sized plan (FR-7: lot size applies per trigger). If the
	// plan survives, handleTradingState will resume mid-plan on the new day.
	if len(active.OrderPlan) != 0 {
		t.Errorf("BUG: stale OrderPlan survives into new trading day: %v (CurrentLotIndex=%d). "+
			"New day will resume mid-plan (only %d of the originally-sized units) instead of rebuilding.",
			active.OrderPlan, active.CurrentLotIndex, active.OrderPlan[1]+active.OrderPlan[2])
	}
	if active.CurrentLotIndex != 0 {
		t.Errorf("BUG: CurrentLotIndex not reset on new day: got %d, want 0", active.CurrentLotIndex)
	}
	if active.LockedStrikes != nil {
		t.Errorf("BUG: LockedStrikes from prior day survive into new day: %+v (frozen-legs runs "+
			"will reuse yesterday's strikes for a fresh trigger)", active.LockedStrikes)
	}
}
