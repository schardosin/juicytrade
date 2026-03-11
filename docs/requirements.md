# Requirements: Add Additional Indicators to Auto Trade

**GitHub Issue:** #4  
**Status:** Draft — Pending Customer Approval

---

## 1. Overview

Expand the Automation View's indicator system to support technical analysis indicators computed from historical price bars. Currently the system supports 5 indicators (VIX, Gap%, Range%, Trend%, FOMC Calendar). This feature adds 8 requested indicators plus 4 additional basic indicators, for a total of 17 available indicators.

New indicators are **added to the available pool** only. They do not impact existing automations — an indicator must be explicitly added to a Trade Automation to be used.

---

## 2. Requested Indicators

### 2.1 Customer-Requested (8)

| Indicator | Description | Parameters | Output Value |
|-----------|-------------|------------|--------------|
| **ADX** | Average Directional Index — measures trend strength regardless of direction | `period` (default: 14) | 0–100 scale |
| **CCI** | Commodity Channel Index — identifies cyclical trends | `period` (default: 20) | Unbounded (typically -200 to +200) |
| **CMO** | Chande Momentum Oscillator — measures momentum | `period` (default: 14) | -100 to +100 |
| **MACD** | Moving Average Convergence Divergence — trend-following momentum | `fast_period` (default: 12), `slow_period` (default: 26), `signal_period` (default: 9) | Unbounded (MACD line value) |
| **Momentum** | Price momentum — rate of change of price | `period` (default: 10) | Unbounded (percentage) |
| **RSI** | Relative Strength Index — measures overbought/oversold | `period` (default: 14) | 0–100 scale |
| **Stochastic** | Stochastic Oscillator — compares closing price to price range | `k_period` (default: 14), `d_period` (default: 3) | 0–100 scale (%K value) |
| **Stochastic RSI** | Stochastic applied to RSI values | `rsi_period` (default: 14), `stoch_period` (default: 14), `k_period` (default: 3), `d_period` (default: 3) | 0–100 scale (%K value) |

### 2.2 Additional Basic Indicators (4)

| Indicator | Description | Parameters | Output Value |
|-----------|-------------|------------|--------------|
| **SMA** | Simple Moving Average — average closing price over N periods | `period` (default: 20) | Price value |
| **EMA** | Exponential Moving Average — weighted average favoring recent prices | `period` (default: 20) | Price value |
| **ATR** | Average True Range — measures volatility | `period` (default: 14) | Price value (in points) |
| **Bollinger Band %B** | Position within Bollinger Bands — identifies relative price position | `period` (default: 20), `std_dev` (default: 2.0) | Typically 0–1 (can go outside) |

---

## 3. Self-Describing Indicator Parameters (Key Design)

Each indicator type **declares its own parameter definitions** so the UI can dynamically render the correct input fields without hardcoding per-indicator logic.

### 3.1 Backend: Parameter Definition Struct

A new `IndicatorParamDef` struct describes each parameter an indicator requires:

```go
type IndicatorParamDef struct {
    Key          string  `json:"key"`           // e.g., "period", "fast_period"
    Label        string  `json:"label"`         // e.g., "Period", "Fast Period"
    DefaultValue float64 `json:"default_value"` // e.g., 14
    Min          float64 `json:"min"`           // e.g., 1
    Max          float64 `json:"max"`           // e.g., 500
    Step         float64 `json:"step"`          // e.g., 1
    Type         string  `json:"type"`          // "int" or "float"
}
```

### 3.2 Backend: Indicator Metadata Struct

Each indicator type is described by an `IndicatorMeta` that the frontend can consume:

```go
type IndicatorMeta struct {
    Type        IndicatorType     `json:"type"`
    Label       string            `json:"label"`        // "RSI", "MACD", etc.
    Description string            `json:"description"`  // Human-readable description
    Category    string            `json:"category"`     // "momentum", "trend", "volatility", "market", "calendar"
    Params      []IndicatorParamDef `json:"params"`     // Ordered list of parameter definitions
    ValueRange  string            `json:"value_range"`  // e.g., "0-100", "unbounded", etc.
    NeedsSymbol bool              `json:"needs_symbol"` // Whether this indicator requires a symbol
}
```

### 3.3 Backend: Registry of Indicator Metadata

A function/map that returns `[]IndicatorMeta` for all available indicators (existing + new). This is served via a **new API endpoint** `GET /api/indicators/metadata` so the frontend can query the available indicators and their parameter definitions dynamically.

Example entries:

```go
// RSI - simple, single period
IndicatorMeta{
    Type: IndicatorRSI, Label: "RSI", Description: "Relative Strength Index — measures overbought/oversold conditions",
    Category: "momentum", NeedsSymbol: true, ValueRange: "0-100",
    Params: []IndicatorParamDef{
        {Key: "period", Label: "Period", DefaultValue: 14, Min: 2, Max: 500, Step: 1, Type: "int"},
    },
}

// MACD - multiple parameters
IndicatorMeta{
    Type: IndicatorMACD, Label: "MACD", Description: "Moving Average Convergence Divergence — trend-following momentum indicator",
    Category: "momentum", NeedsSymbol: true, ValueRange: "unbounded",
    Params: []IndicatorParamDef{
        {Key: "fast_period", Label: "Fast Period", DefaultValue: 12, Min: 2, Max: 500, Step: 1, Type: "int"},
        {Key: "slow_period", Label: "Slow Period", DefaultValue: 26, Min: 2, Max: 500, Step: 1, Type: "int"},
        {Key: "signal_period", Label: "Signal Period", DefaultValue: 9, Min: 2, Max: 500, Step: 1, Type: "int"},
    },
}

// Existing indicators also get metadata (backward compat — params will be empty for VIX, Calendar)
IndicatorMeta{
    Type: IndicatorVIX, Label: "VIX", Description: "CBOE Volatility Index",
    Category: "market", NeedsSymbol: false, ValueRange: "0-100+",
    Params: []IndicatorParamDef{},
}
```

### 3.4 Frontend: Dynamic Rendering

The `AutomationConfigForm.vue` component:
1. Fetches indicator metadata from `GET /api/indicators/metadata` on mount (or uses a cached/static version).
2. When user clicks "Add Indicator" and selects a type, the parameter definitions for that type are looked up from the metadata.
3. The UI **dynamically renders** input fields based on `params[]` — looping over the array and creating labeled number inputs with defaults, min/max validation, and step.
4. No hardcoded per-indicator field logic needed — adding a new indicator in the future only requires updating the backend metadata registry.

### 3.5 IndicatorConfig Extension

The existing `IndicatorConfig` struct is extended with a `Params` map to store the configured parameter values:

```go
type IndicatorConfig struct {
    ID        string             `json:"id,omitempty"`
    Type      IndicatorType      `json:"type"`
    Enabled   bool               `json:"enabled"`
    Operator  Operator           `json:"operator"`
    Threshold float64            `json:"threshold"`
    Symbol    string             `json:"symbol,omitempty"`
    Params    map[string]float64 `json:"params,omitempty"` // NEW: e.g., {"period": 14} or {"fast_period": 12, "slow_period": 26, "signal_period": 9}
}
```

- All parameter values are stored in the `Params` map using the `key` from the parameter definition.
- When `Params` is nil or a key is missing, the backend uses the default value from the indicator's metadata.
- Existing indicators (VIX, Gap, Range, Trend, Calendar) have no params, so existing configs deserialize unchanged.

---

## 4. Technical Design Requirements

### 4.1 Backend: New IndicatorType Constants (types.go)

Add 12 new constants:
- `IndicatorADX`, `IndicatorCCI`, `IndicatorCMO`, `IndicatorMACD`, `IndicatorMomentum`, `IndicatorRSI`, `IndicatorStoch`, `IndicatorStochRSI`
- `IndicatorSMA`, `IndicatorEMA`, `IndicatorATR`, `IndicatorBBPercent`

### 4.2 Backend: Indicator Calculations (indicators/service.go + technical.go)

- Each new indicator fetches historical daily bars via `s.providerManager.GetHistoricalBars()` — the existing pattern used by Gap/Range/Trend.
- Number of bars to fetch: `max(period * 2, 50)` to ensure enough data for calculation warm-up.
- All calculations use standard formulas (Wilder's smoothing for RSI/ADX/ATR, EMA for MACD).
- Add cases to `EvaluateIndicator` switch statement.
- Add formatting to `formatDetails` for each new type.
- **NEW FILE**: `technical.go` — Pure calculation functions for all technical indicators (no dependencies on service layer, easy to unit test).

### 4.3 Backend: Metadata API Endpoint

Add a new handler/route: `GET /api/indicators/metadata`
- Returns the full `[]IndicatorMeta` for all indicator types (existing + new).
- This endpoint requires no authentication beyond what's already in place.
- Response is cacheable (indicator metadata rarely changes).

### 4.4 Backend: Timeframe

All new indicators calculate on **daily (D) bars** from the historical data API. This is consistent with the existing indicators which all use daily data.

### 4.5 Frontend: AutomationConfigForm.vue

- Fetch indicator metadata from the API.
- Replace the hardcoded `indicatorTypes` array with data from the metadata endpoint.
- When an indicator is added, dynamically render parameter fields based on the metadata's `params[]` array.
- Pre-fill default values from metadata.
- Validate min/max on parameter inputs.
- Display indicator with its parameters in the row (e.g., "RSI (14)" or "MACD (12/26/9)").
- Update `formatIndicatorType()`, `getIndicatorDescription()`, and `getIndicatorDefaultSymbol()` to use metadata.

### 4.6 Frontend: Indicator Row Display

- For new indicators with parameters, display a compact summary in the indicator row (e.g., "RSI (14)", "MACD (12/26/9)", "SMA (20)").
- The symbol field remains available for all new indicators (user can specify which symbol to calculate the indicator for).

---

## 5. Acceptance Criteria

### AC-1: Metadata endpoint returns all indicators
- **Given** the backend is running
- **When** a GET request is made to `/api/indicators/metadata`
- **Then** the response contains metadata for all 17 indicator types (5 existing + 12 new)
- **And** each entry includes type, label, description, category, params, valueRange, and needsSymbol

### AC-2: New indicator types available in Add Indicator dialog
- **Given** a user is on the Automation Config Form
- **When** they click "Add Indicator"
- **Then** they see all 12 new indicator types (ADX, CCI, CMO, MACD, Momentum, RSI, Stoch, Stoch RSI, SMA, EMA, ATR, Bollinger %B) alongside the existing 5

### AC-3: Dynamic parameter rendering
- **Given** a user adds an indicator with parameters (e.g., RSI)
- **When** the indicator row is displayed
- **Then** parameter input fields are dynamically rendered based on the indicator's metadata
- **And** fields have the correct default values, labels, min/max constraints, and step values
- **And** no hardcoded per-indicator UI logic is required

### AC-4: Multi-parameter indicators render correctly
- **Given** a user adds MACD, Stochastic, Stochastic RSI, or Bollinger %B
- **When** the indicator row is displayed
- **Then** all parameter fields for that indicator are shown with correct defaults
- **And** values are stored in the `params` map on save

### AC-5: Backend calculation correctness
- **Given** a configured indicator with a valid symbol and parameters
- **When** the indicator is evaluated (via "Test" button or automation run)
- **Then** the backend fetches sufficient historical bars and returns the correct calculated value
- **And** the value matches the standard formula for that indicator type

### AC-6: Existing automations unaffected
- **Given** existing automation configs with VIX, Gap, Range, Trend, or Calendar indicators
- **When** the system is updated
- **Then** existing configs continue to work exactly as before (backward compatible)
- **And** the `params` field defaults to nil/empty for existing indicators
- **And** existing indicators also appear in the metadata endpoint with empty params

### AC-7: Test functionality works for new indicators
- **Given** a user configures a new indicator (e.g., RSI < 70 on symbol SPY)
- **When** they click "Test" or "Test All"
- **Then** the indicator is evaluated and the result (value, pass/fail) is displayed
- **And** stale/error handling works the same as existing indicators

### AC-8: Indicator evaluation in automation run
- **Given** an automation config includes new indicators
- **When** the automation runs and reaches indicator evaluation
- **Then** all new indicators are evaluated alongside existing ones
- **And** all enabled indicators must pass for a trade to execute (existing logic)

### AC-9: Error handling
- **Given** an indicator with an invalid symbol or insufficient historical data
- **When** the indicator is evaluated
- **Then** the result includes an appropriate error message
- **And** cached fallback values work the same as existing indicators

### AC-10: Default parameters used when not specified
- **Given** an indicator config where `params` is nil or missing specific keys
- **When** the indicator is evaluated
- **Then** the backend uses the default values from the indicator metadata
- **And** the frontend pre-fills the defaults when a new indicator is added

---

## 6. Out of Scope

- Intraday timeframes (all indicators use daily bars)
- Indicator parameter optimization / backtesting
- Compound indicators (e.g., "RSI above 30 AND below 70" — user can add two RSI instances)
- Charting/visualization of indicator values on the chart
- MACD histogram or signal line as separate outputs (only MACD line value is used for threshold comparison)

---

## 7. Edge Cases

1. **Insufficient historical data**: If the provider returns fewer bars than needed for the calculation, return an error message like "Insufficient data for RSI(14): need 15 bars, got 10"
2. **Zero division**: CMO/RSI with all-same prices → handle gracefully (return 50 for RSI, 0 for CMO)
3. **New config vs existing config**: Existing configs with no `params` field must deserialize correctly — Go will default `map` to nil
4. **Symbol resolution**: New indicators should use the indicator's configured symbol, or fallback to QQQ if empty (consistent with Gap/Range/Trend pattern)
5. **Parameter validation**: If a user provides a period outside the min/max range, the backend should clamp to the valid range and continue (not error)

---

## 8. Files to Modify

### Backend (Go)
1. `trade-backend-go/internal/automation/types/types.go` — Add new IndicatorType constants, IndicatorParamDef, IndicatorMeta structs, extend IndicatorConfig with Params field, add metadata registry function
2. `trade-backend-go/internal/automation/indicators/service.go` — Add calculation methods and switch cases for new indicators
3. `trade-backend-go/internal/automation/indicators/technical.go` — **NEW FILE** — Pure calculation functions for all technical indicators
4. `trade-backend-go/internal/automation/api/handler.go` (or equivalent) — Add `GET /api/indicators/metadata` endpoint

### Frontend (Vue)
5. `trade-app/src/components/automation/AutomationConfigForm.vue` — Fetch metadata, dynamic param rendering, updated indicator display

### Tests
6. `trade-backend-go/internal/automation/indicators/technical_test.go` — **NEW FILE** — Unit tests for all indicator calculation functions
