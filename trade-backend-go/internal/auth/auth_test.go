package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConfigLoading(t *testing.T) {
	os.Setenv("AUTH_METHOD", "simple")
	os.Setenv("AUTH_SIMPLE_USERNAME", "admin")
	os.Setenv("AUTH_SIMPLE_PASSWORD", "secret")
	defer os.Unsetenv("AUTH_METHOD")
	defer os.Unsetenv("AUTH_SIMPLE_USERNAME")
	defer os.Unsetenv("AUTH_SIMPLE_PASSWORD")

	cfg := LoadConfig()
	if cfg.Method != AuthMethodSimple {
		t.Errorf("Expected method simple, got %s", cfg.Method)
	}
	if cfg.SimpleUsername != "admin" {
		t.Errorf("Expected username admin, got %s", cfg.SimpleUsername)
	}
}

func TestJWT(t *testing.T) {
	cfg := &AuthConfig{
		JWTSecretKey:     "test-secret",
		JWTExpireMinutes: 60,
		JWTAlgorithm:     "HS256",
	}

	user := &User{
		Username: "testuser",
		Email:    "test@example.com",
		FullName: "Test User",
	}

	token, err := CreateAccessToken(user, cfg)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	validatedUser, err := ValidateToken(token, cfg)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if validatedUser.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, validatedUser.Username)
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &AuthConfig{
		Method:           AuthMethodSimple,
		SimpleUsername:   "admin",
		SimplePassword:   "secret",
		JWTSecretKey:     "test-secret",
		JWTExpireMinutes: 60,
		JWTAlgorithm:     "HS256",
		SessionCookieName: "session",
	}

	router := gin.New()
	router.Use(AuthenticationMiddleware(cfg))
	router.GET("/protected", func(c *gin.Context) {
		user := GetCurrentUser(c)
		c.JSON(200, gin.H{"user": user.Username})
	})

	// Test unauthenticated
	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound { // Redirects to login
		t.Errorf("Expected status 302, got %d", w.Code)
	}

	// Test authenticated with cookie
	user := &User{Username: "admin"}
	token, _ := CreateAccessToken(user, cfg)
	
	req = httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
