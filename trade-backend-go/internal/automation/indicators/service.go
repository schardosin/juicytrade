package indicators

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"trade-backend-go/internal/automation/types"
	"trade-backend-go/internal/providers"
)

// Service handles indicator calculations using the provider abstraction layer
type Service struct {
	providerManager *providers.ProviderManager
	fomcDates       []time.Time // Pre-loaded FOMC dates
}

// NewService creates a new indicator service
func NewService(pm *providers.ProviderManager) *Service {
	s := &Service{
		providerManager: pm,
		fomcDates:       loadFOMCDates(),
	}
	return s
}

// EvaluateIndicator calculates a single indicator
// Each indicator uses its own configured symbol (or defaults like QQQ for Gap/Range/Trend, VIX for vix)
func (s *Service) EvaluateIndicator(ctx context.Context, config types.IndicatorConfig) *types.IndicatorResult {
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
	default:
		err = fmt.Errorf("unknown indicator type: %s", config.Type)
	}

	if err != nil {
		result.Error = err.Error()
		result.Pass = false
		slog.Error("Indicator evaluation failed", "type", config.Type, "error", err)
		return result
	}

	// Evaluate the condition
	result.Pass = result.Evaluate()
	result.Details = s.formatDetails(result)

	return result
}

// EvaluateAllIndicators evaluates all indicators
// Each indicator uses its own configured symbol (or defaults like QQQ for Gap/Range/Trend)
func (s *Service) EvaluateAllIndicators(ctx context.Context, configs []types.IndicatorConfig) []types.IndicatorResult {
	results := make([]types.IndicatorResult, 0, len(configs))

	for _, config := range configs {
		result := s.EvaluateIndicator(ctx, config)
		results = append(results, *result)
	}

	return results
}

// AllIndicatorsPass checks if all enabled indicators pass
func (s *Service) AllIndicatorsPass(results []types.IndicatorResult) bool {
	for _, result := range results {
		if result.Enabled && !result.Pass {
			return false
		}
	}
	return true
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

	// For other symbols (UVXY, VXX, etc.), use streaming quotes
	quotes, err := s.providerManager.GetStockQuotes(ctx, []string{symbol})
	if err != nil {
		return 0, fmt.Errorf("failed to get quote for %s: %w", symbol, err)
	}

	if quote, ok := quotes[symbol]; ok {
		// Try Last price first
		if quote.Last != nil && *quote.Last > 0 {
			return *quote.Last, nil
		}
		// Fall back to bid/ask midpoint
		if quote.Bid != nil && quote.Ask != nil && *quote.Bid > 0 && *quote.Ask > 0 {
			return (*quote.Bid + *quote.Ask) / 2, nil
		}
		// Fall back to just bid or ask
		if quote.Bid != nil && *quote.Bid > 0 {
			return *quote.Bid, nil
		}
		if quote.Ask != nil && *quote.Ask > 0 {
			return *quote.Ask, nil
		}
	}

	return 0, fmt.Errorf("quote not available for %s", symbol)
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

	// Get current price
	quotes, err := s.providerManager.GetStockQuotes(ctx, []string{symbol})
	if err != nil {
		return 0, fmt.Errorf("failed to get current price for %s: %w", symbol, err)
	}

	quote, ok := quotes[symbol]
	if !ok {
		return 0, fmt.Errorf("quote not available for %s", symbol)
	}

	// Try Last price first, fall back to bid/ask midpoint
	var currentPrice float64
	if quote.Last != nil && *quote.Last > 0 {
		currentPrice = *quote.Last
	} else if quote.Bid != nil && quote.Ask != nil && *quote.Bid > 0 && *quote.Ask > 0 {
		currentPrice = (*quote.Bid + *quote.Ask) / 2
	} else if quote.Bid != nil && *quote.Bid > 0 {
		currentPrice = *quote.Bid
	} else if quote.Ask != nil && *quote.Ask > 0 {
		currentPrice = *quote.Ask
	} else {
		return 0, fmt.Errorf("current price not available for %s (no last, bid, or ask)", symbol)
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
