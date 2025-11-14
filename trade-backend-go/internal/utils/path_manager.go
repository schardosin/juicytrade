package utils

import (
	"log/slog"
	"os"
	"path/filepath"
)

// PathManager manages paths for configuration and data files.
// Exact conversion of Python PathManager class.
type PathManager struct {
	dataDir   string
	configDir string
	cacheDir  string
}

// NewPathManager creates a new path manager.
// Exact conversion of Python PathManager.__init__ method.
func NewPathManager() *PathManager {
	pm := &PathManager{}
	pm.dataDir = pm.determineDataDirectory()
	pm.configDir = filepath.Join(pm.dataDir, "config")
	pm.cacheDir = filepath.Join(pm.dataDir, "cache")
	
	pm.ensureDirectoryStructure()
	return pm
}

// determineDataDirectory determines the appropriate data directory based on environment.
// Exact conversion of Python _determine_data_directory method.
func (pm *PathManager) determineDataDirectory() string {
	containerDataDir := "/app/data"
	
	if _, err := os.Stat(containerDataDir); err == nil {
		slog.Info("📁 Using container data directory: " + containerDataDir)
		return containerDataDir
	} else {
		// Development mode - use parent directory (project root)
		// Go server runs from trade-backend-go, but config files are in parent directory
		currentDir, _ := os.Getwd()
		parentDir := filepath.Join(currentDir, "..")
		absParentDir, _ := filepath.Abs(parentDir)
		slog.Info("📁 Using development data directory: " + absParentDir)
		return absParentDir
	}
}

// ensureDirectoryStructure ensures required directory structure exists.
// Exact conversion of Python _ensure_directory_structure method.
func (pm *PathManager) ensureDirectoryStructure() {
	if pm.IsContainerMode() {
		// Only create subdirectories in container mode
		os.MkdirAll(pm.configDir, 0755)
		os.MkdirAll(pm.cacheDir, 0755)
		slog.Info("📁 Created directory structure in " + pm.dataDir)
	}
}

// DataDir gets the data directory path.
func (pm *PathManager) DataDir() string {
	return pm.dataDir
}

// ConfigDir gets the config directory path.
func (pm *PathManager) ConfigDir() string {
	return pm.configDir
}

// CacheDir gets the cache directory path.
func (pm *PathManager) CacheDir() string {
	return pm.cacheDir
}

// IsContainerMode checks if running in container mode.
// Exact conversion of Python is_container_mode method.
func (pm *PathManager) IsContainerMode() bool {
	return pm.dataDir == "/app/data"
}

// GetConfigFilePath gets the full path for a configuration file.
// Exact conversion of Python get_config_file_path method.
func (pm *PathManager) GetConfigFilePath(filename string) string {
	if pm.IsContainerMode() {
		return filepath.Join(pm.configDir, filename)
	} else {
		// Development mode - use root directory
		return filepath.Join(pm.dataDir, filename)
	}
}

// GetCacheFilePath gets the full path for a cache file.
// Exact conversion of Python get_cache_file_path method.
func (pm *PathManager) GetCacheFilePath(filename string) string {
	if pm.IsContainerMode() {
		return filepath.Join(pm.cacheDir, filename)
	} else {
		// Development mode - use cache subdirectory
		cacheDir := filepath.Join(pm.dataDir, "cache")
		os.MkdirAll(cacheDir, 0755)
		return filepath.Join(cacheDir, filename)
	}
}

// GetStatus gets current path configuration status.
// Exact conversion of Python get_status method.
func (pm *PathManager) GetStatus() map[string]interface{} {
	mode := "development"
	if pm.IsContainerMode() {
		mode = "container"
	}
	
	return map[string]interface{}{
		"mode":               mode,
		"data_dir":           pm.dataDir,
		"config_dir":         pm.configDir,
		"cache_dir":          pm.cacheDir,
		"data_dir_exists":    dirExists(pm.dataDir),
		"config_dir_exists":  dirExists(pm.configDir),
		"cache_dir_exists":   dirExists(pm.cacheDir),
	}
}

// Helper function to check if directory exists
func dirExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// Global instance - exact same as Python
var GlobalPathManager = NewPathManager()
