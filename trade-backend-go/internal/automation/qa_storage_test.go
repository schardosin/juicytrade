package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"trade-backend-go/internal/automation/types"
)

// =============================================================================
// 4.1 Migration Idempotency
// =============================================================================

// 4.1.1: Run migrateIndicatorGroups on a config with flat indicators.
// Verify migration occurs (returns true). Then run it again on the same config.
// Verify it returns false (no-op) and the group structure is unchanged
// (same group name, same indicator count).
func TestQA_MigrateIndicatorGroups_IdempotentOnSecondRun(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Test Config",
				Indicators: []types.IndicatorConfig{
					{ID: "ind_1", Type: types.IndicatorVIX, Enabled: true, Operator: types.OperatorLessThan, Threshold: 20},
					{ID: "ind_2", Type: types.IndicatorGap, Enabled: true, Operator: types.OperatorLessThan, Threshold: 1.0},
				},
			},
		},
	}

	// First run — should migrate
	result1 := s.migrateIndicatorGroups()
	if !result1 {
		t.Fatal("expected first migrateIndicatorGroups to return true")
	}

	config := s.configs["cfg1"]
	if len(config.IndicatorGroups) != 1 {
		t.Fatalf("expected 1 group after first migration, got %d", len(config.IndicatorGroups))
	}

	// Record the state after first migration
	groupName := config.IndicatorGroups[0].Name
	groupID := config.IndicatorGroups[0].ID
	indicatorCount := len(config.IndicatorGroups[0].Indicators)

	// Second run — should be a no-op
	result2 := s.migrateIndicatorGroups()
	if result2 {
		t.Error("expected second migrateIndicatorGroups to return false (idempotent)")
	}

	// Verify structure is unchanged
	if len(config.IndicatorGroups) != 1 {
		t.Fatalf("expected 1 group after second run, got %d", len(config.IndicatorGroups))
	}
	if config.IndicatorGroups[0].Name != groupName {
		t.Errorf("group name changed: expected %q, got %q", groupName, config.IndicatorGroups[0].Name)
	}
	if config.IndicatorGroups[0].ID != groupID {
		t.Errorf("group ID changed: expected %q, got %q", groupID, config.IndicatorGroups[0].ID)
	}
	if len(config.IndicatorGroups[0].Indicators) != indicatorCount {
		t.Errorf("indicator count changed: expected %d, got %d", indicatorCount, len(config.IndicatorGroups[0].Indicators))
	}
}

// 4.1.2: Run migrateIndicatorIDs on a config with empty indicator IDs.
// Verify migration occurs. Record the generated IDs. Run it again.
// Verify returns false and IDs are unchanged.
func TestQA_MigrateIndicatorIDs_IdempotentOnSecondRun(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Test Config",
				Indicators: []types.IndicatorConfig{
					{ID: "", Type: types.IndicatorVIX, Enabled: true},
					{ID: "", Type: types.IndicatorGap, Enabled: true},
				},
			},
		},
	}

	// First run — should migrate
	result1 := s.migrateIndicatorIDs()
	if !result1 {
		t.Fatal("expected first migrateIndicatorIDs to return true")
	}

	config := s.configs["cfg1"]

	// Record generated IDs
	id0 := config.Indicators[0].ID
	id1 := config.Indicators[1].ID

	if id0 == "" || id1 == "" {
		t.Fatal("expected non-empty IDs after first migration")
	}
	if !strings.HasPrefix(id0, "ind_") || !strings.HasPrefix(id1, "ind_") {
		t.Errorf("expected IDs to start with 'ind_', got %q and %q", id0, id1)
	}

	// Second run — should be a no-op
	result2 := s.migrateIndicatorIDs()
	if result2 {
		t.Error("expected second migrateIndicatorIDs to return false (idempotent)")
	}

	// Verify IDs are unchanged
	if config.Indicators[0].ID != id0 {
		t.Errorf("indicator 0 ID changed: expected %q, got %q", id0, config.Indicators[0].ID)
	}
	if config.Indicators[1].ID != id1 {
		t.Errorf("indicator 1 ID changed: expected %q, got %q", id1, config.Indicators[1].ID)
	}
}

// 4.1.3: Config has both Indicators: [] and IndicatorGroups: [].
// Run migration twice. Both return false. Nothing changes.
func TestQA_MigrateIndicatorGroups_IdempotentWithEmptyConfig(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:              "cfg1",
				Name:            "Empty Config",
				Indicators:      []types.IndicatorConfig{},
				IndicatorGroups: []types.IndicatorGroup{},
			},
		},
	}

	result1 := s.migrateIndicatorGroups()
	if result1 {
		t.Error("expected first run to return false for empty config")
	}

	result2 := s.migrateIndicatorGroups()
	if result2 {
		t.Error("expected second run to return false for empty config")
	}

	config := s.configs["cfg1"]
	if len(config.Indicators) != 0 {
		t.Errorf("expected Indicators to remain empty, got %d", len(config.Indicators))
	}
	if len(config.IndicatorGroups) != 0 {
		t.Errorf("expected IndicatorGroups to remain empty, got %d", len(config.IndicatorGroups))
	}
}

// =============================================================================
// 4.2 Migration Ordering
// =============================================================================

// 4.2.1: Config with flat Indicators where one indicator has ID: "".
// Run migrateIndicatorIDs first, then migrateIndicatorGroups.
// Verify the indicator inside the resulting group has a non-empty ID
// (IDs were backfilled before grouping).
func TestQA_MigrationOrder_IDsBeforeGroups(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Needs Both Migrations",
				Indicators: []types.IndicatorConfig{
					{ID: "", Type: types.IndicatorVIX, Enabled: true, Operator: types.OperatorLessThan, Threshold: 20},
					{ID: "ind_existing", Type: types.IndicatorGap, Enabled: true, Operator: types.OperatorLessThan, Threshold: 1.0},
				},
			},
		},
	}

	// Step 1: backfill IDs
	idResult := s.migrateIndicatorIDs()
	if !idResult {
		t.Fatal("expected migrateIndicatorIDs to return true")
	}

	// Verify the ID was backfilled in the flat Indicators array
	config := s.configs["cfg1"]
	if config.Indicators[0].ID == "" {
		t.Fatal("expected first indicator to have a generated ID after migrateIndicatorIDs")
	}
	generatedID := config.Indicators[0].ID

	// Step 2: group migration
	groupResult := s.migrateIndicatorGroups()
	if !groupResult {
		t.Fatal("expected migrateIndicatorGroups to return true")
	}

	// Verify the indicator inside the group has the non-empty ID
	if len(config.IndicatorGroups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(config.IndicatorGroups))
	}
	groupInd := config.IndicatorGroups[0].Indicators[0]
	if groupInd.ID == "" {
		t.Error("expected indicator inside group to have a non-empty ID")
	}
	if groupInd.ID != generatedID {
		t.Errorf("expected indicator ID %q inside group, got %q", generatedID, groupInd.ID)
	}
}

// 4.2.2: Config already has IndicatorGroups with one indicator missing an ID.
// Run migrateIndicatorIDs. Verify it backfills the ID inside the group
// (not just in the legacy Indicators array).
func TestQA_MigrationOrder_IDsInGroupIndicators(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Group With Missing ID",
				IndicatorGroups: []types.IndicatorGroup{
					{
						ID:   "grp_1",
						Name: "Group A",
						Indicators: []types.IndicatorConfig{
							{ID: "", Type: types.IndicatorRSI, Enabled: true, Operator: types.OperatorGreaterThan, Threshold: 30},
							{ID: "ind_ok", Type: types.IndicatorVIX, Enabled: true},
						},
					},
				},
			},
		},
	}

	result := s.migrateIndicatorIDs()
	if !result {
		t.Fatal("expected migrateIndicatorIDs to return true")
	}

	config := s.configs["cfg1"]
	ind := config.IndicatorGroups[0].Indicators[0]
	if ind.ID == "" {
		t.Error("expected indicator inside group to have a backfilled ID")
	}
	if !strings.HasPrefix(ind.ID, "ind_") {
		t.Errorf("expected backfilled ID to start with 'ind_', got %q", ind.ID)
	}

	// Verify existing ID is unchanged
	ind2 := config.IndicatorGroups[0].Indicators[1]
	if ind2.ID != "ind_ok" {
		t.Errorf("expected existing ID 'ind_ok' unchanged, got %q", ind2.ID)
	}
}

// 4.2.3: Config with flat Indicators, two have empty IDs. Simulate the full
// load() migration chain: migrateIndicatorIDs then migrateIndicatorGroups.
// Verify: (1) both indicators now have IDs, (2) they are in a single "Default"
// group, (3) config.Indicators is empty, (4) overall result is true (needsSave).
func TestQA_MigrationOrder_FullChain(t *testing.T) {
	s := &Storage{
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Full Chain Test",
				Indicators: []types.IndicatorConfig{
					{ID: "", Type: types.IndicatorVIX, Enabled: true, Operator: types.OperatorLessThan, Threshold: 20},
					{ID: "", Type: types.IndicatorGap, Enabled: true, Operator: types.OperatorLessThan, Threshold: 1.5, Symbol: "SPY"},
				},
			},
		},
	}

	// Simulate the load() migration chain
	needsSave := false
	if s.migrateIndicatorIDs() {
		needsSave = true
	}
	if s.migrateIndicatorGroups() {
		needsSave = true
	}

	// (4) needsSave should be true
	if !needsSave {
		t.Error("expected needsSave to be true after full migration chain")
	}

	config := s.configs["cfg1"]

	// (3) config.Indicators should be empty
	if len(config.Indicators) != 0 {
		t.Errorf("expected Indicators to be empty after migration, got %d", len(config.Indicators))
	}

	// (2) Single "Default" group
	if len(config.IndicatorGroups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(config.IndicatorGroups))
	}
	if config.IndicatorGroups[0].Name != "Default" {
		t.Errorf("expected group name 'Default', got %q", config.IndicatorGroups[0].Name)
	}

	// (1) Both indicators have IDs
	for i, ind := range config.IndicatorGroups[0].Indicators {
		if ind.ID == "" {
			t.Errorf("expected indicator[%d] to have an ID, got empty", i)
		}
		if !strings.HasPrefix(ind.ID, "ind_") {
			t.Errorf("expected indicator[%d] ID to start with 'ind_', got %q", i, ind.ID)
		}
	}

	// Verify indicator data is preserved
	if len(config.IndicatorGroups[0].Indicators) != 2 {
		t.Fatalf("expected 2 indicators in group, got %d", len(config.IndicatorGroups[0].Indicators))
	}
	if config.IndicatorGroups[0].Indicators[0].Type != types.IndicatorVIX {
		t.Errorf("expected first indicator type VIX, got %q", config.IndicatorGroups[0].Indicators[0].Type)
	}
	if config.IndicatorGroups[0].Indicators[1].Symbol != "SPY" {
		t.Errorf("expected second indicator symbol 'SPY', got %q", config.IndicatorGroups[0].Indicators[1].Symbol)
	}
}

// =============================================================================
// 4.3 JSON Round-Trip Persistence
// =============================================================================

// helper: create a temp file for storage tests
func createTempFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "automations.json")
}

// helper: write JSON to a file
func writeJSON(t *testing.T, path string, data interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

// 4.3.1: Create a Storage with a temp file. Insert a config with 2 indicator
// groups (each with 2 indicators). Call saveWithoutLock(). Create a new Storage
// pointing to the same file. Call load(). Verify the loaded config has identical
// groups, indicators, IDs, names, thresholds, operators, params.
func TestQA_Storage_JSONPersistence_GroupsSurviveWriteRead(t *testing.T) {
	tmpFile := createTempFile(t)

	// Create and populate storage
	s1 := &Storage{
		filePath: tmpFile,
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:         "cfg1",
				Name:       "Persist Test",
				Symbol:     "SPX",
				Indicators: []types.IndicatorConfig{}, // empty legacy
				IndicatorGroups: []types.IndicatorGroup{
					{
						ID:   "grp_1",
						Name: "Low Vol",
						Indicators: []types.IndicatorConfig{
							{
								ID:        "ind_vix",
								Type:      types.IndicatorVIX,
								Enabled:   true,
								Operator:  types.OperatorLessThan,
								Threshold: 20,
							},
							{
								ID:        "ind_rsi",
								Type:      types.IndicatorRSI,
								Enabled:   true,
								Operator:  types.OperatorGreaterThan,
								Threshold: 30,
								Symbol:    "SPY",
								Params:    map[string]float64{"period": 21},
							},
						},
					},
					{
						ID:   "grp_2",
						Name: "High Vol",
						Indicators: []types.IndicatorConfig{
							{
								ID:        "ind_atr",
								Type:      types.IndicatorATR,
								Enabled:   false,
								Operator:  types.OperatorLessThan,
								Threshold: 5.0,
								Symbol:    "QQQ",
								Params:    map[string]float64{"period": 14},
							},
							{
								ID:        "ind_bb",
								Type:      types.IndicatorBBPercent,
								Enabled:   true,
								Operator:  types.OperatorGreaterThan,
								Threshold: 0.8,
								Symbol:    "QQQ",
								Params:    map[string]float64{"period": 20, "std_dev": 2.0},
							},
						},
					},
				},
				EntryTime:     "12:25",
				EntryTimezone: "America/New_York",
				Enabled:       true,
			},
		},
	}

	// Save to disk
	if err := s1.saveWithoutLock(); err != nil {
		t.Fatalf("saveWithoutLock failed: %v", err)
	}

	// Load from disk into a fresh Storage
	s2 := &Storage{
		filePath: tmpFile,
		configs:  make(map[string]*AutomationConfig),
	}
	if err := s2.load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// Verify the loaded config matches
	config, exists := s2.configs["cfg1"]
	if !exists {
		t.Fatal("config 'cfg1' not found after load")
	}
	if config.Name != "Persist Test" {
		t.Errorf("Name: expected 'Persist Test', got %q", config.Name)
	}
	if config.Symbol != "SPX" {
		t.Errorf("Symbol: expected 'SPX', got %q", config.Symbol)
	}

	// Verify groups
	if len(config.IndicatorGroups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(config.IndicatorGroups))
	}

	g1 := config.IndicatorGroups[0]
	if g1.ID != "grp_1" {
		t.Errorf("group 1 ID: expected 'grp_1', got %q", g1.ID)
	}
	if g1.Name != "Low Vol" {
		t.Errorf("group 1 Name: expected 'Low Vol', got %q", g1.Name)
	}
	if len(g1.Indicators) != 2 {
		t.Fatalf("group 1: expected 2 indicators, got %d", len(g1.Indicators))
	}
	if g1.Indicators[0].ID != "ind_vix" || g1.Indicators[0].Threshold != 20 {
		t.Errorf("group 1 ind 0: unexpected ID=%q Threshold=%v", g1.Indicators[0].ID, g1.Indicators[0].Threshold)
	}
	if g1.Indicators[1].Params["period"] != 21 {
		t.Errorf("group 1 ind 1 params[period]: expected 21, got %v", g1.Indicators[1].Params["period"])
	}
	if g1.Indicators[1].Symbol != "SPY" {
		t.Errorf("group 1 ind 1 symbol: expected 'SPY', got %q", g1.Indicators[1].Symbol)
	}

	g2 := config.IndicatorGroups[1]
	if g2.ID != "grp_2" {
		t.Errorf("group 2 ID: expected 'grp_2', got %q", g2.ID)
	}
	if g2.Indicators[0].Enabled != false {
		t.Error("group 2 ind 0: expected Enabled=false")
	}
	if g2.Indicators[0].Operator != types.OperatorLessThan {
		t.Errorf("group 2 ind 0: expected operator 'lt', got %q", g2.Indicators[0].Operator)
	}
	if g2.Indicators[1].Params["std_dev"] != 2.0 {
		t.Errorf("group 2 ind 1 params[std_dev]: expected 2.0, got %v", g2.Indicators[1].Params["std_dev"])
	}
}

// 4.3.2: After saveWithoutLock, read the raw JSON file.
// Verify "version": "1.1" is present.
func TestQA_Storage_JSONPersistence_VersionBump(t *testing.T) {
	tmpFile := createTempFile(t)

	s := &Storage{
		filePath: tmpFile,
		configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:   "cfg1",
				Name: "Version Test",
			},
		},
	}

	if err := s.saveWithoutLock(); err != nil {
		t.Fatalf("saveWithoutLock failed: %v", err)
	}

	// Read raw JSON from disk
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	rawJSON := string(data)
	if !strings.Contains(rawJSON, `"version": "1.1"`) {
		t.Errorf("expected JSON to contain '\"version\": \"1.1\"', got:\n%s", rawJSON)
	}
}

// 4.3.3: Write a v1.0-style JSON file (flat indicators, no indicator_groups).
// Create a Storage that loads this file. Verify:
// (1) config now has IndicatorGroups with a "Default" group
// (2) Indicators is empty
// (3) the file on disk is updated with v1.1 format
func TestQA_Storage_JSONPersistence_LegacyFileAutoMigrates(t *testing.T) {
	tmpFile := createTempFile(t)

	// Write a v1.0-style JSON file
	legacyData := StorageData{
		Version: "1.0",
		Configs: map[string]*AutomationConfig{
			"cfg_legacy": {
				ID:     "cfg_legacy",
				Name:   "Legacy Config",
				Symbol: "NDX",
				Indicators: []types.IndicatorConfig{
					{ID: "ind_1", Type: types.IndicatorVIX, Enabled: true, Operator: types.OperatorLessThan, Threshold: 20},
					{ID: "ind_2", Type: types.IndicatorGap, Enabled: true, Operator: types.OperatorLessThan, Threshold: 1.5, Symbol: "SPY"},
				},
				EntryTime:     "12:25",
				EntryTimezone: "America/New_York",
				Enabled:       true,
			},
		},
	}
	writeJSON(t, tmpFile, legacyData)

	// Load into a new Storage — should trigger auto-migration
	s := &Storage{
		filePath: tmpFile,
		configs:  make(map[string]*AutomationConfig),
	}
	if err := s.load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	config := s.configs["cfg_legacy"]
	if config == nil {
		t.Fatal("config 'cfg_legacy' not found after load")
	}

	// (1) Should have IndicatorGroups with a "Default" group
	if len(config.IndicatorGroups) != 1 {
		t.Fatalf("expected 1 indicator group, got %d", len(config.IndicatorGroups))
	}
	if config.IndicatorGroups[0].Name != "Default" {
		t.Errorf("expected group name 'Default', got %q", config.IndicatorGroups[0].Name)
	}
	if len(config.IndicatorGroups[0].Indicators) != 2 {
		t.Errorf("expected 2 indicators in Default group, got %d", len(config.IndicatorGroups[0].Indicators))
	}

	// (2) Indicators should be empty
	if len(config.Indicators) != 0 {
		t.Errorf("expected Indicators to be empty after migration, got %d", len(config.Indicators))
	}

	// (3) File on disk should be updated with v1.1
	updatedData, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	rawJSON := string(updatedData)
	if !strings.Contains(rawJSON, `"version": "1.1"`) {
		t.Errorf("expected updated file to contain '\"version\": \"1.1\"'")
	}
	// The file should also contain indicator_groups
	if !strings.Contains(rawJSON, `"indicator_groups"`) {
		t.Errorf("expected updated file to contain 'indicator_groups'")
	}
}

// 4.3.4: After migration, verify the JSON file contains "indicators": []
// (empty array, not omitted and not null) for backward compatibility
// with older frontends.
func TestQA_Storage_JSONPersistence_EmptyIndicatorsField(t *testing.T) {
	tmpFile := createTempFile(t)

	// Write a v1.0-style JSON file with flat indicators
	legacyData := StorageData{
		Version: "1.0",
		Configs: map[string]*AutomationConfig{
			"cfg1": {
				ID:     "cfg1",
				Name:   "Empty Indicators Test",
				Symbol: "SPX",
				Indicators: []types.IndicatorConfig{
					{ID: "ind_1", Type: types.IndicatorVIX, Enabled: true, Operator: types.OperatorLessThan, Threshold: 20},
				},
			},
		},
	}
	writeJSON(t, tmpFile, legacyData)

	// Load — triggers migration
	s := &Storage{
		filePath: tmpFile,
		configs:  make(map[string]*AutomationConfig),
	}
	if err := s.load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// Read the raw JSON that was written during migration
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	rawJSON := string(data)

	// Verify "indicators": [] is present (empty array, not null, not omitted)
	if !strings.Contains(rawJSON, `"indicators": []`) {
		t.Errorf("expected JSON to contain '\"indicators\": []' (empty array), got:\n%s", rawJSON)
	}

	// Make sure it's not null
	if strings.Contains(rawJSON, `"indicators": null`) {
		t.Errorf("expected 'indicators' to be [] not null")
	}
}
