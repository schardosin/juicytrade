package indicators

import (
	"testing"

	"trade-backend-go/internal/automation/types"
)

// --- AllIndicatorsPass tests (pure logic on IndicatorResult slices) ---

func TestAllIndicatorsPass_AllPass(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{
		{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false},
		{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: false},
	}
	if !s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass to return true when all pass")
	}
}

func TestAllIndicatorsPass_OneFails(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{
		{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false},
		{Type: types.IndicatorGap, Enabled: true, Pass: false, Stale: false},
	}
	if s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass to return false when one fails")
	}
}

func TestAllIndicatorsPass_StaleBlocks(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{
		{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false},
		{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: true},
	}
	if s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass to return false when one is stale")
	}
}

func TestAllIndicatorsPass_DisabledSkipped(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{
		{Type: types.IndicatorVIX, Enabled: false, Pass: false, Stale: false},
		{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: false},
	}
	if !s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass to return true when disabled indicator is skipped")
	}
}

func TestAllIndicatorsPass_Empty(t *testing.T) {
	s := &Service{}
	results := []types.IndicatorResult{}
	if !s.AllIndicatorsPass(results) {
		t.Error("expected AllIndicatorsPass to return true for empty results (vacuously true)")
	}
}

// --- Group-level OR logic tests ---
// These replicate the OR composition logic used by EvaluateIndicatorGroups
// without requiring provider access. The helper simulates the same pattern:
// iterate groups, AND within each group (AllIndicatorsPass), OR across groups.

// computeGroupORLogic replicates the OR composition from EvaluateIndicatorGroups
// for testing purposes without provider dependencies.
func computeGroupORLogic(s *Service, groups []types.IndicatorGroup, resultsByGroup [][]types.IndicatorResult) ([]types.GroupResult, []types.IndicatorResult, bool) {
	groupResults := make([]types.GroupResult, 0, len(groups))
	allFlatResults := make([]types.IndicatorResult, 0)
	anyGroupPasses := false

	for i, group := range groups {
		var indicatorResults []types.IndicatorResult
		if i < len(resultsByGroup) {
			indicatorResults = resultsByGroup[i]
		}
		groupPass := s.AllIndicatorsPass(indicatorResults)

		groupResults = append(groupResults, types.GroupResult{
			GroupID:          group.ID,
			GroupName:        group.Name,
			Pass:             groupPass,
			IndicatorResults: indicatorResults,
		})

		allFlatResults = append(allFlatResults, indicatorResults...)

		if groupPass {
			anyGroupPasses = true
		}
	}

	return groupResults, allFlatResults, anyGroupPasses
}

func TestGroupResultORLogic(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_1", Name: "Low Vol", Indicators: []types.IndicatorConfig{
			{ID: "ind_1", Type: types.IndicatorVIX, Enabled: true},
		}},
	}
	resultsByGroup := [][]types.IndicatorResult{
		{{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false}},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true when single group passes")
	}
	if len(groupResults) != 1 {
		t.Fatalf("expected 1 group result, got %d", len(groupResults))
	}
	if !groupResults[0].Pass {
		t.Error("expected group result Pass=true")
	}
}

func TestGroupResultORLogic_TwoGroupsFirstPasses(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_1", Name: "Group A"},
		{ID: "grp_2", Name: "Group B"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: all pass
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: true, Stale: false},
			{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: false},
		},
		// Group B: one fails
		{
			{Type: types.IndicatorRSI, Enabled: true, Pass: false, Stale: false},
		},
	}

	groupResults, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if !anyGroupPasses {
		t.Error("expected anyGroupPasses=true when first group passes")
	}
	if !groupResults[0].Pass {
		t.Error("expected Group A to pass")
	}
	if groupResults[1].Pass {
		t.Error("expected Group B to fail")
	}
}

func TestGroupResultORLogic_AllFail(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_1", Name: "Group A"},
		{ID: "grp_2", Name: "Group B"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: fails
		{{Type: types.IndicatorVIX, Enabled: true, Pass: false, Stale: false}},
		// Group B: stale
		{{Type: types.IndicatorGap, Enabled: true, Pass: true, Stale: true}},
	}

	_, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if anyGroupPasses {
		t.Error("expected anyGroupPasses=false when all groups fail")
	}
}

func TestGroupResultORLogic_EmptyGroups(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{}
	resultsByGroup := [][]types.IndicatorResult{}

	_, _, anyGroupPasses := computeGroupORLogic(s, groups, resultsByGroup)

	if anyGroupPasses {
		t.Error("expected anyGroupPasses=false for empty groups")
	}
}

func TestFlatResultsAggregation(t *testing.T) {
	s := &Service{}
	groups := []types.IndicatorGroup{
		{ID: "grp_1", Name: "Group A"},
		{ID: "grp_2", Name: "Group B"},
	}
	resultsByGroup := [][]types.IndicatorResult{
		// Group A: 2 indicators
		{
			{Type: types.IndicatorVIX, Enabled: true, Pass: true, Value: 15.5},
			{Type: types.IndicatorGap, Enabled: true, Pass: true, Value: 0.3},
		},
		// Group B: 1 indicator
		{
			{Type: types.IndicatorRSI, Enabled: true, Pass: false, Value: 72.0},
		},
	}

	_, flatResults, _ := computeGroupORLogic(s, groups, resultsByGroup)

	if len(flatResults) != 3 {
		t.Fatalf("expected 3 flat results, got %d", len(flatResults))
	}
	// Verify order: Group A indicators first, then Group B
	if flatResults[0].Type != types.IndicatorVIX {
		t.Errorf("expected first flat result to be VIX, got %s", flatResults[0].Type)
	}
	if flatResults[1].Type != types.IndicatorGap {
		t.Errorf("expected second flat result to be Gap, got %s", flatResults[1].Type)
	}
	if flatResults[2].Type != types.IndicatorRSI {
		t.Errorf("expected third flat result to be RSI, got %s", flatResults[2].Type)
	}
	// Verify values preserved
	if flatResults[0].Value != 15.5 {
		t.Errorf("expected VIX value 15.5, got %f", flatResults[0].Value)
	}
	if flatResults[2].Value != 72.0 {
		t.Errorf("expected RSI value 72.0, got %f", flatResults[2].Value)
	}
}
