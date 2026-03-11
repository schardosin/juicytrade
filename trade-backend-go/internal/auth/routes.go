package auth

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers authentication routes
func RegisterRoutes(router gin.IRouter, handler *AuthHandler) {
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/login", handler.Login)
		authGroup.POST("/logout", handler.Logout)
		authGroup.GET("/status", handler.Status)
		authGroup.GET("/config", handler.ConfigInfo)
		authGroup.GET("/login", handler.LoginPage)
		authGroup.GET("/user", handler.Me)
		
		// OAuth
		authGroup.GET("/oauth/authorize", handler.OAuthAuthorize)
		authGroup.GET("/oauth/callback", handler.OAuthCallback)
	}
}
