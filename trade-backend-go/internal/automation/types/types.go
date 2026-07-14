package types

import (
	"fmt"
	"math"
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

	// New — Momentum
	IndicatorRSI      IndicatorType = "rsi"
	IndicatorMACD     IndicatorType = "macd"
	IndicatorMomentum IndicatorType = "momentum"
	IndicatorCMO      IndicatorType = "cmo"
	IndicatorStoch    IndicatorType = "stoch"
	IndicatorStochRSI IndicatorType = "stoch_rsi"

	// New — Trend
	IndicatorADX IndicatorType = "adx"
	IndicatorCCI IndicatorType = "cci"
	IndicatorSMA IndicatorType = "sma"
	IndicatorEMA IndicatorType = "ema"

	// New — Volatility
	IndicatorATR       IndicatorType = "atr"
	IndicatorBBPercent IndicatorType = "bb_percent"
)

// Operator defines comparison operators for indicators
type Operator string

const (
	OperatorGreaterThan Operator = "gt"
	OperatorLessThan    Operator = "lt"
	OperatorEqual       Operator = "eq"
	OperatorNotEqual    Operator = "ne"
)

// IndicatorParamDef describes a single parameter for an indicator type.
// The frontend uses this to dynamically render input fields.
type IndicatorParamDef struct {
	Key          string  `json:"key"`           // Parameter key, e.g. "period", "fast_period"
	Label        string  `json:"label"`         // Display label, e.g. "Period", "Fast Period"
	DefaultValue float64 `json:"default_value"` // Default value, e.g. 14
	Min          float64 `json:"min"`           // Minimum allowed value
	Max          float64 `json:"max"`           // Maximum allowed value
	Step         float64 `json:"step"`          // Input step increment
	Type         string  `json:"type"`          // "int" or "float"
}

// IndicatorMeta describes an indicator type's full metadata.
// Served to the frontend via GET /api/automation/indicators/metadata.
type IndicatorMeta struct {
	Type        IndicatorType       `json:"type"`         // e.g. "rsi"
	Label       string              `json:"label"`        // e.g. "RSI"
	Description string              `json:"description"`  // Human-readable description
	Category    string              `json:"category"`     // "momentum", "trend", "volatility", "market", "calendar"
	Params      []IndicatorParamDef `json:"params"`       // Ordered list of parameter definitions (empty for VIX, Calendar, etc.)
	ValueRange  string              `json:"value_range"`  // e.g. "0-100", "unbounded"
	NeedsSymbol bool                `json:"needs_symbol"` // Whether this indicator requires a symbol input
}

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

// CapitalMode selects how MaxCapital is interpreted.
type CapitalMode string

const (
	CapitalModeFixed   CapitalMode = "fixed"   // MaxCapital is a dollar amount (default / legacy)
	CapitalModePercent CapitalMode = "percent" // MaxCapital derived from % of account Net Liq
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
	// Params holds indicator-specific parameters (e.g., period, fast_period)
	Params map[string]float64 `json:"params,omitempty"`
}

// IndicatorGroup defines a named group of indicators that are AND-ed together.
// Multiple groups are OR-ed: if ANY group passes, the automation proceeds.
type IndicatorGroup struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Indicators []IndicatorConfig `json:"indicators"`
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
	ParamSummary  string        `json:"param_summary,omitempty"` // e.g., "14" or "12/26/9"
}

// GroupResult contains the evaluation result for an entire indicator group.
type GroupResult struct {
	GroupID          string            `json:"group_id"`
	GroupName        string            `json:"group_name"`
	Pass             bool              `json:"pass"`
	IndicatorResults []IndicatorResult `json:"indicator_results"`
}

// IronCondorSideConfig defines per-side configuration for an Iron Condor
type IronCondorSideConfig struct {
	TargetDelta float64 `json:"target_delta"` // Target delta for this side's short strike (e.g., 0.05)
	Width       int     `json:"width"`        // Spread width in points for this side (e.g., 50)
}

// TradeConfiguration defines the trade parameters for an automation
type TradeConfiguration struct {
	Strategy          TradeStrategy `json:"strategy"`                      // "put_spread", "call_spread", "iron_condor"
	Width             int           `json:"width"`                         // Spread width (e.g., 20, 30) - used for put_spread/call_spread
	TargetDelta       float64       `json:"target_delta"`                  // Target delta for short strike (e.g., 0.05) - used for put_spread/call_spread
	MaxCapital        float64       `json:"max_capital"`                   // Maximum capital to use (dollars; used when capital_mode == fixed)
	CapitalMode       CapitalMode   `json:"capital_mode,omitempty"`        // "fixed" | "percent"; empty => fixed (backward compatible)
	MaxCapitalPercent float64       `json:"max_capital_percent,omitempty"` // 1..100, percent of account Net Liq (used when capital_mode == percent)
	OrderType         string        `json:"order_type"`                    // "limit" or "market"
	TimeInForce       string        `json:"time_in_force"`                 // "day" or "gtc"
	PriceLadderStep   float64       `json:"price_ladder_step"`             // Price decrement step (e.g., 0.05)
	MaxAttempts       int           `json:"max_attempts"`                  // Maximum order replacement attempts
	AttemptInterval   int           `json:"attempt_interval"`              // Seconds between price reductions
	DeltaDriftLimit   float64       `json:"delta_drift_limit"`             // Max delta drift before replacing (e.g., 0.01)
	StartingOffset    float64       `json:"starting_offset,omitempty"`     // Amount below mid to start (e.g., 0.10)
	MinCredit         float64       `json:"min_credit,omitempty"`          // Minimum acceptable credit (stop if below)
	ExpirationMode    string        `json:"expiration_mode,omitempty"`     // "0dte", "1dte", "2dte", "custom"
	CustomExpiration  string        `json:"custom_expiration,omitempty"`   // Custom expiration date (YYYY-MM-DD)
	// Iron Condor specific - per-side delta and width configuration
	PutSideConfig  *IronCondorSideConfig `json:"put_side_config,omitempty"`  // Put side config (iron_condor only)
	CallSideConfig *IronCondorSideConfig `json:"call_side_config,omitempty"` // Call side config (iron_condor only)
	// Lot size (multi-order) execution
	LotSize   int  `json:"lot_size,omitempty"`   // Units per order. 0/1 = single order (today's behavior). >=2 splits the capital-derived total into sequential lots.
	LegsDrift bool `json:"legs_drift,omitempty"` // false = all lots reuse the first lot's strikes, no delta drift at all; true = re-select strikes before each lot and apply mid-order drift.
}

// RecurrenceMode defines how the automation repeats
type RecurrenceMode string

const (
	RecurrenceOnce  RecurrenceMode = "once"  // Run once, then stop (default)
	RecurrenceDaily RecurrenceMode = "daily" // Reset daily and run again each trading day
)

// AutomationConfig defines a complete automation configuration
type AutomationConfig struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	Symbol          string             `json:"symbol"`                     // Underlying symbol (e.g., "NDX", "SPX")
	Indicators      []IndicatorConfig  `json:"indicators"`                 // List of indicator configurations (legacy, kept for backward compat)
	IndicatorGroups []IndicatorGroup   `json:"indicator_groups,omitempty"` // Grouped indicators: groups are OR-ed, indicators within AND-ed
	EntryTime       string             `json:"entry_time"`                 // Entry time in HH:MM format (e.g., "12:25")
	EntryTimezone   string             `json:"entry_timezone"`             // Timezone (e.g., "America/New_York")
	Enabled         bool               `json:"enabled"`                    // Whether this automation is active
	Recurrence      RecurrenceMode     `json:"recurrence"`                 // "once" or "daily" (default: "once")
	TradeConfig     TradeConfiguration `json:"trade_config"`               // Trade parameters
	Created         time.Time          `json:"created"`
	Updated         time.Time          `json:"updated"`
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
	GroupResults      []GroupResult     `json:"group_results,omitempty"`
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
	// Capital resolution (percent mode). Zero/omitted in fixed mode.
	ResolvedMaxCapital float64    `json:"resolved_max_capital,omitempty"` // Dollars resolved from % of Net Liq
	ResolvedNetLiq     float64    `json:"resolved_net_liq,omitempty"`     // Net Liq (account.equity) read at resolution
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`          // Timestamp of the latest capital resolution
	// Multi-order (lot size) execution state.
	OrderPlan       []int                      `json:"order_plan,omitempty"`        // Per-lot quantities, e.g. [2,2,2,1]. Derived once when entering trading for lot 0.
	CurrentLotIndex int                        `json:"current_lot_index,omitempty"` // 0-based index of the lot currently being placed/monitored.
	LockedStrikes   *StrikeSelection           `json:"locked_strikes,omitempty"`    // Strikes from lot 0 (spread), reused when legs_drift=false.
	LockedICStrikes *IronCondorStrikeSelection `json:"locked_ic_strikes,omitempty"` // Iron Condor equivalent of LockedStrikes.
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

// IronCondorStrikeSelection represents selected strikes for both sides of an Iron Condor
type IronCondorStrikeSelection struct {
	PutSide            *StrikeSelection `json:"put_side"`
	CallSide           *StrikeSelection `json:"call_side"`
	Expiry             string           `json:"expiry"`
	TotalNaturalCredit float64          `json:"total_natural_credit"` // Sum of both sides' natural credits
	TotalMidCredit     float64          `json:"total_mid_credit"`     // Sum of both sides' mid credits
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

// GenerateGroupID creates a unique ID for an indicator group
func GenerateGroupID() string {
	return fmt.Sprintf("grp_%d_%s", time.Now().UnixNano(), randomString(4))
}

// GetIndicatorMetadata returns metadata for all available indicator types.
// This is the single source of truth for indicator definitions.
func GetIndicatorMetadata() []IndicatorMeta {
	return []IndicatorMeta{
		// ---- Existing: Market ----
		{
			Type: IndicatorVIX, Label: "VIX", Category: "market",
			Description: "CBOE Volatility Index — measures market fear/uncertainty",
			Params:      []IndicatorParamDef{}, ValueRange: "0-100+", NeedsSymbol: false,
		},
		{
			Type: IndicatorGap, Label: "Gap %", Category: "market",
			Description: "Gap percentage: (Open - PrevClose) / PrevClose × 100",
			Params:      []IndicatorParamDef{}, ValueRange: "unbounded", NeedsSymbol: true,
		},
		{
			Type: IndicatorRange, Label: "Range %", Category: "market",
			Description: "Range percentage: (High - Low) / Open × 100",
			Params:      []IndicatorParamDef{}, ValueRange: "0+", NeedsSymbol: true,
		},
		{
			Type: IndicatorTrend, Label: "Trend %", Category: "market",
			Description: "Trend percentage: (Current - Open) / Open × 100",
			Params:      []IndicatorParamDef{}, ValueRange: "unbounded", NeedsSymbol: true,
		},
		// ---- Existing: Calendar ----
		{
			Type: IndicatorCalendar, Label: "FOMC Calendar", Category: "calendar",
			Description: "FOMC meeting day indicator: 1 = FOMC day, 0 = not FOMC day",
			Params:      []IndicatorParamDef{}, ValueRange: "0-1", NeedsSymbol: false,
		},
		// ---- New: Momentum ----
		{
			Type: IndicatorRSI, Label: "RSI", Category: "momentum",
			Description: "Relative Strength Index — measures overbought/oversold conditions",
			ValueRange:  "0-100", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 14, Min: 2, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorMACD, Label: "MACD", Category: "momentum",
			Description: "Moving Average Convergence Divergence — trend-following momentum (MACD line value)",
			ValueRange:  "unbounded", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "fast_period", Label: "Fast Period", DefaultValue: 12, Min: 2, Max: 500, Step: 1, Type: "int"},
				{Key: "slow_period", Label: "Slow Period", DefaultValue: 26, Min: 2, Max: 500, Step: 1, Type: "int"},
				{Key: "signal_period", Label: "Signal Period", DefaultValue: 9, Min: 2, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorMomentum, Label: "Momentum", Category: "momentum",
			Description: "Price momentum — rate of change: (Close - Close[n]) / Close[n] × 100",
			ValueRange:  "unbounded", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 10, Min: 1, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorCMO, Label: "CMO", Category: "momentum",
			Description: "Chande Momentum Oscillator — measures momentum on a -100 to +100 scale",
			ValueRange:  "-100 to 100", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 14, Min: 2, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorStoch, Label: "Stochastic", Category: "momentum",
			Description: "Stochastic Oscillator — compares closing price to price range (%K value)",
			ValueRange:  "0-100", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "k_period", Label: "K Period", DefaultValue: 14, Min: 1, Max: 500, Step: 1, Type: "int"},
				{Key: "d_period", Label: "D Period", DefaultValue: 3, Min: 1, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorStochRSI, Label: "Stochastic RSI", Category: "momentum",
			Description: "Stochastic oscillator applied to RSI values (%K value)",
			ValueRange:  "0-100", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "rsi_period", Label: "RSI Period", DefaultValue: 14, Min: 2, Max: 500, Step: 1, Type: "int"},
				{Key: "stoch_period", Label: "Stoch Period", DefaultValue: 14, Min: 1, Max: 500, Step: 1, Type: "int"},
				{Key: "k_period", Label: "K Period", DefaultValue: 3, Min: 1, Max: 500, Step: 1, Type: "int"},
				{Key: "d_period", Label: "D Period", DefaultValue: 3, Min: 1, Max: 500, Step: 1, Type: "int"},
			},
		},
		// ---- New: Trend ----
		{
			Type: IndicatorADX, Label: "ADX", Category: "trend",
			Description: "Average Directional Index — measures trend strength regardless of direction",
			ValueRange:  "0-100", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 14, Min: 2, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorCCI, Label: "CCI", Category: "trend",
			Description: "Commodity Channel Index — identifies cyclical trends",
			ValueRange:  "unbounded", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 20, Min: 2, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorSMA, Label: "SMA", Category: "trend",
			Description: "Simple Moving Average — average closing price over N periods",
			ValueRange:  "price", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 20, Min: 1, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorEMA, Label: "EMA", Category: "trend",
			Description: "Exponential Moving Average — weighted average favoring recent prices",
			ValueRange:  "price", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 20, Min: 1, Max: 500, Step: 1, Type: "int"},
			},
		},
		// ---- New: Volatility ----
		{
			Type: IndicatorATR, Label: "ATR", Category: "volatility",
			Description: "Average True Range — measures price volatility in points",
			ValueRange:  "0+", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 14, Min: 1, Max: 500, Step: 1, Type: "int"},
			},
		},
		{
			Type: IndicatorBBPercent, Label: "Bollinger %B", Category: "volatility",
			Description: "Bollinger Band %B — position within Bollinger Bands (0=lower, 1=upper)",
			ValueRange:  "typically 0-1", NeedsSymbol: true,
			Params: []IndicatorParamDef{
				{Key: "period", Label: "Period", DefaultValue: 20, Min: 2, Max: 500, Step: 1, Type: "int"},
				{Key: "std_dev", Label: "Std Deviations", DefaultValue: 2.0, Min: 0.5, Max: 5.0, Step: 0.1, Type: "float"},
			},
		},
	}
}

// NewIndicatorConfig creates a new indicator config with minimal defaults.
// For technical indicators, default params are populated from metadata.
// User must configure threshold and symbol themselves - no suggested values
func NewIndicatorConfig(indicatorType IndicatorType) IndicatorConfig {
	config := IndicatorConfig{
		ID:        GenerateIndicatorID(),
		Type:      indicatorType,
		Enabled:   true,
		Operator:  OperatorEqual, // Default operator, user can change
		Threshold: 0,             // No default value - user must set
		Symbol:    "",            // No default symbol - user must set
	}
	// Populate default params from metadata
	for _, meta := range GetIndicatorMetadata() {
		if meta.Type == indicatorType && len(meta.Params) > 0 {
			config.Params = make(map[string]float64)
			for _, p := range meta.Params {
				config.Params[p.Key] = p.DefaultValue
			}
			break
		}
	}
	return config
}

// NewTradeConfiguration creates a new trade config with defaults
func NewTradeConfiguration() TradeConfiguration {
	return TradeConfiguration{
		Strategy:        StrategyPutSpread,
		Width:           20,
		TargetDelta:     0.05,
		MaxCapital:      5000,
		CapitalMode:     CapitalModeFixed,
		OrderType:       "limit",
		TimeInForce:     "day",
		PriceLadderStep: 0.05,
		MaxAttempts:     10,
		AttemptInterval: 30, // 30 seconds between attempts
		DeltaDriftLimit: 0.01,
		LotSize:         1,     // single order by default (equivalent to unset)
		LegsDrift:       false, // freeze legs across lots by default
	}
}

// NewAutomationConfig creates a new automation config with defaults
// Note: Indicators array starts empty - user adds only the indicators they need
func NewAutomationConfig(name, symbol string) *AutomationConfig {
	return &AutomationConfig{
		ID:              generateID(),
		Name:            name,
		Symbol:          symbol,
		Indicators:      []IndicatorConfig{}, // Legacy - kept empty for backward compat
		IndicatorGroups: []IndicatorGroup{},  // Empty - user adds groups via UI
		EntryTime:       "12:25",
		EntryTimezone:   "America/New_York",
		Enabled:         true,
		TradeConfig:     NewTradeConfiguration(),
		Created:         time.Now(),
		Updated:         time.Now(),
	}
}

// GetEffectiveIndicatorGroups returns the indicator groups to evaluate.
// If IndicatorGroups is populated, it is returned directly.
// Otherwise, legacy Indicators are wrapped in a single "Default" group for backward compatibility.
func (c *AutomationConfig) GetEffectiveIndicatorGroups() []IndicatorGroup {
	if len(c.IndicatorGroups) > 0 {
		return c.IndicatorGroups
	}
	if len(c.Indicators) > 0 {
		return []IndicatorGroup{
			{ID: "default", Name: "Default", Indicators: c.Indicators},
		}
	}
	return []IndicatorGroup{}
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

// EffectiveCapitalMode returns the capital mode, defaulting empty to fixed.
// This preserves backward compatibility for configs persisted before capital_mode existed.
func (tc *TradeConfiguration) EffectiveCapitalMode() CapitalMode {
	if tc.CapitalMode == "" {
		return CapitalModeFixed
	}
	return tc.CapitalMode
}

// ResolveMaxCapital computes the effective dollar cap for the given Net Liq.
// In fixed mode it returns MaxCapital and ignores netLiq. In percent mode it
// returns netLiq * (MaxCapitalPercent / 100). There is NO fallback: percent mode
// returns an error when the percent is out of range or netLiq is nil/non-positive.
func (tc *TradeConfiguration) ResolveMaxCapital(netLiq *float64) (float64, error) {
	if tc.EffectiveCapitalMode() == CapitalModeFixed {
		return tc.MaxCapital, nil
	}

	pct := tc.MaxCapitalPercent
	// Reject non-finite percentages first: NaN slips through plain range
	// comparisons (every comparison against NaN is false), so guard explicitly.
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		return 0, fmt.Errorf("invalid max_capital_percent %v (must be a finite number 1..100)", pct)
	}
	if pct < 1 || pct > 100 {
		return 0, fmt.Errorf("invalid max_capital_percent %.2f (must be 1..100)", pct)
	}
	if netLiq == nil {
		return 0, fmt.Errorf("net liq unavailable; cannot resolve percentage cap")
	}
	// Reject non-finite Net Liq for the same reason as the percent guard above.
	if math.IsNaN(*netLiq) || math.IsInf(*netLiq, 0) {
		return 0, fmt.Errorf("net liq is not a finite number; cannot resolve percentage cap")
	}
	if *netLiq <= 0 {
		return 0, fmt.Errorf("net liq non-positive; cannot resolve percentage cap")
	}
	return *netLiq * (pct / 100.0), nil
}

// CalculateUnits calculates the number of spread units based on capital and width
func (tc *TradeConfiguration) CalculateUnits() int {
	return tc.CalculateUnitsWithCapital(tc.MaxCapital)
}

// CalculateUnitsWithCapital calculates the number of spread units using an
// externally-resolved dollar cap (from fixed $ or % of Net Liq). This keeps the
// sizing formula in one place while allowing percent-mode callers to size from a
// live-resolved value.
func (tc *TradeConfiguration) CalculateUnitsWithCapital(resolvedMaxCapital float64) int {
	width := tc.Width

	// For Iron Condor, use the wider of the two sides for position sizing
	// Max loss in an IC is the wider spread minus total credit received
	if tc.Strategy == StrategyIronCondor && tc.PutSideConfig != nil && tc.CallSideConfig != nil {
		putWidth := tc.PutSideConfig.Width
		callWidth := tc.CallSideConfig.Width
		if putWidth > callWidth {
			width = putWidth
		} else {
			width = callWidth
		}
	}

	if width <= 0 {
		return 0
	}
	// Max risk per unit = width * 100 (options multiplier)
	maxRiskPerUnit := float64(width) * 100.0
	units := int(resolvedMaxCapital / maxRiskPerUnit)
	if units < 1 {
		return 0
	}
	return units
}

// EffectiveLotSize returns the configured lot size, treating any value < 1
// (unset/0 or negative) as 1. A lot size of 1 means a single order — today's
// legacy behavior.
func (tc *TradeConfiguration) EffectiveLotSize() int {
	if tc.LotSize < 1 {
		return 1
	}
	return tc.LotSize
}

// SplitIntoLots splits a capital-derived total into sequential per-lot quantities.
//
//	totalUnits: the hard cap from CalculateUnitsWithCapital (never exceeded).
//	lotSize:    units per order (values < 1 are treated as 1).
//
// Returns an ordered slice whose elements sum EXACTLY to totalUnits:
//   - fullLots  = totalUnits / lotSize   (orders of lotSize units)
//   - remainder = totalUnits % lotSize   -> appended as a final smaller lot
//
// Examples:
//
//	(7, 2) -> [2, 2, 2, 1]
//	(6, 2) -> [2, 2, 2]
//	(5, 5) -> [5]
//	(3, 5) -> [3]      (lot larger than total => single lot for the total)
//	(0, _) -> []       (insufficient capital => no lots)
func SplitIntoLots(totalUnits, lotSize int) []int {
	if totalUnits <= 0 {
		return []int{}
	}
	if lotSize < 1 {
		lotSize = 1
	}
	plan := make([]int, 0)
	full := totalUnits / lotSize
	rem := totalUnits % lotSize
	for i := 0; i < full; i++ {
		plan = append(plan, lotSize)
	}
	if rem > 0 {
		plan = append(plan, rem)
	}
	return plan
}

// IsMultiLot reports whether this run is executing a multi-order plan (more than
// one lot). Single-order/legacy runs return false so they never engage the
// multi-order code paths (OD-1).
func (a *ActiveAutomation) IsMultiLot() bool {
	return len(a.OrderPlan) > 1
}

// HasMorePendingLots reports whether there are lots remaining after the current
// one — i.e. filling the current lot should advance the plan rather than
// terminate the run.
func (a *ActiveAutomation) HasMorePendingLots() bool {
	return a.CurrentLotIndex < len(a.OrderPlan)-1
}

// DriftAllowed reports whether mid-order delta-drift strike replacement is
// permitted for the current run. It is disabled for multi-lot runs with frozen
// legs (legs_drift == false). Single-order runs are unaffected, preserving
// today's behavior exactly regardless of legs_drift (OD-1).
func (a *ActiveAutomation) DriftAllowed(deltaDriftLimit float64) bool {
	if deltaDriftLimit <= 0 {
		return false
	}
	if a.IsMultiLot() && !a.Config.TradeConfig.LegsDrift {
		return false
	}
	return true
}

// ShouldReuseLockedStrikes reports whether the given lot should reuse lot 0's
// locked strikes instead of re-selecting them. This is true for lots after the
// first when legs_drift is false. Lot 0 always finds fresh strikes; when
// legs_drift is true every lot re-selects.
func (a *ActiveAutomation) ShouldReuseLockedStrikes(lotIndex int) bool {
	return lotIndex >= 1 && !a.Config.TradeConfig.LegsDrift
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
