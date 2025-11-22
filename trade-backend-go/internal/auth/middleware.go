package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthenticationMiddleware creates a Gin middleware for authentication
func AuthenticationMiddleware(cfg *AuthConfig) gin.HandlerFunc {
	// Default exempt paths
	exemptPaths := []string{
		"/",
		"/docs",
		"/redoc",
		"/openapi.json",
		"/health",
		"/auth/login",
		"/auth/logout",
		"/auth/oauth/authorize",
		"/auth/oauth/callback",
		"/auth/config",
		"/auth/status",
		"/ws",
	}

	return func(c *gin.Context) {
		// Skip if auth is disabled
		if !cfg.IsEnabled() {
			c.Next()
			return
		}

		// Check for exempt paths
		path := c.Request.URL.Path
		for _, exempt := range exemptPaths {
			if path == exempt || path == exempt+"/" {
				c.Next()
				return
			}
		}

		// Special handling for WebSocket upgrade
		if path == "/ws" && strings.ToLower(c.GetHeader("Upgrade")) == "websocket" {
			c.Next()
			return
		}

		var user *User
		var err error

		switch cfg.Method {
		case AuthMethodSimple:
			user, err = authenticateSimple(c, cfg)
		case AuthMethodToken:
			user, err = authenticateToken(c, cfg)
		case AuthMethodHeader:
			user, err = authenticateHeader(c, cfg)
		case AuthMethodOAuth:
			user, err = authenticateOAuth(c, cfg)
		}

		if user != nil {
			c.Set("user", user)
			c.Set("authenticated", true)
			c.Next()
		} else {
			// Authentication failed
			handleAuthError(c, err)
			c.Abort()
		}
	}
}

func authenticateSimple(c *gin.Context, cfg *AuthConfig) (*User, error) {
	// Check session cookie first
	if cookie, err := c.Cookie(cfg.SessionCookieName); err == nil {
		if user, err := ValidateToken(cookie, cfg); err == nil {
			return user, nil
		}
	}

	// Check Basic Auth header
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Basic ") {
		payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if err == nil {
			parts := strings.SplitN(string(payload), ":", 2)
			if len(parts) == 2 {
				username, password := parts[0], parts[1]
				if username == cfg.SimpleUsername && password == cfg.SimplePassword {
					return &User{
						Username:  username,
						FullName:  username,
						LastLogin: time.Now(),
					}, nil
				}
			}
		}
	}

	return nil, nil
}

func authenticateToken(c *gin.Context, cfg *AuthConfig) (*User, error) {
	// Check Bearer token
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return ValidateToken(token, cfg)
	}

	// Check cookie
	if cookie, err := c.Cookie(cfg.SessionCookieName); err == nil {
		return ValidateToken(cookie, cfg)
	}

	return nil, nil
}

func authenticateHeader(c *gin.Context, cfg *AuthConfig) (*User, error) {
	username := c.GetHeader(cfg.HeaderName)
	if username == "" {
		return nil, nil
	}

	return &User{
		Username:  username,
		Email:     c.GetHeader("X-Remote-Email"),
		FullName:  c.GetHeader("X-Remote-Name"),
		LastLogin: time.Now(),
	}, nil
}

func authenticateOAuth(c *gin.Context, cfg *AuthConfig) (*User, error) {
	// OAuth uses session cookies
	if cookie, err := c.Cookie(cfg.SessionCookieName); err == nil {
		return ValidateToken(cookie, cfg)
	}
	return nil, nil
}

func handleAuthError(c *gin.Context, err error) {
	// For API requests, return JSON
	if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Authentication required",
			"error":   "authentication_required",
		})
		return
	}

	// For web requests, redirect to login
	c.Redirect(http.StatusFound, "/auth/login?next="+c.Request.URL.Path)
}

// GetCurrentUser retrieves the user from context
func GetCurrentUser(c *gin.Context) *User {
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(*User); ok {
			return u
		}
	}
	return nil
}
