package handlers

import (
	"context"
	"net/http"
	"strings"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/utils"

	"github.com/gin-gonic/gin"
)

// MarketDataHandler handles market data endpoints.
// Exact conversion of Python market data endpoints.
type MarketDataHandler struct {
	providerManager ProviderManagerInterface
	expirationCache *utils.ExpirationCache
}

// ProviderManagerInterface defines the interface for provider manager
type ProviderManagerInterface interface {
	GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error)
	GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error)
}

// NewMarketDataHandler creates a new market data handler.
func NewMarketDataHandler(providerManager ProviderManagerInterface) *MarketDataHandler {
	return &MarketDataHandler{
		providerManager: providerManager,
		expirationCache: utils.NewExpirationCache(),
	}
}

// GetStockPrices handles the /prices/stocks endpoint.
// Exact conversion of Python get_stock_prices endpoint.
func (h *MarketDataHandler) GetStockPrices(c *gin.Context) {
	// Get symbols parameter (same as Python)
	symbolsParam := c.Query("symbols")
	if symbolsParam == "" {
		response := models.NewApiResponse(
			false,
			nil,
			stringPtr("Missing required parameter: symbols"),
			stringPtr("Please provide symbols parameter"),
		)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Parse symbols (same as Python)
	symbols := strings.Split(symbolsParam, ",")
	for i, symbol := range symbols {
		symbols[i] = strings.TrimSpace(strings.ToUpper(symbol))
	}

	// Get quotes from provider
	quotes, err := h.providerManager.GetStockQuotes(c.Request.Context(), symbols)
	if err != nil {
		response := models.NewApiResponse(
			false,
			nil,
			stringPtr("Failed to fetch stock quotes"),
			stringPtr(err.Error()),
		)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Convert to response format (same structure as Python)
	result := make(map[string]interface{})
	for symbol, quote := range quotes {
		result[symbol] = map[string]interface{}{
			"symbol":    quote.Symbol,
			"ask":       quote.Ask,
			"bid":       quote.Bid,
			"last":      quote.Last,
			"timestamp": quote.Timestamp,
		}
	}

	response := models.NewApiResponse(
		true,
		result,
		nil,
		stringPtr("Stock quotes retrieved successfully"),
	)

	c.JSON(http.StatusOK, response)
}

// GetExpirationDates handles the /expiration_dates endpoint.
// Exact conversion of Python get_expiration_dates endpoint.
// Uses an in-memory cache with same-day invalidation to avoid redundant provider calls.
func (h *MarketDataHandler) GetExpirationDates(c *gin.Context) {
	// Get symbol parameter (same as Python)
	symbol := c.Query("symbol")
	if symbol == "" {
		response := models.NewApiResponse(
			false,
			nil,
			stringPtr("Missing required parameter: symbol"),
			stringPtr("Please provide symbol parameter"),
		)
		c.JSON(http.StatusBadRequest, response)
		return
	}

	// Normalize symbol (same as Python)
	symbol = strings.TrimSpace(strings.ToUpper(symbol))

	// Check cache first - expiration dates are stable within a trading day
	if cachedDates, found := h.expirationCache.GetExpirationDates(symbol); found {
		result := map[string]interface{}{
			"expiration_dates": cachedDates,
		}
		response := models.NewApiResponse(
			true,
			result,
			nil,
			stringPtr("Expiration dates retrieved from cache"),
		)
		c.JSON(http.StatusOK, response)
		return
	}

	// Cache miss - fetch from provider
	dates, err := h.providerManager.GetExpirationDates(c.Request.Context(), symbol)
	if err != nil {
		response := models.NewApiResponse(
			false,
			nil,
			stringPtr("Failed to fetch expiration dates"),
			stringPtr(err.Error()),
		)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	// Store in cache for subsequent requests
	if dates != nil {
		h.expirationCache.SetExpirationDates(symbol, dates)
	}

	// Wrap the dates in the expected structure with expiration_dates node
	result := map[string]interface{}{
		"expiration_dates": dates,
	}

	response := models.NewApiResponse(
		true,
		result,
		nil,
		stringPtr("Expiration dates retrieved successfully"),
	)

	c.JSON(http.StatusOK, response)
}
