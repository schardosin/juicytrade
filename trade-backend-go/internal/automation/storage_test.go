package automation

import (
	"strings"
	"testing"

	"trade-backend-go/internal/automation/types"
)

func TestMigrateIndicatorGroups_FlatToGroup(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Test Automation",
				Indicators: []types.IndicatorConfig{
					{ID: "ind_1", Type: types.IndicatorVIX, Enabled: true, Operator: types.OperatorLessThan, Threshold: 20},
					{ID: "ind_2", Type: types.IndicatorGap, Enabled: true, Operator: types.OperatorLessThan, Threshold: 1.0},
				},
			},
		},
	}

	result := s.migrateIndicatorGroups()

	if !result {
		t.Error("expected migration to return true")
	}

	config := s.configs["cfg1"]

	// Indicators should be cleared
	if len(config.Indicators) != 0 {
		t.Errorf("expected Indicators to be empty after migration, got %d", len(config.Indicators))
	}

	// IndicatorGroups should have 1 group
	if len(config.IndicatorGroups) != 1 {
		t.Fatalf("expected 1 IndicatorGroup, got %d", len(config.IndicatorGroups))
	}

	group := config.IndicatorGroups[0]
	if group.Name != "Default" {
		t.Errorf("expected group name 'Default', got %q", group.Name)
	}
	if !strings.HasPrefix(group.ID, "grp_") {
		t.Errorf("expected group ID to start with 'grp_', got %q", group.ID)
	}
	if len(group.Indicators) != 2 {
		t.Errorf("expected 2 indicators in group, got %d", len(group.Indicators))
	}
	if group.Indicators[0].ID != "ind_1" {
		t.Errorf("expected first indicator ID 'ind_1', got %q", group.Indicators[0].ID)
	}
	if group.Indicators[1].ID != "ind_2" {
		t.Errorf("expected second indicator ID 'ind_2', got %q", group.Indicators[1].ID)
	}
}

func TestMigrateIndicatorGroups_AlreadyMigrated(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Already Migrated",
				IndicatorGroups: []types.IndicatorGroup{
					{
						ID:   "grp_existing",
						Name: "Existing Group",
						Indicators: []types.IndicatorConfig{
							{ID: "ind_1", Type: types.IndicatorVIX, Enabled: true},
						},
					},
				},
			},
		},
	}

	result := s.migrateIndicatorGroups()

	if result {
		t.Error("expected migration to return false for already migrated config")
	}

	config := s.configs["cfg1"]
	if len(config.IndicatorGroups) != 1 {
		t.Errorf("expected IndicatorGroups unchanged at 1, got %d", len(config.IndicatorGroups))
	}
	if config.IndicatorGroups[0].ID != "grp_existing" {
		t.Errorf("expected group ID unchanged 'grp_existing', got %q", config.IndicatorGroups[0].ID)
	}
}

func TestMigrateIndicatorGroups_EmptyIndicators(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:         "cfg1",
				Name:       "Empty Config",
				Indicators: []types.IndicatorConfig{},
			},
		},
	}

	result := s.migrateIndicatorGroups()

	if result {
		t.Error("expected migration to return false for empty indicators")
	}

	config := s.configs["cfg1"]
	if len(config.IndicatorGroups) != 0 {
		t.Errorf("expected no IndicatorGroups for empty config, got %d", len(config.IndicatorGroups))
	}
}

func TestMigrateIndicatorGroups_MultipleConfigs(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"unmigrated": {
				ID:   "unmigrated",
				Name: "Needs Migration",
				Indicators: []types.IndicatorConfig{
					{ID: "ind_1", Type: types.IndicatorVIX, Enabled: true},
				},
			},
			"already_done": {
				ID:   "already_done",
				Name: "Already Done",
				IndicatorGroups: []types.IndicatorGroup{
					{ID: "grp_1", Name: "Group 1", Indicators: []types.IndicatorConfig{
						{ID: "ind_2", Type: types.IndicatorGap, Enabled: true},
					}},
				},
			},
			"empty": {
				ID:         "empty",
				Name:       "Empty",
				Indicators: []types.IndicatorConfig{},
			},
		},
	}

	result := s.migrateIndicatorGroups()

	if !result {
		t.Error("expected migration to return true (one config needed migration)")
	}

	// Unmigrated config should now have a group
	unmigrated := s.configs["unmigrated"]
	if len(unmigrated.IndicatorGroups) != 1 {
		t.Fatalf("expected 1 group for unmigrated config, got %d", len(unmigrated.IndicatorGroups))
	}
	if unmigrated.IndicatorGroups[0].Name != "Default" {
		t.Errorf("expected group name 'Default', got %q", unmigrated.IndicatorGroups[0].Name)
	}
	if len(unmigrated.Indicators) != 0 {
		t.Errorf("expected Indicators cleared, got %d", len(unmigrated.Indicators))
	}

	// Already done config should be unchanged
	alreadyDone := s.configs["already_done"]
	if len(alreadyDone.IndicatorGroups) != 1 {
		t.Errorf("expected already_done groups unchanged at 1, got %d", len(alreadyDone.IndicatorGroups))
	}
	if alreadyDone.IndicatorGroups[0].ID != "grp_1" {
		t.Errorf("expected already_done group ID unchanged, got %q", alreadyDone.IndicatorGroups[0].ID)
	}

	// Empty config should remain empty
	empty := s.configs["empty"]
	if len(empty.IndicatorGroups) != 0 {
		t.Errorf("expected empty config to have no groups, got %d", len(empty.IndicatorGroups))
	}
}

func TestMigrateIndicatorIDs_AlsoCoversGroups(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Config With Groups",
				IndicatorGroups: []types.IndicatorGroup{
					{
						ID:   "grp_1",
						Name: "Group A",
						Indicators: []types.IndicatorConfig{
							{ID: "", Type: types.IndicatorVIX, Enabled: true},             // Empty ID - needs migration
							{ID: "ind_existing", Type: types.IndicatorGap, Enabled: true}, // Has ID - skip
						},
					},
					{
						ID:   "grp_2",
						Name: "Group B",
						Indicators: []types.IndicatorConfig{
							{ID: "", Type: types.IndicatorRSI, Enabled: true}, // Empty ID - needs migration
						},
					},
				},
			},
		},
	}

	result := s.migrateIndicatorIDs()

	if !result {
		t.Error("expected migration to return true")
	}

	config := s.configs["cfg1"]

	// Group A: first indicator should now have an ID
	ind0 := config.IndicatorGroups[0].Indicators[0]
	if ind0.ID == "" {
		t.Error("expected first indicator in Group A to have a generated ID")
	}
	if !strings.HasPrefix(ind0.ID, "ind_") {
		t.Errorf("expected generated ID to start with 'ind_', got %q", ind0.ID)
	}

	// Group A: second indicator should be unchanged
	ind1 := config.IndicatorGroups[0].Indicators[1]
	if ind1.ID != "ind_existing" {
		t.Errorf("expected existing ID unchanged, got %q", ind1.ID)
	}

	// Group B: indicator should now have an ID
	ind2 := config.IndicatorGroups[1].Indicators[0]
	if ind2.ID == "" {
		t.Error("expected indicator in Group B to have a generated ID")
	}
	if !strings.HasPrefix(ind2.ID, "ind_") {
		t.Errorf("expected generated ID to start with 'ind_', got %q", ind2.ID)
	}
}
