package providers

import (
	"encoding/json"
	"sort"
	"testing"
)

// ============================================================================
// QA Tests for Step 3 — Provider Types and Credential Field Changes (AC-1)
// ============================================================================

// TestProviderType_AuthMethod_JSONOmitEmpty verifies that the AuthMethod field
// uses omitempty correctly:
//   - For providers with empty AuthMethod (e.g., Alpaca), the "auth_method"
//     key must NOT appear in the JSON output.
//   - For Schwab (AuthMethod: "oauth"), the "auth_method" key must appear
//     with value "oauth".
func TestProviderType_AuthMethod_JSONOmitEmpty(t *testing.T) {
	// --- Alpaca: AuthMethod is "" → must be omitted from JSON ---
	alpaca := ProviderTypes["alpaca"]
	alpacaJSON, err := json.Marshal(alpaca)
	if err != nil {
		t.Fatalf("failed to marshal alpaca ProviderType: %v", err)
	}

	// Unmarshal into a generic map to check key presence.
	var alpacaMap map[string]interface{}
	if err := json.Unmarshal(alpacaJSON, &alpacaMap); err != nil {
		t.Fatalf("failed to unmarshal alpaca JSON: %v", err)
	}

	if _, exists := alpacaMap["auth_method"]; exists {
		t.Errorf("Alpaca JSON contains \"auth_method\" key; expected it to be omitted (omitempty) when AuthMethod is empty, got: %v", alpacaMap["auth_method"])
	}

	// --- Schwab: AuthMethod is "oauth" → must appear in JSON ---
	schwab := ProviderTypes["schwab"]
	schwabJSON, err := json.Marshal(schwab)
	if err != nil {
		t.Fatalf("failed to marshal schwab ProviderType: %v", err)
	}

	var schwabMap map[string]interface{}
	if err := json.Unmarshal(schwabJSON, &schwabMap); err != nil {
		t.Fatalf("failed to unmarshal schwab JSON: %v", err)
	}

	authMethod, exists := schwabMap["auth_method"]
	if !exists {
		t.Fatal("Schwab JSON missing \"auth_method\" key; expected \"oauth\"")
	}
	if authMethod != "oauth" {
		t.Errorf("Schwab auth_method = %v, want \"oauth\"", authMethod)
	}

	// --- Verify all other providers also omit auth_method ---
	for _, name := range []string{"tradier", "tastytrade", "public"} {
		pt := ProviderTypes[name]
		ptJSON, err := json.Marshal(pt)
		if err != nil {
			t.Fatalf("failed to marshal %s ProviderType: %v", name, err)
		}

		var ptMap map[string]interface{}
		if err := json.Unmarshal(ptJSON, &ptMap); err != nil {
			t.Fatalf("failed to unmarshal %s JSON: %v", name, err)
		}

		if _, exists := ptMap["auth_method"]; exists {
			t.Errorf("%s JSON contains \"auth_method\" key; expected omission for non-OAuth providers", name)
		}
	}
}

// TestApplyDefaults_SchwabLive verifies that calling ApplyDefaults with an
// empty credentials map for Schwab live account type fills in the default
// values for callback_url and base_url.
func TestApplyDefaults_SchwabLive(t *testing.T) {
	result := ApplyDefaults("schwab", "live", map[string]interface{}{})

	// callback_url should be defaulted
	callbackURL, ok := result["callback_url"]
	if !ok {
		t.Fatal("callback_url not set by ApplyDefaults")
	}
	if callbackURL != "https://127.0.0.1/callback" {
		t.Errorf("callback_url = %v, want \"https://127.0.0.1/callback\"", callbackURL)
	}

	// base_url should be defaulted
	baseURL, ok := result["base_url"]
	if !ok {
		t.Fatal("base_url not set by ApplyDefaults")
	}
	if baseURL != "https://api.schwabapi.com" {
		t.Errorf("base_url = %v, want \"https://api.schwabapi.com\"", baseURL)
	}

	// app_key and app_secret have no defaults — they must NOT appear
	if _, exists := result["app_key"]; exists {
		t.Error("app_key should not be set by ApplyDefaults (no default value)")
	}
	if _, exists := result["app_secret"]; exists {
		t.Error("app_secret should not be set by ApplyDefaults (no default value)")
	}

	// Verify ApplyDefaults does NOT inject removed fields
	if _, exists := result["refresh_token"]; exists {
		t.Error("refresh_token should not be injected by ApplyDefaults")
	}
	if _, exists := result["account_hash"]; exists {
		t.Error("account_hash should not be injected by ApplyDefaults")
	}

	// Also verify paper has the same defaults
	paperResult := ApplyDefaults("schwab", "paper", map[string]interface{}{})
	if paperResult["callback_url"] != "https://127.0.0.1/callback" {
		t.Errorf("paper callback_url = %v, want \"https://127.0.0.1/callback\"", paperResult["callback_url"])
	}
	if paperResult["base_url"] != "https://api.schwabapi.com" {
		t.Errorf("paper base_url = %v, want \"https://api.schwabapi.com\"", paperResult["base_url"])
	}
}

// TestSchwabCredentialFieldNames verifies that the exact set of credential
// field names for both live and paper account types is
// ["app_key", "app_secret", "callback_url", "base_url"] — no more, no less.
// The comparison is order-independent.
func TestSchwabCredentialFieldNames(t *testing.T) {
	expected := []string{"app_key", "app_secret", "base_url", "callback_url"}
	sort.Strings(expected)

	pt := ProviderTypes["schwab"]

	for _, acctType := range []string{"live", "paper"} {
		fields, ok := pt.CredentialFields[acctType]
		if !ok {
			t.Fatalf("[%s] credential fields not found", acctType)
		}

		var names []string
		for _, f := range fields {
			names = append(names, f.Name)
		}
		sort.Strings(names)

		if len(names) != len(expected) {
			t.Errorf("[%s] expected %d fields %v, got %d fields %v", acctType, len(expected), expected, len(names), names)
			continue
		}

		for i := range expected {
			if names[i] != expected[i] {
				t.Errorf("[%s] field mismatch at index %d: expected %q, got %q (full: expected %v, got %v)", acctType, i, expected[i], names[i], expected, names)
				break
			}
		}
	}
}

// TestSchwabNoRefreshTokenOrAccountHash explicitly verifies that the
// field names "refresh_token" and "account_hash" do NOT appear in any
// Schwab credential field list (live or paper). These fields are managed
// internally via the OAuth flow and should never be shown in the UI.
func TestSchwabNoRefreshTokenOrAccountHash(t *testing.T) {
	pt := ProviderTypes["schwab"]

	forbiddenNames := []string{"refresh_token", "account_hash"}

	for _, acctType := range []string{"live", "paper"} {
		fields, ok := pt.CredentialFields[acctType]
		if !ok {
			t.Fatalf("[%s] credential fields not found", acctType)
		}

		for _, f := range fields {
			for _, forbidden := range forbiddenNames {
				if f.Name == forbidden {
					t.Errorf("[%s] found forbidden field %q in credential fields; OAuth-managed fields must not appear in UI", acctType, forbidden)
				}
			}
		}
	}
}
