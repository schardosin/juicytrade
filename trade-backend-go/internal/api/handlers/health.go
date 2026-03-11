package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"trade-backend-go/internal/config"
	"trade-backend-go/internal/models"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
// Exact conversion of Python health check logic.
type HealthHandler struct {
	strategyServiceURL string
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler() *HealthHandler {
	cfg := config.LoadSettings()
	return &HealthHandler{
		strategyServiceURL: cfg.StrategyServiceURL,
	}
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
	// Check strategy service status
	strategiesConnected, strategiesStatus := h.checkStrategyService()

	response := models.NewApiResponse(
		true,
		map[string]interface{}{
			"status":    "healthy",
			"service":   "trade-backend-go",
			"version":   "1.0.0",
			"uptime":    "running",  // TODO: Calculate actual uptime
			"providers": []string{}, // TODO: Get from provider manager
			"connections": map[string]interface{}{
				"streaming": false, // TODO: Get actual streaming status
				"database":  false, // TODO: Get actual database status
			},
			"strategies_connected": strategiesConnected,
			"strategies_status":    strategiesStatus,
			"timestamp":            time.Now().Format(time.RFC3339),
		},
		nil,
		stringPtr("Detailed health check"),
	)

	c.JSON(http.StatusOK, response)
}

// checkStrategyService checks if the strategy service is running
func (h *HealthHandler) checkStrategyService() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := h.strategyServiceURL + "/api/strategies/status"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, "connection error"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "unreachable"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "status code: " + string(rune(resp.StatusCode))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "parse error"
	}

	status, ok := result["status"].(string)
	if !ok {
		return false, "invalid response"
	}

	if status == "ready" || status == "degraded" {
		return true, status
	}

	return false, status
}
