package automation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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

// NewStorage creates a new storage instance
func NewStorage(dataDir string) (*Storage, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	filePath := filepath.Join(dataDir, "automations.json")
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

	return nil
}

// save writes configurations to disk
func (s *Storage) save() error {
	storageData := StorageData{
		Version:   "1.0",
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
