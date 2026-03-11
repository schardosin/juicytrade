# Implementation Plan: Add Additional Indicators to Auto Trade

**GitHub Issue:** #4
**Design Doc:** `docs/design.md`
**Status:** Ready for Implementation

---

## Prerequisites

- Read `docs/design.md` in full before starting.
- All existing tests must pass before and after each step.
- Each step is a self-contained, committable unit.

---

## Step 1: types.go — Constants, Structs, and Metadata Registry

### Files to Modify
- `trade-backend-go/internal/automation/types/types.go`

### Changes

**1a. Add 12 new `IndicatorType` constants** (Design §3.1)

After the existing `IndicatorCalendar` constant (line 18), add:

```go
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
```

**1b. Add `IndicatorParamDef` struct** (Design §3.2)

Add after the `Operator` constants block:

```go
type IndicatorParamDef struct {
    Key          string  `json:"key"`
    Label        string  `json:"label"`
    DefaultValue float64 `json:"default_value"`
    Min          float64 `json:"min"`
    Max          float64 `json:"max"`
    Step         float64 `json:"step"`
    Type         string  `json:"type"` // "int" or "float"
}
```

**1c. Add `IndicatorMeta` struct** (Design §3.2)

```go
type IndicatorMeta struct {
    Type        IndicatorType     `json:"type"`
    Label       string            `json:"label"`
    Description string            `json:"description"`
    Category    string            `json:"category"`
    Params      []IndicatorParamDef `json:"params"`
    ValueRange  string            `json:"value_range"`
    NeedsSymbol bool              `json:"needs_symbol"`
}
```

**1d. Add `Params` field to `IndicatorConfig`** (Design §3.3)

Add after the `Symbol` field (line 63):

```go
Params map[string]float64 `json:"params,omitempty"`
```

**1e. Add `ParamSummary` field to `IndicatorResult`** (Design §7.3, Option A)

Add after the `Error` field (line 79):

```go
ParamSummary string `json:"param_summary,omitempty"`
```

**1f. Add `GetIndicatorMetadata()` function** (Design §4.1)

Add the full registry function returning `[]IndicatorMeta` for all 17 indicators (5 existing + 12 new). Copy the complete registry from Design §4.1.

**1g. Update `NewIndicatorConfig()`** (Design §3.5)

Update the factory function (lines 249-258) to populate default params from metadata when the indicator has parameters defined.

### Tests to Run
```bash
cd trade-backend-go && go build ./...
cd trade-backend-go && go vet ./...
```

### Commit Message
```
feat(types): add indicator type constants, metadata structs, and registry

Add 12 new IndicatorType constants (RSI, MACD, ADX, CCI, etc.),
IndicatorParamDef and IndicatorMeta structs for self-describing
indicators, Params field on IndicatorConfig, ParamSummary field
on IndicatorResult, and the GetIndicatorMetadata() registry function
that serves as the single source of truth for all 17 indicators.
```

---

## Step 2: technical.go — Pure Calculation Functions

### Files to Create
- `trade-backend-go/internal/automation/indicators/technical.go`

### Changes

Create a new file in `package indicators` containing all pure calculation functions. These functions take `[]float64` slices and return `(float64, error)`. They import only `math`, `fmt`, and `errors` from the standard library.

**Functions to implement** (Design §6.3–6.14, §6.16):

| Function | Signature | Design Section |
|----------|-----------|----------------|
| `CalcSMA` | `(closes []float64, period int) (float64, error)` | §6.3 |
| `CalcEMA` | `(closes []float64, period int) (float64, error)` | §6.4 |
| `CalcRSI` | `(closes []float64, period int) (float64, error)` | §6.5 |
| `CalcMACD` | `(closes []float64, fastPeriod, slowPeriod, signalPeriod int) (float64, error)` | §6.6 |
| `CalcMomentum` | `(closes []float64, period int) (float64, error)` | §6.7 |
| `CalcCMO` | `(closes []float64, period int) (float64, error)` | §6.8 |
| `CalcStochastic` | `(highs, lows, closes []float64, kPeriod, dPeriod int) (float64, error)` | §6.9 |
| `CalcStochRSI` | `(closes []float64, rsiPeriod, stochPeriod, kPeriod, dPeriod int) (float64, error)` | §6.10 |
| `CalcADX` | `(highs, lows, closes []float64, period int) (float64, error)` | §6.11 |
| `CalcCCI` | `(highs, lows, closes []float64, period int) (float64, error)` | §6.12 |
| `CalcATR` | `(highs, lows, closes []float64, period int) (float64, error)` | §6.13 |
| `CalcBollingerPercentB` | `(closes []float64, period int, stdDev float64) (float64, error)` | §6.14 |

**Internal helpers** (unexported):

| Helper | Purpose |
|--------|---------|
| `calcEMASeries(data []float64, period int) []float64` | Full EMA series for MACD signal line |
| `calcRSISeries(closes []float64, period int) []float64` | Full RSI series for StochRSI |

**Also add** the `extractOHLC()` helper (Design §6.2) that converts `[]map[string]interface{}` bar data to typed OHLC slices. This helper reuses the existing `getFloatFromBar()` function already in `service.go`.

**Key implementation notes:**
- RSI uses Wilder's smoothing (Design §6.5). Edge case: all-same prices → return 50.
- MACD returns the MACD line value only, not histogram (Design §6.6).
- ADX uses Wilder's smoothing for TR/+DM/-DM and then for DX (Design §6.11).
- Stochastic returns raw (fast) %K (Design §6.9).
- CCI uses Lambert's constant 0.015 (Design §6.12).
- Bollinger %B: if bandWidth == 0 return 0.5 (Design §6.14).
- All functions return descriptive errors for insufficient data, e.g., `"CalcRSI: insufficient data, need %d closes, got %d"`.

### Tests to Run
```bash
cd trade-backend-go && go build ./internal/automation/indicators/...
cd trade-backend-go && go vet ./internal/automation/indicators/...
```

### Commit Message
```
feat(indicators): add pure calculation functions for 12 technical indicators

New file technical.go with stateless calculation functions for
SMA, EMA, RSI, MACD, Momentum, CMO, Stochastic, StochRSI, ADX,
CCI, ATR, and Bollinger %B. All functions take plain float64
slices and return (float64, error), with no service-layer deps.
Includes extractOHLC helper and internal EMA/RSI series helpers.
```

---

## Step 3: technical_test.go — Unit Tests

### Files to Create
- `trade-backend-go/internal/automation/indicators/technical_test.go`

### Changes

Create a comprehensive test file in `package indicators` with test cases per Design §9.2.

**Test cases per function:**

| Function | Test Cases |
|----------|------------|
| `CalcSMA` | Known result `[1,2,3,4,5]` period=3 → 4.0; period > len → error |
| `CalcEMA` | Constant prices converge to SMA; known spreadsheet value; insufficient data error |
| `CalcRSI` | All rising → near 100; all falling → near 0; all same → 50; Wilder's known value (§9.3); insufficient data |
| `CalcMACD` | Rising prices → positive MACD; insufficient data error |
| `CalcMomentum` | `[100, 105]` period=1 → 5.0; `[100, 95]` period=1 → -5.0 |
| `CalcCMO` | Monotonic rise → near +100; monotonic fall → near -100; no movement → 0 |
| `CalcStochastic` | Close at high → 100; close at low → 0; close at midpoint → 50 |
| `CalcStochRSI` | Output in 0-100 range; extreme inputs produce extreme outputs |
| `CalcADX` | Strong trend data → ADX > 25; flat data → ADX < 20 |
| `CalcCCI` | Price at SMA → CCI near 0; insufficient data error |
| `CalcATR` | Constant-range bars → ATR equals that range; Wilder's smoothing correctness |
| `CalcBollingerPercentB` | Close at SMA → 0.5; close at upper band → 1.0; close at lower band → 0.0 |

**Also include:**
- `TestExtractOHLC` — verify conversion from map bars to typed slices.
- Use `assert.InDelta` with tolerance 0.5 for known reference values to handle floating-point precision.

### Tests to Run
```bash
cd trade-backend-go && go test ./internal/automation/indicators/... -v -count=1
```

All tests must pass. This validates the pure math before any service integration.

### Commit Message
```
test(indicators): add unit tests for all technical indicator calculations

Comprehensive test coverage for CalcSMA, CalcEMA, CalcRSI, CalcMACD,
CalcMomentum, CalcCMO, CalcStochastic, CalcStochRSI, CalcADX, CalcCCI,
CalcATR, CalcBollingerPercentB, and extractOHLC. Tests cover known
values, edge cases (all-same prices, zero division), and insufficient
data errors.
```

---

## Step 4: service.go — Switch Cases, Helpers, and Formatting

### Files to Modify
- `trade-backend-go/internal/automation/indicators/service.go`

### Changes

**4a. Add helper functions** (Design §3.4)

Add `getParamOrDefault()` and `getIntParam()` near the top of the file (after the cache helpers):

```go
func getParamOrDefault(config types.IndicatorConfig, key string, defaultVal float64) float64
func getIntParam(config types.IndicatorConfig, key string, defaultVal int) int
```

**4b. Add `getHistoricalBarsForIndicator()` method** (Design §7.3b)

Add a shared method on `*Service` that:
1. Defaults symbol to "QQQ" if empty.
2. Enforces `barsNeeded >= 50`.
3. Calls `s.providerManager.GetHistoricalBars(ctx, symbol, "D", nil, nil, barsNeeded)`.
4. Calls `extractOHLC(bars)` from `technical.go`.
5. Returns `(opens, highs, lows, closes []float64, actualSymbol string, err error)`.

**4c. Add 12 new cases to `EvaluateIndicator()` switch** (Design §7.3c)

In the switch statement in `EvaluateIndicator()` (currently lines 217-255), add cases for each new indicator type. Each case:
1. Resolves symbol (config.Symbol or "QQQ").
2. Extracts params via `getIntParam()`/`getParamOrDefault()` with design defaults.
3. Computes `barsNeeded` per the formula in Design §6.15.
4. Calls `s.getHistoricalBarsForIndicator()`.
5. Calls the appropriate `Calc*` function from `technical.go`.
6. Sets `result.ParamSummary` (e.g., `"14"` for RSI, `"12/26/9"` for MACD).
7. Sets `result.Symbol`.

Use the complete mapping table from Design §7.3c for all 12 indicators.

**4d. Add 12 new cases to `formatDetails()`** (Design §7.3d)

In the `formatDetails` method (currently lines 463-498), add formatting for each new type using `result.ParamSummary`. Pattern:

```go
case types.IndicatorRSI:
    return fmt.Sprintf("RSI(%s) %.2f %s %.2f (%s)",
        result.ParamSummary, result.Value, operatorStr, result.Threshold, passStr)
```

For indicators with price-scale output (SMA, EMA, ATR), use `%.2f`. For percentage indicators (Momentum, CMO), use `%.2f%%`. For unbounded (MACD, CCI), use `%.4f`.

### Tests to Run
```bash
cd trade-backend-go && go build ./...
cd trade-backend-go && go vet ./...
cd trade-backend-go && go test ./internal/automation/indicators/... -v -count=1
```

Existing unit tests from Step 3 should still pass. The new switch cases can't be fully integration-tested without a running provider, but `go build` verifies compilation.

### Commit Message
```
feat(service): wire 12 technical indicators into EvaluateIndicator

Add getParamOrDefault/getIntParam helpers, getHistoricalBarsForIndicator
shared method, 12 new switch cases in EvaluateIndicator (RSI, MACD,
Momentum, CMO, Stochastic, StochRSI, ADX, CCI, SMA, EMA, ATR,
Bollinger %B), and 12 new formatDetails cases with param summaries.
```

---

## Step 5: automation.go — GetIndicatorMetadata Handler

### Files to Modify
- `trade-backend-go/internal/api/handlers/automation.go`

### Changes

**5a. Add `GetIndicatorMetadata` method** (Design §5.2)

Add a new handler method to `AutomationHandler`:

```go
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
```

Place this method after `GetFOMCDates()` (line 415) to keep indicator-related handlers grouped together.

**Note:** The `types` import is already present (line 7). No new imports needed.

### Tests to Run
```bash
cd trade-backend-go && go build ./...
cd trade-backend-go && go vet ./...
```

### Commit Message
```
feat(api): add GET /automation/indicators/metadata handler

New GetIndicatorMetadata handler on AutomationHandler that returns
the full indicator metadata registry (17 indicators with params,
descriptions, categories) for frontend dynamic rendering.
```

---

## Step 6: main.go — Route Registration

### Files to Modify
- `trade-backend-go/cmd/server/main.go`

### Changes

**6a. Register the metadata route** (Design §5.3)

In `registerAutomationRoutes()` (line 1696), add the new route **after** the FOMC dates route (line 1715) and **before** the parameterized `/:id` routes (line 1722):

```go
// Indicator metadata (must be before :id routes)
g.GET("/automation/indicators/metadata", h.GetIndicatorMetadata)
```

The placement is critical: Gin matches routes sequentially, so `/automation/indicators/metadata` must be registered before `/automation/:id/...` routes to avoid `indicators` being captured as an `:id` parameter.

**Exact insertion point:** After line 1715 (`g.GET("/automation/fomc-dates", h.GetFOMCDates)`), before line 1717 (`// Tracking endpoints`).

### Tests to Run
```bash
cd trade-backend-go && go build ./cmd/server/...
cd trade-backend-go && go vet ./...
```

Optionally start the server and verify:
```bash
curl http://localhost:8080/api/automation/indicators/metadata | jq '.data.total'
# Expected: 17
```

### Commit Message
```
feat(routes): register GET /automation/indicators/metadata endpoint

Add metadata route in registerAutomationRoutes before parameterized
:id routes to avoid Gin route conflicts. The endpoint serves the
indicator metadata registry for frontend dynamic rendering.
```

---

## Step 7: api.js — Frontend API Method

### Files to Modify
- `trade-app/src/services/api.js`

### Changes

**7a. Add `getIndicatorMetadata()` method** (Design §8.10)

Add a new method in the `// === Automation APIs ===` section (after `previewStrikes` at line 1417):

```javascript
// Get indicator metadata (types, params, descriptions) for dynamic UI rendering
async getIndicatorMetadata() {
  try {
    const response = await apiClient.get(`${API_BASE_URL}/automation/indicators/metadata`);
    return response.data;
  } catch (error) {
    console.error("Error fetching indicator metadata:", error);
    throw error;
  }
},
```

This follows the existing pattern used by `getFomcDates()` and `previewStrikes()`: uses `apiClient` (which includes auth cookies), returns `response.data` (the full API response with `success`, `data`, `message`).

### Tests to Run
```bash
cd trade-app && npm run build
```

Build must succeed. The method can be tested manually via browser console: `api.getIndicatorMetadata().then(r => console.log(r.data.total))` → should return 17.

### Commit Message
```
feat(api): add getIndicatorMetadata method for indicator metadata endpoint

New API method that calls GET /automation/indicators/metadata to
fetch indicator type definitions (params, descriptions, categories)
for the metadata-driven indicator UI.
```

---

## Step 8: AutomationConfigForm.vue — Metadata-Driven UI

### Files to Modify
- `trade-app/src/components/automation/AutomationConfigForm.vue`

### Changes

This is the largest single step. It replaces hardcoded indicator definitions with a metadata-driven approach.

**8a. Add metadata state variables** (Design §8.3)

In the `setup()` function, after the existing state declarations (around line 756), add:

```javascript
// Indicator metadata from API
const indicatorMetadata = ref([])
const indicatorMetadataMap = ref({})
```

**8b. Add `fetchIndicatorMetadata()` function** (Design §8.3)

```javascript
const fetchIndicatorMetadata = async () => {
  try {
    const response = await api.getIndicatorMetadata()
    const indicators = response.data?.indicators || []
    indicatorMetadata.value = indicators
    const map = {}
    indicators.forEach(meta => { map[meta.type] = meta })
    indicatorMetadataMap.value = map
  } catch (err) {
    console.error('Failed to fetch indicator metadata:', err)
    // Fallback to hardcoded types if API fails
  }
}
```

**8c. Update `onMounted()`** (Design §8.3)

Change the `onMounted` hook (line 1128) to fetch metadata first:

```javascript
onMounted(async () => {
  await fetchIndicatorMetadata()
  if (isEditMode.value) {
    await loadConfig()
  }
})
```

**8d. Replace hardcoded `indicatorTypes`** (Design §8.4)

Replace the static `indicatorTypes` array (lines 833-839) with a computed property:

```javascript
const indicatorTypes = computed(() => {
  if (indicatorMetadata.value.length === 0) {
    // Fallback while metadata loads or if API fails
    return [
      { value: 'vix', label: 'VIX', description: 'CBOE Volatility Index', category: 'market' },
      { value: 'gap', label: 'Gap %', description: 'Gap percentage', category: 'market' },
      { value: 'range', label: 'Range %', description: 'Range percentage', category: 'market' },
      { value: 'trend', label: 'Trend %', description: 'Trend percentage', category: 'market' },
      { value: 'calendar', label: 'FOMC Calendar', description: 'FOMC meeting day', category: 'calendar' },
    ]
  }
  return indicatorMetadata.value.map(meta => ({
    value: meta.type,
    label: meta.label,
    description: meta.description,
    category: meta.category,
  }))
})
```

**8e. Add grouped indicator dialog** (Design §8.9)

Add computed property for grouped indicators:

```javascript
const groupedIndicatorTypes = computed(() => {
  const groups = {}
  indicatorTypes.value.forEach(type => {
    const cat = type.category || 'other'
    if (!groups[cat]) groups[cat] = []
    groups[cat].push(type)
  })
  return groups
})

const categoryLabels = {
  market: 'Market',
  calendar: 'Calendar',
  momentum: 'Momentum',
  trend: 'Trend',
  volatility: 'Volatility',
}
```

Update the Add Indicator dialog template (lines 229-247) to render by category:

```html
<div v-for="(types, category) in groupedIndicatorTypes" :key="category">
  <h4 class="category-header">{{ categoryLabels[category] || category }}</h4>
  <div
    v-for="type in types"
    :key="type.value"
    class="indicator-type-option"
    @click="addIndicator(type.value)"
  >
    <div class="option-header">
      <span class="option-label">{{ type.label }}</span>
    </div>
    <p class="option-description">{{ type.description }}</p>
  </div>
</div>
```

**8f. Update `addIndicator()`** (Design §8.7)

Replace the current `addIndicator` function (lines 888-899) to populate default params from metadata:

```javascript
const addIndicator = (type) => {
  const meta = indicatorMetadataMap.value[type]
  const params = {}
  if (meta && meta.params && meta.params.length > 0) {
    meta.params.forEach(p => {
      params[p.key] = p.default_value
    })
  }

  const newIndicator = {
    id: generateIndicatorId(),
    type: type,
    enabled: true,
    operator: 'eq',
    threshold: 0,
    symbol: '',
    params: Object.keys(params).length > 0 ? params : undefined,
  }
  config.value.indicators.push(newIndicator)
  showAddIndicatorDialog.value = false
}
```

**8g. Add metadata-driven helper functions** (Design §8.6)

Add new helpers and update existing ones:

```javascript
const getIndicatorParams = (type) => {
  return indicatorMetadataMap.value[type]?.params || []
}

const getIndicatorNeedsSymbol = (type) => {
  const meta = indicatorMetadataMap.value[type]
  if (!meta) return type !== 'calendar'
  return meta.needs_symbol
}
```

Update `formatIndicatorType()` (line 842) to use metadata:
```javascript
const formatIndicatorType = (type) => {
  return indicatorMetadataMap.value[type]?.label || type
}
```

Update `getIndicatorDescription()` (line 853) to use metadata:
```javascript
const getIndicatorDescription = (type) => {
  return indicatorMetadataMap.value[type]?.description || ''
}
```

Update `getIndicatorDefaultSymbol()` (line 864) to handle new indicators:
```javascript
const getIndicatorDefaultSymbol = (type) => {
  if (type === 'vix') return 'VIX'
  if (type === 'calendar') return ''
  return 'QQQ'
}
```

Add `formatIndicatorTypeWithParams()` (Design §8.8):
```javascript
const formatIndicatorTypeWithParams = (indicator) => {
  const label = formatIndicatorType(indicator.type)
  const params = getIndicatorParams(indicator.type)
  if (params.length === 0 || !indicator.params) return label

  const values = params.map(p => {
    const val = indicator.params[p.key] ?? p.default_value
    return p.type === 'float' ? val.toFixed(1) : Math.round(val)
  })
  return `${label} (${values.join('/')})`
}
```

**8h. Update indicator row template** for dynamic params (Design §8.5)

In the indicator row (lines 126-198), make these changes:

1. Replace `formatIndicatorType(indicator.type)` with `formatIndicatorTypeWithParams(indicator)` in the type label (line 130).

2. Add dynamic parameter inputs in the `.indicator-config` div (after the threshold input, before the symbol input):
```html
<template v-if="getIndicatorParams(indicator.type).length > 0">
  <div
    v-for="paramDef in getIndicatorParams(indicator.type)"
    :key="paramDef.key"
    class="param-input-group"
  >
    <label class="param-label">{{ paramDef.label }}</label>
    <InputNumber
      v-model="indicator.params[paramDef.key]"
      :min="paramDef.min"
      :max="paramDef.max"
      :step="paramDef.step"
      :minFractionDigits="paramDef.type === 'float' ? 1 : 0"
      :maxFractionDigits="paramDef.type === 'float' ? 2 : 0"
      class="param-value-input"
      :disabled="!indicator.enabled"
    />
  </div>
</template>
```

3. Update the symbol `v-if` condition (line 152) to use metadata:
```html
v-if="getIndicatorNeedsSymbol(indicator.type)"
```

**8i. Add CSS for param inputs and category headers** (Design §8.11, §8.12)

Add to the `<style scoped>` section:

```css
.param-input-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.param-label {
  font-size: 0.75rem;
  color: var(--text-tertiary);
  white-space: nowrap;
}

:deep(.param-value-input.p-inputnumber) {
  width: 70px;
}

:deep(.param-value-input .p-inputnumber-input) {
  width: 70px;
  border-radius: var(--radius-sm);
}

.category-header {
  color: var(--color-brand);
  font-size: 0.85rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 12px 0 6px 0;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--border-primary);
}

@media (max-width: 768px) {
  .param-input-group {
    flex-direction: column;
    align-items: flex-start;
  }
  :deep(.param-value-input.p-inputnumber) {
    width: 100%;
  }
}
```

**8j. Export new refs and methods from setup()**

Add to the return statement: `indicatorMetadata`, `indicatorMetadataMap`, `groupedIndicatorTypes`, `categoryLabels`, `getIndicatorParams`, `getIndicatorNeedsSymbol`, `formatIndicatorTypeWithParams`.

### Tests to Run
```bash
cd trade-app && npm run build
```

Build must succeed. Then manual verification:
1. Navigate to Automation Config Form.
2. Click "Add Indicator" — dialog should show all 17 indicators grouped by category.
3. Add RSI — should show a "Period" input with default 14, min 2, max 500.
4. Add MACD — should show Fast Period (12), Slow Period (26), Signal Period (9) inputs.
5. Add existing VIX — should have no parameter inputs (backward compatible).
6. Click "Test All" — all indicators should evaluate and show results.
7. Save config, reload page, verify params persist.

### Commit Message
```
feat(ui): metadata-driven indicator UI with dynamic parameter rendering

Replace hardcoded indicator types with metadata fetched from
GET /automation/indicators/metadata. Add Indicator dialog now
groups by category (Market, Momentum, Trend, Volatility, Calendar).
New indicators render dynamic parameter inputs from metadata
(period, fast/slow/signal periods, std_dev). Parameter values
persist in config.params map. Existing indicators work unchanged.
```

---

## Verification Checklist

After all 8 steps, verify against the acceptance criteria from `docs/requirements.md`:

| AC | Check | How to Verify |
|----|-------|---------------|
| AC-1 | Metadata endpoint returns 17 indicators | `curl /api/automation/indicators/metadata \| jq '.data.total'` → 17 |
| AC-2 | All indicators in Add dialog | Open form, click Add Indicator, count entries |
| AC-3 | Dynamic param rendering | Add RSI, verify period input with defaults |
| AC-4 | Multi-param indicators | Add MACD, verify 3 param fields; add BB%B, verify period + std_dev |
| AC-5 | Calculation correctness | Unit tests pass (`go test ./internal/automation/indicators/...`) |
| AC-6 | Existing configs unaffected | Load an existing automation, verify no changes |
| AC-7 | Test button works | Add RSI on SPY, click Test, verify value + pass/fail |
| AC-8 | Evaluation in automation run | Start automation with new indicators, verify evaluation |
| AC-9 | Error handling | Test indicator with invalid symbol, verify error message |
| AC-10 | Default params | Add indicator, verify defaults pre-filled; remove params from JSON, verify backend uses defaults |
