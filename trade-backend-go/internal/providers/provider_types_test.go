package providers

import (
	"testing"
)

func TestProviderTypes_SchwabAuthMethod(t *testing.T) {
	pt, exists := ProviderTypes["schwab"]
	if !exists {
		t.Fatal("schwab provider type not found")
	}
	if pt.AuthMethod != "oauth" {
		t.Errorf("expected AuthMethod 'oauth', got %q", pt.AuthMethod)
	}
}

func TestProviderTypes_SchwabCredentialFields(t *testing.T) {
	pt := ProviderTypes["schwab"]

	for _, acctType := range []string{"live", "paper"} {
		fields, ok := pt.CredentialFields[acctType]
		if !ok {
			t.Fatalf("expected credential fields for account type %q", acctType)
		}

		if len(fields) != 4 {
			t.Errorf("[%s] expected 4 credential fields, got %d", acctType, len(fields))
		}

		// Verify expected field names
		expectedNames := map[string]bool{
			"app_key":      false,
			"app_secret":   false,
			"callback_url": false,
			"base_url":     false,
		}
		for _, f := range fields {
			if _, ok := expectedNames[f.Name]; ok {
				expectedNames[f.Name] = true
			}
		}
		for name, found := range expectedNames {
			if !found {
				t.Errorf("[%s] expected field %q not found", acctType, name)
			}
		}

		// Verify removed fields are NOT present
		for _, f := range fields {
			if f.Name == "refresh_token" {
				t.Errorf("[%s] refresh_token should have been removed from credential fields", acctType)
			}
			if f.Name == "account_hash" {
				t.Errorf("[%s] account_hash should have been removed from credential fields", acctType)
			}
		}
	}
}

func TestProviderTypes_SchwabCallbackDefault(t *testing.T) {
	pt := ProviderTypes["schwab"]

	for _, acctType := range []string{"live", "paper"} {
		fields := pt.CredentialFields[acctType]
		for _, f := range fields {
			if f.Name == "callback_url" {
				if f.Default != "https://127.0.0.1/callback" {
					t.Errorf("[%s] expected callback_url default 'https://127.0.0.1/callback', got %q", acctType, f.Default)
				}
				return
			}
		}
		t.Errorf("[%s] callback_url field not found", acctType)
	}
}

func TestProviderTypes_SchwabHelpText(t *testing.T) {
	pt := ProviderTypes["schwab"]

	fields := pt.CredentialFields["live"]
	helpTexts := make(map[string]string)
	for _, f := range fields {
		helpTexts[f.Name] = f.HelpText
	}

	if helpTexts["app_key"] == "" {
		t.Error("expected HelpText on app_key")
	}
	if helpTexts["app_secret"] == "" {
		t.Error("expected HelpText on app_secret")
	}
	if helpTexts["callback_url"] == "" {
		t.Error("expected HelpText on callback_url")
	}
	if helpTexts["base_url"] == "" {
		t.Error("expected HelpText on base_url")
	}
}

func TestProviderTypes_OtherProvidersUnchanged(t *testing.T) {
	// Verify other providers have no AuthMethod set (empty string = simple auth)
	for _, name := range []string{"alpaca", "tradier", "tastytrade"} {
		pt, exists := ProviderTypes[name]
		if !exists {
			t.Errorf("provider type %q not found", name)
			continue
		}
		if pt.AuthMethod != "" {
			t.Errorf("expected empty AuthMethod for %q, got %q", name, pt.AuthMethod)
		}
	}
}
