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

// TestNewTradeConfiguration_LotDefaults verifies the constructor defaults are
// backward compatible (single order, frozen legs).
func TestNewTradeConfiguration_LotDefaults(t *testing.T) {
	tc := NewTradeConfiguration()
	if tc.LotSize != 1 {
		t.Errorf("expected default LotSize 1, got %d", tc.LotSize)
	}
	if tc.LegsDrift {
		t.Error("expected default LegsDrift false")
	}
	if tc.EffectiveLotSize() != 1 {
		t.Errorf("expected default EffectiveLotSize 1, got %d", tc.EffectiveLotSize())
	}
}
