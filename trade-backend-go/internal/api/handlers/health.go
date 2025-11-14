package handlers

import (
	"net/http"
	"time"

	"trade-backend-go/internal/models"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
// Exact conversion of Python health check logic.
type HealthHandler struct{}

// NewHealthHandler creates a new health handler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health handles the root health check endpoint.
// Exact conversion of Python root() endpoint.
func (h *HealthHandler) Health(c *gin.Context) {
	response := models.NewApiResponse(
		true,
		map[string]interface{}{
			"status":    "healthy",
			"service":   "trade-backend-go",
			"timestamp": time.Now().Format(time.RFC3339),
		},
		nil,
		stringPtr("Service is running"),
	)
	
	c.JSON(http.StatusOK, response)
}

// HealthCheck handles the detailed health check endpoint.
// Exact conversion of Python health_check() endpoint.
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	response := models.NewApiResponse(
		true,
		map[string]interface{}{
			"status":      "healthy",
			"service":     "trade-backend-go",
			"version":     "1.0.0",
			"uptime":      "running", // TODO: Calculate actual uptime
			"providers":   []string{}, // TODO: Get from provider manager
			"connections": map[string]interface{}{
				"streaming": false, // TODO: Get actual streaming status
				"database":  false, // TODO: Get actual database status
			},
			"timestamp": time.Now().Format(time.RFC3339),
		},
		nil,
		stringPtr("Detailed health check"),
	)
	
	c.JSON(http.StatusOK, response)
}
