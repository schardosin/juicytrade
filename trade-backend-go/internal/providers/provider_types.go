package providers

import "strings"

// CredentialField represents a credential field definition.
// Exact conversion of Python credential field structure.
type CredentialField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
	Default     string `json:"default,omitempty"`
}

// ProviderCapabilities represents provider capabilities.
// Exact conversion of Python capabilities structure.
type ProviderCapabilities struct {
	Rest      []string `json:"rest"`
	Streaming []string `json:"streaming"`
}

// ProviderType represents a provider type definition.
// Exact conversion of Python provider type structure.
type ProviderType struct {
	Name                  string                           `json:"name"`
	Description           string                           `json:"description"`
	SupportsAccountTypes  []string                         `json:"supports_account_types"`
	Capabilities          ProviderCapabilities             `json:"capabilities"`
	CredentialFields      map[string][]CredentialField     `json:"credential_fields"`
}

// Provider type definitions with credential field specifications.
// Exact conversion of Python PROVIDER_TYPES.
var ProviderTypes = map[string]ProviderType{
	"alpaca": {
		Name:                 "Alpaca",
		Description:          "Alpaca Trading API",
		SupportsAccountTypes: []string{"live", "paper"},
		Capabilities: ProviderCapabilities{
			Rest:      []string{"expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar", "greeks"},
			Streaming: []string{"streaming_quotes", "trade_account"},
		},
		CredentialFields: map[string][]CredentialField{
			"live": {
				{Name: "api_key", Label: "API Key", Type: "text", Required: true, Placeholder: "Your Alpaca Live API Key"},
				{Name: "api_secret", Label: "API Secret", Type: "password", Required: true, Placeholder: "Your Alpaca Live API Secret"},
				{Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://api.alpaca.markets"},
				{Name: "data_url", Label: "Data URL", Type: "text", Required: false, Default: "https://data.alpaca.markets"},
			},
			"paper": {
				{Name: "api_key", Label: "API Key", Type: "text", Required: true, Placeholder: "Your Alpaca Paper API Key"},
				{Name: "api_secret", Label: "API Secret", Type: "password", Required: true, Placeholder: "Your Alpaca Paper API Secret"},
				{Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://paper-api.alpaca.markets"},
				{Name: "data_url", Label: "Data URL", Type: "text", Required: false, Default: "https://data.alpaca.markets"},
			},
		},
	},
	"tradier": {
		Name:                 "Tradier",
		Description:          "Tradier Brokerage API",
		SupportsAccountTypes: []string{"live", "paper"},
		Capabilities: ProviderCapabilities{
			Rest:      []string{"expiration_dates", "options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar", "greeks"},
			Streaming: []string{"streaming_quotes"},
		},
		CredentialFields: map[string][]CredentialField{
			"live": {
				{Name: "api_key", Label: "API Key", Type: "password", Required: true, Placeholder: "Your Tradier Live API Key"},
				{Name: "account_id", Label: "Account ID", Type: "text", Required: true, Placeholder: "Your Tradier Live Account ID"},
				{Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://api.tradier.com"},
				{Name: "stream_url", Label: "Stream URL", Type: "text", Required: false, Default: "wss://ws.tradier.com/v1/markets/events"},
			},
			"paper": {
				{Name: "api_key", Label: "API Key", Type: "password", Required: true, Placeholder: "Your Tradier Sandbox API Key"},
				{Name: "account_id", Label: "Account ID", Type: "text", Required: true, Placeholder: "Your Tradier Sandbox Account ID"},
				{Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://sandbox.tradier.com"},
				{Name: "stream_url", Label: "Stream URL", Type: "text", Required: false, Default: "wss://ws.sandbox.tradier.com/v1/markets/events"},
			},
		},
	},
	"public": {
		Name:                 "Public.com",
		Description:          "Public.com Trading API",
		SupportsAccountTypes: []string{"live"},
		Capabilities: ProviderCapabilities{
			Rest:      []string{"expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date"},
			Streaming: []string{},
		},
		CredentialFields: map[string][]CredentialField{
			"live": {
				{Name: "api_secret", Label: "API Secret", Type: "password", Required: true, Placeholder: "Your Public.com API Secret"},
				{Name: "account_id", Label: "Account ID", Type: "text", Required: true, Placeholder: "Your Public.com Account ID"},
			},
		},
	},
	"tastytrade": {
		Name:                 "TastyTrade",
		Description:          "TastyTrade Brokerage API",
		SupportsAccountTypes: []string{"live", "paper"},
		Capabilities: ProviderCapabilities{
			Rest:      []string{"expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"},
			Streaming: []string{"streaming_quotes", "trade_account", "streaming_greeks"},
		},
		CredentialFields: map[string][]CredentialField{
			"live": {
				{Name: "client_id", Label: "Client ID", Type: "text", Required: false, Placeholder: "Your TastyTrade OAuth2 Client ID"},
				{Name: "client_secret", Label: "Client Secret", Type: "password", Required: true, Placeholder: "Your TastyTrade OAuth2 Client Secret"},
				{Name: "refresh_token", Label: "Refresh Token", Type: "password", Required: false, Placeholder: "Your TastyTrade OAuth2 Refresh Token (Create Grant in portal)"},
				{Name: "authorization_code", Label: "Authorization Code", Type: "text", Required: false, Placeholder: "Optional one-time code for token exchange"},
				{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: false, Placeholder: "Optional redirect URI for auth code exchange"},
				{Name: "account_id", Label: "Account ID", Type: "text", Required: true, Placeholder: "Your TastyTrade Account ID"},
				{Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://api.tastytrade.com"},
			},
			"paper": {
				{Name: "client_id", Label: "Client ID", Type: "text", Required: false, Placeholder: "Your TastyTrade Sandbox OAuth2 Client ID"},
				{Name: "client_secret", Label: "Client Secret", Type: "password", Required: true, Placeholder: "Your TastyTrade Sandbox OAuth2 Client Secret"},
				{Name: "refresh_token", Label: "Refresh Token", Type: "password", Required: false, Placeholder: "Your TastyTrade Sandbox OAuth2 Refresh Token (Create Grant in portal)"},
				{Name: "authorization_code", Label: "Authorization Code", Type: "text", Required: false, Placeholder: "Optional one-time code for token exchange"},
				{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: false, Placeholder: "Optional redirect URI for auth code exchange"},
				{Name: "account_id", Label: "Account ID", Type: "text", Required: true, Placeholder: "Your TastyTrade Sandbox Account ID"},
				{Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://api.cert.tastyworks.com"},
			},
		},
	},
}

// Legacy provider capabilities for backward compatibility.
// Exact conversion of Python PROVIDER_CAPABILITIES.
var LegacyProviderCapabilities = map[string]map[string]interface{}{
	"alpaca": {
		"capabilities": map[string]interface{}{
			"rest":      []string{"stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar", "greeks"},
			"streaming": []string{"streaming_quotes", "trade_account"},
		},
		"paper":        false,
		"display_name": "Alpaca",
	},
	"alpaca_paper": {
		"capabilities": map[string]interface{}{
			"rest":      []string{"stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar", "greeks"},
			"streaming": []string{"streaming_quotes", "trade_account"},
		},
		"paper":        true,
		"display_name": "Alpaca",
	},
	"public": {
		"capabilities": map[string]interface{}{
			"rest":      []string{"stock_quotes", "options_chain", "trade_account", "next_market_date"},
			"streaming": []string{},
		},
		"paper":        false,
		"display_name": "Public.com",
	},
	"tradier": {
		"capabilities": map[string]interface{}{
			"rest":      []string{"options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar", "greeks"},
			"streaming": []string{"streaming_quotes"},
		},
		"paper":        false,
		"display_name": "Tradier",
	},
	"tradier_paper": {
		"capabilities": map[string]interface{}{
			"rest":      []string{"options_chain", "next_market_date", "stock_quotes", "trade_account", "symbol_lookup", "historical_data", "market_calendar", "greeks"},
			"streaming": []string{"streaming_quotes"},
		},
		"paper":        true,
		"display_name": "Tradier",
	},
	"tastytrade": {
		"capabilities": map[string]interface{}{
			"rest":      []string{"stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"},
			"streaming": []string{"streaming_quotes", "trade_account", "streaming_greeks"},
		},
		"paper":        false,
		"display_name": "TastyTrade",
	},
	"tastytrade_paper": {
		"capabilities": map[string]interface{}{
			"rest":      []string{"stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar"},
			"streaming": []string{"streaming_quotes", "trade_account", "streaming_greeks"},
		},
		"paper":        true,
		"display_name": "TastyTrade",
	},
}

// GetProviderTypes gets all available provider types.
// Exact conversion of Python get_provider_types function.
func GetProviderTypes() map[string]ProviderType {
	return ProviderTypes
}

// GetProviderType gets specific provider type definition.
// Exact conversion of Python get_provider_type function.
func GetProviderType(providerType string) ProviderType {
	if pt, exists := ProviderTypes[providerType]; exists {
		return pt
	}
	return ProviderType{}
}

// GetCredentialFields gets credential fields for a specific provider type and account type.
// Exact conversion of Python get_credential_fields function.
func GetCredentialFields(providerType, accountType string) []CredentialField {
	if pt, exists := ProviderTypes[providerType]; exists {
		if fields, exists := pt.CredentialFields[accountType]; exists {
			return fields
		}
	}
	return []CredentialField{}
}

// ValidateCredentials validates credentials against provider type requirements.
// Exact conversion of Python validate_credentials function.
func ValidateCredentials(providerType, accountType string, credentials map[string]interface{}) []string {
	var errors []string
	fields := GetCredentialFields(providerType, accountType)
	
	for _, field := range fields {
		if field.Required {
			if value, exists := credentials[field.Name]; !exists || value == "" {
				errors = append(errors, "Missing required field: "+field.Label)
			}
		}
	}
	
	return errors
}

// ApplyDefaults applies default values to credentials.
// Exact conversion of Python apply_defaults function.
func ApplyDefaults(providerType, accountType string, credentials map[string]interface{}) map[string]interface{} {
	fields := GetCredentialFields(providerType, accountType)
	result := make(map[string]interface{})
	
	// Copy existing credentials
	for k, v := range credentials {
		result[k] = v
	}
	
	// Apply defaults for missing fields
	for _, field := range fields {
		if _, exists := result[field.Name]; !exists && field.Default != "" {
			result[field.Name] = field.Default
		}
	}
	
	return result
}

// IsSensitiveField determines if a credential field contains sensitive information.
// Exact conversion of Python is_sensitive_field function.
func IsSensitiveField(fieldName string) bool {
	sensitiveFields := []string{"password", "api_key", "api_secret", "client_secret", "refresh_token"}
	fieldLower := strings.ToLower(fieldName)
	
	for _, sensitive := range sensitiveFields {
		if strings.Contains(fieldLower, sensitive) {
			return true
		}
	}
	return false
}

// GetVisibleCredentials gets non-sensitive credential values that can be displayed in UI.
// Exact conversion of Python get_visible_credentials function.
func GetVisibleCredentials(instanceData map[string]interface{}) map[string]interface{} {
	credentials, _ := instanceData["credentials"].(map[string]interface{})
	visibleCreds := make(map[string]interface{})
	
	for fieldName, fieldValue := range credentials {
		if !IsSensitiveField(fieldName) {
			visibleCreds[fieldName] = fieldValue
		}
	}
	
	return visibleCreds
}

// GetMaskedCredentials gets indicators for which sensitive fields have values set.
// Exact conversion of Python get_masked_credentials function.
func GetMaskedCredentials(instanceData map[string]interface{}) map[string]bool {
	credentials, _ := instanceData["credentials"].(map[string]interface{})
	maskedCreds := make(map[string]bool)
	
	for fieldName, fieldValue := range credentials {
		if IsSensitiveField(fieldName) {
			valueStr, _ := fieldValue.(string)
			maskedCreds[fieldName] = valueStr != "" && strings.TrimSpace(valueStr) != ""
		}
	}
	
	return maskedCreds
}

// GetDefaultCredentials gets default credential values for a provider type and account type.
// Exact conversion of Python get_default_credentials function.
func GetDefaultCredentials(providerType, accountType string) map[string]interface{} {
	fields := GetCredentialFields(providerType, accountType)
	defaults := make(map[string]interface{})
	
	for _, field := range fields {
		if field.Default != "" {
			defaults[field.Name] = field.Default
		}
	}
	
	return defaults
}

// GetLegacyProviderCapabilities gets legacy provider capabilities for backward compatibility.
// Exact conversion of Python PROVIDER_CAPABILITIES.
func GetLegacyProviderCapabilities() map[string]map[string]interface{} {
	return LegacyProviderCapabilities
}
