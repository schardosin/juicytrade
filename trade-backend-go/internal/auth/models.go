package auth

import (
	"time"
)

// User represents an authenticated user
type User struct {
	Username  string                 `json:"username"`
	Email     string                 `json:"email,omitempty"`
	FullName  string                 `json:"full_name,omitempty"`
	LastLogin time.Time              `json:"last_login"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	User        *User  `json:"user"`
}

// LogoutResponse represents the logout response
type LogoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// AuthStatus represents the authentication status
type AuthStatus struct {
	Authenticated bool       `json:"authenticated"`
	Method        string     `json:"method"`
	User          *User      `json:"user"`
	ExpiresAt     *time.Time `json:"expires_at"`
}
