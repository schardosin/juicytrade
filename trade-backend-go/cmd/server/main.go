package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"trade-backend-go/internal/api/handlers"
	"trade-backend-go/internal/config"
	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers"
	"trade-backend-go/internal/services/ivx"
	"trade-backend-go/internal/streaming"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.LoadSettings()
	
	// Set Gin mode based on config
	if cfg.LogLevel == "DEBUG" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	
	// Create Gin router
	router := gin.Default()
	
	// Add CORS middleware (same as Python FastAPI CORS)
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})
	
	// Initialize provider manager (same as Python provider_manager)
	providers.InitializeProviderManager()
	providerManager := providers.GlobalProviderManager

	// Initialize IVx services
	ivxCache := ivx.NewCache()

	// Initialize streaming manager and connect to providers
	streamingMgr := streaming.GetStreamingManager()
	ctx := context.Background()
	if err := streamingMgr.Connect(ctx); err != nil {
		log.Printf("Warning: Failed to initialize streaming: %v", err)
	}
	
	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	marketDataHandler := handlers.NewMarketDataHandler(providerManager)
	webSocketHandler := handlers.NewWebSocketHandler()
	
	// Setup routes - exact same paths as Python FastAPI
	router.GET("/", healthHandler.Health)
	router.GET("/health", healthHandler.HealthCheck)
	
	// Symbol-specific endpoints - MUST be first to avoid conflicts
	router.GET("/symbol/:symbol/range/52week", func(c *gin.Context) {
		symbol := c.Param("symbol")
		
		// Get 1 year of daily data (same as Python implementation)
		endDate := time.Now()
		startDate := endDate.AddDate(-1, 0, 0) // 1 year ago
		
		startDateStr := startDate.Format("2006-01-02")
		endDateStr := endDate.Format("2006-01-02")
		
		bars, err := providerManager.GetHistoricalBars(c.Request.Context(), symbol, "D", &startDateStr, &endDateStr, 365)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if len(bars) == 0 {
			c.JSON(404, gin.H{
				"success": false,
				"message": "No historical data available for symbol",
			})
			return
		}
		
		// Calculate 52-week high and low
		var high, low float64
		for i, bar := range bars {
			if barHigh, ok := bar["high"].(float64); ok {
				if i == 0 || barHigh > high {
					high = barHigh
				}
			}
			if barLow, ok := bar["low"].(float64); ok {
				if i == 0 || barLow < low {
					low = barLow
				}
			}
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"symbol":     symbol,
				"high_52w":   high,
				"low_52w":    low,
				"data_points": len(bars),
			},
			"message": fmt.Sprintf("Retrieved 52-week range for %s", symbol),
		})
	})
	
	router.GET("/symbol/:symbol/volume/average", func(c *gin.Context) {
		symbol := c.Param("symbol")
		days := 20
		
		if daysStr := c.Query("days"); daysStr != "" {
			if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil {
				days = 20
			}
		}
		
		// Get historical data for the specified number of days (same as Python implementation)
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -days*2) // Get double to account for weekends/holidays
		
		startDateStr := startDate.Format("2006-01-02")
		endDateStr := endDate.Format("2006-01-02")
		
		bars, err := providerManager.GetHistoricalBars(c.Request.Context(), symbol, "D", &startDateStr, &endDateStr, days*2)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if len(bars) == 0 {
			c.JSON(404, gin.H{
				"success": false,
				"message": "No historical data available for symbol",
			})
			return
		}
		
		// Calculate average volume
		var totalVolume float64
		validBars := 0
		for _, bar := range bars {
			if volume, ok := bar["volume"].(float64); ok && volume > 0 {
				totalVolume += volume
				validBars++
			}
		}
		
		var averageVolume float64
		if validBars > 0 {
			averageVolume = totalVolume / float64(validBars)
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"symbol":         symbol,
				"average_volume": averageVolume,
				"days":           days,
				"data_points":    validBars,
			},
			"message": fmt.Sprintf("Retrieved %d-day average volume for %s", days, symbol),
		})
	})
	
	// Market data routes - exact same paths as Python
	router.GET("/prices/stocks", marketDataHandler.GetStockPrices)
	router.GET("/expiration_dates", marketDataHandler.GetExpirationDates)
	
	// Account & Portfolio endpoints - exact same paths as Python
	router.GET("/account", func(c *gin.Context) {
		account, err := providerManager.GetAccount(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if account == nil {
			c.JSON(404, gin.H{
				"success": false,
				"message": "Account information not available",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    account,
			"message": "Retrieved account information",
		})
	})
	
	router.GET("/positions", func(c *gin.Context) {
		enhancedPositions, err := providerManager.GetPositionsEnhanced(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{
				"success":   false,
				"data":      nil,
				"error":     err.Error(),
				"message":   err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}
		
		if enhancedPositions == nil {
			c.JSON(404, gin.H{
				"success":   false,
				"data":      nil,
				"error":     "Enhanced positions not available",
				"message":   "Enhanced positions not available",
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success":   true,
			"data":      enhancedPositions,
			"error":     nil,
			"message":   fmt.Sprintf("Retrieved %d symbol groups with hierarchical structure", len(enhancedPositions.SymbolGroups)),
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})
	
	router.GET("/orders", func(c *gin.Context) {
		status := c.DefaultQuery("status", "open")
		
		orders, err := providerManager.GetOrders(c.Request.Context(), status)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		responseData := map[string]interface{}{
			"orders":       orders,
			"total_orders": len(orders),
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    responseData,
			"message": fmt.Sprintf("Retrieved %d orders with status '%s'", len(orders), status),
		})
	})
	
	router.GET("/open_orders", func(c *gin.Context) {
		orders, err := providerManager.GetOrders(c.Request.Context(), "open")
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		responseData := map[string]interface{}{
			"orders":       orders,
			"total_orders": len(orders),
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    responseData,
			"message": fmt.Sprintf("Retrieved %d open orders", len(orders)),
		})
	})
	
	router.POST("/orders", func(c *gin.Context) {
		var orderRequest map[string]interface{}
		if err := c.ShouldBindJSON(&orderRequest); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid order request: " + err.Error(),
			})
			return
		}
		
		order, err := providerManager.PlaceOrder(c.Request.Context(), orderRequest)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if order == nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Failed to place order",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    order,
			"message": "Order placed successfully.",
		})
	})
	
	router.POST("/orders/multi-leg", func(c *gin.Context) {
		var orderRequest map[string]interface{}
		if err := c.ShouldBindJSON(&orderRequest); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid multi-leg order request: " + err.Error(),
			})
			return
		}
		
		order, err := providerManager.PlaceMultiLegOrder(c.Request.Context(), orderRequest)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if order == nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Failed to place multi-leg order",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    order,
			"message": "Multi-leg order placed successfully.",
		})
	})
	
	router.DELETE("/orders/:order_id", func(c *gin.Context) {
		orderID := c.Param("order_id")
		if orderID == "" {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Order ID is required",
			})
			return
		}
		
		result, err := providerManager.CancelOrder(c.Request.Context(), orderID)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if !result {
			c.JSON(400, gin.H{
				"success": false,
				"message": fmt.Sprintf("Failed to cancel order %s", orderID),
			})
			return
		}
		
		responseData := map[string]interface{}{
			"order_id": orderID,
			"status":   "cancelled",
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    responseData,
			"message": fmt.Sprintf("Order %s cancelled successfully", orderID),
		})
	})
	
	// Symbol lookup endpoint - exact same path as Python
	router.GET("/symbols/lookup", func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Query parameter 'q' is required",
			})
			return
		}
		
		results, err := providerManager.LookupSymbols(c.Request.Context(), query)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    map[string]interface{}{"symbols": results},
			"message": fmt.Sprintf("Found %d symbols matching '%s'", len(results), query),
		})
	})
	
	// Options chain endpoints - exact same paths as Python
	router.GET("/options_chain_basic", func(c *gin.Context) {
		symbol := c.Query("symbol")
		expiry := c.Query("expiry")
		if symbol == "" || expiry == "" {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Parameters 'symbol' and 'expiry' are required",
			})
			return
		}
		
		var underlyingPrice *float64
		if priceStr := c.Query("underlying_price"); priceStr != "" {
			var price float64
			if _, err := fmt.Sscanf(priceStr, "%f", &price); err != nil {
				c.JSON(400, gin.H{
					"success": false,
					"message": "Invalid underlying_price parameter",
				})
				return
			}
			underlyingPrice = &price
		}
		
		strikeCount := 20
		if countStr := c.Query("strike_count"); countStr != "" {
			if _, err := fmt.Sscanf(countStr, "%d", &strikeCount); err != nil {
				strikeCount = 20
			}
		}
		
		var optionType *string
		if typeStr := c.Query("type"); typeStr != "" {
			optionType = &typeStr
		}
		
		var underlyingSymbol *string
		if symStr := c.Query("underlying_symbol"); symStr != "" {
			underlyingSymbol = &symStr
		}
		
		contracts, err := providerManager.GetOptionsChainBasic(c.Request.Context(), symbol, expiry, underlyingPrice, strikeCount, optionType, underlyingSymbol)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    contracts,
			"message": fmt.Sprintf("Retrieved %d basic option contracts (%d strikes around ATM, no price data)", len(contracts), strikeCount),
		})
	})
	
	router.GET("/options_chain_smart", func(c *gin.Context) {
		symbol := c.Query("symbol")
		expiry := c.Query("expiry")
		if symbol == "" || expiry == "" {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Parameters 'symbol' and 'expiry' are required",
			})
			return
		}
		
		var underlyingPrice *float64
		if priceStr := c.Query("underlying_price"); priceStr != "" {
			var price float64
			if _, err := fmt.Sscanf(priceStr, "%f", &price); err != nil {
				c.JSON(400, gin.H{
					"success": false,
					"message": "Invalid underlying_price parameter",
				})
				return
			}
			underlyingPrice = &price
		}
		
		atmRange := 20
		if rangeStr := c.Query("atm_range"); rangeStr != "" {
			if _, err := fmt.Sscanf(rangeStr, "%d", &atmRange); err != nil {
				atmRange = 20
			}
		}
		
		includeGreeks := c.Query("include_greeks") == "true"
		strikesOnly := c.Query("strikes_only") == "true"
		
		contracts, err := providerManager.GetOptionsChainSmart(c.Request.Context(), symbol, expiry, underlyingPrice, atmRange, includeGreeks, strikesOnly)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    contracts,
			"message": fmt.Sprintf("Retrieved %d smart option contracts (no price data)", len(contracts)),
		})
	})
	
	router.GET("/options_greeks", func(c *gin.Context) {
		symbols := c.Query("symbols")
		if symbols == "" {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Parameter 'symbols' is required",
			})
			return
		}
		
		symbolList := []string{}
		for _, s := range strings.Split(symbols, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				symbolList = append(symbolList, trimmed)
			}
		}
		
		if len(symbolList) == 0 {
			c.JSON(200, gin.H{
				"success": true,
				"data":    map[string]interface{}{"greeks": map[string]interface{}{}},
				"message": "No symbols provided",
			})
			return
		}
		
		greeksData, err := providerManager.GetOptionsGreeksBatch(c.Request.Context(), symbolList)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"greeks": greeksData,
			},
			"message": fmt.Sprintf("Retrieved Greeks for %d option symbols", len(symbolList)),
		})
	})
	
	// Historical data endpoints - exact same paths as Python
	router.GET("/next_market_date", func(c *gin.Context) {
		nextDate, err := providerManager.GetNextMarketDate(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    map[string]interface{}{"next_market_date": nextDate},
			"message": "Retrieved next market date",
		})
	})
	
	router.GET("/chart/historical/:symbol", func(c *gin.Context) {
		symbol := c.Param("symbol")
		timeframe := c.DefaultQuery("timeframe", "D")
		startDate := c.Query("start_date")
		endDate := c.Query("end_date")
		limit := 500
		
		if limitStr := c.Query("limit"); limitStr != "" {
			if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
				limit = 500
			}
		}
		
		var startPtr, endPtr *string
		if startDate != "" {
			startPtr = &startDate
		}
		if endDate != "" {
			endPtr = &endDate
		}
		
		bars, err := providerManager.GetHistoricalBars(c.Request.Context(), symbol, timeframe, startPtr, endPtr, limit)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"symbol":    symbol,
				"timeframe": timeframe,
				"bars":      bars,
				"count":     len(bars),
			},
			"message": fmt.Sprintf("Retrieved %d bars for %s", len(bars), symbol),
		})
	})
	
	// Provider Configuration endpoints - exact same paths as Python
	router.GET("/providers/config", func(c *gin.Context) {
		config := providerManager.GetConfig()
		c.JSON(200, gin.H{
			"success": true,
			"data":    config,
		})
	})
	
	router.PUT("/providers/config", func(c *gin.Context) {
		var newConfig map[string]interface{}
		if err := c.ShouldBindJSON(&newConfig); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid configuration data: " + err.Error(),
			})
			return
		}
		
		success := providerManager.UpdateConfig(newConfig)
		if success {
			c.JSON(200, gin.H{
				"success": true,
				"message": "Provider config updated successfully.",
				"data":    map[string]interface{}{"streaming_restarted": false},
			})
		} else {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Failed to update provider config",
			})
		}
	})
	
	router.POST("/providers/config/reset", func(c *gin.Context) {
		providerManager.ResetConfig()
		c.JSON(200, gin.H{
			"success": true,
			"message": "Provider config reset to default.",
		})
	})
	
	router.GET("/providers/available", func(c *gin.Context) {
		providers := providerManager.GetAvailableProviders()
		c.JSON(200, gin.H{
			"success": true,
			"data":    providers,
		})
	})
	
	// Provider Types endpoint - exact same path as Python
	router.GET("/providers/types", func(c *gin.Context) {
		providerTypes := providers.GetProviderTypes()
		c.JSON(200, gin.H{
			"success": true,
			"data":    providerTypes,
			"message": "Retrieved provider type definitions",
		})
	})
	
	// Provider Instance Management endpoints - exact same paths as Python
	router.GET("/providers/instances", func(c *gin.Context) {
		instances := providerManager.GetAvailableProviderInstances()
		c.JSON(200, gin.H{
			"success": true,
			"data":    instances,
			"message": fmt.Sprintf("Retrieved %d provider instances", len(instances)),
		})
	})
	
	router.POST("/providers/instances", func(c *gin.Context) {
		var request map[string]interface{}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request data: " + err.Error(),
			})
			return
		}
		
		// Extract required fields
		providerType, _ := request["provider_type"].(string)
		accountType, _ := request["account_type"].(string)
		displayName, _ := request["display_name"].(string)
		credentials, _ := request["credentials"].(map[string]interface{})
		
		if providerType == "" || accountType == "" || displayName == "" {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Missing required fields: provider_type, account_type, display_name",
			})
			return
		}
		
		// Apply defaults to credentials
		credentialsWithDefaults := providers.ApplyDefaults(providerType, accountType, credentials)
		
		// Generate unique instance ID
		credentialStore := providers.NewCredentialStore()
		instanceID := credentialStore.GenerateInstanceID(providerType, accountType, displayName)
		
		// Add instance to credential store
		success := credentialStore.AddInstance(instanceID, providerType, accountType, displayName, credentialsWithDefaults)
		if !success {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Failed to create provider instance",
			})
			return
		}
		
		// Reinitialize providers to include the new instance
		providerManager.InitializeActiveProviders()
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    map[string]interface{}{"instance_id": instanceID},
			"message": fmt.Sprintf("Provider instance '%s' created successfully", displayName),
		})
	})
	
	router.PUT("/providers/instances/:instance_id", func(c *gin.Context) {
		instanceID := c.Param("instance_id")
		
		var request map[string]interface{}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request data: " + err.Error(),
			})
			return
		}
		
		credentialStore := providers.NewCredentialStore()
		
		// Check if instance exists
		instance := credentialStore.GetInstance(instanceID)
		if instance == nil {
			c.JSON(404, gin.H{
				"success": false,
				"message": "Provider instance not found",
			})
			return
		}
		
		// Prepare updates
		updates := make(map[string]interface{})
		if displayName, ok := request["display_name"].(string); ok {
			updates["display_name"] = displayName
		}
		
		if credentials, ok := request["credentials"].(map[string]interface{}); ok {
			// Get existing credentials and merge with new ones
			existingCredentials, _ := instance["credentials"].(map[string]interface{})
			if existingCredentials == nil {
				existingCredentials = make(map[string]interface{})
			}
			
			// Merge credentials (skip empty sensitive fields)
			mergedCredentials := make(map[string]interface{})
			for k, v := range existingCredentials {
				mergedCredentials[k] = v
			}
			for k, v := range credentials {
				if str, ok := v.(string); ok && (str == "" || str == "••••••••") {
					continue // Skip empty or masked values
				}
				mergedCredentials[k] = v
			}
			
			// Apply defaults
			providerType, _ := instance["provider_type"].(string)
			accountType, _ := instance["account_type"].(string)
			credentialsWithDefaults := providers.ApplyDefaults(providerType, accountType, mergedCredentials)
			updates["credentials"] = credentialsWithDefaults
		}
		
		// Update instance
		success := credentialStore.UpdateInstance(instanceID, updates)
		if !success {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Failed to update provider instance",
			})
			return
		}
		
		// Reinitialize providers if credentials were updated
		if _, hasCredentials := request["credentials"]; hasCredentials {
			providerManager.InitializeActiveProviders()
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    map[string]interface{}{"instance_id": instanceID},
			"message": "Provider instance updated successfully",
		})
	})
	
	router.PUT("/providers/instances/:instance_id/toggle", func(c *gin.Context) {
		instanceID := c.Param("instance_id")
		
		credentialStore := providers.NewCredentialStore()
		newActiveState := credentialStore.ToggleInstance(instanceID)
		
		if newActiveState == nil {
			c.JSON(404, gin.H{
				"success": false,
				"message": "Provider instance not found",
			})
			return
		}
		
		// Reinitialize providers to reflect the change
		providerManager.InitializeActiveProviders()
		
		action := "deactivated"
		if *newActiveState {
			action = "activated"
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    map[string]interface{}{"instance_id": instanceID, "active": *newActiveState},
			"message": fmt.Sprintf("Provider instance %s successfully", action),
		})
	})
	
	router.DELETE("/providers/instances/:instance_id", func(c *gin.Context) {
		instanceID := c.Param("instance_id")
		
		credentialStore := providers.NewCredentialStore()
		
		// Check if instance exists
		instance := credentialStore.GetInstance(instanceID)
		if instance == nil {
			c.JSON(404, gin.H{
				"success": false,
				"message": "Provider instance not found",
			})
			return
		}
		
		// Delete the instance
		success := credentialStore.DeleteInstance(instanceID)
		if !success {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Failed to delete provider instance",
			})
			return
		}
		
		// Reinitialize providers to remove the deleted instance
		providerManager.InitializeActiveProviders()
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    map[string]interface{}{"instance_id": instanceID},
			"message": "Provider instance deleted successfully",
		})
	})
	
	router.POST("/providers/instances/test", func(c *gin.Context) {
		var request map[string]interface{}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request data: " + err.Error(),
			})
			return
		}
		
		providerType, _ := request["provider_type"].(string)
		accountType, _ := request["account_type"].(string)
		credentials, _ := request["credentials"].(map[string]interface{})
		
		if providerType == "" || accountType == "" {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Missing required fields: provider_type, account_type",
			})
			return
		}
		
		// Test the connection using provider manager
		result := providerManager.TestProviderCredentials(c.Request.Context(), providerType, accountType, credentials)
		
		success, _ := result["success"].(bool)
		message, _ := result["message"].(string)
		
		c.JSON(200, gin.H{
			"success": success,
			"data":    result,
			"message": message,
		})
	})
	
	// Authentication endpoints - exact same paths as Python
	router.GET("/auth/config", func(c *gin.Context) {
		// Return basic auth config - for now, auth is disabled in Go backend
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"method":               "disabled",
				"enabled":              false,
				"oauth_provider":       nil,
				"supports_methods":     []string{"simple", "oauth", "token", "header", "disabled"},
				"session_cookie_name":  "juicytrade_session",
			},
			"message": "Authentication configuration retrieved",
		})
	})
	
	router.GET("/auth/status", func(c *gin.Context) {
		// Return auth status - for now, always unauthenticated since auth is disabled
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"authenticated": false,
				"method":        "disabled",
				"user":          nil,
				"expires_at":    nil,
			},
			"message": "Authentication status retrieved",
		})
	})
	
	router.POST("/auth/login", func(c *gin.Context) {
		// Login endpoint - for now, return error since auth is disabled
		c.JSON(400, gin.H{
			"success": false,
			"message": "Authentication is disabled",
		})
	})
	
	router.POST("/auth/logout", func(c *gin.Context) {
		// Logout endpoint - for now, just return success
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"success": true,
				"message": "Logout successful",
			},
			"message": "Logout successful",
		})
	})
	
	router.GET("/auth/user", func(c *gin.Context) {
		// Get current user - for now, return unauthorized since auth is disabled
		c.JSON(401, gin.H{
			"success": false,
			"message": "Not authenticated",
		})
	})
	
	// Setup endpoints - exact same paths as Python
	router.GET("/setup/status", func(c *gin.Context) {
		// Exact conversion of Python get_setup_status logic
		
		// Get current provider configuration
		config := providerManager.GetConfig()
		
		// Define mandatory services that must be configured (same as Python)
		mandatoryServices := []string{
			"trade_account",
			"options_chain", 
			"historical_data",
			"symbol_lookup",
			"streaming_quotes",
		}
		
		// Check if we have service routing configuration
		if config == nil || len(config) == 0 {
			c.JSON(200, gin.H{
				"success": true,
				"data": map[string]interface{}{
					"is_setup_complete":         false,
					"missing_mandatory_services": mandatoryServices,
					"configured_services":       map[string]interface{}{},
					"has_providers":             false,
				},
				"message": "No service routing configuration found",
			})
			return
		}
		
		// Check each mandatory service
		missingServices := []string{}
		for _, service := range mandatoryServices {
			routedProvider, exists := config[service]
			if !exists || routedProvider == "" {
				missingServices = append(missingServices, service)
			}
		}
		
		isSetupComplete := len(missingServices) == 0
		hasProviders := len(providerManager.GetAvailableProviderInstances()) > 0
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"is_setup_complete":         isSetupComplete,
				"missing_mandatory_services": missingServices,
				"configured_services":       config,
				"has_providers":             hasProviders,
			},
			"message": "Setup status checked successfully",
		})
	})
	
	// Subscription status endpoint - exact same path as Python
	router.GET("/subscriptions/status", func(c *gin.Context) {
		status := providerManager.GetSubscriptionStatus()
		c.JSON(200, gin.H{
			"success":   true,
			"data":      status,
			"error":     nil,
			"message":   "Retrieved subscription status",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})
	
	// IVx (Implied Volatility) endpoints - exact same paths as Python
	router.GET("/api/ivx/:symbol", func(c *gin.Context) {
		symbol := c.Param("symbol")
		startTime := time.Now()

		log.Printf("📊 API request for IVx data for %s", symbol)

		// Check cache first
		cachedData := ivxCache.Get(symbol)
		if cachedData != nil {
			log.Printf("📦 Returning cached IVx data for %s (%d expirations)", symbol, len(cachedData))
			response := models.NewApiResponse(true, models.IVxResponse{
				Symbol:    symbol,
				Expirations: cachedData,
				Cached:    true,
			}, nil, stringPtr(fmt.Sprintf("Retrieved cached IVx data for %s (%d expirations)", symbol, len(cachedData))))
			c.JSON(200, response)
			return
		}

		// Calculate fresh data
		log.Printf("🔄 Calculating fresh IVx data for %s", symbol)

		// Get all available expirations
		expirationDatesRaw, err := providerManager.GetExpirationDates(c.Request.Context(), symbol)
		if err != nil || len(expirationDatesRaw) == 0 {
		response := models.NewApiResponse(false, models.IVxResponse{
			Symbol:     symbol,
			Expirations: []models.IVxExpiration{},
		}, stringPtr("No expiration dates available"), stringPtr(fmt.Sprintf("No expiration dates available for %s", symbol)))
			c.JSON(404, response)
			return
		}

		// expirationDatesRaw is already []map[string]interface{}, no conversion needed
		expirationDates := expirationDatesRaw

		if len(expirationDates) == 0 {
			response := models.NewApiResponse(false, models.IVxResponse{
				Symbol:     symbol,
				Expirations: []models.IVxExpiration{},
			}, stringPtr("No valid expiration dates available"), stringPtr(fmt.Sprintf("No valid expiration dates available for %s", symbol)))
			c.JSON(404, response)
			return
		}

		// Get underlying price with fallback logic (same as Python)
		underlyingPrice, err := getUnderlyingPrice(c.Request.Context(), providerManager, symbol)
		if err != nil || underlyingPrice == nil {
			log.Printf("❌ No valid price available for %s: %v", symbol, err)
			response := models.NewApiResponse(false, models.IVxResponse{
				Symbol:     symbol,
				Expirations: []models.IVxExpiration{},
			}, stringPtr("No valid price available"), stringPtr(fmt.Sprintf("No valid price available for %s", symbol)))
			c.JSON(404, response)
			return
		}

		// Process expirations in parallel
		ivxResults := processExpirationsParallel(c.Request.Context(), expirationDates, providerManager, *underlyingPrice, symbol)

		calculationTime := time.Since(startTime).Seconds()

		// Cache the results if we have any
		if len(ivxResults) > 0 {
			ivxCache.Set(symbol, ivxResults)
			log.Printf("💾 Cached IVx data for %s (%d expirations)", symbol, len(ivxResults))
		}

		response := models.NewApiResponse(true, models.IVxResponse{
			Symbol:          symbol,
			Expirations:     ivxResults,
			Cached:          false,
			CalculationTime: &calculationTime,
		}, nil, stringPtr(fmt.Sprintf("Calculated IVx data for %s (%d expirations) in %.2fs", symbol, len(ivxResults), calculationTime)))

		c.JSON(200, response)
	})
	
	// WebSocket endpoint - exact same path as Python
	router.GET("/ws", webSocketHandler.HandleWebSocket)
	
	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: router,
	}
	
	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s:%d", cfg.Host, cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	
	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
	
	// Graceful shutdown with timeout (same as Python)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	
	log.Println("Server exited")
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// getUnderlyingPrice gets the underlying price with fallback logic (same as Python implementation)
func getUnderlyingPrice(ctx context.Context, providerManager *providers.ProviderManager, symbol string) (*float64, error) {
	log.Printf("🔍 Getting stock quote for %s", symbol)
	quote, err := providerManager.GetStockQuote(ctx, symbol)
	if err != nil {
		log.Printf("⚠️ Error getting stock quote for %s: %v", symbol, err)
		// Don't return error immediately, try fallback
	}

	log.Printf("🔍 Quote result: %+v", quote)

	var underlyingPrice *float64
	if quote != nil {
		// Try to get price from last, then bid/ask midpoint, then individual bid or ask
		if quote.Last != nil && *quote.Last > 0 {
			underlyingPrice = quote.Last
			log.Printf("📈 Using last price for %s: $%.2f", symbol, *underlyingPrice)
		} else if quote.Bid != nil && quote.Ask != nil && *quote.Bid > 0 && *quote.Ask > 0 {
			midpoint := (*quote.Bid + *quote.Ask) / 2
			underlyingPrice = &midpoint
			log.Printf("📈 Using bid/ask midpoint for %s: $%.2f (bid: $%.2f, ask: $%.2f)", symbol, *underlyingPrice, *quote.Bid, *quote.Ask)
		} else if quote.Bid != nil && *quote.Bid > 0 {
			underlyingPrice = quote.Bid
			log.Printf("📈 Using bid price for %s: $%.2f", symbol, *underlyingPrice)
		} else if quote.Ask != nil && *quote.Ask > 0 {
			underlyingPrice = quote.Ask
			log.Printf("📈 Using ask price for %s: $%.2f", symbol, *underlyingPrice)
		}
	}

	// Fallback for indices like SPX where direct quotes might fail (same as Python)
	if underlyingPrice == nil {
		log.Printf("⚠️ Direct quote failed for %s, trying fallback via options chain", symbol)
		// Try to get underlying price from options chain (same as Python implementation)
		// Get the first available expiration to extract underlying price
		expirationDatesRaw, err := providerManager.GetExpirationDates(ctx, symbol)
		if err == nil && len(expirationDatesRaw) > 0 {
			// Use the first expiration date
			firstExpiry := ""
			expMap := expirationDatesRaw[0]
			if expMap != nil {
				if date, ok := expMap["date"].(string); ok {
					firstExpiry = date
				}
			}

			if firstExpiry != "" {
				log.Printf("🔄 Trying options chain fallback for %s using expiry %s", symbol, firstExpiry)

				// Get basic options chain for this expiration
				optionsChain, err := providerManager.GetOptionsChainBasic(ctx, symbol, firstExpiry, nil, 5, nil, &symbol)
				if err == nil && len(optionsChain) > 0 {
					// Some providers include underlying price in options response
					// For now, estimate from ATM strikes as fallback (same as Python)
					strikes := make([]float64, len(optionsChain))
					for i, contract := range optionsChain {
						if contract != nil {
							strikes[i] = contract.StrikePrice
						}
					}

					if len(strikes) > 0 {
						// Sort strikes and use middle one as approximation
						sort.Float64s(strikes)
						estimatedPrice := strikes[len(strikes)/2]
						underlyingPrice = &estimatedPrice
						log.Printf("📈 Estimated underlying price from strikes for %s: $%.2f", symbol, *underlyingPrice)
					}
				} else {
					log.Printf("❌ Options chain fallback failed for %s: %v", symbol, err)
				}
			}
		} else {
			log.Printf("❌ Could not get expiration dates for fallback: %v", err)
		}
	}

	if underlyingPrice == nil {
		return nil, fmt.Errorf("no valid price available for %s", symbol)
	}

	return underlyingPrice, nil
}

// processExpirationsParallel processes multiple expirations concurrently using goroutines
func processExpirationsParallel(ctx context.Context, expirationDates []map[string]interface{}, providerManager *providers.ProviderManager, underlyingPrice float64, symbol string) []models.IVxExpiration {
	if len(expirationDates) == 0 {
		return []models.IVxExpiration{}
	}

	log.Printf("🔄 Processing %d expiration dates for %s", len(expirationDates), symbol)

	// Use a semaphore for concurrency control (similar to Python's semaphore)
	concurrencyLimit := 10 // Increased from 5 to reduce batching pauses
	semaphore := make(chan struct{}, concurrencyLimit)
	results := make(chan *models.IVxExpiration, len(expirationDates))

	// Worker function to process a single expiration
	processExpiration := func(expirationDate map[string]interface{}) {
		defer func() { <-semaphore }() // Release semaphore

		// Extract date string and other info
		dateStr, ok := expirationDate["date"].(string)
		if !ok || dateStr == "" {
			log.Printf("⚠️ Expiration missing 'date' field: %+v", expirationDate)
			results <- nil
			return
		}

		dateStrSymbol, _ := expirationDate["symbol"].(string)
		if dateStrSymbol == "" {
			dateStrSymbol = symbol
		}

		log.Printf("📅 Processing expiration: %s", dateStr)

		// Get basic options chain (same as Python - step 1)
		optionsChain, err := providerManager.GetOptionsChainBasic(ctx, dateStrSymbol, dateStr, &underlyingPrice, 20, nil, &symbol)
		if err != nil {
			log.Printf("❌ Error getting options chain for %s %s: %v", symbol, dateStr, err)
			results <- nil
			return
		}

		if len(optionsChain) == 0 {
			log.Printf("⚠️ No options chain for %s %s", symbol, dateStr)
			results <- nil
			return
		}

		// Find ATM strike (same as Python - step 2)
		atmStrike := 0.0
		minDiff := math.MaxFloat64
		for _, contract := range optionsChain {
			if contract != nil {
				diff := math.Abs(contract.StrikePrice - underlyingPrice)
				if diff < minDiff {
					minDiff = diff
					atmStrike = contract.StrikePrice
				}
			}
		}

		if atmStrike == 0 {
			log.Printf("⚠️ Could not find ATM strike for %s %s", symbol, dateStr)
			results <- nil
			return
		}

		// Find ATM call and put contracts (same as Python - step 3)
		var atmCall, atmPut *models.OptionContract
		for _, contract := range optionsChain {
			if contract != nil && contract.StrikePrice == atmStrike {
				if contract.Type == "call" {
					atmCall = contract
				} else if contract.Type == "put" {
					atmPut = contract
				}
			}
		}

		if atmCall == nil || atmPut == nil {
			log.Printf("⚠️ Could not find ATM call/put for %s %s at strike %.2f", symbol, dateStr, atmStrike)
			results <- nil
			return
		}

		// Get live greeks/IV data for ATM options (same as Python - step 4)
		symbolsToFetch := []string{atmCall.Symbol, atmPut.Symbol}
		
		// Python uses get_streaming_greeks_batch, but Go doesn't have this method yet
		// For now, use the regular GetOptionsGreeksBatch method
		greeksResults, err := providerManager.GetOptionsGreeksBatch(ctx, symbolsToFetch)
		if err != nil {
			log.Printf("❌ Error fetching greeks for %s %s: %v", symbol, dateStr, err)
			results <- nil
			return
		}

		// Extract valid IVs (same as Python - step 5)
		var validIVs []float64
		for _, greeks := range greeksResults {
			if greeks != nil {
				if iv, ok := greeks["implied_volatility"].(float64); ok && iv > 0 {
					validIVs = append(validIVs, iv)
				}
			}
		}

		if len(validIVs) == 0 {
			log.Printf("⚠️ Failed to get IV for ATM options for %s %s", symbol, dateStr)
			results <- nil
			return
		}

		// Calculate IVx (average of ATM call and put IV - same as Python)
		ivx := 0.0
		for _, iv := range validIVs {
			ivx += iv
		}
		ivx /= float64(len(validIVs))

		// Calculate DTE with Eastern Time logic (same as Python)
		dte, isExpired := calculateDaysToExpiration(dateStr)
		if dte <= 0 && !isExpired {
			log.Printf("⏰ Skipping truly expired expiration %s %s (DTE: %.4f)", symbol, dateStr, dte)
			results <- nil
			return
		}

		// For very small DTE (expired 0DTE), ensure minimum calculation value
		if dte < 0.0001 {
			dte = 0.0001
		}

		// Calculate expected move with time precision (same as Python)
		expectedMove := underlyingPrice * ivx * math.Sqrt(dte/365)

		// Adjust IVx for expired options (same as Python)
		adjustedIVx := ivx
		calculationMethod := "atm_iv_average"
		if isExpired {
			// For expired options, IVx should be near zero since there's no time value
			adjustedIVx = 0.001 // Very small value (0.1%)
			expectedMove = underlyingPrice * adjustedIVx * math.Sqrt(dte/365)
			calculationMethod = "atm_iv_average_expired_adjusted"
		}

		// Create result (same format as Python)
		result := &models.IVxExpiration{
			ExpirationDate:      dateStr,
			DaysToExpiration:    dte,
			IVxPercent:          math.Round(adjustedIVx*100*100) / 100, // Round to 2 decimal places
			ExpectedMoveDollars: math.Round(expectedMove*100) / 100,    // Round to 2 decimal places
			CalculationMethod:   calculationMethod,
			OptionsCount:        len(optionsChain),
			IsExpired:           isExpired,
		}

		results <- result
	}

	// Start goroutines for all expirations
	for _, exp := range expirationDates {
		semaphore <- struct{}{} // Acquire semaphore
		go processExpiration(exp)
	}

	// Collect results
	var ivxResults []models.IVxExpiration
	for i := 0; i < len(expirationDates); i++ {
		result := <-results
		if result != nil {
			ivxResults = append(ivxResults, *result)
		}
	}

	close(results)
	log.Printf("✅ Successfully calculated IVx for %d/%d expirations for %s", len(ivxResults), len(expirationDates), symbol)

	return ivxResults
}

// calculateDaysToExpiration calculates DTE with Eastern Time logic (same as Python)
func calculateDaysToExpiration(expirationDate string) (float64, bool) {
	// Parse expiration date
	expDate, err := time.Parse("2006-01-02", expirationDate)
	if err != nil {
		log.Printf("❌ Error parsing expiration date %s: %v", expirationDate, err)
		return 0, false
	}

	// Always use Eastern Time for all date calculations (fixes UTC/EST server issues)
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("❌ Error loading Eastern timezone: %v", err)
		return 0, false
	}

	now := time.Now().In(et)
	today := now.Truncate(24 * time.Hour)
	expDate = expDate.In(et)

	// For same-day expiration (0DTE), calculate fractional time to 4:00 PM ET
	if expDate.Equal(today) {
		// Market close is 4:00 PM ET
		marketClose := time.Date(today.Year(), today.Month(), today.Day(), 16, 0, 0, 0, et)

		// If after market close, use minimal time for expired 0DTE calculation
		if now.After(marketClose) {
			log.Printf("📅 0DTE after market close for %s - using minimal time for expired calculation", expirationDate)
			return 0.0001, true // Very small value (about 0.1 hours) to show expired but calculate IVx
		} else {
			// Calculate fractional time remaining until market close
			timeRemaining := marketClose.Sub(now)
			dteHours := timeRemaining.Hours()
			dte := dteHours / 24 // Convert to fractional days

			log.Printf("📅 0DTE calculation for %s: %.2f hours = %.4f days remaining", expirationDate, dteHours, dte)
			return dte, false
		}
	} else {
		// For future expirations, use standard day calculation but add time precision
		fullDays := expDate.Sub(today).Hours() / 24

		// Add fractional day based on current time (assume 4:00 PM ET expiration)
		marketCloseToday := time.Date(today.Year(), today.Month(), today.Day(), 16, 0, 0, 0, et)
		if now.Before(marketCloseToday) {
			timeRemainingToday := marketCloseToday.Sub(now)
			fractionalDay := timeRemainingToday.Hours() / 24
			fullDays += fractionalDay
		}

		// Ensure future dates have positive DTE
		if fullDays <= 0 {
			fullDays = 0.0001 // Minimal value for edge cases
		}

		return fullDays, false
	}
}
