package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	Config *AuthConfig
	// In-memory state storage for OAuth (use Redis in production)
	oauthStates sync.Map
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *AuthConfig) *AuthHandler {
	return &AuthHandler{
		Config: cfg,
	}
}

// Login handles username/password login
func (h *AuthHandler) Login(c *gin.Context) {
	if !h.Config.IsEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Authentication is disabled"})
		return
	}

	var req LoginRequest
	// Support both JSON and Form data
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
			return
		}
	} else {
		req.Username = c.PostForm("username")
		req.Password = c.PostForm("password")
	}

	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Username and password are required"})
		return
	}

	var user *User
	if h.Config.Method == AuthMethodSimple || h.Config.Method == AuthMethodToken {
		if req.Username == h.Config.SimpleUsername && req.Password == h.Config.SimplePassword {
			user = &User{
				Username:  req.Username,
				FullName:  req.Username,
				LastLogin: time.Now(),
			}
		}
	}

	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid credentials"})
		return
	}

	token, err := CreateAccessToken(user, h.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create token"})
		return
	}

	h.setSessionCookie(c, token)

	// If it's a form submission, redirect
	if c.ContentType() != "application/json" {
		next := c.PostForm("next")
		if next == "" {
			next = "/"
		}
		c.Redirect(http.StatusFound, next)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": LoginResponse{
			Success:     true,
			Message:     "Login successful",
			AccessToken: token,
			TokenType:   "bearer",
			ExpiresIn:   h.Config.JWTExpireMinutes * 60,
			User:        user,
		},
		"message": "Login successful",
	})
}

// Logout handles logout
func (h *AuthHandler) Logout(c *gin.Context) {
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": LogoutResponse{
			Success: true,
			Message: "Logout successful",
		},
		"message": "Logout successful",
	})
}

// Status returns the current authentication status
func (h *AuthHandler) Status(c *gin.Context) {
	var user *User
	authenticated := false

	// Manually check auth since this endpoint is exempt
	if h.Config.IsEnabled() {
		if cookie, err := c.Cookie(h.Config.SessionCookieName); err == nil {
			if u, err := ValidateToken(cookie, h.Config); err == nil {
				user = u
				authenticated = true
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": AuthStatus{
			Authenticated: authenticated,
			Method:        string(h.Config.Method),
			User:          user,
		},
		"message": "Authentication status retrieved",
	})
}

// Config returns public auth configuration
func (h *AuthHandler) ConfigInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"method":              h.Config.Method,
			"enabled":             h.Config.IsEnabled(),
			"oauth_provider":      h.Config.OAuthProvider,
			"supports_methods":    []string{"simple", "oauth", "token", "header", "disabled"},
			"session_cookie_name": h.Config.SessionCookieName,
		},
		"message": "Authentication configuration retrieved",
	})
}

// LoginPage renders the login page
func (h *AuthHandler) LoginPage(c *gin.Context) {
	if !h.Config.IsEnabled() {
		c.Writer.WriteString("<h1>Authentication is disabled</h1>")
		return
	}

	// Check if already authenticated
	if cookie, err := c.Cookie(h.Config.SessionCookieName); err == nil {
		if _, err := ValidateToken(cookie, h.Config); err == nil {
			next := c.Query("next")
			if next == "" {
				next = "/"
			}
			c.Redirect(http.StatusFound, next)
			return
		}
	}

	next := c.Query("next")
	
	// Simple HTML template (similar to Python's)
	html := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<title>JuicyTrade - Login</title>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<style>
			body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
			.container { max-width: 400px; margin: 0 auto; background: white; 
						 padding: 40px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
			h1 { text-align: center; color: #333; margin-bottom: 30px; }
			.method-info { text-align: center; margin-bottom: 20px; color: #666; font-size: 14px; }
			input { width: 100%%; padding: 8px; margin-bottom: 15px; border: 1px solid #ddd; border-radius: 4px; box-sizing: border-box; }
			button { width: 100%%; padding: 12px; background: #007bff; color: white; border: none; border-radius: 4px; font-weight: bold; cursor: pointer; }
			.oauth-btn { display: block; width: 100%%; padding: 12px; background: #4285f4; color: white; text-decoration: none; border-radius: 4px; font-weight: bold; text-align: center; box-sizing: border-box; }
		</style>
	</head>
	<body>
		<div class="container">
			<h1>JuicyTrade Login</h1>
			<div class="method-info">Authentication Method: %s</div>
			%s
			%s
		</div>
	</body>
	</html>
	`, strings.Title(string(h.Config.Method)), h.renderOAuthButton(next), h.renderSimpleForm(next))

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}

func (h *AuthHandler) renderOAuthButton(next string) string {
	if h.Config.Method != AuthMethodOAuth || h.Config.OAuthProvider == "" {
		return ""
	}
	
	url := "/auth/oauth/authorize"
	if next != "" {
		url += "?next=" + urlEncode(next)
	}
	
	return fmt.Sprintf(`<a href="%s" class="oauth-btn">Login with %s</a>`, url, strings.Title(string(h.Config.OAuthProvider)))
}

func (h *AuthHandler) renderSimpleForm(next string) string {
	if h.Config.Method != AuthMethodSimple && h.Config.Method != AuthMethodToken {
		return ""
	}

	nextInput := ""
	if next != "" {
		nextInput = fmt.Sprintf(`<input type="hidden" name="next" value="%s">`, next)
	}

	return fmt.Sprintf(`
	<form method="post" action="/auth/login">
		%s
		<label>Username:</label>
		<input type="text" name="username" required>
		<label>Password:</label>
		<input type="password" name="password" required>
		<button type="submit">Login</button>
	</form>
	`, nextInput)
}

// OAuthAuthorize initiates OAuth flow
func (h *AuthHandler) OAuthAuthorize(c *gin.Context) {
	if h.Config.Method != AuthMethodOAuth {
		c.JSON(http.StatusBadRequest, gin.H{"message": "OAuth not configured"})
		return
	}

	conf := h.getOAuthConfig()
	if conf == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "OAuth configuration error"})
		return
	}

	// Generate state
	b := make([]byte, 32)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	// Get next parameter, but filter out OAuth callback URLs to prevent loops
	next := c.Query("next")
	if strings.Contains(next, "/auth/oauth/callback") || strings.Contains(next, "/auth/oauth/authorize") {
		next = "/" // Reset to home if trying to use OAuth URLs as next
	}

	// Store state
	h.oauthStates.Store(state, map[string]interface{}{
		"created_at": time.Now(),
		"next":       next,
	})

	url := conf.AuthCodeURL(state)
	c.Redirect(http.StatusFound, url)
}

// OAuthCallback handles OAuth callback
func (h *AuthHandler) OAuthCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")

	// Validate state
	stateData, ok := h.oauthStates.Load(state)
	if !ok {
		c.String(http.StatusBadRequest, "Invalid or expired state parameter. Please try logging in again.")
		return
	}
	h.oauthStates.Delete(state) // Consume state

	conf := h.getOAuthConfig()
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to exchange authorization code: %v", err)
		return
	}

	// Get user info
	client := conf.Client(context.Background(), token)
	userInfo, err := h.getUserInfo(client)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user information: %v", err)
		return
	}

	// Check authorization
	email, ok := userInfo["email"].(string)
	if !ok || email == "" {
		c.String(http.StatusInternalServerError, "Failed to get email from OAuth provider")
		return
	}
	
	if !h.Config.IsUserAuthorized(email) {
		c.String(http.StatusForbidden, "Access Denied: %s is not authorized", email)
		return
	}

	// Create user
	name, _ := userInfo["name"].(string)
	if name == "" {
		name = email
	}
	
	user := &User{
		Username:  email, // Use email as username
		Email:     email,
		FullName:  name,
		LastLogin: time.Now(),
		Metadata: map[string]interface{}{
			"oauth_provider": h.Config.OAuthProvider,
		},
	}

	accessToken, err := CreateAccessToken(user, h.Config)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create access token: %v", err)
		return
	}

	// Set session cookie with explicit settings
	h.setSessionCookie(c, accessToken)

	// Redirect to home or specified next URL
	data := stateData.(map[string]interface{})
	next := "/"
	if n, ok := data["next"].(string); ok && n != "" && n != "/auth/oauth/callback" {
		// Ensure we don't redirect back to the callback URL
		next = n
	}

	c.Redirect(http.StatusFound, next)
}

// Me returns current user info
func (h *AuthHandler) Me(c *gin.Context) {
	user := GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Not authenticated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
		"message": "User information retrieved",
	})
}

// Helpers

func (h *AuthHandler) setSessionCookie(c *gin.Context, token string) {
	maxAge := h.Config.JWTExpireMinutes * 60
	
	// Cookie domain should be empty or start with a dot for proper domain matching
	// If domain is "example.com", browser won't accept it - needs to be "" or ".example.com"
	domain := h.Config.CookieDomain
	if domain != "" && !strings.HasPrefix(domain, ".") {
		domain = "" // Use current domain instead
	}
	
	c.SetCookie(
		h.Config.SessionCookieName,
		token,
		maxAge,
		"/",
		domain,
		h.Config.SecureCookies,
		false, // HttpOnly (false to allow JS access if needed, matching Python)
	)
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	c.SetCookie(
		h.Config.SessionCookieName,
		"",
		-1,
		"/",
		h.Config.CookieDomain,
		h.Config.SecureCookies,
		true,
	)
}

func (h *AuthHandler) getOAuthConfig() *oauth2.Config {
	var endpoint oauth2.Endpoint
	var scopes []string

	switch h.Config.OAuthProvider {
	case OAuthProviderGoogle:
		endpoint = google.Endpoint
		scopes = []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"}
	case OAuthProviderGithub:
		endpoint = github.Endpoint
		scopes = []string{"user:email"}
	case OAuthProviderMicrosoft:
		endpoint = microsoft.AzureADEndpoint("") // Common
		scopes = []string{"User.Read"}
	default:
		return nil
	}

	return &oauth2.Config{
		ClientID:     h.Config.OAuthClientID,
		ClientSecret: h.Config.OAuthClientSecret,
		RedirectURL:  h.Config.OAuthRedirectURI,
		Scopes:       scopes,
		Endpoint:     endpoint,
	}
}

func (h *AuthHandler) getUserInfo(client *http.Client) (map[string]interface{}, error) {
	var url string
	switch h.Config.OAuthProvider {
	case OAuthProviderGoogle:
		url = "https://www.googleapis.com/oauth2/v2/userinfo"
	case OAuthProviderGithub:
		url = "https://api.github.com/user"
	case OAuthProviderMicrosoft:
		url = "https://graph.microsoft.com/v1.0/me"
	default:
		return nil, fmt.Errorf("unsupported provider")
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Normalize data
	result := make(map[string]interface{})
	
	// Email
	if email, ok := data["email"].(string); ok {
		result["email"] = email
	} else if h.Config.OAuthProvider == OAuthProviderGithub {
		// GitHub might not return email in public profile, need separate call
		// For simplicity, we'll skip that complex logic here and assume it's in the profile or handle it later
		// In a real app, you'd fetch /user/emails
		result["email"] = "" 
	}

	// Name
	if name, ok := data["name"].(string); ok {
		result["name"] = name
	} else if login, ok := data["login"].(string); ok {
		result["name"] = login
	}

	return result, nil
}

func urlEncode(s string) string {
	return url.QueryEscape(s)
}
