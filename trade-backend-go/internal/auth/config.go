package auth

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AuthMethod represents the authentication method
type AuthMethod string

const (
	AuthMethodDisabled AuthMethod = "disabled"
	AuthMethodSimple   AuthMethod = "simple"
	AuthMethodOAuth    AuthMethod = "oauth"
	AuthMethodToken    AuthMethod = "token"
	AuthMethodHeader   AuthMethod = "header"
)

// OAuthProvider represents the OAuth provider
type OAuthProvider string

const (
	OAuthProviderGoogle    OAuthProvider = "google"
	OAuthProviderGithub    OAuthProvider = "github"
	OAuthProviderMicrosoft OAuthProvider = "microsoft"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Method AuthMethod

	// Simple auth
	SimpleUsername string
	SimplePassword string

	// OAuth
	OAuthProvider     OAuthProvider
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURI  string
	OAuthAllowedEmails []string
	OAuthAllowedDomains []string

	// JWT
	JWTSecretKey     string
	JWTExpireMinutes int
	JWTAlgorithm     string

	// Header
	HeaderName string

	// Session
	SessionCookieName string
	SessionMaxAge     int

	// Security
	EnableCSRF    bool
	SecureCookies bool
	CookieDomain  string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *AuthConfig {
	cfg := &AuthConfig{
		Method:            AuthMethod(getEnv("AUTH_METHOD", "disabled")),
		SimpleUsername:    getEnv("AUTH_SIMPLE_USERNAME", ""),
		SimplePassword:    getEnv("AUTH_SIMPLE_PASSWORD", ""),
		OAuthClientID:     getEnv("AUTH_OAUTH_CLIENT_ID", ""),
		OAuthClientSecret: getEnv("AUTH_OAUTH_CLIENT_SECRET", ""),
		OAuthRedirectURI:  getEnv("AUTH_OAUTH_REDIRECT_URI", ""),
		JWTSecretKey:      getEnv("AUTH_JWT_SECRET_KEY", "juicytrade-default-secret-key-change-in-production"),
		JWTAlgorithm:      getEnv("AUTH_JWT_ALGORITHM", "HS256"),
		HeaderName:        getEnv("AUTH_HEADER_NAME", "X-Remote-User"),
		SessionCookieName: getEnv("AUTH_SESSION_COOKIE_NAME", "juicytrade_session"),
		CookieDomain:      getEnv("AUTH_COOKIE_DOMAIN", ""),
	}

	// Parse OAuth provider
	if provider := getEnv("AUTH_OAUTH_PROVIDER", ""); provider != "" {
		cfg.OAuthProvider = OAuthProvider(strings.ToLower(provider))
	}

	// Parse numeric values
	cfg.JWTExpireMinutes = getEnvInt("AUTH_JWT_EXPIRE_MINUTES", 1440)
	cfg.SessionMaxAge = getEnvInt("AUTH_SESSION_MAX_AGE", 86400)

	// Parse booleans
	cfg.EnableCSRF = getEnvBool("AUTH_ENABLE_CSRF", true)
	cfg.SecureCookies = getEnvBool("AUTH_SECURE_COOKIES", false)

	// Parse lists
	if emails := getEnv("AUTH_OAUTH_ALLOWED_EMAILS", ""); emails != "" {
		for _, e := range strings.Split(emails, ",") {
			if trimmed := strings.TrimSpace(e); trimmed != "" {
				cfg.OAuthAllowedEmails = append(cfg.OAuthAllowedEmails, strings.ToLower(trimmed))
			}
		}
	}

	if domains := getEnv("AUTH_OAUTH_ALLOWED_DOMAINS", ""); domains != "" {
		for _, d := range strings.Split(domains, ",") {
			if trimmed := strings.TrimSpace(d); trimmed != "" {
				cfg.OAuthAllowedDomains = append(cfg.OAuthAllowedDomains, strings.ToLower(trimmed))
			}
		}
	}

	return cfg
}

// IsEnabled checks if authentication is enabled
func (c *AuthConfig) IsEnabled() bool {
	return c.Method != AuthMethodDisabled
}

// Validate checks configuration validity
func (c *AuthConfig) Validate() error {
	if !c.IsEnabled() {
		return nil
	}

	var errors []string

	switch c.Method {
	case AuthMethodSimple:
		if c.SimpleUsername == "" {
			errors = append(errors, "AUTH_SIMPLE_USERNAME is required for simple authentication")
		}
		if c.SimplePassword == "" {
			errors = append(errors, "AUTH_SIMPLE_PASSWORD is required for simple authentication")
		}
	case AuthMethodOAuth:
		if c.OAuthProvider == "" {
			errors = append(errors, "AUTH_OAUTH_PROVIDER is required for OAuth authentication")
		}
		if c.OAuthClientID == "" {
			errors = append(errors, "AUTH_OAUTH_CLIENT_ID is required for OAuth authentication")
		}
		if c.OAuthClientSecret == "" {
			errors = append(errors, "AUTH_OAUTH_CLIENT_SECRET is required for OAuth authentication")
		}
		if c.OAuthRedirectURI == "" {
			errors = append(errors, "AUTH_OAUTH_REDIRECT_URI is required for OAuth authentication")
		}
	case AuthMethodToken:
		if c.JWTSecretKey == "juicytrade-default-secret-key-change-in-production" {
			// Just a warning in production, but we'll allow it for now
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("authentication configuration errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// IsUserAuthorized checks if a user email is authorized for OAuth access
func (c *AuthConfig) IsUserAuthorized(email string) bool {
	if email == "" {
		return false
	}

	email = strings.ToLower(strings.TrimSpace(email))

	// Check allowed emails list
	if len(c.OAuthAllowedEmails) > 0 {
		for _, allowed := range c.OAuthAllowedEmails {
			if email == allowed {
				return true
			}
		}
	}

	// Check allowed domains list
	if len(c.OAuthAllowedDomains) > 0 {
		parts := strings.Split(email, "@")
		if len(parts) == 2 {
			domain := parts[1]
			for _, allowed := range c.OAuthAllowedDomains {
				if domain == allowed {
					return true
				}
			}
		}
	}

	// If no restrictions are configured, allow all users
	if len(c.OAuthAllowedEmails) == 0 && len(c.OAuthAllowedDomains) == 0 {
		return true
	}

	return false
}

// Helper functions
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		return strings.ToLower(value) == "true"
	}
	return fallback
}
