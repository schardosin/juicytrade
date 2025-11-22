package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"trade-backend-go/internal/api/handlers"
	"trade-backend-go/internal/auth"
	"trade-backend-go/internal/config"
	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers"
	"trade-backend-go/internal/services/ivx"
	"trade-backend-go/internal/services/watchlist"
	"trade-backend-go/internal/streaming"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load .env file is handled by config package via Viper

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
	ivxService := ivx.NewService(providerManager, ivxCache)
	
	// Initialize watchlist manager
	watchlistMgr := watchlist.GetManager()

	// Initialize streaming manager and connect to providers
	streamingMgr := streaming.GetStreamingManager()
	ctx := context.Background()
	if err := streamingMgr.Connect(ctx); err != nil {
		log.Printf("Warning: Failed to initialize streaming: %v", err)
	}
	
	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	marketDataHandler := handlers.NewMarketDataHandler(providerManager)
	// Initialize WebSocket handler with IVx service
	wsHandler := handlers.NewWebSocketHandler(ivxService)

	// Initialize authentication
	authConfig := auth.LoadConfig()
	if err := authConfig.Validate(); err != nil {
		log.Printf("Warning: Authentication configuration error: %v", err)
	}
	authHandler := auth.NewAuthHandler(authConfig)
	
	// Setup API group
	api := router.Group("/api")
	
	// Register auth routes under /api for API access
	auth.RegisterRoutes(api, authHandler)
	
	// Also register auth routes at root level for OAuth callbacks (ingress routes /auth to backend)
	auth.RegisterRoutes(router, authHandler)
	
	// Setup and configuration endpoints (NO AUTH REQUIRED - for setup wizard)
	api.GET("/setup/status", func(c *gin.Context) {
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
		if len(config) == 0 {
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
			"message": "Setup status retrieved successfully",
		})
	})
	
	api.GET("/providers/types", func(c *gin.Context) {
		types := providerManager.GetProviderTypes()
		c.JSON(200, gin.H{
			"success": true,
			"data":    types,
		})
	})
	
	api.GET("/providers/instances", func(c *gin.Context) {
		// Read directly from credential store to get the latest data
		credentialStore := providers.NewCredentialStore()
		instances := credentialStore.GetAllInstances()
		c.JSON(200, gin.H{
			"success": true,
			"data":    instances,
		})
	})
	
	api.POST("/providers/instances/test", func(c *gin.Context) {
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
				"message": "Missing provider_type or account_type",
			})
			return
		}
		
		result := providerManager.TestProviderCredentials(c.Request.Context(), providerType, accountType, credentials)
		c.JSON(200, result)
	})
	
	// Provider instance management endpoints (CREATE, UPDATE, DELETE, TOGGLE)
	api.POST("/providers/instances", func(c *gin.Context) {
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
				"message": "Failed to add provider instance",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"instance_id": instanceID,
			},
			"message": "Provider instance created successfully",
		})
	})
	
	api.PUT("/providers/instances/:instance_id", func(c *gin.Context) {
		instanceID := c.Param("instance_id")
		
		var request map[string]interface{}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request data: " + err.Error(),
			})
			return
		}
		
		// Build updates map
		updates := make(map[string]interface{})
		if displayName, ok := request["display_name"].(string); ok && displayName != "" {
			updates["display_name"] = displayName
		}
		if credentials, ok := request["credentials"].(map[string]interface{}); ok {
			updates["credentials"] = credentials
		}
		
		credentialStore := providers.NewCredentialStore()
		success := credentialStore.UpdateInstance(instanceID, updates)
		if !success {
			c.JSON(404, gin.H{
				"success": false,
				"message": "Provider instance not found",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"message": "Provider instance updated successfully",
		})
	})
	
	api.PUT("/providers/instances/:instance_id/toggle", func(c *gin.Context) {
		instanceID := c.Param("instance_id")
		
		credentialStore := providers.NewCredentialStore()
		newState := credentialStore.ToggleInstance(instanceID)
		if newState == nil {
			c.JSON(404, gin.H{
				"success": false,
				"message": "Provider instance not found",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"active": *newState,
			},
			"message": "Provider instance toggled successfully",
		})
	})
	
	api.DELETE("/providers/instances/:instance_id", func(c *gin.Context) {
		instanceID := c.Param("instance_id")
		
		credentialStore := providers.NewCredentialStore()
		success := credentialStore.DeleteInstance(instanceID)
		if !success {
			c.JSON(404, gin.H{
				"success": false,
				"message": "Provider instance not found",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"message": "Provider instance deleted successfully",
		})
	})
	
	// Apply authentication middleware to API group (all routes below require auth)
	api.Use(auth.AuthenticationMiddleware(authConfig))
	
	// Setup routes - exact same paths as Python FastAPI
	router.GET("/", healthHandler.Health)
	router.GET("/health", healthHandler.HealthCheck)
	
	// WebSocket endpoint
	router.GET("/ws", wsHandler.HandleWebSocket)
	
	// Symbol-specific endpoints - MUST be first to avoid conflicts
	api.GET("/symbol/:symbol/range/52week", func(c *gin.Context) {
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
	
	api.GET("/symbol/:symbol/volume/average", func(c *gin.Context) {
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
	api.GET("/prices/stocks", marketDataHandler.GetStockPrices)
	api.GET("/expiration_dates", marketDataHandler.GetExpirationDates)
	
	// Account & Portfolio endpoints - exact same paths as Python
	api.GET("/account", func(c *gin.Context) {
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
	
	api.GET("/positions", func(c *gin.Context) {
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
	
	api.GET("/orders", func(c *gin.Context) {
		status := c.DefaultQuery("status", "open")
		
		log.Printf("📊 GET /orders - status=%s from IP=%s", status, c.ClientIP())
		
		orders, err := providerManager.GetOrders(c.Request.Context(), status)
		if err != nil {
			log.Printf("❌ GET /orders - Error: %v (status=%s)", err, status)
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		log.Printf("✅ GET /orders - Retrieved %d orders (status=%s)", len(orders), status)
		
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
	
	api.GET("/open_orders", func(c *gin.Context) {
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
	
	api.POST("/orders", func(c *gin.Context) {
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
	
	api.POST("/orders/single-leg", func(c *gin.Context) {
		var orderRequest map[string]interface{}
		if err := c.ShouldBindJSON(&orderRequest); err != nil {
			fmt.Printf("ERROR: Invalid single-leg order request: %v\n", err)
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid single-leg order request: " + err.Error(),
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
				"message": "Failed to place single-leg order",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    order,
			"message": "Single-leg order placed successfully.",
		})
	})

	api.POST("/orders/multi-leg", func(c *gin.Context) {
		var orderRequest map[string]interface{}
		if err := c.ShouldBindJSON(&orderRequest); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid multi-leg order request: " + err.Error(),
			})
			return
		}
		
		log.Printf("DEBUG: Multi-leg order request received: %+v", orderRequest)
		
		order, err := providerManager.PlaceMultiLegOrder(c.Request.Context(), orderRequest)
		if err != nil {
			log.Printf("ERROR: PlaceMultiLegOrder failed: %v (type: %T)", err, err)
			log.Printf("ERROR: Full error details: %+v", err)
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
	
	api.POST("/orders/preview", func(c *gin.Context) {
		var orderRequest map[string]interface{}
		if err := c.ShouldBindJSON(&orderRequest); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid order preview request: " + err.Error(),
			})
			return
		}
		
		previewResult, err := providerManager.PreviewOrder(c.Request.Context(), orderRequest)
		if err != nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if previewResult == nil {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Failed to preview order",
			})
			return
		}
		
		// Check if the preview result indicates an error status
		if status, ok := previewResult["status"].(string); ok && status == "error" {
			// Return 422 for validation errors (same as Python implementation)
			c.JSON(422, gin.H{
				"success": false,
				"data":    previewResult,
				"message": "Order validation failed",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    previewResult,
			"message": "Order preview completed successfully.",
		})
	})
	
	api.DELETE("/orders/:order_id", func(c *gin.Context) {
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
	api.GET("/symbols/lookup", func(c *gin.Context) {
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
	api.GET("/options_chain_basic", func(c *gin.Context) {
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
	
	api.GET("/options_chain_smart", func(c *gin.Context) {
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
	
	api.GET("/options_greeks", func(c *gin.Context) {
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
	api.GET("/next_market_date", func(c *gin.Context) {
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
	
	api.GET("/chart/historical/:symbol", func(c *gin.Context) {
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
	api.GET("/providers/config", func(c *gin.Context) {
		config := providerManager.GetConfig()
		c.JSON(200, gin.H{
			"success": true,
			"data":    config,
		})
	})
	
	api.PUT("/providers/config", func(c *gin.Context) {
		var newConfig map[string]interface{}
		if err := c.ShouldBindJSON(&newConfig); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid configuration data: " + err.Error(),
			})
			return
		}
		
		// Log what the UI sent
		log.Printf("📥 RECEIVED CONFIG FROM UI:")
		for key, value := range newConfig {
			log.Printf("  - %s: %v (type: %T)", key, value, value)
		}
		
		// Get available instances for comparison
		credentialStore := providers.NewCredentialStore()
		availableInstances := credentialStore.GetAllInstances()
		log.Printf("📋 AVAILABLE INSTANCES:")
		for instanceID := range availableInstances {
			log.Printf("  - %s", instanceID)
		}
		
		success := providerManager.UpdateConfig(newConfig)
		
		// Log what actually got saved
		savedConfig := providerManager.GetConfig()
		log.Printf("💾 SAVED CONFIG:")
		for key, value := range savedConfig {
			log.Printf("  - %s: %s", key, value)
		}
		
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
	
	api.POST("/providers/config/reset", func(c *gin.Context) {
		providerManager.ResetConfig()
		c.JSON(200, gin.H{
			"success": true,
			"message": "Provider config reset to default.",
		})
	})
	
	api.GET("/providers/available", func(c *gin.Context) {
		providers := providerManager.GetAvailableProviders()
		c.JSON(200, gin.H{
			"success": true,
			"data":    providers,
		})
	})
	
	// Provider Types endpoint - exact same path as Python
	api.GET("/subscriptions/status", func(c *gin.Context) {
		providerStatus := providerManager.GetSubscriptionStatus()
		wsStats := wsHandler.GetConnectionStats()
		
		// Create response map and add both provider and WebSocket data
		responseData := map[string]interface{}{
			// Provider data
			"quote_subscriptions":        providerStatus.QuoteSubscriptions,
			"greeks_subscriptions":       providerStatus.GreeksSubscriptions,
			"total_quote_subscriptions":  providerStatus.TotalQuoteSubscriptions,
			"total_greeks_subscriptions": providerStatus.TotalGreeksSubscriptions,
			"is_connected":               providerStatus.IsConnected,
			"quote_providers":            providerStatus.QuoteProviders,
			"greeks_providers":           providerStatus.GreeksProviders,
		}
		
		// Add WebSocket data (includes IVx subscriptions)
		for k, v := range wsStats {
			responseData[k] = v
		}
		
		c.JSON(200, gin.H{
			"success":   true,
			"data":      responseData,
			"error":     nil,
			"message":   "Retrieved subscription status",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})
	
	// IVx (Implied Volatility) endpoints - exact same paths as Python
	// IVx endpoint removed in favor of WebSocket streaming
	/*
	api.GET("/ivx/:symbol", func(c *gin.Context) {
		symbol := c.Param("symbol")
		log.Printf("📊 API request for IVx data for %s", symbol)

		response, err := ivxService.GetIVxForSymbol(c.Request.Context(), symbol)
		if err != nil {
			log.Printf("❌ Error calculating IVx for %s: %v", symbol, err)
			
			// Return 404 if no data found, similar to original behavior
			errorResponse := models.NewApiResponse(false, models.IVxResponse{
				Symbol:     symbol,
				Expirations: []models.IVxExpiration{},
			}, stringPtr(err.Error()), stringPtr(fmt.Sprintf("Error calculating IVx: %v", err)))
			c.JSON(404, errorResponse)
			return
		}

		// Wrap in standard API response if not already (GetIVxForSymbol returns IVxResponse struct, we need ApiResponse wrapper)
		// Note: GetIVxForSymbol returns *models.IVxResponse, but we need to wrap it in models.ApiResponse
		// The original code constructed ApiResponse manually.
		
		msg := fmt.Sprintf("Calculated IVx data for %s (%d expirations)", symbol, len(response.Expirations))
		if response.CalculationTime != nil {
			msg += fmt.Sprintf(" in %.2fs", *response.CalculationTime)
		}
		
		apiResponse := models.NewApiResponse(true, *response, nil, stringPtr(msg))
		c.JSON(200, apiResponse)
	})
	*/
	
	// Watchlist endpoints - exact same paths as Python
	api.GET("/watchlists", func(c *gin.Context) {
		data := watchlistMgr.GetAllWatchlists()
		c.JSON(200, gin.H{
			"success": true,
			"data":    data,
			"message": fmt.Sprintf("Retrieved %d watchlists", data.TotalWatchlists),
		})
	})
	
	api.GET("/watchlists/active", func(c *gin.Context) {
		activeWatchlist := watchlistMgr.GetActiveWatchlist()
		activeWatchlistID := watchlistMgr.GetActiveWatchlistID()
		
		if activeWatchlist == nil {
			c.JSON(404, gin.H{
				"success": false,
				"message": "No active watchlist found",
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"active_watchlist_id": activeWatchlistID,
				"active_watchlist":    activeWatchlist,
			},
			"message": "Retrieved active watchlist",
		})
	})
	
	api.PUT("/watchlists/active", func(c *gin.Context) {
		var request models.SetActiveWatchlistRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request: " + err.Error(),
			})
			return
		}
		
		if err := watchlistMgr.SetActiveWatchlist(request.WatchlistID); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		activeWatchlist := watchlistMgr.GetActiveWatchlist()
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"active_watchlist_id": request.WatchlistID,
				"active_watchlist":    activeWatchlist,
			},
			"message": fmt.Sprintf("Active watchlist set to '%s'", request.WatchlistID),
		})
	})
	
	api.GET("/watchlists/:watchlist_id", func(c *gin.Context) {
		watchlistID := c.Param("watchlist_id")
		watchlist := watchlistMgr.GetWatchlist(watchlistID)
		
		if watchlist == nil {
			c.JSON(404, gin.H{
				"success": false,
				"message": fmt.Sprintf("Watchlist '%s' not found", watchlistID),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    watchlist,
			"message": fmt.Sprintf("Retrieved watchlist '%s'", watchlistID),
		})
	})
	
	api.POST("/watchlists", func(c *gin.Context) {
		var request models.CreateWatchlistRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request: " + err.Error(),
			})
			return
		}
		
		if err := request.Validate(); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		watchlistID, err := watchlistMgr.CreateWatchlist(request.Name, request.Symbols)
		if err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		watchlist := watchlistMgr.GetWatchlist(watchlistID)
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"watchlist_id": watchlistID,
				"watchlist":    watchlist,
			},
			"message": fmt.Sprintf("Watchlist '%s' created successfully", request.Name),
		})
	})
	
	api.PUT("/watchlists/:watchlist_id", func(c *gin.Context) {
		watchlistID := c.Param("watchlist_id")
		
		var request models.UpdateWatchlistRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request: " + err.Error(),
			})
			return
		}
		
		if err := request.Validate(); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		updated, err := watchlistMgr.UpdateWatchlist(watchlistID, request.Name, request.Symbols)
		if err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		if !updated {
			c.JSON(200, gin.H{
				"success": true,
				"data":    map[string]interface{}{"watchlist_id": watchlistID},
				"message": "No changes made to watchlist",
			})
			return
		}
		
		watchlist := watchlistMgr.GetWatchlist(watchlistID)
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"watchlist_id": watchlistID,
				"watchlist":    watchlist,
			},
			"message": fmt.Sprintf("Watchlist '%s' updated successfully", watchlistID),
		})
	})
	
	api.DELETE("/watchlists/:watchlist_id", func(c *gin.Context) {
		watchlistID := c.Param("watchlist_id")
		
		if err := watchlistMgr.DeleteWatchlist(watchlistID); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		c.JSON(200, gin.H{
			"success": true,
			"data":    map[string]interface{}{"watchlist_id": watchlistID, "deleted": true},
			"message": fmt.Sprintf("Watchlist '%s' deleted successfully", watchlistID),
		})
	})
	
	api.POST("/watchlists/:watchlist_id/symbols", func(c *gin.Context) {
		watchlistID := c.Param("watchlist_id")
		
		var request models.AddSymbolRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request: " + err.Error(),
			})
			return
		}
		
		if err := request.Validate(); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		// Basic symbol validation
		if !watchlistMgr.ValidateSymbol(request.Symbol) {
			c.JSON(400, gin.H{
				"success": false,
				"message": fmt.Sprintf("Invalid symbol: %s", request.Symbol),
			})
			return
		}
		
		if err := watchlistMgr.AddSymbol(watchlistID, request.Symbol); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		watchlist := watchlistMgr.GetWatchlist(watchlistID)
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"watchlist_id": watchlistID,
				"symbol":       request.Symbol,
				"watchlist":    watchlist,
			},
			"message": fmt.Sprintf("Symbol '%s' added to watchlist", request.Symbol),
		})
	})
	
	api.DELETE("/watchlists/:watchlist_id/symbols/:symbol", func(c *gin.Context) {
		watchlistID := c.Param("watchlist_id")
		symbol := c.Param("symbol")
		
		if err := watchlistMgr.RemoveSymbol(watchlistID, symbol); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		watchlist := watchlistMgr.GetWatchlist(watchlistID)
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"watchlist_id": watchlistID,
				"symbol":       symbol,
				"watchlist":    watchlist,
			},
			"message": fmt.Sprintf("Symbol '%s' removed from watchlist", symbol),
		})
	})
	
	api.POST("/watchlists/search", func(c *gin.Context) {
		var request models.SearchWatchlistsRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "Invalid request: " + err.Error(),
			})
			return
		}
		
		if err := request.Validate(); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		
		results := watchlistMgr.SearchWatchlists(request.Query)
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"query":         request.Query,
				"results":       results,
				"total_results": len(results),
			},
			"message": fmt.Sprintf("Found %d watchlists matching '%s'", len(results), request.Query),
		})
	})
	
	api.GET("/watchlists/symbols/all", func(c *gin.Context) {
		symbols := watchlistMgr.GetAllSymbols()
		
		c.JSON(200, gin.H{
			"success": true,
			"data": map[string]interface{}{
				"symbols":       symbols,
				"total_symbols": len(symbols),
			},
			"message": fmt.Sprintf("Retrieved %d unique symbols from all watchlists", len(symbols)),
		})
	})
	
	// WebSocket endpoint - exact same path as Python

	
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


