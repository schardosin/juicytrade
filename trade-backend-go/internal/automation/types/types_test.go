package types

import (
	"strings"
	"testing"
)

func TestGetEffectiveIndicatorGroups_HasGroups(t *testing.T) {
	config := &AutomationConfig{
		IndicatorGroups: []IndicatorGroup{
			{
				ID:   "grp_1",
				Name: "Low Vol",
				Indicators: []IndicatorConfig{
					{ID: "ind_1", Type: IndicatorVIX, Enabled: true, Operator: OperatorLessThan, Threshold: 20},
				},
			},
			{
				ID:   "grp_2",
				Name: "High Vol",
				Indicators: []IndicatorConfig{
					{ID: "ind_2", Type: IndicatorVIX, Enabled: true, Operator: OperatorGreaterThan, Threshold: 30},
				},
			},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].ID != "grp_1" {
		t.Errorf("expected first group ID 'grp_1', got %q", groups[0].ID)
	}
	if groups[1].Name != "High Vol" {
		t.Errorf("expected second group name 'High Vol', got %q", groups[1].Name)
	}
	if len(groups[0].Indicators) != 1 || len(groups[1].Indicators) != 1 {
		t.Errorf("expected 1 indicator per group, got %d and %d", len(groups[0].Indicators), len(groups[1].Indicators))
	}
}

func TestGetEffectiveIndicatorGroups_LegacyOnly(t *testing.T) {
	config := &AutomationConfig{
		Indicators: []IndicatorConfig{
			{ID: "ind_1", Type: IndicatorVIX, Enabled: true, Operator: OperatorLessThan, Threshold: 20},
			{ID: "ind_2", Type: IndicatorGap, Enabled: true, Operator: OperatorLessThan, Threshold: 1.0},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group for legacy config, got %d", len(groups))
	}
	if groups[0].ID != "default" {
		t.Errorf("expected group ID 'default', got %q", groups[0].ID)
	}
	if groups[0].Name != "Default" {
		t.Errorf("expected group name 'Default', got %q", groups[0].Name)
	}
	if len(groups[0].Indicators) != 2 {
		t.Errorf("expected 2 indicators in default group, got %d", len(groups[0].Indicators))
	}
}

func TestGetEffectiveIndicatorGroups_BothEmpty(t *testing.T) {
	config := &AutomationConfig{}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups when both empty, got %d", len(groups))
	}
}

func TestGetEffectiveIndicatorGroups_GroupsTakePriority(t *testing.T) {
	config := &AutomationConfig{
		Indicators: []IndicatorConfig{
			{ID: "legacy_ind", Type: IndicatorVIX, Enabled: true},
		},
		IndicatorGroups: []IndicatorGroup{
			{
				ID:   "grp_1",
				Name: "Primary",
				Indicators: []IndicatorConfig{
					{ID: "new_ind", Type: IndicatorRSI, Enabled: true},
				},
			},
		},
	}

	groups := config.GetEffectiveIndicatorGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (from IndicatorGroups), got %d", len(groups))
	}
	if groups[0].ID != "grp_1" {
		t.Errorf("expected group from IndicatorGroups to win, got ID %q", groups[0].ID)
	}
	if groups[0].Indicators[0].ID != "new_ind" {
		t.Errorf("expected indicator from IndicatorGroups, got ID %q", groups[0].Indicators[0].ID)
	}
}

func TestGenerateGroupID_Format(t *testing.T) {
	id := GenerateGroupID()
	if !strings.HasPrefix(id, "grp_") {
		t.Errorf("expected group ID to start with 'grp_', got %q", id)
	}
	// Format: grp_{unixnano}_{4chars} — should have at least "grp_" + digits + "_" + 4 chars
	parts := strings.SplitN(id, "_", 3)
	if len(parts) != 3 {
		t.Errorf("expected 3 parts separated by '_', got %d from %q", len(parts), id)
	}
	if parts[0] != "grp" {
		t.Errorf("expected first part 'grp', got %q", parts[0])
	}
	// The last part should be 4+ chars (the random suffix may contain underscores from unixnano)
	// Just verify total length is reasonable: "grp_" (4) + timestamp (~19) + "_" (1) + random (4) = ~28
	if len(id) < 10 {
		t.Errorf("expected group ID length >= 10, got %d for %q", len(id), id)
	}

	// Verify uniqueness (generate a second one)
	id2 := GenerateGroupID()
	if id == id2 {
		t.Errorf("expected unique IDs, but got same ID twice: %q", id)
	}
}
