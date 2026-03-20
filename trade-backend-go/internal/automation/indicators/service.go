package indicators

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/providers"
)

// cacheKey generates a unique key for caching indicator results
// Uses indicator ID to support multiple instances of the same indicator type
func cacheKey(configID string, indicatorID string) string {
	return fmt.Sprintf("%s:%s", configID, indicatorID)
}

// cachedResult stores a successful indicator result for fallback
type cachedResult struct {
	Value     float64
	Timestamp time.Time
}

// cachedQuote stores a short-lived quote to prevent concurrent fetch race conditions
type cachedQuote struct {
	Price     float64
	Timestamp time.Time
}

// quoteCacheTTL is how long to cache real-time quotes (prevents race condition when
// multiple automations request the same symbol's quote simultaneously)
const quoteCacheTTL = 5 * time.Second

// Service handles indicator calculations using the provider abstraction layer
type Service struct {
	providerManager *providers.ProviderManager
	fomcDates       []time.Time // Pre-loaded FOMC dates

	// Cache for last known good indicator values (for stale fallback)
	cacheMu sync.RWMutex
	cache   map[string]*cachedResult

	// Short-term cache for real-time quotes (prevents concurrent fetch race)
	quoteCacheMu sync.RWMutex
	quoteCache   map[string]*cachedQuote

	// Per-symbol fetch locks to prevent concurrent fetches for the same symbol
	// This eliminates the check-then-act race condition
	quoteFetchMu   sync.Mutex
	quoteFetchLock map[string]*sync.Mutex
}

// NewService creates a new indicator service
func NewService(pm *providers.ProviderManager) *Service {
	s := &Service{
		providerManager: pm,
		fomcDates:       loadFOMCDates(),
		cache:           make(map[string]*cachedResult),
		quoteCache:      make(map[string]*cachedQuote),
		quoteFetchLock:  make(map[string]*sync.Mutex),
	}
	return s
}

// getQuoteFetchLock returns a per-symbol mutex for serializing quote fetches
func (s *Service) getQuoteFetchLock(symbol string) *sync.Mutex {
	s.quoteFetchMu.Lock()
	defer s.quoteFetchMu.Unlock()

	if lock, exists := s.quoteFetchLock[symbol]; exists {
		return lock
	}
	lock := &sync.Mutex{}
	s.quoteFetchLock[symbol] = lock
	return lock
}

// getCachedQuote retrieves a short-lived cached quote if still valid
func (s *Service) getCachedQuote(symbol string) (float64, bool) {
	s.quoteCacheMu.RLock()
	defer s.quoteCacheMu.RUnlock()

	cached, exists := s.quoteCache[symbol]
	if !exists {
		return 0, false
	}

	// Check if cache is still valid
	if time.Since(cached.Timestamp) > quoteCacheTTL {
		return 0, false
	}

	return cached.Price, true
}

// setCachedQuote stores a short-lived quote
func (s *Service) setCachedQuote(symbol string, price float64) {
	s.quoteCacheMu.Lock()
	defer s.quoteCacheMu.Unlock()

	s.quoteCache[symbol] = &cachedQuote{
		Price:     price,
		Timestamp: time.Now(),
	}
}

// getCachedResult retrieves a cached indicator result
func (s *Service) getCachedResult(configID string, indicatorID string) *cachedResult {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return s.cache[cacheKey(configID, indicatorID)]
}

// setCachedResult stores a successful indicator result
func (s *Service) setCachedResult(configID string, indicatorID string, value float64) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[cacheKey(configID, indicatorID)] = &cachedResult{
		Value:     value,
		Timestamp: time.Now(),
	}
}

// ClearCache removes all cached results (useful for testing)
func (s *Service) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache = make(map[string]*cachedResult)
}

// getParamOrDefault extracts a parameter value from the config's Params map.
// If the key is missing or Params is nil, returns the provided default value.
func getParamOrDefault(config types.IndicatorConfig, key string, defaultVal float64) float64 {
	if config.Params != nil {
		if val, ok := config.Params[key]; ok {
			return val
		}
	}
	return defaultVal
}

// getIntParam is a convenience wrapper that returns the param as an int.
func getIntParam(config types.IndicatorConfig, key string, defaultVal int) int {
	return int(getParamOrDefault(config, key, float64(defaultVal)))
}

// getQuoteWithCache fetches a quote with race-condition-safe caching.
// Uses a per-symbol lock to ensure only one goroutine fetches while others wait,
// eliminating the check-then-act race condition.
func (s *Service) getQuoteWithCache(ctx context.Context, symbol string) (float64, error) {
	// Fast path: check cache without lock
	if cachedPrice, ok := s.getCachedQuote(symbol); ok {
		slog.Debug("Using cached quote (fast path)",
			"symbol", symbol,
			"price", cachedPrice)
		return cachedPrice, nil
	}

	// Slow path: acquire per-symbol lock to prevent concurrent fetches
	lock := s.getQuoteFetchLock(symbol)
	lock.Lock()
	defer lock.Unlock()

	// Double-check cache after acquiring lock (another goroutine may have populated it)
	if cachedPrice, ok := s.getCachedQuote(symbol); ok {
		slog.Debug("Using cached quote (after lock)",
			"symbol", symbol,
			"price", cachedPrice)
		return cachedPrice, nil
	}

	// Cache miss - fetch from provider
	slog.Debug("Fetching quote from provider",
		"symbol", symbol)

	quotes, err := s.providerManager.GetStockQuotes(ctx, []string{symbol})
	if err != nil {
		return 0, fmt.Errorf("failed to get current price for %s: %w", symbol, err)
	}

	quote, ok := quotes[symbol]
	if !ok {
		return 0, fmt.Errorf("quote not available for %s", symbol)
	}

	// Extract price: try Last first, fall back to bid/ask midpoint
	var price float64
	if quote.Last != nil && *quote.Last > 0 {
		price = *quote.Last
	} else if quote.Bid != nil && quote.Ask != nil && *quote.Bid > 0 && *quote.Ask > 0 {
		price = (*quote.Bid + *quote.Ask) / 2
	} else if quote.Bid != nil && *quote.Bid > 0 {
		price = *quote.Bid
	} else if quote.Ask != nil && *quote.Ask > 0 {
		price = *quote.Ask
	} else {
		return 0, fmt.Errorf("current price not available for %s (no last, bid, or ask)", symbol)
	}

	// Cache the quote for subsequent requests
	s.setCachedQuote(symbol, price)
	slog.Info("Fetched and cached quote",
		"symbol", symbol,
		"price", price)

	return price, nil
}

// EvaluateIndicator calculates a single indicator
// Each indicator uses its own configured symbol (or defaults like QQQ for Gap/Range/Trend, VIX for vix)
// configID is used for caching - pass empty string for preview/test mode (no caching)
func (s *Service) EvaluateIndicator(ctx context.Context, configID string, config types.IndicatorConfig) *types.IndicatorResult {
	result := &types.IndicatorResult{
		Type:      config.Type,
		Symbol:    config.Symbol, // Will be overwritten below with actual symbol used
		Threshold: config.Threshold,
		Operator:  config.Operator,
		Enabled:   config.Enabled,
		Timestamp: time.Now(),
	}

	// If indicator is disabled, mark as passing
	if !config.Enabled {
		result.Pass = true
		result.Details = "Indicator disabled"
		return result
	}

	var err error

	switch config.Type {
	case types.IndicatorVIX:
		// Use custom symbol if provided (e.g., UVXY, VXX), otherwise try VIX variants
		vixSymbol := config.Symbol
		if vixSymbol == "" {
			vixSymbol = "VIX"
		}
		result.Value, err = s.GetVIXValue(ctx, vixSymbol)
		result.Symbol = vixSymbol
	case types.IndicatorGap:
		// Use indicator's symbol if configured, otherwise default to QQQ for market reference
		gapSymbol := config.Symbol
		if gapSymbol == "" {
			gapSymbol = "QQQ"
		}
		result.Value, err = s.GetGapPercent(ctx, gapSymbol)
		result.Symbol = gapSymbol
	case types.IndicatorRange:
		// Use indicator's symbol if configured, otherwise default to QQQ for market reference
		rangeSymbol := config.Symbol
		if rangeSymbol == "" {
			rangeSymbol = "QQQ"
		}
		result.Value, err = s.GetRangePercent(ctx, rangeSymbol)
		result.Symbol = rangeSymbol
	case types.IndicatorTrend:
		// Use indicator's symbol if configured, otherwise default to QQQ for market reference
		trendSymbol := config.Symbol
		if trendSymbol == "" {
			trendSymbol = "QQQ"
		}
		result.Value, err = s.GetTrendPercent(ctx, trendSymbol)
		result.Symbol = trendSymbol
	case types.IndicatorCalendar:
		result.Value, err = s.GetFOMCValue(ctx)
		result.Symbol = "FOMC"

	// ---- New: Momentum indicators ----
	case types.IndicatorRSI:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 14)
		barsNeeded := period * 3
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcRSI(closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	case types.IndicatorMACD:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		fastPeriod := getIntParam(config, "fast_period", 12)
		slowPeriod := getIntParam(config, "slow_period", 26)
		signalPeriod := getIntParam(config, "signal_period", 9)
		barsNeeded := slowPeriod + signalPeriod + slowPeriod
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcMACD(closes, fastPeriod, slowPeriod, signalPeriod)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d/%d/%d", fastPeriod, slowPeriod, signalPeriod)

	case types.IndicatorMomentum:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 10)
		barsNeeded := period + 1
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcMomentum(closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	case types.IndicatorCMO:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 14)
		barsNeeded := period + 1
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcCMO(closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	case types.IndicatorStoch:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		kPeriod := getIntParam(config, "k_period", 14)
		dPeriod := getIntParam(config, "d_period", 3)
		barsNeeded := kPeriod + dPeriod
		_, highs, lows, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcStochastic(highs, lows, closes, kPeriod, dPeriod)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d/%d", kPeriod, dPeriod)

	case types.IndicatorStochRSI:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		rsiPeriod := getIntParam(config, "rsi_period", 14)
		stochPeriod := getIntParam(config, "stoch_period", 14)
		kPeriod := getIntParam(config, "k_period", 3)
		dPeriod := getIntParam(config, "d_period", 3)
		barsNeeded := rsiPeriod*3 + stochPeriod
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcStochRSI(closes, rsiPeriod, stochPeriod, kPeriod, dPeriod)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d/%d/%d/%d", rsiPeriod, stochPeriod, kPeriod, dPeriod)

	// ---- New: Trend indicators ----
	case types.IndicatorADX:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 14)
		barsNeeded := period * 3
		_, highs, lows, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcADX(highs, lows, closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	case types.IndicatorCCI:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 20)
		barsNeeded := period
		_, highs, lows, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcCCI(highs, lows, closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	case types.IndicatorSMA:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 20)
		barsNeeded := period
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcSMA(closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	case types.IndicatorEMA:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 20)
		barsNeeded := period * 2
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcEMA(closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	// ---- New: Volatility indicators ----
	case types.IndicatorATR:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 14)
		barsNeeded := period * 2
		_, highs, lows, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcATR(highs, lows, closes, period)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d", period)

	case types.IndicatorBBPercent:
		symbol := config.Symbol
		if symbol == "" {
			symbol = "QQQ"
		}
		period := getIntParam(config, "period", 20)
		stdDev := getParamOrDefault(config, "std_dev", 2.0)
		barsNeeded := period
		_, _, _, closes, actualSymbol, fetchErr := s.getHistoricalBarsForIndicator(ctx, symbol, barsNeeded)
		if fetchErr != nil {
			err = fetchErr
		} else {
			result.Value, err = CalcBollingerPercentB(closes, period, stdDev)
		}
		result.Symbol = actualSymbol
		result.ParamSummary = fmt.Sprintf("%d/%.1f", period, stdDev)

	default:
		err = fmt.Errorf("unknown indicator type: %s", config.Type)
	}

	if err != nil {
		result.Error = err.Error()
		result.Pass = false
		result.Stale = true

		// Try to use last known good value for display (only if we have a configID for caching)
		if configID != "" && config.ID != "" {
			if cached := s.getCachedResult(configID, config.ID); cached != nil {
				result.Value = cached.Value
				result.LastGoodValue = &cached.Value
				result.Details = fmt.Sprintf("STALE: Last good value from %s - %s",
					cached.Timestamp.Format("15:04:05"), err.Error())
				slog.Warn("Indicator evaluation failed, using cached value",
					"type", config.Type,
					"indicatorID", config.ID,
					"cachedValue", cached.Value,
					"cachedAt", cached.Timestamp,
					"error", err)
			} else {
				result.Details = fmt.Sprintf("STALE: No cached value available - %s", err.Error())
				slog.Error("Indicator evaluation failed, no cached value available",
					"type", config.Type,
					"indicatorID", config.ID,
					"error", err)
			}
		} else {
			// Preview/test mode - no caching
			slog.Error("Indicator evaluation failed (preview mode)", "type", config.Type, "error", err)
		}
		return result
	}

	// Success - cache this result (only if we have a configID and indicatorID)
	if configID != "" && config.ID != "" {
		s.setCachedResult(configID, config.ID, result.Value)
	}

	// Evaluate the condition
	result.Pass = result.Evaluate()
	result.Details = s.formatDetails(result)

	return result
}

// EvaluateAllIndicators evaluates all indicators
// Each indicator uses its own configured symbol (or defaults like QQQ for Gap/Range/Trend)
// configID is used for caching - pass empty string for preview/test mode (no caching)
func (s *Service) EvaluateAllIndicators(ctx context.Context, configID string, configs []types.IndicatorConfig) []types.IndicatorResult {
	results := make([]types.IndicatorResult, 0, len(configs))

	for _, config := range configs {
		result := s.EvaluateIndicator(ctx, configID, config)
		results = append(results, *result)
	}

	return results
}

// AllIndicatorsPass checks if all enabled indicators pass
// Returns false if any enabled indicator is stale OR failing
func (s *Service) AllIndicatorsPass(results []types.IndicatorResult) bool {
	for _, result := range results {
		if result.Enabled && (result.Stale || !result.Pass) {
			return false
		}
	}
	return true
}

// EvaluateIndicatorGroups evaluates all indicator groups and returns per-group results.
// All groups are always evaluated (no short-circuit) so the dashboard shows the full picture.
// Returns: (groupResults, flatResults, anyGroupPasses)
func (s *Service) EvaluateIndicatorGroups(
	ctx context.Context,
	configID string,
	groups []types.IndicatorGroup,
) ([]types.GroupResult, []types.IndicatorResult, bool) {
	groupResults := make([]types.GroupResult, 0, len(groups))
	allFlatResults := make([]types.IndicatorResult, 0)
	anyGroupPasses := false

	for _, group := range groups {
		indicatorResults := s.EvaluateAllIndicators(ctx, configID, group.Indicators)
		groupPass := s.AllIndicatorsPass(indicatorResults)

		groupResults = append(groupResults, types.GroupResult{
			GroupID:          group.ID,
			GroupName:        group.Name,
			Pass:             groupPass,
			IndicatorResults: indicatorResults,
		})

		allFlatResults = append(allFlatResults, indicatorResults...)

		if groupPass {
			anyGroupPasses = true
		}

		slog.Info("📊 Group evaluation complete",
			"group", group.Name,
			"groupID", group.ID,
			"pass", groupPass,
			"indicators", len(group.Indicators),
			"configID", configID)
	}

	slog.Info("📊 All groups evaluated",
		"totalGroups", len(groups),
		"anyGroupPasses", anyGroupPasses,
		"totalIndicators", len(allFlatResults),
		"configID", configID)

	return groupResults, allFlatResults, anyGroupPasses
}

// GetVIXValue fetches the current VIX value
// If customSymbol is provided (e.g., "UVXY", "VXX"), use streaming quotes
// For VIX index, use historical bars API (streaming quotes don't work for indices)
func (s *Service) GetVIXValue(ctx context.Context, customSymbol string) (float64, error) {
	// Determine the symbol to use
	symbol := "VIX"
	if customSymbol != "" {
		symbol = customSymbol
	}

	// For VIX index symbol, use historical bars (streaming quotes don't work for indices)
	if symbol == "VIX" || symbol == "$VIX.X" || symbol == "^VIX" {
		// Get 1 day of historical data to get the latest close
		bars, err := s.providerManager.GetHistoricalBars(ctx, "VIX", "D", nil, nil, 1)
		if err != nil {
			return 0, fmt.Errorf("failed to get VIX historical data: %w", err)
		}
		if len(bars) == 0 {
			return 0, fmt.Errorf("no VIX historical data available")
		}
		// Use the close price from the most recent bar
		closePrice := getFloatFromBar(bars[0], "close")
		if closePrice > 0 {
			return closePrice, nil
		}
		return 0, fmt.Errorf("VIX close price not available in historical data")
	}

	// For other symbols (UVXY, VXX, etc.), use race-condition-safe caching
	return s.getQuoteWithCache(ctx, symbol)
}

// GetGapPercent calculates the gap percentage: (Open - PrevClose) / PrevClose * 100
func (s *Service) GetGapPercent(ctx context.Context, symbol string) (float64, error) {
	dailyData, err := s.GetDailyData(ctx, symbol)
	if err != nil {
		return 0, err
	}

	if dailyData.PreviousClose == 0 {
		return 0, fmt.Errorf("previous close is zero for %s", symbol)
	}

	gap := ((dailyData.Open - dailyData.PreviousClose) / dailyData.PreviousClose) * 100
	return gap, nil
}

// GetRangePercent calculates the range percentage: (High - Low) / Open * 100
func (s *Service) GetRangePercent(ctx context.Context, symbol string) (float64, error) {
	dailyData, err := s.GetDailyData(ctx, symbol)
	if err != nil {
		return 0, err
	}

	if dailyData.Open == 0 {
		return 0, fmt.Errorf("open is zero for %s", symbol)
	}

	rangeVal := ((dailyData.High - dailyData.Low) / dailyData.Open) * 100
	return rangeVal, nil
}

// GetTrendPercent calculates the trend percentage: (Current - Open) / Open * 100
func (s *Service) GetTrendPercent(ctx context.Context, symbol string) (float64, error) {
	// Get daily data for open price
	dailyData, err := s.GetDailyData(ctx, symbol)
	if err != nil {
		return 0, err
	}

	if dailyData.Open == 0 {
		return 0, fmt.Errorf("open is zero for %s", symbol)
	}

	// Get current price with race-condition-safe caching
	currentPrice, err := s.getQuoteWithCache(ctx, symbol)
	if err != nil {
		return 0, err
	}

	trend := ((currentPrice - dailyData.Open) / dailyData.Open) * 100
	return trend, nil
}

// GetFOMCValue returns 1 if today is an FOMC day, 0 otherwise
func (s *Service) GetFOMCValue(ctx context.Context) (float64, error) {
	now := time.Now().In(s.getNewYorkLocation())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, fomcDate := range s.fomcDates {
		if today.Equal(fomcDate) {
			return 1, nil // FOMC day
		}
	}
	return 0, nil // Not FOMC day
}

// getHistoricalBarsForIndicator fetches daily bars and returns OHLC arrays.
// symbol defaults to "QQQ" if empty.
func (s *Service) getHistoricalBarsForIndicator(ctx context.Context, symbol string, barsNeeded int) (
	opens, highs, lows, closes []float64, actualSymbol string, err error) {

	if symbol == "" {
		symbol = "QQQ"
	}
	if barsNeeded < 50 {
		barsNeeded = 50
	}

	bars, err := s.providerManager.GetHistoricalBars(ctx, symbol, "D", nil, nil, barsNeeded)
	if err != nil {
		return nil, nil, nil, nil, symbol, fmt.Errorf("failed to get historical bars for %s: %w", symbol, err)
	}

	opens, highs, lows, closes = extractOHLC(bars)
	return opens, highs, lows, closes, symbol, nil
}

// GetDailyData fetches daily OHLC data for a symbol
func (s *Service) GetDailyData(ctx context.Context, symbol string) (*types.DailyData, error) {
	// Request 2 days of daily data to get previous close
	// Pass nil for startDate and endDate to use defaults
	// Use "D" timeframe which is the standard format for daily bars
	bars, err := s.providerManager.GetHistoricalBars(ctx, symbol, "D", nil, nil, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical bars for %s: %w", symbol, err)
	}

	if len(bars) < 2 {
		return nil, fmt.Errorf("insufficient historical data for %s (got %d bars, need 2)", symbol, len(bars))
	}

	// Bars are returned in ascending order (oldest first)
	// So bars[len-1] is most recent (today), bars[len-2] is previous day
	todayBar := bars[len(bars)-1]
	prevBar := bars[len(bars)-2]

	dailyData := &types.DailyData{
		Symbol:        symbol,
		Date:          getStringFromBar(todayBar, "date"),
		Open:          getFloatFromBar(todayBar, "open"),
		High:          getFloatFromBar(todayBar, "high"),
		Low:           getFloatFromBar(todayBar, "low"),
		Close:         getFloatFromBar(todayBar, "close"),
		PreviousClose: getFloatFromBar(prevBar, "close"),
		Volume:        getIntFromBar(todayBar, "volume"),
		Timestamp:     time.Now(),
	}

	return dailyData, nil
}

// GetFOMCDates returns the list of FOMC meeting dates
func (s *Service) GetFOMCDates() []time.Time {
	return s.fomcDates
}

// formatDetails creates a human-readable description of the indicator result
func (s *Service) formatDetails(result *types.IndicatorResult) string {
	operatorStr := ""
	switch result.Operator {
	case types.OperatorGreaterThan:
		operatorStr = ">"
	case types.OperatorLessThan:
		operatorStr = "<"
	case types.OperatorEqual:
		operatorStr = "="
	case types.OperatorNotEqual:
		operatorStr = "!="
	}

	passStr := "PASS"
	if !result.Pass {
		passStr = "FAIL"
	}

	switch result.Type {
	case types.IndicatorVIX:
		return fmt.Sprintf("VIX %.2f %s %.2f (%s)", result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorGap:
		return fmt.Sprintf("Gap %.2f%% %s %.2f%% (%s)", result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorRange:
		return fmt.Sprintf("Range %.2f%% %s %.2f%% (%s)", result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorTrend:
		return fmt.Sprintf("Trend %.2f%% %s %.2f%% (%s)", result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorCalendar:
		if result.Value == 1 {
			return fmt.Sprintf("FOMC Day: Yes (%s)", passStr)
		}
		return fmt.Sprintf("FOMC Day: No (%s)", passStr)

	// ---- New: Momentum indicators ----
	case types.IndicatorRSI:
		return fmt.Sprintf("RSI(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorMACD:
		return fmt.Sprintf("MACD(%s) %.4f %s %.4f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorMomentum:
		return fmt.Sprintf("Momentum(%s) %.2f%% %s %.2f%% (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorCMO:
		return fmt.Sprintf("CMO(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorStoch:
		return fmt.Sprintf("Stoch(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorStochRSI:
		return fmt.Sprintf("StochRSI(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)

	// ---- New: Trend indicators ----
	case types.IndicatorADX:
		return fmt.Sprintf("ADX(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorCCI:
		return fmt.Sprintf("CCI(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorSMA:
		return fmt.Sprintf("SMA(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorEMA:
		return fmt.Sprintf("EMA(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)

	// ---- New: Volatility indicators ----
	case types.IndicatorATR:
		return fmt.Sprintf("ATR(%s) %.2f %s %.2f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
	case types.IndicatorBBPercent:
		return fmt.Sprintf("BB%%B(%s) %.4f %s %.4f (%s)", result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)

	default:
		return fmt.Sprintf("%.2f %s %.2f (%s)", result.Value, operatorStr, result.Threshold, passStr)
	}
}

func (s *Service) getNewYorkLocation() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}

// Helper functions to extract values from bar data
func getFloatFromBar(bar map[string]interface{}, key string) float64 {
	if val, ok := bar[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0
}

func getIntFromBar(bar map[string]interface{}, key string) int64 {
	if val, ok := bar[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

func getStringFromBar(bar map[string]interface{}, key string) string {
	if val, ok := bar[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// loadFOMCDates returns pre-configured FOMC meeting dates
// These are the announcement dates (typically Wednesdays)
func loadFOMCDates() []time.Time {
	ny, _ := time.LoadLocation("America/New_York")

	dates := []time.Time{
		// 2025 FOMC Dates
		time.Date(2025, 1, 29, 0, 0, 0, 0, ny),
		time.Date(2025, 3, 19, 0, 0, 0, 0, ny),
		time.Date(2025, 5, 7, 0, 0, 0, 0, ny),
		time.Date(2025, 6, 18, 0, 0, 0, 0, ny),
		time.Date(2025, 7, 30, 0, 0, 0, 0, ny),
		time.Date(2025, 9, 17, 0, 0, 0, 0, ny),
		time.Date(2025, 11, 5, 0, 0, 0, 0, ny),
		time.Date(2025, 12, 17, 0, 0, 0, 0, ny),

		// 2026 FOMC Dates (from PineScript)
		time.Date(2026, 1, 28, 0, 0, 0, 0, ny),
		time.Date(2026, 3, 18, 0, 0, 0, 0, ny),
		time.Date(2026, 4, 29, 0, 0, 0, 0, ny),
		time.Date(2026, 6, 17, 0, 0, 0, 0, ny),
		time.Date(2026, 7, 29, 0, 0, 0, 0, ny),
		time.Date(2026, 9, 16, 0, 0, 0, 0, ny),
		time.Date(2026, 10, 28, 0, 0, 0, 0, ny),
		time.Date(2026, 12, 9, 0, 0, 0, 0, ny),
	}

	return dates
}
