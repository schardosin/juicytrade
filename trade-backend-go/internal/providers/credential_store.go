package providers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"trade-backend-go/internal/utils"
)

// CredentialStore manages provider credentials and instances.
// Exact conversion of Python ProviderCredentialStore class.
type CredentialStore struct {
	credentialsFile string
	data            map[string]map[string]interface{}
}

// NewCredentialStore creates a new credential store.
// Exact conversion of Python ProviderCredentialStore.__init__ method.
func NewCredentialStore() *CredentialStore {
	cs := &CredentialStore{
		credentialsFile: "provider_credentials.json",
	}
	cs.data = cs.loadCredentials()
	return cs
}

// loadCredentials loads credentials from JSON file.
// Exact conversion of Python _load_credentials method.
func (cs *CredentialStore) loadCredentials() map[string]map[string]interface{} {
	credentialsPath := cs.getCredentialsPath()
	
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		slog.Info(fmt.Sprintf("📝 Creating new credentials file: %s", credentialsPath))
		return make(map[string]map[string]interface{})
	}
	
	file, err := os.Open(credentialsPath)
	if err != nil {
		slog.Error(fmt.Sprintf("❌ Error opening %s: %v", credentialsPath, err))
		return make(map[string]map[string]interface{})
	}
	defer file.Close()
	
	var data map[string]map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		slog.Error(fmt.Sprintf("❌ Error loading %s: %v", credentialsPath, err))
		return make(map[string]map[string]interface{})
	}
	
	slog.Info(fmt.Sprintf("✅ Loaded provider credentials from %s", credentialsPath))
	return data
}

// saveCredentials saves credentials to JSON file.
// Exact conversion of Python _save_credentials method.
func (cs *CredentialStore) saveCredentials() error {
	credentialsPath := cs.getCredentialsPath()
	
	// Ensure directory exists
	dir := filepath.Dir(credentialsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	
	file, err := os.Create(credentialsPath)
	if err != nil {
		slog.Error(fmt.Sprintf("❌ Error creating %s: %v", credentialsPath, err))
		return err
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cs.data); err != nil {
		slog.Error(fmt.Sprintf("❌ Error saving %s: %v", credentialsPath, err))
		return err
	}
	
	slog.Info(fmt.Sprintf("💾 Saved provider credentials to %s", credentialsPath))
	return nil
}

// getCredentialsPath gets the full path to the credentials file.
// Uses PathManager to match Python path_manager logic exactly
func (cs *CredentialStore) getCredentialsPath() string {
	return utils.GlobalPathManager.GetConfigFilePath(cs.credentialsFile)
}

// GetAllInstances gets all provider instances.
// Exact conversion of Python get_all_instances method.
func (cs *CredentialStore) GetAllInstances() map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	for k, v := range cs.data {
		result[k] = make(map[string]interface{})
		for k2, v2 := range v {
			result[k][k2] = v2
		}
	}
	return result
}

// GetActiveInstances gets only active provider instances.
// Exact conversion of Python get_active_instances method.
func (cs *CredentialStore) GetActiveInstances() map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	for instanceID, instanceData := range cs.data {
		if active, ok := instanceData["active"].(bool); ok && active {
			result[instanceID] = make(map[string]interface{})
			for k, v := range instanceData {
				result[instanceID][k] = v
			}
		}
	}
	return result
}

// GetInstance gets a specific provider instance.
// Exact conversion of Python get_instance method.
func (cs *CredentialStore) GetInstance(instanceID string) map[string]interface{} {
	if instanceData, exists := cs.data[instanceID]; exists {
		result := make(map[string]interface{})
		for k, v := range instanceData {
			result[k] = v
		}
		return result
	}
	return nil
}

// AddInstance adds a new provider instance.
// Exact conversion of Python add_instance method.
func (cs *CredentialStore) AddInstance(instanceID, providerType, accountType, displayName string, credentials map[string]interface{}) bool {
	cs.data[instanceID] = map[string]interface{}{
		"active":        true,
		"provider_type": providerType,
		"account_type":  accountType,
		"display_name":  displayName,
		"credentials":   credentials,
		"created_at":    time.Now().Unix(),
		"updated_at":    time.Now().Unix(),
	}
	
	if err := cs.saveCredentials(); err != nil {
		slog.Error(fmt.Sprintf("❌ Error adding provider instance %s: %v", instanceID, err))
		return false
	}
	
	slog.Info(fmt.Sprintf("➕ Added provider instance: %s", instanceID))
	return true
}

// UpdateInstance updates an existing provider instance.
// Exact conversion of Python update_instance method.
func (cs *CredentialStore) UpdateInstance(instanceID string, updates map[string]interface{}) bool {
	if _, exists := cs.data[instanceID]; !exists {
		slog.Warn(fmt.Sprintf("⚠️ Provider instance not found: %s", instanceID))
		return false
	}
	
	updates["updated_at"] = time.Now().Unix()
	for k, v := range updates {
		cs.data[instanceID][k] = v
	}
	
	if err := cs.saveCredentials(); err != nil {
		slog.Error(fmt.Sprintf("❌ Error updating provider instance %s: %v", instanceID, err))
		return false
	}
	
	slog.Info(fmt.Sprintf("✏️ Updated provider instance: %s", instanceID))
	return true
}

// DeleteInstance deletes a provider instance.
// Exact conversion of Python delete_instance method.
func (cs *CredentialStore) DeleteInstance(instanceID string) bool {
	if _, exists := cs.data[instanceID]; exists {
		delete(cs.data, instanceID)
		
		if err := cs.saveCredentials(); err != nil {
			slog.Error(fmt.Sprintf("❌ Error deleting provider instance %s: %v", instanceID, err))
			return false
		}
		
		slog.Info(fmt.Sprintf("🗑️ Deleted provider instance: %s", instanceID))
		return true
	}
	
	slog.Warn(fmt.Sprintf("⚠️ Provider instance not found for deletion: %s", instanceID))
	return false
}

// ToggleInstance toggles active status of a provider instance.
// Exact conversion of Python toggle_instance method.
func (cs *CredentialStore) ToggleInstance(instanceID string) *bool {
	if instanceData, exists := cs.data[instanceID]; exists {
		currentActive, _ := instanceData["active"].(bool)
		newActive := !currentActive
		
		instanceData["active"] = newActive
		instanceData["updated_at"] = time.Now().Unix()
		
		if err := cs.saveCredentials(); err != nil {
			slog.Error(fmt.Sprintf("❌ Error toggling provider instance %s: %v", instanceID, err))
			return nil
		}
		
		slog.Info(fmt.Sprintf("🔄 Toggled provider instance %s: %t → %t", instanceID, currentActive, newActive))
		return &newActive
	}
	
	slog.Warn(fmt.Sprintf("⚠️ Provider instance not found: %s", instanceID))
	return nil
}

// GetInstancesByType gets all instances of a specific provider type.
// Exact conversion of Python get_instances_by_type method.
func (cs *CredentialStore) GetInstancesByType(providerType string) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	for instanceID, instanceData := range cs.data {
		if pt, ok := instanceData["provider_type"].(string); ok && pt == providerType {
			result[instanceID] = make(map[string]interface{})
			for k, v := range instanceData {
				result[instanceID][k] = v
			}
		}
	}
	return result
}

// ValidateInstanceID checks if instance ID is unique.
// Exact conversion of Python validate_instance_id method.
func (cs *CredentialStore) ValidateInstanceID(instanceID string) bool {
	_, exists := cs.data[instanceID]
	return !exists
}

// GenerateInstanceID generates a unique instance ID.
// Exact conversion of Python generate_instance_id method.
func (cs *CredentialStore) GenerateInstanceID(providerType, accountType, displayName string) string {
	baseName := displayName
	if baseName == "" {
		baseName = fmt.Sprintf("%s_%s", providerType, accountType)
	}
	
	// Clean up the base name
	baseName = cleanString(baseName)
	baseID := fmt.Sprintf("%s_%s_%s", providerType, accountType, baseName)
	
	// Ensure uniqueness
	counter := 1
	instanceID := baseID
	for !cs.ValidateInstanceID(instanceID) {
		instanceID = fmt.Sprintf("%s_%d", baseID, counter)
		counter++
	}
	
	return instanceID
}

// cleanString cleans a string for use in IDs
func cleanString(s string) string {
	// Simple cleanup - replace spaces and remove parentheses
	result := ""
	for _, r := range s {
		if r == ' ' {
			result += "_"
		} else if r != '(' && r != ')' {
			result += string(r)
		}
	}
	return result
}
