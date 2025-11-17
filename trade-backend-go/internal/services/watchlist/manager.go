package watchlist

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"trade-backend-go/internal/models"

	"github.com/google/uuid"
)

// Manager handles watchlist operations with JSON file storage
type Manager struct {
	mu               sync.RWMutex
	watchlists       map[string]*models.Watchlist
	activeWatchlist  string
	version          string
	configFile       string
	configDir        string
}

// watchlistConfig represents the JSON structure for persistence
type watchlistConfig struct {
	Watchlists      map[string]*models.Watchlist `json:"watchlists"`
	ActiveWatchlist string                       `json:"active_watchlist"`
	Version         string                       `json:"version"`
	LastUpdated     string                       `json:"last_updated"`
}

var (
	globalManager *Manager
	once          sync.Once
)

// GetManager returns the global watchlist manager instance
func GetManager() *Manager {
	once.Do(func() {
		globalManager = NewManager("watchlist.json")
	})
	return globalManager
}

// NewManager creates a new watchlist manager
func NewManager(configFile string) *Manager {
	// Get config directory from environment or use default
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "./data/config"
	}

	m := &Manager{
		watchlists:      make(map[string]*models.Watchlist),
		activeWatchlist: "default",
		version:         "1.0",
		configFile:      configFile,
		configDir:       configDir,
	}

	m.loadConfig()
	return m
}

// loadConfig loads watchlist configuration from JSON file
func (m *Manager) loadConfig() {
	configPath := filepath.Join(m.configDir, m.configFile)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("📋 Watchlist config file not found, creating default configuration")
		m.createDefaultConfig()
		return
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("❌ Error reading watchlist config: %v", err)
		m.createDefaultConfig()
		return
	}

	// Parse JSON
	var config watchlistConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("❌ Error parsing watchlist config: %v", err)
		m.createDefaultConfig()
		return
	}

	m.watchlists = config.Watchlists
	m.activeWatchlist = config.ActiveWatchlist
	m.version = config.Version

	log.Printf("📋 Loaded %d watchlists from %s", len(m.watchlists), configPath)
}

// createDefaultConfig creates default watchlist configuration
func (m *Manager) createDefaultConfig() {
	now := time.Now().UTC()

	m.watchlists = map[string]*models.Watchlist{
		"default": {
			ID:        "default",
			Name:      "My Watchlist",
			Symbols:   []string{"SPY", "QQQ", "AAPL"},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	m.activeWatchlist = "default"
	m.version = "1.0"

	m.saveConfig()
}

// saveConfig saves watchlist configuration to JSON file with atomic write
func (m *Manager) saveConfig() error {
	// Ensure config directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Prepare data structure
	config := watchlistConfig{
		Watchlists:      m.watchlists,
		ActiveWatchlist: m.activeWatchlist,
		Version:         m.version,
		LastUpdated:     time.Now().UTC().Format(time.RFC3339),
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Atomic write - write to temp file first, then rename
	configPath := filepath.Join(m.configDir, m.configFile)
	tempFile := configPath + ".tmp"

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, configPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	log.Printf("💾 Saved watchlist config to %s", configPath)
	return nil
}

// GetAllWatchlists returns all watchlists with metadata
func (m *Manager) GetAllWatchlists() *models.WatchlistsResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &models.WatchlistsResponse{
		Watchlists:      m.watchlists,
		ActiveWatchlist: m.activeWatchlist,
		TotalWatchlists: len(m.watchlists),
		Version:         m.version,
	}
}

// GetWatchlist returns a specific watchlist by ID
func (m *Manager) GetWatchlist(watchlistID string) *models.Watchlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.watchlists[watchlistID]
}

// CreateWatchlist creates a new watchlist
func (m *Manager) CreateWatchlist(name string, symbols []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("watchlist name cannot be empty")
	}

	// Generate unique ID
	watchlistID := m.generateWatchlistID(name)

	// Check if ID already exists (shouldn't happen with UUID, but be safe)
	if _, exists := m.watchlists[watchlistID]; exists {
		return "", fmt.Errorf("watchlist with ID '%s' already exists", watchlistID)
	}

	// Create watchlist
	now := time.Now().UTC()
	if symbols == nil {
		symbols = []string{}
	}

	m.watchlists[watchlistID] = &models.Watchlist{
		ID:        watchlistID,
		Name:      name,
		Symbols:   symbols,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := m.saveConfig(); err != nil {
		return "", err
	}

	log.Printf("✅ Created watchlist '%s' with ID '%s'", name, watchlistID)
	return watchlistID, nil
}

// UpdateWatchlist updates an existing watchlist
func (m *Manager) UpdateWatchlist(watchlistID string, name *string, symbols []string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	watchlist, exists := m.watchlists[watchlistID]
	if !exists {
		return false, fmt.Errorf("watchlist '%s' not found", watchlistID)
	}

	updated := false

	if name != nil && strings.TrimSpace(*name) != "" {
		watchlist.Name = strings.TrimSpace(*name)
		updated = true
	}

	if symbols != nil {
		// Clean symbols
		cleanSymbols := []string{}
		for _, s := range symbols {
			if trimmed := strings.TrimSpace(strings.ToUpper(s)); trimmed != "" {
				cleanSymbols = append(cleanSymbols, trimmed)
			}
		}
		watchlist.Symbols = cleanSymbols
		updated = true
	}

	if updated {
		watchlist.UpdatedAt = time.Now().UTC()
		if err := m.saveConfig(); err != nil {
			return false, err
		}
		log.Printf("✅ Updated watchlist '%s'", watchlistID)
	}

	return updated, nil
}

// DeleteWatchlist deletes a watchlist
func (m *Manager) DeleteWatchlist(watchlistID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watchlists[watchlistID]; !exists {
		return fmt.Errorf("watchlist '%s' not found", watchlistID)
	}

	// Don't allow deleting the last watchlist
	if len(m.watchlists) <= 1 {
		return fmt.Errorf("cannot delete the last remaining watchlist")
	}

	// If deleting the active watchlist, switch to another one
	if m.activeWatchlist == watchlistID {
		// Find another watchlist to make active
		for id := range m.watchlists {
			if id != watchlistID {
				m.activeWatchlist = id
				break
			}
		}
	}

	// Delete the watchlist
	delete(m.watchlists, watchlistID)

	if err := m.saveConfig(); err != nil {
		return err
	}

	log.Printf("✅ Deleted watchlist '%s'", watchlistID)
	return nil
}

// AddSymbol adds a symbol to a watchlist
func (m *Manager) AddSymbol(watchlistID, symbol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	watchlist, exists := m.watchlists[watchlistID]
	if !exists {
		return fmt.Errorf("watchlist '%s' not found", watchlistID)
	}

	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}

	// Check if symbol already exists
	for _, s := range watchlist.Symbols {
		if s == symbol {
			return fmt.Errorf("symbol '%s' already exists in watchlist", symbol)
		}
	}

	// Add symbol
	watchlist.Symbols = append(watchlist.Symbols, symbol)
	watchlist.UpdatedAt = time.Now().UTC()

	if err := m.saveConfig(); err != nil {
		return err
	}

	log.Printf("✅ Added '%s' to watchlist '%s'", symbol, watchlistID)
	return nil
}

// RemoveSymbol removes a symbol from a watchlist
func (m *Manager) RemoveSymbol(watchlistID, symbol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	watchlist, exists := m.watchlists[watchlistID]
	if !exists {
		return fmt.Errorf("watchlist '%s' not found", watchlistID)
	}

	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}

	// Find and remove symbol
	found := false
	newSymbols := []string{}
	for _, s := range watchlist.Symbols {
		if s != symbol {
			newSymbols = append(newSymbols, s)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("symbol '%s' not found in watchlist", symbol)
	}

	watchlist.Symbols = newSymbols
	watchlist.UpdatedAt = time.Now().UTC()

	if err := m.saveConfig(); err != nil {
		return err
	}

	log.Printf("✅ Removed '%s' from watchlist '%s'", symbol, watchlistID)
	return nil
}

// SetActiveWatchlist sets the active watchlist
func (m *Manager) SetActiveWatchlist(watchlistID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watchlists[watchlistID]; !exists {
		return fmt.Errorf("watchlist '%s' not found", watchlistID)
	}

	m.activeWatchlist = watchlistID

	if err := m.saveConfig(); err != nil {
		return err
	}

	log.Printf("✅ Set active watchlist to '%s'", watchlistID)
	return nil
}

// GetActiveWatchlist returns the currently active watchlist
func (m *Manager) GetActiveWatchlist() *models.Watchlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.watchlists[m.activeWatchlist]
}

// GetActiveWatchlistID returns the ID of the currently active watchlist
func (m *Manager) GetActiveWatchlistID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.activeWatchlist
}

// GetAllSymbols returns all unique symbols across all watchlists
func (m *Manager) GetAllSymbols() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	symbolSet := make(map[string]bool)
	for _, watchlist := range m.watchlists {
		for _, symbol := range watchlist.Symbols {
			symbolSet[symbol] = true
		}
	}

	symbols := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbols = append(symbols, symbol)
	}

	// Sort symbols
	// Simple bubble sort for small lists
	for i := 0; i < len(symbols); i++ {
		for j := i + 1; j < len(symbols); j++ {
			if symbols[i] > symbols[j] {
				symbols[i], symbols[j] = symbols[j], symbols[i]
			}
		}
	}

	return symbols
}

// SearchWatchlists searches watchlists by name or symbols
func (m *Manager) SearchWatchlists(query string) []*models.Watchlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		// Return all watchlists
		result := make([]*models.Watchlist, 0, len(m.watchlists))
		for _, watchlist := range m.watchlists {
			result = append(result, watchlist)
		}
		return result
	}

	matches := []*models.Watchlist{}
	for _, watchlist := range m.watchlists {
		// Search in name
		if strings.Contains(strings.ToLower(watchlist.Name), query) {
			matches = append(matches, watchlist)
			continue
		}

		// Search in symbols
		for _, symbol := range watchlist.Symbols {
			if strings.Contains(strings.ToLower(symbol), query) {
				matches = append(matches, watchlist)
				break
			}
		}
	}

	return matches
}

// ValidateSymbol performs basic symbol validation
func (m *Manager) ValidateSymbol(symbol string) bool {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return false
	}

	// Basic validation - alphanumeric, 1-10 characters
	if len(symbol) < 1 || len(symbol) > 10 {
		return false
	}

	for _, r := range symbol {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}

	return true
}

// generateWatchlistID generates a unique ID for a watchlist
func (m *Manager) generateWatchlistID(name string) string {
	// Create a clean ID based on name, but ensure uniqueness with UUID
	cleanName := ""
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cleanName += string(r)
		}
	}

	if len(cleanName) > 20 {
		cleanName = cleanName[:20]
	}

	uniqueSuffix := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s", cleanName, uniqueSuffix)
}
