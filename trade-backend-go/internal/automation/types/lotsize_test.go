package types

import (
	"reflect"
	"testing"
)

// TestSplitIntoLots_Examples pins the exact behavior called out in the
// requirements and architecture (FR-2, AC-2, AC-3).
func TestSplitIntoLots_Examples(t *testing.T) {
	cases := []struct {
		name     string
		total    int
		lotSize  int
		expected []int
	}{
		{"seven_by_two_remainder", 7, 2, []int{2, 2, 2, 1}},
		{"six_by_two_even", 6, 2, []int{2, 2, 2}},
		{"five_by_five_single", 5, 5, []int{5}},
		{"three_by_five_lot_larger_than_total", 3, 5, []int{3}},
		{"zero_total_no_lots", 0, 2, []int{}},
		{"one_by_three_single", 1, 3, []int{1}},
		{"lot_size_one_fragments", 4, 1, []int{1, 1, 1, 1}},
		{"lot_size_zero_treated_as_one", 3, 0, []int{1, 1, 1}},
		{"lot_size_negative_treated_as_one", 2, -5, []int{1, 1}},
		{"negative_total_no_lots", -4, 2, []int{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitIntoLots(tc.total, tc.lotSize)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("SplitIntoLots(%d, %d) = %v, want %v", tc.total, tc.lotSize, got, tc.expected)
			}
		})
	}
}

// TestSplitIntoLots_SumInvariant verifies over a grid of inputs that the plan
// always sums EXACTLY to the total (never exceeds capital, FR-3/AC-4) and every
// lot is > 0 and <= the effective lot size.
func TestSplitIntoLots_SumInvariant(t *testing.T) {
	for total := 0; total <= 40; total++ {
		for lotSize := 1; lotSize <= 12; lotSize++ {
			plan := SplitIntoLots(total, lotSize)
			sum := 0
			for i, q := range plan {
				if q <= 0 {
					t.Errorf("SplitIntoLots(%d,%d) lot %d = %d, want > 0", total, lotSize, i, q)
				}
				if q > lotSize {
					t.Errorf("SplitIntoLots(%d,%d) lot %d = %d exceeds lotSize %d", total, lotSize, i, q, lotSize)
				}
				sum += q
			}
			if sum != total {
				t.Errorf("SplitIntoLots(%d,%d) sums to %d, want %d (plan=%v)", total, lotSize, sum, total, plan)
			}
			if total == 0 && len(plan) != 0 {
				t.Errorf("SplitIntoLots(0,%d) should be empty, got %v", lotSize, plan)
			}
		}
	}
}

// TestSplitIntoLots_RemainderIsLastLot verifies the remainder is a final SMALLER
// lot appended after all full lots (AC-3 ordering).
func TestSplitIntoLots_RemainderIsLastLot(t *testing.T) {
	plan := SplitIntoLots(7, 2)
	last := plan[len(plan)-1]
	if last != 1 {
		t.Errorf("expected remainder lot of 1 at the end, got %d (plan=%v)", last, plan)
	}
	for i := 0; i < len(plan)-1; i++ {
		if plan[i] != 2 {
			t.Errorf("expected full lots of 2 before the remainder, got %d at index %d", plan[i], i)
		}
	}
}

// TestEffectiveLotSize covers the normalization of unset/negative lot sizes.
// Under the new semantics EffectiveLotSize is only the divisor for lot
// splitting: <1 normalizes to 1, positive values pass through. Single-order
// gating lives in IsSingleOrder, not here.
func TestEffectiveLotSize(t *testing.T) {
	cases := []struct {
		lotSize  int
		expected int
	}{
		{0, 1},
		{-3, 1},
		{1, 1},
		{4, 4},
		{100, 100},
	}
	for _, tc := range cases {
		tc := tc
		conf := &TradeConfiguration{LotSize: tc.lotSize}
		if got := conf.EffectiveLotSize(); got != tc.expected {
			t.Errorf("EffectiveLotSize() with LotSize=%d = %d, want %d", tc.lotSize, got, tc.expected)
		}
	}
}

// TestIsSingleOrder pins the new semantics: only unset (0) or negative lot
// sizes mean a single order. LotSize == 1 is units-per-order (NOT single).
func TestIsSingleOrder(t *testing.T) {
	cases := []struct {
		lotSize int
		single  bool
	}{
		{0, true},
		{-1, true},
		{-100, true},
		{1, false},
		{2, false},
		{50, false},
	}
	for _, tc := range cases {
		tc := tc
		conf := &TradeConfiguration{LotSize: tc.lotSize}
		if got := conf.IsSingleOrder(); got != tc.single {
			t.Errorf("IsSingleOrder() with LotSize=%d = %v, want %v", tc.lotSize, got, tc.single)
		}
	}
}

// TestNewTradeConfiguration_LotDefaults verifies the constructor defaults are
// backward compatible: unset lot size (0) means single order, legs frozen.
func TestNewTradeConfiguration_LotDefaults(t *testing.T) {
	tc := NewTradeConfiguration()
	if tc.LotSize != 0 {
		t.Errorf("expected default LotSize 0 (single order), got %d", tc.LotSize)
	}
	if tc.LegsDrift {
		t.Error("expected default LegsDrift false")
	}
	if !tc.IsSingleOrder() {
		t.Error("expected default config to be single order")
	}
}

// planForConfig mirrors the engine's plan-building decision at
// engine.go handleTradingState plan-initialization: a single-order config
// produces one lot for the full total; any positive lot size splits the total
// via SplitIntoLots. This lets us pin the end-to-end semantic contract without
// standing up broker mocks.
func planForConfig(tc *TradeConfiguration, units int) []int {
	if tc.IsSingleOrder() {
		return []int{units}
	}
	return SplitIntoLots(units, tc.LotSize)
}

// TestLotSizeSemantics_PlanFromConfig pins the customer-approved semantic change:
//   - lot_size = 0 (unset)  -> single order for the full total.
//   - lot_size = 1, total 6 -> six sequential single-unit orders.
//   - lot_size = 2, total 6 -> [2,2,2] (unchanged).
//   - negative              -> single order (treated as unset).
func TestLotSizeSemantics_PlanFromConfig(t *testing.T) {
	const total = 6
	cases := []struct {
		name     string
		lotSize  int
		expected []int
	}{
		{"unset_zero_single_full_order", 0, []int{6}},
		{"negative_single_full_order", -3, []int{6}},
		{"one_unit_per_order", 1, []int{1, 1, 1, 1, 1, 1}},
		{"two_units_per_lot", 2, []int{2, 2, 2}},
		{"three_units_per_lot", 3, []int{3, 3}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			conf := &TradeConfiguration{LotSize: tc.lotSize}
			got := planForConfig(conf, total)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("planForConfig(lot_size=%d, total=%d) = %v, want %v", tc.lotSize, total, got, tc.expected)
			}
			// Every plan must sum to the total — capital cap is never exceeded.
			sum := 0
			for _, q := range got {
				sum += q
			}
			if sum != total {
				t.Errorf("plan %v sums to %d, want %d", got, sum, total)
			}
		})
	}
}

// TestLotSizeSemantics_OneUnitIsMultiLot confirms lot_size=1 is NOT single order
// under the new semantics — it must engage the multi-lot execution path.
func TestLotSizeSemantics_OneUnitIsMultiLot(t *testing.T) {
	conf := &TradeConfiguration{LotSize: 1}
	if conf.IsSingleOrder() {
		t.Fatal("lot_size=1 must NOT be single order under the new semantics")
	}
	active := &ActiveAutomation{
		Config:    &AutomationConfig{TradeConfig: TradeConfiguration{LotSize: 1}},
		OrderPlan: planForConfig(conf, 4),
	}
	if !active.IsMultiLot() {
		t.Errorf("lot_size=1 with total 4 must be multi-lot; got plan %v", active.OrderPlan)
	}
}
