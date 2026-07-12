package automation

import (
	"path/filepath"
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

func TestMigrateMaxCapitalMode_EmptyNormalizesToFixed(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:          "cfg1",
				Name:        "Legacy Fixed",
				TradeConfig: types.TradeConfiguration{MaxCapital: 5000}, // MaxCapitalMode == ""
			},
		},
	}

	if migrated := s.migrateMaxCapitalMode(); !migrated {
		t.Error("expected migration to return true for empty mode")
	}
	if got := s.configs["cfg1"].TradeConfig.MaxCapitalMode; got != types.MaxCapitalModeFixed {
		t.Errorf("expected mode normalized to fixed, got %q", got)
	}
	// MaxCapital must be untouched (behavior-preserving).
	if s.configs["cfg1"].TradeConfig.MaxCapital != 5000 {
		t.Errorf("MaxCapital should be unchanged, got %v", s.configs["cfg1"].TradeConfig.MaxCapital)
	}
}

func TestMigrateMaxCapitalMode_PercentLeftUntouched(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Percent",
				TradeConfig: types.TradeConfiguration{
					MaxCapitalMode:    types.MaxCapitalModePercent,
					MaxCapitalPercent: 60,
				},
			},
		},
	}

	if migrated := s.migrateMaxCapitalMode(); migrated {
		t.Error("expected no migration for a config already in percent mode")
	}
	if got := s.configs["cfg1"].TradeConfig.MaxCapitalMode; got != types.MaxCapitalModePercent {
		t.Errorf("percent mode must be left untouched, got %q", got)
	}
}

func TestMigrateMaxCapitalMode_ExplicitFixedNotChanged(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:          "cfg1",
				Name:        "Explicit Fixed",
				TradeConfig: types.TradeConfiguration{MaxCapitalMode: types.MaxCapitalModeFixed, MaxCapital: 5000},
			},
		},
	}
	if migrated := s.migrateMaxCapitalMode(); migrated {
		t.Error("expected no migration for a config already explicitly fixed")
	}
}

// ---- GAP G-2 (test-plan.md): config persistence round-trip ----

// TestStorage_PercentConfigRoundTrip creates a percent-mode config via the
// Storage API, persists it to a temp file, then loads it into a brand-new
// Storage instance pointing at the same file and asserts that
// max_capital_mode and max_capital_percent survive the disk round-trip.
// A t.TempDir()-scoped file is used so nothing pollutes the working tree.
func TestStorage_PercentConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "automations.json")

	// Writer store: seed via Create() (which calls save()).
	writer := &Storage{
		filePath: path,
		configs:  make(map[string]*AutomationConfig),
	}
	cfg := &AutomationConfig{
		ID:     "cfg-percent",
		Name:   "Percent RoundTrip",
		Symbol: "SPX",
		TradeConfig: types.TradeConfiguration{
			Strategy:          types.StrategyPutSpread,
			Width:             20,
			MaxCapital:        5000, // retained but unused in percent mode
			MaxCapitalMode:    types.MaxCapitalModePercent,
			MaxCapitalPercent: 60,
		},
	}
	if err := writer.Create(cfg); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Reader store: fresh instance, same file, load from disk.
	reader := &Storage{
		filePath: path,
		configs:  make(map[string]*AutomationConfig),
	}
	if err := reader.load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	got, err := reader.Get("cfg-percent")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.TradeConfig.MaxCapitalMode != types.MaxCapitalModePercent {
		t.Errorf("MaxCapitalMode: got %q, want %q", got.TradeConfig.MaxCapitalMode, types.MaxCapitalModePercent)
	}
	if got.TradeConfig.MaxCapitalPercent != 60 {
		t.Errorf("MaxCapitalPercent: got %v, want 60", got.TradeConfig.MaxCapitalPercent)
	}
	// MaxCapital is retained across the round-trip (used when toggling back to fixed).
	if got.TradeConfig.MaxCapital != 5000 {
		t.Errorf("MaxCapital: got %v, want 5000", got.TradeConfig.MaxCapital)
	}
}

// TestStorage_UpdateToPercentRoundTrip creates a fixed config, updates it to
// percent mode via Update() (which persists), then reloads from disk and
// asserts the percent fields survived the update + round-trip.
func TestStorage_UpdateToPercentRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "automations.json")

	writer := &Storage{
		filePath: path,
		configs:  make(map[string]*AutomationConfig),
	}
	// Seed a fixed config.
	if err := writer.Create(&AutomationConfig{
		ID:     "cfg-upd",
		Name:   "Starts Fixed",
		Symbol: "SPX",
		TradeConfig: types.TradeConfiguration{
			Strategy:       types.StrategyPutSpread,
			Width:          20,
			MaxCapital:     5000,
			MaxCapitalMode: types.MaxCapitalModeFixed,
		},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update it to percent mode.
	if err := writer.Update(&AutomationConfig{
		ID:     "cfg-upd",
		Name:   "Now Percent",
		Symbol: "SPX",
		TradeConfig: types.TradeConfiguration{
			Strategy:          types.StrategyPutSpread,
			Width:             20,
			MaxCapital:        5000,
			MaxCapitalMode:    types.MaxCapitalModePercent,
			MaxCapitalPercent: 42,
		},
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Reload from disk into a fresh store.
	reader := &Storage{
		filePath: path,
		configs:  make(map[string]*AutomationConfig),
	}
	if err := reader.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	got, err := reader.Get("cfg-upd")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.TradeConfig.MaxCapitalMode != types.MaxCapitalModePercent {
		t.Errorf("MaxCapitalMode: got %q, want %q", got.TradeConfig.MaxCapitalMode, types.MaxCapitalModePercent)
	}
	if got.TradeConfig.MaxCapitalPercent != 42 {
		t.Errorf("MaxCapitalPercent: got %v, want 42", got.TradeConfig.MaxCapitalPercent)
	}
}
