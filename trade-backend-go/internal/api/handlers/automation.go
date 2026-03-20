package handlers

import (
	"net/http"

	"trade-backend-go/internal/automation"
	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/providers"

	"github.com/gin-gonic/gin"
)

// AutomationHandler handles automation-related HTTP endpoints
type AutomationHandler struct {
	engine *automation.Engine
}

// NewAutomationHandler creates a new automation handler
func NewAutomationHandler(pm *providers.ProviderManager) (*AutomationHandler, error) {
	engine, err := automation.NewEngine(pm)
	if err != nil {
		return nil, err
	}

	return &AutomationHandler{
		engine: engine,
	}, nil
}

// GetEngine returns the automation engine for WebSocket integration
func (h *AutomationHandler) GetEngine() *automation.Engine {
	return h.engine
}

// ListConfigs returns all automation configurations
func (h *AutomationHandler) ListConfigs(c *gin.Context) {
	configs := h.engine.GetStorage().GetAll()

	// Add running status to each config
	result := make([]map[string]interface{}, len(configs))
	for i, cfg := range configs {
		result[i] = map[string]interface{}{
			"config":     cfg,
			"is_running": h.engine.IsRunning(cfg.ID),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"configs": result,
			"total":   len(configs),
		},
		"message": "Retrieved automation configurations",
	})
}

// GetConfig returns a single automation configuration
func (h *AutomationHandler) GetConfig(c *gin.Context) {
	id := c.Param("id")

	config, err := h.engine.GetStorage().Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"config":     config,
			"is_running": h.engine.IsRunning(id),
		},
		"message": "Retrieved automation configuration",
	})
}

// CreateConfig creates a new automation configuration
func (h *AutomationHandler) CreateConfig(c *gin.Context) {
	var config types.AutomationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate required fields
	if config.Name == "" || config.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Name and symbol are required",
		})
		return
	}

	// Set defaults if not provided — only apply default indicators if both
	// Indicators and IndicatorGroups are empty
	if len(config.Indicators) == 0 && len(config.IndicatorGroups) == 0 {
		config.Indicators = []types.IndicatorConfig{
			types.NewIndicatorConfig(types.IndicatorVIX),
			types.NewIndicatorConfig(types.IndicatorGap),
			types.NewIndicatorConfig(types.IndicatorRange),
			types.NewIndicatorConfig(types.IndicatorTrend),
			types.NewIndicatorConfig(types.IndicatorCalendar),
		}
	}

	if config.EntryTime == "" {
		config.EntryTime = "12:25"
	}

	if config.EntryTimezone == "" {
		config.EntryTimezone = "America/New_York"
	}

	if config.TradeConfig.Strategy == "" {
		config.TradeConfig = types.NewTradeConfiguration()
	}

	if err := h.engine.GetStorage().Create(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
		"message": "Automation configuration created successfully",
	})
}

// UpdateConfig updates an existing automation configuration
func (h *AutomationHandler) UpdateConfig(c *gin.Context) {
	id := c.Param("id")

	// Check if running
	if h.engine.IsRunning(id) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Cannot update a running automation. Stop it first.",
		})
		return
	}

	var config types.AutomationConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	config.ID = id

	if err := h.engine.GetStorage().Update(&config); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
		"message": "Automation configuration updated successfully",
	})
}

// DeleteConfig deletes an automation configuration
func (h *AutomationHandler) DeleteConfig(c *gin.Context) {
	id := c.Param("id")

	// Stop if running
	if h.engine.IsRunning(id) {
		_ = h.engine.Stop(id)
	}

	if err := h.engine.GetStorage().Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Automation configuration deleted successfully",
	})
}

// StartAutomation starts an automation
func (h *AutomationHandler) StartAutomation(c *gin.Context) {
	id := c.Param("id")

	if err := h.engine.Start(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	status, _ := h.engine.GetStatus(id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
		"message": "Automation started successfully",
	})
}

// StopAutomation stops an automation
func (h *AutomationHandler) StopAutomation(c *gin.Context) {
	id := c.Param("id")

	if err := h.engine.Stop(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Automation stopped successfully",
	})
}

// ResetTradedToday resets the TradedToday flag for a daily automation
func (h *AutomationHandler) ResetTradedToday(c *gin.Context) {
	id := c.Param("id")

	if err := h.engine.ResetTradedToday(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	status, _ := h.engine.GetStatus(id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
		"message": "TradedToday reset successfully - automation can trade again today",
	})
}

// GetAutomationStatus returns the status of an automation
func (h *AutomationHandler) GetAutomationStatus(c *gin.Context) {
	id := c.Param("id")

	status, err := h.engine.GetStatus(id)
	if err != nil {
		// Not running, check if config exists
		config, configErr := h.engine.GetStorage().Get(id)
		if configErr != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Automation not found",
			})
			return
		}

		// Return idle status with config
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"config":     config,
				"status":     types.StatusIdle,
				"is_running": false,
			},
			"message": "Automation is not running",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
		"message": "Retrieved automation status",
	})
}

// GetAllStatus returns status of all automations
func (h *AutomationHandler) GetAllStatus(c *gin.Context) {
	runningStatus := h.engine.GetAllStatus()
	configs := h.engine.GetStorage().GetAll()

	// Build combined result
	result := make([]map[string]interface{}, 0, len(configs))
	for _, config := range configs {
		if status, running := runningStatus[config.ID]; running {
			result = append(result, map[string]interface{}{
				"config":     config,
				"status":     status.Status,
				"is_running": true,
				"details":    status,
			})
		} else {
			result = append(result, map[string]interface{}{
				"config":     config,
				"status":     types.StatusIdle,
				"is_running": false,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"automations":   result,
			"total":         len(configs),
			"total_running": len(runningStatus),
		},
		"message": "Retrieved all automation statuses",
	})
}

// EvaluateIndicators evaluates indicators without starting the automation
func (h *AutomationHandler) EvaluateIndicators(c *gin.Context) {
	id := c.Param("id")

	config, err := h.engine.GetStorage().Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	groupResults, flatResults, anyGroupPasses := h.engine.EvaluateIndicators(c.Request.Context(), config)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"indicators":    flatResults,
			"group_results": groupResults,
			"all_pass":      anyGroupPasses,
			"symbol":        config.Symbol,
		},
		"message": "Indicators evaluated successfully",
	})
}

// EvaluateIndicatorsPreview evaluates indicators for a config that hasn't been saved yet
func (h *AutomationHandler) EvaluateIndicatorsPreview(c *gin.Context) {
	var request struct {
		Indicators      []types.IndicatorConfig `json:"indicators"`
		IndicatorGroups []types.IndicatorGroup  `json:"indicator_groups"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Determine which groups to evaluate
	var groups []types.IndicatorGroup
	if len(request.IndicatorGroups) > 0 {
		groups = request.IndicatorGroups
	} else if len(request.Indicators) > 0 {
		groups = []types.IndicatorGroup{
			{ID: "preview_default", Name: "Default", Indicators: request.Indicators},
		}
	} else {
		// Existing default indicator fallback
		groups = []types.IndicatorGroup{
			{ID: "preview_default", Name: "Default", Indicators: []types.IndicatorConfig{
				types.NewIndicatorConfig(types.IndicatorVIX),
				types.NewIndicatorConfig(types.IndicatorGap),
				types.NewIndicatorConfig(types.IndicatorRange),
				types.NewIndicatorConfig(types.IndicatorTrend),
				types.NewIndicatorConfig(types.IndicatorCalendar),
			}},
		}
	}

	// Empty configID for preview mode - no caching
	groupResults, flatResults, anyGroupPasses := h.engine.GetIndicatorService().EvaluateIndicatorGroups(
		c.Request.Context(), "", groups)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"indicators":    flatResults,
			"group_results": groupResults,
			"all_pass":      anyGroupPasses,
		},
		"message": "Indicators evaluated successfully",
	})
}

// GetFOMCDates returns the list of FOMC meeting dates
func (h *AutomationHandler) GetFOMCDates(c *gin.Context) {
	dates := h.engine.GetIndicatorService().GetFOMCDates()

	// Convert to string array for JSON
	dateStrings := make([]string, len(dates))
	for i, d := range dates {
		dateStrings[i] = d.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"dates": dateStrings,
			"total": len(dateStrings),
		},
		"message": "Retrieved FOMC meeting dates",
	})
}

// GetIndicatorMetadata returns metadata for all available indicator types
func (h *AutomationHandler) GetIndicatorMetadata(c *gin.Context) {
	metadata := types.GetIndicatorMetadata()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"indicators": metadata,
			"total":      len(metadata),
		},
		"message": "Retrieved indicator metadata",
	})
}

// ToggleEnabled enables or disables an automation config
func (h *AutomationHandler) ToggleEnabled(c *gin.Context) {
	id := c.Param("id")

	var request struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// If disabling and running, stop it
	if !request.Enabled && h.engine.IsRunning(id) {
		_ = h.engine.Stop(id)
	}

	if err := h.engine.GetStorage().SetEnabled(id, request.Enabled); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"id":      id,
			"enabled": request.Enabled,
		},
		"message": "Automation enabled status updated",
	})
}

// PreviewStrikesRequest is the request body for previewing strikes
type PreviewStrikesRequest struct {
	Symbol           string  `json:"symbol" binding:"required"`
	Strategy         string  `json:"strategy" binding:"required"`
	TargetDelta      float64 `json:"target_delta"` // Used for put_spread/call_spread
	Width            int     `json:"width"`        // Used for put_spread/call_spread
	ExpirationMode   string  `json:"expiration_mode"`
	CustomExpiration string  `json:"custom_expiration"`
	// Iron Condor specific - per-side configuration
	PutTargetDelta  float64 `json:"put_target_delta"`  // Put side target delta (iron_condor only)
	PutWidth        int     `json:"put_width"`         // Put side width (iron_condor only)
	CallTargetDelta float64 `json:"call_target_delta"` // Call side target delta (iron_condor only)
	CallWidth       int     `json:"call_width"`        // Call side width (iron_condor only)
}

// GetAutomationLogs returns logs for a specific automation
func (h *AutomationHandler) GetAutomationLogs(c *gin.Context) {
	id := c.Param("id")

	// Try to get from running automation first
	status, err := h.engine.GetStatus(id)
	if err != nil {
		// Not running - return empty logs with message
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"logs":       []interface{}{},
				"total":      0,
				"is_running": false,
			},
			"message": "Automation is not running",
		})
		return
	}

	// Get logs from the active automation
	logs := status.Logs
	if logs == nil {
		logs = []types.AutomationLog{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"logs":       logs,
			"total":      len(logs),
			"is_running": true,
			"status":     status.Status,
			"message":    status.Message,
		},
		"message": "Retrieved automation logs",
	})
}

// PreviewStrikes previews what strikes would be selected for a given configuration
func (h *AutomationHandler) PreviewStrikes(c *gin.Context) {
	var request PreviewStrikesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Set defaults
	if request.ExpirationMode == "" {
		request.ExpirationMode = "0dte"
	}

	strategy := types.TradeStrategy(request.Strategy)

	// Iron Condor: preview both sides
	if strategy == types.StrategyIronCondor {
		config := &types.AutomationConfig{
			Symbol: request.Symbol,
			TradeConfig: types.TradeConfiguration{
				Strategy:         strategy,
				ExpirationMode:   request.ExpirationMode,
				CustomExpiration: request.CustomExpiration,
				PutSideConfig: &types.IronCondorSideConfig{
					TargetDelta: request.PutTargetDelta,
					Width:       request.PutWidth,
				},
				CallSideConfig: &types.IronCondorSideConfig{
					TargetDelta: request.CallTargetDelta,
					Width:       request.CallWidth,
				},
			},
		}

		icStrikes, err := h.engine.PreviewStrikesIronCondor(c.Request.Context(), config)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to preview Iron Condor strikes: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"strategy": "iron_condor",
				"put_side": map[string]interface{}{
					"short_leg":      icStrikes.PutSide.ShortLeg,
					"long_leg":       icStrikes.PutSide.LongLeg,
					"natural_credit": icStrikes.PutSide.NaturalCredit,
					"mid_credit":     icStrikes.PutSide.MidCredit,
					"option_type":    icStrikes.PutSide.OptionType,
				},
				"call_side": map[string]interface{}{
					"short_leg":      icStrikes.CallSide.ShortLeg,
					"long_leg":       icStrikes.CallSide.LongLeg,
					"natural_credit": icStrikes.CallSide.NaturalCredit,
					"mid_credit":     icStrikes.CallSide.MidCredit,
					"option_type":    icStrikes.CallSide.OptionType,
				},
				"spread": map[string]interface{}{
					"total_natural_credit": icStrikes.TotalNaturalCredit,
					"total_mid_credit":     icStrikes.TotalMidCredit,
					"expiry":               icStrikes.Expiry,
				},
			},
			"message": "Iron Condor strike preview generated successfully",
		})
		return
	}

	// Credit spread: existing single-side preview
	config := &types.AutomationConfig{
		Symbol: request.Symbol,
		TradeConfig: types.TradeConfiguration{
			Strategy:         strategy,
			TargetDelta:      request.TargetDelta,
			Width:            request.Width,
			ExpirationMode:   request.ExpirationMode,
			CustomExpiration: request.CustomExpiration,
		},
	}

	// Get the strikes preview
	strikes, err := h.engine.PreviewStrikes(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to preview strikes: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"short_leg": strikes.ShortLeg,
			"long_leg":  strikes.LongLeg,
			"spread": map[string]interface{}{
				"natural_credit": strikes.NaturalCredit,
				"mid_credit":     strikes.MidCredit,
				"expiry":         strikes.Expiry,
			},
			"option_type": strikes.OptionType,
		},
		"message": "Strike preview generated successfully",
	})
}

// GetTrackedOrders returns all tracked orders with automation info
func (h *AutomationHandler) GetTrackedOrders(c *gin.Context) {
	orders := h.engine.GetTrackingStore().GetAllTrackedOrders()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"orders": orders,
			"total":  len(orders),
		},
		"message": "Retrieved tracked orders",
	})
}

// GetTrackedPositions returns all tracked positions with automation info
func (h *AutomationHandler) GetTrackedPositions(c *gin.Context) {
	positions := h.engine.GetTrackingStore().GetAllTrackedPositions()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"positions": positions,
			"total":     len(positions),
		},
		"message": "Retrieved tracked positions",
	})
}

// GetAutomationOrders returns orders for a specific automation
func (h *AutomationHandler) GetAutomationOrders(c *gin.Context) {
	id := c.Param("id")

	// Verify automation exists
	_, err := h.engine.GetStorage().Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Automation not found",
		})
		return
	}

	orders := h.engine.GetTrackingStore().GetOrdersByAutomation(id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"automation_id": id,
			"orders":        orders,
			"total":         len(orders),
		},
		"message": "Retrieved automation orders",
	})
}

// GetAutomationPositions returns positions for a specific automation
func (h *AutomationHandler) GetAutomationPositions(c *gin.Context) {
	id := c.Param("id")

	// Verify automation exists
	_, err := h.engine.GetStorage().Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Automation not found",
		})
		return
	}

	positions := h.engine.GetTrackingStore().GetPositionsByAutomation(id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"automation_id": id,
			"positions":     positions,
			"total":         len(positions),
		},
		"message": "Retrieved automation positions",
	})
}
