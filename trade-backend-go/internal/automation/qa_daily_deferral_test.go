package automation

import (
	"testing"
	"time"

	"trade-backend-go/internal/automation/types"
)

// TestQA_MarkDailyDeferred_NoSameDayRetry is an ADVERSARIAL test guarding
// against the regression the QA-4 fix could have introduced: a same-day retry
// loop. markDailyDeferred sets LastTradeDate=today AND TradedToday=true. When
// checkAndResetForNewDay runs later the SAME trading day, it MUST NOT reset —
// otherwise a pre-fill failure at 10:00 would immediately re-fire, hammering
// the broker with retries all day (the exact behavior the "defer to next
// trading day" semantics forbid).
//
// This exercises the REAL markDailyDeferred + checkAndResetForNewDay pair
// against the real ActiveAutomation, using the actual NY-timezone date the
// production code computes (currentTradingDay), not a hardcoded string.
func TestQA_MarkDailyDeferred_NoSameDayRetry(t *testing.T) {
	e := &Engine{}

	active := &types.ActiveAutomation{
		Config: &types.AutomationConfig{
			ID:         "cfg-daily-samedays",
			Recurrence: types.RecurrenceDaily,
			TradeConfig: types.TradeConfiguration{
				LotSize:   3,
				LegsDrift: false,
			},
		},
		Status:          types.StatusWaiting,
		OrderPlan:       []int{3, 3, 3},
		CurrentLotIndex: 1,
		LockedStrikes:   &types.StrikeSelection{ShortStrike: 100, LongStrike: 80, OptionType: "put"},
	}

	// Simulate the pre-fill failure branches calling the real helper (they
	// hold e.mu; we call directly for the unit-level assertion).
	e.markDailyDeferred(active)

	if !active.TradedToday {
		t.Fatalf("markDailyDeferred must set TradedToday=true, got false")
	}
	if active.LastTradeDate != currentTradingDay() {
		t.Fatalf("markDailyDeferred must record today's NY date, got %q want %q",
			active.LastTradeDate, currentTradingDay())
	}

	// Now the scheduler ticks again LATER THE SAME DAY. This MUST NOT reset.
	e.checkAndResetForNewDay(active)

	if !active.TradedToday {
		t.Errorf("REGRESSION: same-day tick reset TradedToday -> would immediately retry "+
			"a failed daily trade, producing a same-day retry loop. Status=%s", active.Status)
	}
	if len(active.OrderPlan) != 3 {
		t.Errorf("REGRESSION: same-day tick cleared OrderPlan %v (should persist until a "+
			"genuine new trading day)", active.OrderPlan)
	}
	if active.CurrentLotIndex != 1 {
		t.Errorf("REGRESSION: same-day tick reset CurrentLotIndex to %d (should stay 1 same day)",
			active.CurrentLotIndex)
	}
}

// TestQA_NewDayAfterDeferral_RebuildsFreshPlan verifies the happy new-day
// transition for a run that was deferred via markDailyDeferred: once the
// recorded LastTradeDate no longer matches today, the run resets fully so the
// next trigger rebuilds a fresh, correctly-sized plan (FR-7).
func TestQA_NewDayAfterDeferral_RebuildsFreshPlan(t *testing.T) {
	e := &Engine{}

	ny, _ := time.LoadLocation("America/New_York")
	yesterday := time.Now().In(ny).AddDate(0, 0, -1).Format("2006-01-02")

	active := &types.ActiveAutomation{
		Config: &types.AutomationConfig{
			ID:         "cfg-daily-newday",
			Recurrence: types.RecurrenceDaily,
			TradeConfig: types.TradeConfiguration{LotSize: 2},
		},
		Status:          types.StatusWaiting,
		TradedToday:     true,
		LastTradeDate:   yesterday, // deferred yesterday via markDailyDeferred
		OrderPlan:       []int{2, 2},
		CurrentLotIndex: 0,
		LockedStrikes:   &types.StrikeSelection{ShortStrike: 100, LongStrike: 80, OptionType: "put"},
		LockedICStrikes: &types.IronCondorStrikeSelection{},
	}

	e.checkAndResetForNewDay(active)

	if active.TradedToday {
		t.Errorf("BUG: new day did not clear TradedToday (yesterday=%s)", yesterday)
	}
	if active.OrderPlan != nil {
		t.Errorf("BUG: new day did not clear stale OrderPlan %v", active.OrderPlan)
	}
	if active.CurrentLotIndex != 0 {
		t.Errorf("BUG: CurrentLotIndex not reset, got %d", active.CurrentLotIndex)
	}
	if active.LockedStrikes != nil || active.LockedICStrikes != nil {
		t.Errorf("BUG: locked strikes survived new-day reset (LockedStrikes=%v LockedICStrikes=%v)",
			active.LockedStrikes, active.LockedICStrikes)
	}
}

// TestQA_NotTradedToday_NoReset confirms the fast-path guard: a run that never
// traded today (TradedToday=false) is untouched, even with an empty
// LastTradeDate. This ensures the relaxed guard did NOT start clobbering
// fresh/idle runs.
func TestQA_NotTradedToday_NoReset(t *testing.T) {
	e := &Engine{}
	active := &types.ActiveAutomation{
		Config:          &types.AutomationConfig{ID: "cfg-idle", Recurrence: types.RecurrenceDaily},
		Status:          types.StatusWaiting,
		TradedToday:     false,
		LastTradeDate:   "",
		OrderPlan:       []int{1, 1},
		CurrentLotIndex: 1,
	}

	e.checkAndResetForNewDay(active)

	if active.TradedToday {
		t.Errorf("unexpected: TradedToday flipped to true")
	}
	// A mid-plan run that hasn't finished today must NOT be wiped by the
	// new-day check (that would drop in-progress plan state).
	if len(active.OrderPlan) != 2 || active.CurrentLotIndex != 1 {
		t.Errorf("BUG: idle-guard cleared in-progress plan (OrderPlan=%v idx=%d)",
			active.OrderPlan, active.CurrentLotIndex)
	}
}
