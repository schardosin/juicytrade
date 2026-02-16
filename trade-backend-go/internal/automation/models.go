package automation

// Re-export all types from the types package to maintain backward compatibility
// and allow storage.go and other files in this package to use them directly

import (
	"time"

	"trade-backend-go/internal/automation/types"
)

// Re-export type aliases
type (
	IndicatorType      = types.IndicatorType
	Operator           = types.Operator
	AutomationStatus   = types.AutomationStatus
	TradeStrategy      = types.TradeStrategy
	IndicatorConfig    = types.IndicatorConfig
	IndicatorResult    = types.IndicatorResult
	TradeConfiguration = types.TradeConfiguration
	AutomationConfig   = types.AutomationConfig
	PlacedOrder        = types.PlacedOrder
	AutomationLog      = types.AutomationLog
	ActiveAutomation   = types.ActiveAutomation
	StrikeSelection    = types.StrikeSelection
	DailyData          = types.DailyData
)

// Re-export constants
const (
	IndicatorVIX      = types.IndicatorVIX
	IndicatorGap      = types.IndicatorGap
	IndicatorRange    = types.IndicatorRange
	IndicatorTrend    = types.IndicatorTrend
	IndicatorCalendar = types.IndicatorCalendar

	OperatorGreaterThan = types.OperatorGreaterThan
	OperatorLessThan    = types.OperatorLessThan
	OperatorEqual       = types.OperatorEqual
	OperatorNotEqual    = types.OperatorNotEqual

	StatusIdle       = types.StatusIdle
	StatusWaiting    = types.StatusWaiting
	StatusEvaluating = types.StatusEvaluating
	StatusTrading    = types.StatusTrading
	StatusMonitoring = types.StatusMonitoring
	StatusCompleted  = types.StatusCompleted
	StatusFailed     = types.StatusFailed
	StatusCancelled  = types.StatusCancelled
	StatusError      = types.StatusError

	StrategyPutSpread  = types.StrategyPutSpread
	StrategyCallSpread = types.StrategyCallSpread
	StrategyIronCondor = types.StrategyIronCondor
)

// Re-export constructor functions
var (
	NewIndicatorConfig    = types.NewIndicatorConfig
	NewTradeConfiguration = types.NewTradeConfiguration
	NewAutomationConfig   = types.NewAutomationConfig
	GenerateIndicatorID   = types.GenerateIndicatorID
)

// generateID creates a unique ID for new automation configs
// This is a local helper that wraps the unexported function in types
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
