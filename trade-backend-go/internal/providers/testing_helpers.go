package providers

import (
	"sync"

	"trade-backend-go/internal/providers/base"
)

// NewTestProviderManager creates a ProviderManager with a single mock provider
// registered under the given instanceID. The config routes all operations to
// that instanceID. This is intended for unit tests in other packages.
func NewTestProviderManager(instanceID string, provider base.Provider) *ProviderManager {
	config := make(map[string]string)
	for operation := range DefaultRouting {
		config[operation] = instanceID
	}

	// Create a CredentialStore with a fake active instance so GetConfig() works
	cs := &CredentialStore{
		data: map[string]map[string]interface{}{
			instanceID: {
				"active":        true,
				"provider_type": "mock",
				"account_type":  "test",
			},
		},
	}

	cm := &ConfigManager{
		configFile:      "",
		config:          config,
		credentialStore: cs,
	}

	return &ProviderManager{
		providers:       map[string]base.Provider{instanceID: provider},
		credentialStore: cs,
		configManager:   cm,
		mutex:           sync.RWMutex{},
	}
}
