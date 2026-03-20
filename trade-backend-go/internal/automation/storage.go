package automation

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/utils"
)

// Storage handles persistence of automation configurations
type Storage struct {
	filePath string
	mu       sync.RWMutex
	configs  map[string]*AutomationConfig
}

// StorageData represents the JSON structure for persistence
type StorageData struct {
	Version   string                       `json:"version"`
	UpdatedAt time.Time                    `json:"updated_at"`
	Configs   map[string]*AutomationConfig `json:"configs"`
}

// NewStorage creates a new storage instance using GlobalPathManager for consistent path resolution
func NewStorage() (*Storage, error) {
	// Use GlobalPathManager for consistent path resolution (same as provider configs)
	filePath := utils.GlobalPathManager.GetConfigFilePath("automations.json")

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	slog.Info("Automation storage initialized", "path", filePath)

	s := &Storage{
		filePath: filePath,
		configs:  make(map[string]*AutomationConfig),
	}

	// Load existing data if file exists
	if err := s.load(); err != nil {
		// If file doesn't exist, that's okay - start fresh
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load automations: %w", err)
		}
	}

	return s, nil
}

// load reads configurations from disk
func (s *Storage) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var storageData StorageData
	if err := json.Unmarshal(data, &storageData); err != nil {
		return fmt.Errorf("failed to parse automations file: %w", err)
	}

	s.configs = storageData.Configs
	if s.configs == nil {
		s.configs = make(map[string]*AutomationConfig)
	}

	// Run migrations in order: IDs first (so indicators have IDs before grouping), then groups
	needsSave := false
	if s.migrateIndicatorIDs() {
		needsSave = true
	}
	if s.migrateIndicatorGroups() {
		needsSave = true
	}
	if needsSave {
		if err := s.saveWithoutLock(); err != nil {
			slog.Warn("Failed to save migrated configs", "error", err)
		}
	}

	return nil
}

// migrateIndicatorIDs ensures all indicators have unique IDs
// Returns true if any migrations were made
func (s *Storage) migrateIndicatorIDs() bool {
	migrated := false

	for _, config := range s.configs {
		// Migrate flat Indicators (legacy)
		for i := range config.Indicators {
			if config.Indicators[i].ID == "" {
				config.Indicators[i].ID = types.GenerateIndicatorID()
				slog.Info("Migrated indicator ID",
					"automation", config.Name,
					"type", config.Indicators[i].Type,
					"newID", config.Indicators[i].ID)
				migrated = true
			}
		}
		// Migrate indicators within IndicatorGroups
		for g := range config.IndicatorGroups {
			for i := range config.IndicatorGroups[g].Indicators {
				if config.IndicatorGroups[g].Indicators[i].ID == "" {
					config.IndicatorGroups[g].Indicators[i].ID = types.GenerateIndicatorID()
					slog.Info("Migrated indicator ID (in group)",
						"automation", config.Name,
						"group", config.IndicatorGroups[g].Name,
						"type", config.IndicatorGroups[g].Indicators[i].Type,
						"newID", config.IndicatorGroups[g].Indicators[i].ID)
					migrated = true
				}
			}
		}
	}

	return migrated
}

// migrateIndicatorGroups migrates configs with flat Indicators into a single "Default" IndicatorGroup.
// Returns true if any migrations were made.
func (s *Storage) migrateIndicatorGroups() bool {
	migrated := false
	for _, config := range s.configs {
		if len(config.IndicatorGroups) > 0 {
			continue
		}
		if len(config.Indicators) == 0 {
			continue
		}
		config.IndicatorGroups = []types.IndicatorGroup{
			{
				ID:         types.GenerateGroupID(),
				Name:       "Default",
				Indicators: config.Indicators,
			},
		}
		config.Indicators = []types.IndicatorConfig{}
		slog.Info("Migrated indicators to group",
			"automation", config.Name,
			"groupName", "Default",
			"indicatorCount", len(config.IndicatorGroups[0].Indicators))
		migrated = true
	}
	return migrated
}

// save writes configurations to disk (acquires lock)
func (s *Storage) save() error {
	return s.saveWithoutLock()
}

// saveWithoutLock writes configurations to disk (caller must hold lock)
func (s *Storage) saveWithoutLock() error {
	storageData := StorageData{
		Version:   "1.1",
		UpdatedAt: time.Now(),
		Configs:   s.configs,
	}

	data, err := json.MarshalIndent(storageData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal automations: %w", err)
	}

	// Write to temp file first, then rename for atomic write
	tempFile := s.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write automations file: %w", err)
	}

	if err := os.Rename(tempFile, s.filePath); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to rename automations file: %w", err)
	}

	return nil
}

// GetAll returns all automation configurations
func (s *Storage) GetAll() []*AutomationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make([]*AutomationConfig, 0, len(s.configs))
	for _, config := range s.configs {
		configs = append(configs, config)
	}
	return configs
}

// Get returns a single automation configuration by ID
func (s *Storage) Get(id string) (*AutomationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.configs[id]
	if !exists {
		return nil, fmt.Errorf("automation config not found: %s", id)
	}
	return config, nil
}

// Create adds a new automation configuration
func (s *Storage) Create(config *AutomationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config.ID == "" {
		config.ID = generateID()
	}

	if _, exists := s.configs[config.ID]; exists {
		return fmt.Errorf("automation config already exists: %s", config.ID)
	}

	config.Created = time.Now()
	config.Updated = time.Now()
	s.configs[config.ID] = config

	return s.save()
}

// Update updates an existing automation configuration
func (s *Storage) Update(config *AutomationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.configs[config.ID]; !exists {
		return fmt.Errorf("automation config not found: %s", config.ID)
	}

	config.Updated = time.Now()
	s.configs[config.ID] = config

	return s.save()
}

// Delete removes an automation configuration
func (s *Storage) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.configs[id]; !exists {
		return fmt.Errorf("automation config not found: %s", id)
	}

	delete(s.configs, id)
	return s.save()
}

// GetEnabled returns all enabled automation configurations
func (s *Storage) GetEnabled() []*AutomationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configs := make([]*AutomationConfig, 0)
	for _, config := range s.configs {
		if config.Enabled {
			configs = append(configs, config)
		}
	}
	return configs
}

// SetEnabled enables or disables an automation
func (s *Storage) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, exists := s.configs[id]
	if !exists {
		return fmt.Errorf("automation config not found: %s", id)
	}

	config.Enabled = enabled
	config.Updated = time.Now()
	return s.save()
}
