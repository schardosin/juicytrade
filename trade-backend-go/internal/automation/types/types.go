package types

import (
	"fmt"
	"time"

	"trade-backend-go/internal/models"
)

// IndicatorType defines the type of indicator
type IndicatorType string

const (
	IndicatorVIX      IndicatorType = "vix"
	IndicatorGap      IndicatorType = "gap"
	IndicatorRange    IndicatorType = "range"
	IndicatorTrend    IndicatorType = "trend"
	IndicatorCalendar IndicatorType = "calendar"
)

// Operator defines comparison operators for indicators
type Operator string

const (
	OperatorGreaterThan Operator = "gt"
	OperatorLessThan    Operator = "lt"
	OperatorEqual       Operator = "eq"
	OperatorNotEqual    Operator = "ne"
)

// AutomationStatus defines the state of an automation
type AutomationStatus string

const (
	StatusIdle       AutomationStatus = "idle"       // Not running, waiting to be started
	StatusWaiting    AutomationStatus = "waiting"    // Running, waiting for entry time
	StatusEvaluating AutomationStatus = "evaluating" // Checking indicator conditions
	StatusTrading    AutomationStatus = "trading"    // Placing/managing orders
	StatusMonitoring AutomationStatus = "monitoring" // Order placed, monitoring for fills
	StatusCompleted  AutomationStatus = "completed"  // Successfully filled
	StatusFailed     AutomationStatus = "failed"     // Failed after max attempts
	StatusCancelled  AutomationStatus = "cancelled"  // Manually cancelled
	StatusError      AutomationStatus = "error"      // Error occurred
)

// TradeStrategy defines the type of options strategy
type TradeStrategy string

const (
	StrategyPutSpread  TradeStrategy = "put_spread"
	StrategyCallSpread TradeStrategy = "call_spread"
	StrategyIronCondor TradeStrategy = "iron_condor"
)

// IndicatorConfig defines configuration for a single indicator
type IndicatorConfig struct {
	ID        string        `json:"id,omitempty"` // Unique ID to support multiple instances of same type
	Type      IndicatorType `json:"type"`
	Enabled   bool          `json:"enabled"`
	Operator  Operator      `json:"operator"`
	Threshold float64       `json:"threshold"`
	// Symbol is optional, used for symbol-specific indicators
	Symbol string `json:"symbol,omitempty"`
}

// IndicatorResult contains the result of an indicator evaluation
type IndicatorResult struct {
	Type          IndicatorType `json:"type"`
	Symbol        string        `json:"symbol"`
	Value         float64       `json:"value"`
	LastGoodValue *float64      `json:"last_good_value,omitempty"` // Previous successful value (shown when stale)
	Stale         bool          `json:"stale"`                     // True if data fetch failed, using cached value
	Threshold     float64       `json:"threshold"`
	Operator      Operator      `json:"operator"`
	Pass          bool          `json:"pass"`
	Enabled       bool          `json:"enabled"`
	Timestamp     time.Time     `json:"timestamp"`
	Details       string        `json:"details,omitempty"`
	Error         string        `json:"error,omitempty"`
}

// TradeConfiguration defines the trade parameters for an automation
type TradeConfiguration struct {
	Strategy         TradeStrategy `json:"strategy"`                    // "put_spread", "call_spread", "iron_condor"
	Width            int           `json:"width"`                       // Spread width (e.g., 20, 30)
	TargetDelta      float64       `json:"target_delta"`                // Target delta for short strike (e.g., 0.05)
	MaxCapital       float64       `json:"max_capital"`                 // Maximum capital to use
	OrderType        string        `json:"order_type"`                  // "limit" or "market"
	TimeInForce      string        `json:"time_in_force"`               // "day" or "gtc"
	PriceLadderStep  float64       `json:"price_ladder_step"`           // Price decrement step (e.g., 0.05)
	MaxAttempts      int           `json:"max_attempts"`                // Maximum order replacement attempts
	AttemptInterval  int           `json:"attempt_interval"`            // Seconds between price reductions
	DeltaDriftLimit  float64       `json:"delta_drift_limit"`           // Max delta drift before replacing (e.g., 0.01)
	StartingOffset   float64       `json:"starting_offset,omitempty"`   // Amount below mid to start (e.g., 0.10)
	MinCredit        float64       `json:"min_credit,omitempty"`        // Minimum acceptable credit (stop if below)
	ExpirationMode   string        `json:"expiration_mode,omitempty"`   // "0dte", "1dte", "2dte", "custom"
	CustomExpiration string        `json:"custom_expiration,omitempty"` // Custom expiration date (YYYY-MM-DD)
}

// RecurrenceMode defines how the automation repeats
type RecurrenceMode string

const (
	RecurrenceOnce  RecurrenceMode = "once"  // Run once, then stop (default)
	RecurrenceDaily RecurrenceMode = "daily" // Reset daily and run again each trading day
)

// AutomationConfig defines a complete automation configuration
type AutomationConfig struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	Symbol        string             `json:"symbol"`         // Underlying symbol (e.g., "NDX", "SPX")
	Indicators    []IndicatorConfig  `json:"indicators"`     // List of indicator configurations
	EntryTime     string             `json:"entry_time"`     // Entry time in HH:MM format (e.g., "12:25")
	EntryTimezone string             `json:"entry_timezone"` // Timezone (e.g., "America/New_York")
	Enabled       bool               `json:"enabled"`        // Whether this automation is active
	Recurrence    RecurrenceMode     `json:"recurrence"`     // "once" or "daily" (default: "once")
	TradeConfig   TradeConfiguration `json:"trade_config"`   // Trade parameters
	Created       time.Time          `json:"created"`
	Updated       time.Time          `json:"updated"`
}

// PlacedOrder tracks an order placed by the automation
type PlacedOrder struct {
	OrderID       string            `json:"order_id"`
	AutomationID  string            `json:"automation_id"` // Links order to automation
	ConfigName    string            `json:"config_name"`   // Human-readable automation name
	Legs          []models.OrderLeg `json:"legs"`
	LimitPrice    float64           `json:"limit_price"`
	TargetDelta   float64           `json:"target_delta"`
	ActualDelta   float64           `json:"actual_delta"`
	AttemptNumber int               `json:"attempt_number"`
	Status        string            `json:"status"`
	PlacedAt      time.Time         `json:"placed_at"`
	FilledAt      *time.Time        `json:"filled_at,omitempty"`
	CancelledAt   *time.Time        `json:"cancelled_at,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// AutomationPosition tracks a position created by an automation (after order fill)
type AutomationPosition struct {
	AutomationID string            `json:"automation_id"`
	ConfigName   string            `json:"config_name"`
	OrderID      string            `json:"order_id"`  // Original order that created this position
	Symbol       string            `json:"symbol"`    // Underlying symbol
	Strategy     TradeStrategy     `json:"strategy"`  // put_spread, call_spread, etc.
	Legs         []models.OrderLeg `json:"legs"`      // Position legs
	OpenedAt     time.Time         `json:"opened_at"` // When the order filled
	ClosedAt     *time.Time        `json:"closed_at,omitempty"`
	EntryCredit  float64           `json:"entry_credit"` // Credit received
	ExitDebit    *float64          `json:"exit_debit,omitempty"`
	PnL          *float64          `json:"pnl,omitempty"` // Realized P&L
	Status       string            `json:"status"`        // "open", "closed", "expired"
}

// AutomationLog represents a log entry for an automation
type AutomationLog struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // "info", "warn", "error"
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
}

// ActiveAutomation represents a running automation instance
type ActiveAutomation struct {
	Config            *AutomationConfig `json:"config"`
	Status            AutomationStatus  `json:"status"`
	IndicatorResults  []IndicatorResult `json:"indicator_results"`
	AllIndicatorsPass bool              `json:"all_indicators_pass"`
	PlacedOrders      []PlacedOrder     `json:"placed_orders"`
	CurrentOrder      *PlacedOrder      `json:"current_order,omitempty"`
	StartedAt         time.Time         `json:"started_at"`
	LastEvaluation    time.Time         `json:"last_evaluation"`
	NextAction        *time.Time        `json:"next_action,omitempty"`
	ErrorCount        int               `json:"error_count"`
	Logs              []AutomationLog   `json:"logs"`
	Message           string            `json:"message,omitempty"`
	// Daily recurrence tracking
	TradedToday   bool   `json:"traded_today"`              // Whether a trade was completed today
	LastTradeDate string `json:"last_trade_date,omitempty"` // Date of last completed trade (YYYY-MM-DD)
}

// LegDetail contains bid/ask/mid details for an option leg
type LegDetail struct {
	Symbol string  `json:"symbol"`
	Strike float64 `json:"strike"`
	Delta  float64 `json:"delta"`
	Bid    float64 `json:"bid"`
	Ask    float64 `json:"ask"`
	Mid    float64 `json:"mid"`
}

// StrikeSelection represents selected strikes for a spread
type StrikeSelection struct {
	ShortStrike   float64 `json:"short_strike"`
	LongStrike    float64 `json:"long_strike"`
	ShortSymbol   string  `json:"short_symbol"`
	LongSymbol    string  `json:"long_symbol"`
	ShortDelta    float64 `json:"short_delta"`
	LongDelta     float64 `json:"long_delta"`
	OptionType    string  `json:"option_type"` // "put" or "call"
	Expiry        string  `json:"expiry"`
	NaturalCredit float64 `json:"natural_credit"` // Bid of short - Ask of long
	MidCredit     float64 `json:"mid_credit"`     // Mid price credit
	// Detailed leg information for preview
	ShortLeg *LegDetail `json:"short_leg,omitempty"`
	LongLeg  *LegDetail `json:"long_leg,omitempty"`
}

// DailyData holds daily OHLC data for indicator calculations
type DailyData struct {
	Symbol        string    `json:"symbol"`
	Date          string    `json:"date"`
	Open          float64   `json:"open"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Close         float64   `json:"close"`
	PreviousClose float64   `json:"previous_close"`
	Volume        int64     `json:"volume"`
	Timestamp     time.Time `json:"timestamp"`
}

// GenerateIndicatorID creates a unique ID for an indicator
func GenerateIndicatorID() string {
	return fmt.Sprintf("ind_%d_%s", time.Now().UnixNano(), randomString(4))
}

// NewIndicatorConfig creates a new indicator config with minimal defaults
// User must configure threshold and symbol themselves - no suggested values
func NewIndicatorConfig(indicatorType IndicatorType) IndicatorConfig {
	return IndicatorConfig{
		ID:        GenerateIndicatorID(),
		Type:      indicatorType,
		Enabled:   true,
		Operator:  OperatorEqual, // Default operator, user can change
		Threshold: 0,             // No default value - user must set
		Symbol:    "",            // No default symbol - user must set
	}
}

// NewTradeConfiguration creates a new trade config with defaults
func NewTradeConfiguration() TradeConfiguration {
	return TradeConfiguration{
		Strategy:        StrategyPutSpread,
		Width:           20,
		TargetDelta:     0.05,
		MaxCapital:      5000,
		OrderType:       "limit",
		TimeInForce:     "day",
		PriceLadderStep: 0.05,
		MaxAttempts:     10,
		AttemptInterval: 30, // 30 seconds between attempts
		DeltaDriftLimit: 0.01,
	}
}

// NewAutomationConfig creates a new automation config with defaults
// Note: Indicators array starts empty - user adds only the indicators they need
func NewAutomationConfig(name, symbol string) *AutomationConfig {
	return &AutomationConfig{
		ID:            generateID(),
		Name:          name,
		Symbol:        symbol,
		Indicators:    []IndicatorConfig{}, // Empty - user adds indicators via UI
		EntryTime:     "12:25",
		EntryTimezone: "America/New_York",
		Enabled:       true,
		TradeConfig:   NewTradeConfiguration(),
		Created:       time.Now(),
		Updated:       time.Now(),
	}
}

// AddLog adds a log entry to an active automation
func (a *ActiveAutomation) AddLog(level, message string, details ...string) {
	log := AutomationLog{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}
	if len(details) > 0 {
		log.Details = details[0]
	}
	a.Logs = append(a.Logs, log)

	// Keep only last 100 logs
	if len(a.Logs) > 100 {
		a.Logs = a.Logs[len(a.Logs)-100:]
	}
}

// Evaluate checks if an indicator result passes its condition
func (r *IndicatorResult) Evaluate() bool {
	if !r.Enabled {
		return true // Disabled indicators always pass
	}

	switch r.Operator {
	case OperatorGreaterThan:
		return r.Value > r.Threshold
	case OperatorLessThan:
		return r.Value < r.Threshold
	case OperatorEqual:
		return r.Value == r.Threshold
	case OperatorNotEqual:
		return r.Value != r.Threshold
	default:
		return false
	}
}

// CalculateUnits calculates the number of spread units based on capital and width
func (tc *TradeConfiguration) CalculateUnits() int {
	if tc.Width <= 0 {
		return 0
	}
	// Max risk per unit = width * 100 (options multiplier)
	maxRiskPerUnit := float64(tc.Width) * 100.0
	units := int(tc.MaxCapital / maxRiskPerUnit)
	if units < 1 {
		return 0
	}
	return units
}

// Helper function to generate unique IDs
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
