package types

import (
	"testing"
)

// TestQA_SplitIntoLots_CapitalNeverExceeded_LargeAndBoundary hammers boundary and
// large inputs to prove the plan always sums EXACTLY to the capital-derived total
// (FR-3 / AC-4: never exceed Max Capital) and that no lot is oversized.
func TestQA_SplitIntoLots_CapitalNeverExceeded_LargeAndBoundary(t *testing.T) {
	cases := []struct {
		total   int
		lotSize int
	}{
		{1, 1},
		{1, 1000000},
		{50000, 1},
		{999983, 7}, // prime-ish total, guarantees a remainder
		{2, 2},
		{2, 3}, // total < lotSize -> single lot
	}
	for _, tc := range cases {
		plan := SplitIntoLots(tc.total, tc.lotSize)
		sum := 0
		eff := tc.lotSize
		if eff < 1 {
			eff = 1
		}
		for i, q := range plan {
			if q <= 0 {
				t.Errorf("SplitIntoLots(%d,%d): lot %d is non-positive (%d)", tc.total, tc.lotSize, i, q)
			}
			if q > eff {
				t.Errorf("SplitIntoLots(%d,%d): lot %d = %d exceeds lot size %d", tc.total, tc.lotSize, i, q, eff)
			}
			sum += q
		}
		if sum != tc.total {
			t.Errorf("SplitIntoLots(%d,%d) sums to %d, want %d — capital invariant violated", tc.total, tc.lotSize, sum, tc.total)
		}
	}
}

// TestQA_SplitIntoLots_TotalLessThanLotSize_IsSingleLot pins E-2: when the
// capital-derived total is smaller than the lot size, exactly ONE order for the
// full total is placed (not an error, not zero orders).
func TestQA_SplitIntoLots_TotalLessThanLotSize_IsSingleLot(t *testing.T) {
	plan := SplitIntoLots(3, 5)
	if len(plan) != 1 || plan[0] != 3 {
		t.Fatalf("total<lotSize must yield a single lot of the total; got %v", plan)
	}
	// A single-lot plan must NOT be treated as multi-lot (OD-1 gate).
	a := &ActiveAutomation{
		Config:    &AutomationConfig{TradeConfig: TradeConfiguration{LegsDrift: false, LotSize: 5}},
		OrderPlan: plan,
	}
	if a.IsMultiLot() {
		t.Error("a single-lot plan (total<lotSize) must NOT be multi-lot; would wrongly disable drift")
	}
	// Because it is single-order, drift must remain enabled exactly as today (OD-1).
	if !a.DriftAllowed(0.01) {
		t.Error("single-lot plan must preserve today's drift behavior (OD-1)")
	}
}

// TestQA_InsufficientCapital_YieldsEmptyPlan pins E-3: totalUnits==0 produces no
// lots, so the engine's plan-init path (len(plan)>1 check) never treats it as a
// multi-lot run and the "insufficient capital" failure path is preserved.
func TestQA_InsufficientCapital_YieldsEmptyPlan(t *testing.T) {
	if got := SplitIntoLots(0, 4); len(got) != 0 {
		t.Errorf("insufficient capital (total 0) must yield empty plan, got %v", got)
	}
	// CalculateUnitsWithCapital returns 0 when capital cannot cover one unit.
	tc := &TradeConfiguration{Strategy: StrategyPutSpread, Width: 50}
	if units := tc.CalculateUnitsWithCapital(100.0); units != 0 {
		t.Errorf("capital below one unit (50 width => $5000/unit) must size 0 units, got %d", units)
	}
}

// TestQA_HasMorePendingLots_OutOfRangeIndex ensures the sequencing helper does not
// panic or mis-report when CurrentLotIndex is at/over the plan boundary (defensive
// against a corrupted persisted CurrentLotIndex after restart).
func TestQA_HasMorePendingLots_OutOfRangeIndex(t *testing.T) {
	a := &ActiveAutomation{OrderPlan: []int{2, 2}, CurrentLotIndex: 5}
	if a.HasMorePendingLots() {
		t.Error("index past end of plan must report no more pending lots (guards against re-placing)")
	}
	empty := &ActiveAutomation{OrderPlan: nil, CurrentLotIndex: 0}
	if empty.HasMorePendingLots() {
		t.Error("empty plan must report no more pending lots")
	}
}

// TestQA_LotSize_IntegerDivisionFloor documents that lot sizing uses integer
// floor division on the capital-derived total, matching the requirement examples,
// and that a lot size equal to the total collapses to a single order.
func TestQA_LotSize_IntegerDivisionFloor(t *testing.T) {
	if got := SplitIntoLots(5, 5); len(got) != 1 || got[0] != 5 {
		t.Errorf("total==lotSize should be a single full lot, got %v", got)
	}
	if got := SplitIntoLots(10, 3); len(got) != 4 || got[3] != 1 {
		t.Errorf("10/3 => [3,3,3,1], got %v", got)
	}
}
