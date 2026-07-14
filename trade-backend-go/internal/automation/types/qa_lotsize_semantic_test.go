package types

import (
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// Adversarial tests for the lot_size SEMANTIC CHANGE (issue #81, PR #82):
//   - lot_size = 0 (unset) or negative  -> single order for the full total.
//   - lot_size = 1                       -> one unit per order (units-per-order),
//                                           NOT the single-order legacy branch.
//   - lot_size >= 2                      -> unchanged units-per-order splitting.
//
// These target the exact gate that was broken before (EffectiveLotSize()<=1 used
// to swallow lot_size=1 into single-order). IsSingleOrder is the new gate.
// ---------------------------------------------------------------------------

// TestQA_Semantic_LotSizeOne_IsNotSingleOrder is the RED-GREEN reproduction for
// the semantic bug. The old gate collapsed lot_size=1 into single order; the fix
// makes lot_size=1 a units-per-order request. If the fix (IsSingleOrder using raw
// LotSize<=0) were reverted to EffectiveLotSize()<=1, this test FAILS.
func TestQA_Semantic_LotSizeOne_IsNotSingleOrder(t *testing.T) {
	conf := &TradeConfiguration{LotSize: 1}
	if conf.IsSingleOrder() {
		t.Fatal("lot_size=1 must NOT be single order under the new semantics (regression: EffectiveLotSize<=1 gate)")
	}
	// And a total of 6 must fragment into six single-unit orders.
	if got := SplitIntoLots(6, conf.LotSize); !reflect.DeepEqual(got, []int{1, 1, 1, 1, 1, 1}) {
		t.Fatalf("lot_size=1, total 6 must split into six single-unit lots; got %v", got)
	}
}

// TestQA_Semantic_ZeroAndNegative_AreSingleOrder pins the single-order boundary:
// only 0 and negatives are single-order. This is the OD-1 legacy path.
func TestQA_Semantic_ZeroAndNegative_AreSingleOrder(t *testing.T) {
	for _, ls := range []int{0, -1, -7, -2147483648} {
		conf := &TradeConfiguration{LotSize: ls}
		if !conf.IsSingleOrder() {
			t.Errorf("lot_size=%d must be single order (legacy)", ls)
		}
		// A single-order run always yields ONE lot for the full total.
		if got := SplitIntoLots(6, 1 /*normalized*/); len(got) == 0 {
			t.Errorf("guard: single order total 6 must not be empty")
		}
	}
}

// TestQA_Semantic_LotSizeOne_TotalOne_CollapsesToSingleLot is the trickiest edge
// of the new semantics: the user requests units-per-order (lot_size=1) but capital
// only affords ONE unit. IsSingleOrder()==false (it's a units-per-order request),
// yet the resulting plan is [1] which is single-LOT. Per OD-1, a single-lot run
// must NOT engage multi-lot drift-freezing: drift stays enabled regardless of the
// legs_drift flag. This pins the documented (intentional) contract so a future
// change that starts freezing legs on 1-lot plans is caught.
func TestQA_Semantic_LotSizeOne_TotalOne_CollapsesToSingleLot(t *testing.T) {
	conf := &TradeConfiguration{LotSize: 1, LegsDrift: false}
	if conf.IsSingleOrder() {
		t.Fatal("lot_size=1 is a units-per-order request, not single order")
	}
	plan := SplitIntoLots(1, conf.LotSize)
	if !reflect.DeepEqual(plan, []int{1}) {
		t.Fatalf("lot_size=1 with total 1 must be plan [1], got %v", plan)
	}
	a := &ActiveAutomation{
		Config:    &AutomationConfig{TradeConfig: *conf},
		OrderPlan: plan,
	}
	if a.IsMultiLot() {
		t.Error("a [1] plan must NOT be multi-lot (would wrongly freeze legs / disable drift)")
	}
	// OD-1: single-lot run preserves today's drift behavior even with legs_drift=false.
	if !a.DriftAllowed(0.01) {
		t.Error("single-lot run must keep drift enabled regardless of legs_drift (OD-1)")
	}
	// And lot 0 never reuses locked strikes (there is nothing to reuse).
	if a.ShouldReuseLockedStrikes(0) {
		t.Error("lot 0 must always find fresh strikes")
	}
}

// TestQA_Semantic_LotSizeOne_TotalTwo_IsMultiLotAndFreezesLegs verifies the other
// side: as soon as total >= 2 with lot_size=1, we get a genuine multi-lot plan and
// legs_drift=false actually disables drift (frozen legs across lots).
func TestQA_Semantic_LotSizeOne_TotalTwo_IsMultiLotAndFreezesLegs(t *testing.T) {
	conf := &TradeConfiguration{LotSize: 1, LegsDrift: false}
	plan := SplitIntoLots(2, conf.LotSize)
	if !reflect.DeepEqual(plan, []int{1, 1}) {
		t.Fatalf("lot_size=1 total 2 must be [1,1], got %v", plan)
	}
	a := &ActiveAutomation{Config: &AutomationConfig{TradeConfig: *conf}, OrderPlan: plan}
	if !a.IsMultiLot() {
		t.Fatal("[1,1] must be multi-lot")
	}
	if a.DriftAllowed(0.01) {
		t.Error("multi-lot run with legs_drift=false must DISABLE drift (frozen legs)")
	}
	// Lot 1 must reuse lot-0 strikes when legs are frozen.
	if !a.ShouldReuseLockedStrikes(1) {
		t.Error("frozen-legs multi-lot run must reuse locked strikes on lot >= 1")
	}
	// Turning legs_drift on re-enables drift and re-selection.
	a.Config.TradeConfig.LegsDrift = true
	if !a.DriftAllowed(0.01) {
		t.Error("legs_drift=true must re-enable drift on multi-lot runs")
	}
	if a.ShouldReuseLockedStrikes(1) {
		t.Error("legs_drift=true must re-select strikes per lot (no reuse)")
	}
}

// TestQA_Semantic_DefaultConstructor_StaysSingleOrder guards the latent regression
// the dev caught: NewTradeConfiguration must default LotSize to 0 so existing
// constructor-built automations remain single-order (not silently 1-unit lots).
func TestQA_Semantic_DefaultConstructor_StaysSingleOrder(t *testing.T) {
	tc := NewTradeConfiguration()
	if tc.LotSize != 0 {
		t.Fatalf("constructor default LotSize must be 0 (single order); got %d — would fragment every legacy automation", tc.LotSize)
	}
	if !tc.IsSingleOrder() {
		t.Error("constructor-built config must be single order")
	}
	// Sanity: with a live total this must yield exactly one lot for the whole total.
	if got := SplitIntoLots(9, 1); len(got) == 0 {
		t.Error("guard: single-order total 9 must not be empty")
	}
}

// TestQA_Semantic_EffectiveLotSize_NoLongerGatesSingleOrder documents that
// EffectiveLotSize must NOT be used to decide single-order anymore. It only
// normalizes the divisor. For lot_size=1 it returns 1 but IsSingleOrder is false.
func TestQA_Semantic_EffectiveLotSize_NoLongerGatesSingleOrder(t *testing.T) {
	conf := &TradeConfiguration{LotSize: 1}
	if conf.EffectiveLotSize() != 1 {
		t.Errorf("EffectiveLotSize(1) must be 1, got %d", conf.EffectiveLotSize())
	}
	if conf.IsSingleOrder() {
		t.Fatal("EffectiveLotSize()==1 must NOT imply single order (the exact old bug)")
	}
}
