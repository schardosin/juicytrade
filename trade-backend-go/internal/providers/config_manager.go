package providers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"trade-backend-go/internal/utils"
)

// ConfigManager manages provider configuration and routing.
// Exact conversion of Python ProviderConfigManager class.
type ConfigManager struct {
	configFile      string
	config          map[string]string
	credentialStore *CredentialStore // Cache credential store to avoid repeated file loads
}

// Default routing configuration - exact same as Python
var DefaultRouting = map[string]string{
	"stock_quotes":      "alpaca",
	"options_chain":     "alpaca",
	"trade_account":     "alpaca", // Unified: account, positions, orders
	"symbol_lookup":     "tradier",
	"historical_data":   "tradier",
	"market_calendar":   "tradier",
	"streaming_quotes":  "tradier", // Dedicated streaming for market data
	"greeks":            "tradier", // API-based Greeks
	"streaming_greeks":  "",        // Real-time streaming Greeks (empty = none)
}

// NewConfigManager creates a new config manager.
// Exact conversion of Python ProviderConfigManager.__init__ method.
func NewConfigManager() *ConfigManager {
	cm := &ConfigManager{
		configFile:      "provider_config.json",
		credentialStore: NewCredentialStore(), // Load once and cache
	}
	cm.config = cm.loadConfig()
	return cm
}

// loadConfig loads configuration from JSON file.
// Exact conversion of Python _load_config method.
func (cm *ConfigManager) loadConfig() map[string]string {
	configPath := cm.getConfigPath()
	
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		slog.Info(fmt.Sprintf("Using default provider config"))
		return copyMap(DefaultRouting)
	}
	
	file, err := os.Open(configPath)
	if err != nil {
		slog.Error(fmt.Sprintf("Error loading %s: %v. Using default config.", configPath, err))
		return copyMap(DefaultRouting)
	}
	defer file.Close()
	
	var config map[string]string
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		slog.Error(fmt.Sprintf("Error loading %s: %v. Using default config.", configPath, err))
		return copyMap(DefaultRouting)
	}
	
	slog.Info(fmt.Sprintf("Loaded provider config from %s", configPath))
	return config
}

// saveConfig saves configuration to JSON file.
// Exact conversion of Python _save_config method.
func (cm *ConfigManager) saveConfig() error {
	configPath := cm.getConfigPath()
	
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	
	file, err := os.Create(configPath)
	if err != nil {
		slog.Error(fmt.Sprintf("Error saving %s: %v", configPath, err))
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cm.config); err != nil {
		slog.Error(fmt.Sprintf("Error saving %s: %v", configPath, err))
		return err
	}
	
	slog.Info(fmt.Sprintf("Saved provider config to %s", configPath))
	return nil
}

// getConfigPath gets the full path to the config file.
// Uses PathManager to match Python path_manager logic exactly
func (cm *ConfigManager) getConfigPath() string {
	return utils.GlobalPathManager.GetConfigFilePath(cm.configFile)
}

// GetConfig gets the current configuration.
// Exact conversion of Python get_config method.
func (cm *ConfigManager) GetConfig() map[string]string {
	// Check if there are any active provider instances
	// This logic matches the Python implementation
	availableInstances := cm.credentialStore.GetAllInstances()
	
	activeInstances := make(map[string]map[string]interface{})
	for k, v := range availableInstances {
		if active, ok := v["active"].(bool); ok && active {
			activeInstances[k] = v
		}
	}
	
	// If no active provider instances exist, return empty config
	if len(activeInstances) == 0 {
		return make(map[string]string)
	}
	
	// Otherwise return the current configuration
	return copyMap(cm.config)
}

// UpdateConfig updates the configuration with new values.
// Exact conversion of Python update_config method.
func (cm *ConfigManager) UpdateConfig(newConfig map[string]interface{}) bool {
	validatedConfig := copyMap(cm.config)
	
	// Get available provider instances
	availableInstances := cm.credentialStore.GetAllInstances()
	
	for key, value := range newConfig {
		if _, exists := DefaultRouting[key]; exists {
			// Allow null values for optional configurations
			if value == nil {
				validatedConfig[key] = ""
				slog.Info(fmt.Sprintf("Set %s to null", key))
				continue
			}
			
			valueStr, ok := value.(string)
			if !ok {
				slog.Warn(fmt.Sprintf("Invalid value type for %s: expected string", key))
				continue
			}
			
			// Check if it's a legacy provider name (for backward compatibility)
			if cm.isLegacyProvider(valueStr) {
				if cm.validateLegacyProvider(key, valueStr) {
					validatedConfig[key] = valueStr
				} else {
					slog.Warn(fmt.Sprintf("Invalid legacy provider/capability: %s:%s", key, valueStr))
				}
			} else if _, exists := availableInstances[valueStr]; exists {
				// Check if it's a new provider instance ID
				instanceData := availableInstances[valueStr]
				providerType, _ := instanceData["provider_type"].(string)
				
				// Get provider type capabilities
				providerTypes := GetProviderTypes()
				if typeInfo, exists := providerTypes[providerType]; exists {
					capabilities := typeInfo.Capabilities
					
					// Check if provider supports this capability
					if key == "streaming_quotes" || key == "streaming_greeks" {
						if contains(capabilities.Streaming, key) {
							validatedConfig[key] = valueStr
						} else {
							slog.Warn(fmt.Sprintf("Provider instance %s doesn't support streaming capability: %s", valueStr, key))
						}
					} else if contains(capabilities.Rest, key) {
						validatedConfig[key] = valueStr
					} else {
						slog.Warn(fmt.Sprintf("Provider instance %s doesn't support REST capability: %s", valueStr, key))
					}
				} else {
					slog.Warn(fmt.Sprintf("Unknown provider type for instance %s: %s", valueStr, providerType))
				}
			} else {
				slog.Warn(fmt.Sprintf("Unknown provider or instance: %s", valueStr))
			}
		} else {
			slog.Warn(fmt.Sprintf("Invalid config key: %s", key))
		}
	}
	
	cm.config = validatedConfig
	cm.saveConfig()
	return true
}

// ResetConfig resets configuration to defaults.
// Exact conversion of Python reset_config method.
func (cm *ConfigManager) ResetConfig() {
	cm.config = copyMap(DefaultRouting)
	cm.saveConfig()
}

// GetAvailableProviders gets available providers with their capabilities.
// Exact conversion of Python get_available_providers method.
func (cm *ConfigManager) GetAvailableProviders() map[string]map[string]interface{} {
	// Get available provider instances
	availableInstances := cm.credentialStore.GetAllInstances()
	providerTypes := GetProviderTypes()
	
	result := make(map[string]map[string]interface{})
	
	// Add dynamic provider instances
	for instanceID, instanceData := range availableInstances {
		if active, ok := instanceData["active"].(bool); ok && active { // Only include active instances
			providerType, _ := instanceData["provider_type"].(string)
			accountType, _ := instanceData["account_type"].(string)
			displayName, _ := instanceData["display_name"].(string)
			
			if typeInfo, exists := providerTypes[providerType]; exists {
				result[instanceID] = map[string]interface{}{
					"capabilities":  typeInfo.Capabilities,
					"paper":         (accountType == "paper"),
					"display_name":  displayName,
					"provider_type": providerType,
					"account_type":  accountType,
					"instance_id":   instanceID,
				}
			}
		}
	}
	
	// Add legacy providers for backward compatibility (only if no instances exist)
	if len(result) == 0 {
		legacyProviders := GetLegacyProviderCapabilities()
		for provider, caps := range legacyProviders {
			result[provider] = map[string]interface{}{
				"capabilities": caps["capabilities"],
				"paper":        caps["paper"],
				"display_name": caps["display_name"],
			}
		}
	}
	
	return result
}

// Helper functions

func copyMap(original map[string]string) map[string]string {
	copy := make(map[string]string)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (cm *ConfigManager) isLegacyProvider(provider string) bool {
	legacyProviders := GetLegacyProviderCapabilities()
	_, exists := legacyProviders[provider]
	return exists
}

func (cm *ConfigManager) validateLegacyProvider(capability, provider string) bool {
	legacyProviders := GetLegacyProviderCapabilities()
	if providerInfo, exists := legacyProviders[provider]; exists {
		capabilities := providerInfo["capabilities"].(map[string]interface{})
		
		if capability == "streaming_quotes" {
			if streaming, ok := capabilities["streaming"].([]string); ok {
				return contains(streaming, capability)
			}
		} else {
			if rest, ok := capabilities["rest"].([]string); ok {
				return contains(rest, capability)
			}
		}
	}
	return false
}
