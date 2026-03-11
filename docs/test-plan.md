# Test Plan: Add Additional Indicators to Auto Trade

**GitHub Issue:** #4
**Status:** Ready for Execution
**Covers:** 11 verification areas across backend, frontend, and integration

---

## 1. Backend Build and Vet Verification

**Goal:** Confirm the Go backend compiles and passes static analysis with all new code.

**Files to examine:**
- `trade-backend-go/internal/automation/types/types.go`
- `trade-backend-go/internal/automation/indicators/technical.go`
- `trade-backend-go/internal/automation/indicators/service.go`
- `trade-backend-go/internal/api/handlers/automation.go`
- `trade-backend-go/cmd/server/main.go`

**Checks:**
1. Run `go build ./...` from `trade-backend-go/` — must exit 0 with no errors.
2. Run `go vet ./...` from `trade-backend-go/` — must exit 0 with no warnings.
3. Verify no import cycles introduced (types.go must not import from indicators or service packages).
4. Verify `technical.go` only imports standard library packages (`errors`, `fmt`, `math`).

---

## 2. Existing Unit Tests Verification (55 tests)

**Goal:** All existing and new unit tests pass.

**Files to examine:**
- `trade-backend-go/internal/automation/indicators/technical_test.go`

**Checks:**
1. Run `go test ./internal/automation/indicators/ -v -count=1` from `trade-backend-go/`.
2. Verify all 55 tests pass (0 failures, 0 skips).
3. Verify test output includes tests for all 12 indicator calculation functions plus helpers:
   - `TestExtractOHLC` (2 tests)
   - `TestCalcSMA` (5 tests: Known, AllSame, ExactFit, InsufficientData, ZeroPeriod)
   - `TestCalcEMA` (3 tests: ConstantPrices, KnownCalculation, InsufficientData)
   - `TestCalcRSI` (5 tests: AllRising, AllFalling, AllSame, WilderReference, InsufficientData)
   - `TestCalcMACD` (4 tests: RisingPrices, FallingPrices, InsufficientData, ConstantPrices)
   - `TestCalcMomentum` (5 tests: Positive, Negative, LongerPeriod, InsufficientData, ZeroPrevious)
   - `TestCalcCMO` (4 tests: MonotonicRise, MonotonicFall, NoMovement, InsufficientData)
   - `TestCalcStochastic` (5 tests: CloseAtHigh, CloseAtLow, CloseAtMid, NoRange, InsufficientData)
   - `TestCalcStochRSI` (3 tests: InRange, StrongUptrend, InsufficientData)
   - `TestCalcADX` (4 tests: StrongTrend, FlatMarket, InsufficientData, UnequalLengths)
   - `TestCalcCCI` (3 tests: PriceAtSMA, AboveSMA, InsufficientData)
   - `TestCalcATR` (4 tests: ConstantRange, WithGaps, InsufficientData, WilderSmoothing)
   - `TestCalcBollingerPercentB` (5 tests: AtSMA, AtUpper, AtLower, ExactBands, InsufficientData)
   - `TestCalcEMASeries` (2 tests: Nil, Length)
   - `TestCalcRSISeries` (3 tests: Nil, Length, ConsistentWithCalcRSI)
4. Run `go test ./internal/automation/indicators/ -race` to verify no data races in shared helpers.

---

## 3. Backward Compatibility of Original 5 Indicators

**Goal:** Existing VIX, Gap%, Range%, Trend%, and FOMC Calendar indicators continue to work without changes.

**Files to examine:**
- `trade-backend-go/internal/automation/types/types.go` (lines 13-18 — original constants)
- `trade-backend-go/internal/automation/indicators/service.go` (lines 233-268 — original switch cases)
- `trade-backend-go/internal/automation/indicators/service.go` (lines 721-734 — original formatDetails)
- `trade-backend-go/internal/api/handlers/automation.go` (lines 82-110 — CreateConfig default indicators)

**Checks:**
1. Verify the 5 original `IndicatorType` constants are unchanged:
   - `IndicatorVIX = "vix"`
   - `IndicatorGap = "gap"`
   - `IndicatorRange = "range"`
   - `IndicatorTrend = "trend"`
   - `IndicatorCalendar = "calendar"`
2. Verify the `IndicatorConfig` struct's `Params` field uses `omitempty` JSON tag (line 107) so existing configs with no `params` field deserialize correctly (Params remains `nil`).
3. Verify the 5 original switch cases in `EvaluateIndicator()` are unchanged (lines 234-268).
4. Verify the 5 original formatDetails cases are unchanged (lines 722-734).
5. Verify `CreateConfig` handler still creates default 5 indicators when none provided (lines 102-110).
6. Verify `NewIndicatorConfig()` with an existing type (e.g., `IndicatorVIX`) returns a config with `Params == nil` since VIX has no params defined in metadata (empty `[]IndicatorParamDef{}`).
7. JSON round-trip test: Serialize an `IndicatorConfig{Type: "vix", Enabled: true, Operator: "lt", Threshold: 30}` (no Params field) and deserialize it back — `Params` should remain `nil`.

---

## 4. Metadata Registry Completeness and Correctness (17 entries)

**Goal:** `GetIndicatorMetadata()` returns exactly 17 entries with correct params, categories, defaults, and structure.

**Files to examine:**
- `trade-backend-go/internal/automation/types/types.go` (lines 294-430 — `GetIndicatorMetadata()`)

**Checks:**
1. Verify `GetIndicatorMetadata()` returns exactly 17 entries (`len(result) == 17`).
2. Verify each entry has all required fields populated: `Type`, `Label`, `Description`, `Category`, `Params` (non-nil, even if empty), `ValueRange`, `NeedsSymbol`.
3. Verify entry order matches design (existing first, then momentum, trend, volatility):
   - [0] vix, [1] gap, [2] range, [3] trend, [4] calendar
   - [5] rsi, [6] macd, [7] momentum, [8] cmo, [9] stoch, [10] stoch_rsi
   - [11] adx, [12] cci, [13] sma, [14] ema
   - [15] atr, [16] bb_percent
4. Verify categories:
   - `market`: vix, gap, range, trend
   - `calendar`: calendar
   - `momentum`: rsi, macd, momentum, cmo, stoch, stoch_rsi
   - `trend`: adx, cci, sma, ema
   - `volatility`: atr, bb_percent
5. Verify `NeedsSymbol`:
   - `false` for: vix, calendar
   - `true` for: all other 15 indicators
6. Verify existing indicators have empty params (`len(Params) == 0`): vix, gap, range, trend, calendar.
7. Verify param defaults match requirements:
   - RSI: `period=14`
   - MACD: `fast_period=12, slow_period=26, signal_period=9`
   - Momentum: `period=10`
   - CMO: `period=14`
   - Stochastic: `k_period=14, d_period=3`
   - StochRSI: `rsi_period=14, stoch_period=14, k_period=3, d_period=3`
   - ADX: `period=14`
   - CCI: `period=20`
   - SMA: `period=20`
   - EMA: `period=20`
   - ATR: `period=14`
   - Bollinger %B: `period=20, std_dev=2.0`
8. Verify `std_dev` param (Bollinger %B) has `Type: "float"`, `Step: 0.1`, `Min: 0.5`, `Max: 5.0`.
9. Verify all integer params have `Type: "int"`, `Step: 1`.
10. Verify all period params have `Min >= 1` (or `Min >= 2` where specified) and `Max: 500`.

---

## 5. Route Ordering (Metadata Endpoint Before /:id Routes)

**Goal:** The metadata endpoint is registered before parameterized `:id` routes to prevent Gin from treating `indicators` as an `:id` value.

**Files to examine:**
- `trade-backend-go/cmd/server/main.go` (lines 1696-1733 — `registerAutomationRoutes()`)

**Checks:**
1. Verify `g.GET("/automation/indicators/metadata", h.GetIndicatorMetadata)` is registered at line 1718.
2. Verify this line appears BEFORE all parameterized routes (`:id` routes starting at line 1725):
   - `/automation/:id/start`
   - `/automation/:id/stop`
   - `/automation/:id/status`
   - etc.
3. Verify it also appears after the non-parameterized routes (configs CRUD at lines 1698-1703) but before the comment "Control (parameterized routes)" at line 1724.
4. Verify other static routes are also before `:id` routes: `/automation/status`, `/automation/evaluate`, `/automation/preview-strikes`, `/automation/fomc-dates`, `/automation/tracking/*`.
5. Manually reason about route conflict: GET `/automation/indicators/metadata` should NOT match `/automation/configs/:id` since it uses a different path prefix (`indicators/` vs `configs/`). Confirm Gin does not confuse them.

---

## 6. Technical Calculation Correctness

**Goal:** Each indicator's pure calculation function produces mathematically correct results for known inputs, handles edge cases gracefully, and rejects insufficient data.

**Files to examine:**
- `trade-backend-go/internal/automation/indicators/technical.go` (all functions)
- `trade-backend-go/internal/automation/indicators/technical_test.go` (all tests)

### 6.1 CalcSMA
1. `[1,2,3,4,5]` with period=3 returns `4.0` (average of last 3: 3+4+5).
2. `[10,20,30]` with period=3 returns `20.0`.
3. `[5,10,15]` with period=3 returns `10.0` (exact fit).
4. Period > data length returns error with "insufficient data" message.
5. Period=0 returns error with "period must be positive" message.

### 6.2 CalcEMA
1. Constant prices (all 50.0, 20 values) with period=10 returns `50.0` (converges to constant).
2. `[10,20,30,40,50]` with period=3: seed SMA=20.0, multiplier=0.5, after 40: 30.0, after 50: 40.0.
3. Period > data length returns error.

### 6.3 CalcRSI (Wilder's Smoothing)
1. Monotonically rising (30 values) with period=14: RSI > 95.
2. Monotonically falling (30 values) with period=14: RSI < 5.
3. All-same prices: RSI = 50 (edge case: avgLoss=0, avgGain=0).
4. Wilder's reference data (15 closes from the textbook): RSI within 0.5 of 72.98.
5. Insufficient data (3 closes, period=14): returns error.

### 6.4 CalcMACD
1. Rising prices (60 values): MACD > 0 (fast EMA > slow EMA).
2. Falling prices (60 values): MACD < 0 (fast EMA < slow EMA).
3. Constant prices (60 values): MACD within 0.001 of 0.
4. Insufficient data (3 closes, 12/26/9): returns error.
5. Alignment logic: fastEMA and slowEMA arrays properly offset.

### 6.5 CalcMomentum
1. `[100, 105]` period=1: returns `5.0` (5% gain).
2. `[100, 95]` period=1: returns `-5.0` (-5% loss).
3. `[100, 110, 120, 130, 150]` period=3: `(150-110)/110*100 = 36.3636`.
4. Insufficient data (1 close, period=1): returns error.
5. Zero previous price `[0, 100]` period=1: returns error "zero division".

### 6.6 CalcCMO
1. Monotonic rise (16 values, period=14): CMO = +100.
2. Monotonic fall (16 values, period=14): CMO = -100.
3. No movement (constant 50, 16 values, period=14): CMO = 0.
4. Insufficient data: returns error.

### 6.7 CalcStochastic
1. Close at highest high in window: %K = 100.
2. Close at lowest low in window: %K = 0.
3. Close at midpoint of range: %K = 50.
4. No range (highestHigh == lowestLow): returns 50 (neutral).
5. Unequal-length slices: returns error.
6. Insufficient data: returns error.

### 6.8 CalcStochRSI
1. Result is within 0-100 range for varied input data.
2. Strong consistent uptrend: RSI values converge, so highestRSI == lowestRSI, returns 50.
3. Insufficient data: returns error.

### 6.9 CalcADX
1. Strong uptrend (50 bars with consistent higher highs/lows): ADX > 25.
2. Flat/sideways market (50 bars alternating): ADX < 25.
3. Insufficient data: returns error.
4. Unequal-length slices: returns error.
5. Wilder's smoothing applied correctly to TR, +DM, -DM, then DX.

### 6.10 CalcCCI
1. All identical bars (H=L=C=100): TP at SMA, meanDev=0, returns 0.
2. Most bars at 100, last bar at 120: CCI > 0 (price above mean).
3. Insufficient data: returns error.
4. Lambert's constant 0.015 is used in the formula.

### 6.11 CalcATR
1. Constant range bars (H=110, L=100, C=105, 20 bars): ATR = 10.
2. Bars with gaps (previous close outside next bar's range): TR > H-L.
3. Wilder's smoothing manual check: 5 bars, period=2, computed step by step = 12.5.
4. Insufficient data: returns error.

### 6.12 CalcBollingerPercentB
1. All-same prices (constant 100, 20 values): stddev=0, bandwidth=0, returns 0.5.
2. `[100*19, 120]`: %B > 1.0 (price above upper band).
3. `[100*19, 80]`: %B < 0.0 (price below lower band).
4. `[98,99,100,101,102]` period=5, stddev=2: %B within 0.001 of 0.8536.
5. Insufficient data: returns error.

### 6.13 Internal Helpers
1. `calcEMASeries`: returns nil when len(data) < period; returns correct length (len-period+1).
2. `calcRSISeries`: returns nil when len(closes) < period+1; returns correct length (len-period).
3. `calcRSISeries` last value matches `CalcRSI` for the same input.

---

## 7. Service Integration

**Goal:** The service layer correctly wires calculation functions to the evaluation flow: switch cases, param extraction, historical bar fetching, formatDetails, and ParamSummary.

**Files to examine:**
- `trade-backend-go/internal/automation/indicators/service.go` (lines 133-147 — helpers; 270-471 — new switch cases; 644-661 — getHistoricalBarsForIndicator; 703-768 — formatDetails)

### 7.1 Helper Functions
1. `getParamOrDefault` with nil Params map returns default value.
2. `getParamOrDefault` with Params map containing the key returns the stored value.
3. `getParamOrDefault` with Params map missing the key returns default value.
4. `getIntParam` returns `int(float64)` conversion correctly.

### 7.2 Switch Cases (12 new cases)
For each new indicator type, verify in `EvaluateIndicator()`:

| # | Type | Default Symbol | Param Keys & Defaults | barsNeeded Formula | Calc Function Called | ParamSummary Format |
|---|------|---------------|----------------------|-------------------|---------------------|-------------------|
| 1 | `rsi` | QQQ | `period=14` | `period*3` | `CalcRSI(closes, period)` | `"%d"` |
| 2 | `macd` | QQQ | `fast_period=12, slow_period=26, signal_period=9` | `slow+signal+slow` | `CalcMACD(closes, fast, slow, signal)` | `"%d/%d/%d"` |
| 3 | `momentum` | QQQ | `period=10` | `period+1` | `CalcMomentum(closes, period)` | `"%d"` |
| 4 | `cmo` | QQQ | `period=14` | `period+1` | `CalcCMO(closes, period)` | `"%d"` |
| 5 | `stoch` | QQQ | `k_period=14, d_period=3` | `kPeriod+dPeriod` | `CalcStochastic(highs, lows, closes, k, d)` | `"%d/%d"` |
| 6 | `stoch_rsi` | QQQ | `rsi_period=14, stoch_period=14, k_period=3, d_period=3` | `rsiP*3+stochP` | `CalcStochRSI(closes, rsiP, stochP, kP, dP)` | `"%d/%d/%d/%d"` |
| 7 | `adx` | QQQ | `period=14` | `period*3` | `CalcADX(highs, lows, closes, period)` | `"%d"` |
| 8 | `cci` | QQQ | `period=20` | `period` | `CalcCCI(highs, lows, closes, period)` | `"%d"` |
| 9 | `sma` | QQQ | `period=20` | `period` | `CalcSMA(closes, period)` | `"%d"` |
| 10 | `ema` | QQQ | `period=20` | `period*2` | `CalcEMA(closes, period)` | `"%d"` |
| 11 | `atr` | QQQ | `period=14` | `period*2` | `CalcATR(highs, lows, closes, period)` | `"%d"` |
| 12 | `bb_percent` | QQQ | `period=20, std_dev=2.0` | `period` | `CalcBollingerPercentB(closes, period, stdDev)` | `"%d/%.1f"` |

For each case verify:
1. Empty symbol defaults to `"QQQ"`.
2. `getIntParam` / `getParamOrDefault` called with correct key and default.
3. `barsNeeded` calculation matches the formula above.
4. `getHistoricalBarsForIndicator` called with correct symbol and barsNeeded.
5. Correct OHLC arrays passed to the calculation function (closes only vs highs+lows+closes).
6. `result.Symbol` set to `actualSymbol`.
7. `result.ParamSummary` set with correct format string.

### 7.3 getHistoricalBarsForIndicator
1. Empty symbol defaults to `"QQQ"` (line 648-649).
2. `barsNeeded < 50` is clamped to 50 (line 650-652).
3. Calls `s.providerManager.GetHistoricalBars(ctx, symbol, "D", nil, nil, barsNeeded)`.
4. Passes bar data through `extractOHLC()` to get typed OHLC slices.
5. Returns error properly when provider call fails.

### 7.4 formatDetails (12 new cases)
Verify each new case in `formatDetails()` (lines 737-764):
1. Each type has a corresponding case (no fall-through to default).
2. Format string includes `result.ParamSummary`.
3. RSI/CMO/Stoch/StochRSI/ADX/CCI/SMA/EMA/ATR use `%.2f`.
4. MACD uses `%.4f` (small values).
5. Momentum includes `%%` for percentage display.
6. BB%B uses `%.4f` and escapes `%%B` correctly (`BB%%B`).
7. All cases include operator string and pass/fail string.

### 7.5 ParamSummary Field
1. `IndicatorResult.ParamSummary` field exists with `json:"param_summary,omitempty"` tag (types.go line 124).
2. Single-param indicators produce summary like `"14"`.
3. MACD produces summary like `"12/26/9"`.
4. Stochastic produces `"14/3"`.
5. StochRSI produces `"14/14/3/3"`.
6. BB%B produces `"20/2.0"`.

### 7.6 Error Handling and Caching
1. On fetch error: `result.Error` is set, `result.Stale = true`, `result.Pass = false`.
2. On calculation error (e.g., insufficient data): same error handling applies.
3. Cached value fallback works for new indicators when `configID` is non-empty.
4. Preview mode (empty `configID`): no caching, errors logged at Error level.

---

## 8. API Handler for Metadata Endpoint

**Goal:** `GET /api/automation/indicators/metadata` returns correct JSON response.

**Files to examine:**
- `trade-backend-go/internal/api/handlers/automation.go` (lines 418-428 — `GetIndicatorMetadata`)

**Checks:**
1. Handler calls `types.GetIndicatorMetadata()` and returns 200 OK.
2. Response JSON structure matches design spec:
   ```json
   {
     "success": true,
     "data": {
       "indicators": [...],
       "total": 17
     },
     "message": "Retrieved indicator metadata"
   }
   ```
3. `data.total` equals `len(data.indicators)` equals 17.
4. Each indicator object in the array has JSON keys: `type`, `label`, `description`, `category`, `params`, `value_range`, `needs_symbol`.
5. The `params` array for each indicator has objects with keys: `key`, `label`, `default_value`, `min`, `max`, `step`, `type`.
6. Params array is empty `[]` (not `null`) for existing indicators (VIX, Gap, Range, Trend, Calendar).
7. No authentication beyond what is already applied via middleware (the route is registered after `api.Use(auth.AuthenticationMiddleware)`).

---

## 9. Frontend Build Verification

**Goal:** The Vue frontend builds without errors.

**Files to examine:**
- `trade-app/src/services/api.js` (lines 1409-1417 — `getIndicatorMetadata`)
- `trade-app/src/components/automation/AutomationConfigForm.vue`

**Checks:**
1. Run `npm run build` (or equivalent) from `trade-app/` — must exit 0 with no errors.
2. No TypeScript/ESLint errors related to the new code.
3. No unresolved imports.
4. Build output size is reasonable (no accidental large bundle inclusion).

---

## 10. Frontend Code Review

**Goal:** Verify the metadata-driven UI pattern is correctly implemented: dynamic params, grouped dialog, and formatIndicatorTypeWithParams.

**Files to examine:**
- `trade-app/src/components/automation/AutomationConfigForm.vue` (full file)
- `trade-app/src/services/api.js` (lines 1409-1417)

### 10.1 Metadata Fetching
1. `fetchIndicatorMetadata()` calls `api.getIndicatorMetadata()` (line 860).
2. Response is parsed: `response.data?.indicators || []` (line 861).
3. `indicatorMetadataMap` is built as `{ [type]: meta }` for O(1) lookup (lines 863-865).
4. `fetchIndicatorMetadata()` is called in `onMounted` before `loadConfig` (lines 1210-1213).
5. Fallback: if API fails, `indicatorTypes` computed returns hardcoded 5 original types (lines 874-882).

### 10.2 API Service Method
1. `getIndicatorMetadata()` method exists in `api.js` (lines 1409-1417).
2. It calls `apiClient.get(\`${API_BASE_URL}/automation/indicators/metadata\`)`.
3. Returns `response.data` (the full response body).
4. Error handling: catches and rethrows.

### 10.3 Dynamic Parameter Rendering
1. In the template, `v-if="getIndicatorParams(indicator.type).length > 0 && indicator.params"` gates param rendering (line 152).
2. `v-for="paramDef in getIndicatorParams(indicator.type)"` loops over metadata params (line 154).
3. `InputNumber` is bound to `indicator.params[paramDef.key]` (line 160).
4. `min`, `max`, `step` from metadata are passed to InputNumber (lines 161-163).
5. Float params use `minFractionDigits=1, maxFractionDigits=2` (lines 164-165).
6. Integer params use `minFractionDigits=0, maxFractionDigits=0`.
7. Param inputs are disabled when `!indicator.enabled` (line 168).

### 10.4 addIndicator Method
1. Looks up metadata: `indicatorMetadataMap.value[type]` (line 961).
2. Populates default params from `meta.params` (lines 963-966).
3. Creates indicator with `params: Object.keys(params).length > 0 ? params : undefined` (line 976).
4. Generates a unique frontend ID via `generateIndicatorId()` (line 970).
5. Closes the dialog after adding (line 979).

### 10.5 Grouped Add Indicator Dialog
1. `groupedIndicatorTypes` computed groups `indicatorTypes` by `category` (lines 892-900).
2. `categoryLabels` maps category keys to display names: market, calendar, momentum, trend, volatility (lines 902-908).
3. Template iterates `v-for="(types, category) in groupedIndicatorTypes"` (line 253).
4. Each category has a `<h4>` header with category label (line 254).
5. Each type option shows `label` and `description` (lines 262-264).

### 10.6 formatIndicatorTypeWithParams
1. Calls `formatIndicatorType(indicator.type)` to get base label (line 936).
2. Gets param definitions from metadata (line 937).
3. For indicators with params, builds compact summary: `label (val1/val2/...)` (lines 940-944).
4. Float params formatted with `.toFixed(1)`, integer params with `Math.round()` (line 942).
5. Returns plain label for indicators without params (line 938).
6. Used in the template at line 130: `{{ formatIndicatorTypeWithParams(indicator) }}`.

### 10.7 Helper Methods from Metadata
1. `getIndicatorParams(type)` returns `indicatorMetadataMap.value[type]?.params || []` (line 926).
2. `getIndicatorNeedsSymbol(type)` checks `meta.needs_symbol`, falls back to `type !== 'calendar'` (lines 929-933).
3. `formatIndicatorType(type)` returns `meta.label || type` (line 912).
4. `getIndicatorDescription(type)` returns `meta.description || ''` (line 916).
5. `getIndicatorDefaultSymbol(type)` returns VIX for vix, empty for calendar, QQQ for others (lines 919-923).

### 10.8 Symbol Input Visibility
1. Symbol input shown conditionally: `v-if="getIndicatorNeedsSymbol(indicator.type)"` (line 172).
2. For VIX (`needs_symbol: false`): no symbol input shown.
3. For Calendar (`needs_symbol: false`): no symbol input shown.
4. For all 12 new indicators (`needs_symbol: true`): symbol input shown.
5. Placeholder set to `getIndicatorDefaultSymbol(indicator.type)` (line 174).

---

## 11. Additional Edge-Case and Integration Tests to Write

**Goal:** Identify gaps in the current test suite and specify additional tests that should be written.

### 11.1 Missing Unit Tests for technical.go

**extractOHLC type coercion:**
1. Test with missing keys in bar map (should return 0.0 for missing fields).
2. Test with `nil` values in bar map.
3. Test with string values for numeric fields (should return 0.0).

**CalcMACD alignment:**
1. Test where `fastPeriod > slowPeriod` (unusual but valid) — verify offset logic handles negative case.
2. Test with `fastPeriod == slowPeriod` — MACD should be 0.

**CalcStochastic window behavior:**
1. Test with `kPeriod=1` — should return relative position within single bar.
2. Test with `dPeriod > 1` and verify only `kPeriod` window is used for raw %K.

**CalcBollingerPercentB:**
1. Test with `stdDev=0` — verify behavior (bands collapse to SMA).
2. Test with `stdDev=1` — different band width than default.

**CalcStochRSI edge cases:**
1. Test with all-same prices — RSI series all 50, stochastic returns 50.
2. Test with alternating up/down prices — verify RSI values vary and stochastic works.

### 11.2 Missing Service Integration Tests

**getParamOrDefault / getIntParam:**
1. Write unit tests for `getParamOrDefault` with nil Params, missing key, and present key.
2. Write unit tests for `getIntParam` to verify float-to-int truncation.

**getHistoricalBarsForIndicator:**
1. Mock `providerManager.GetHistoricalBars` and verify:
   - Called with `"D"` timeframe.
   - Called with minimum 50 bars.
   - Empty symbol replaced with `"QQQ"`.
   - Error propagated correctly.

**EvaluateIndicator integration (with mocked provider):**
1. Test RSI case: mock bars, verify CalcRSI called with correct closes, verify result has correct ParamSummary.
2. Test MACD case: verify 3 params extracted correctly, barsNeeded formula correct.
3. Test Stochastic case: verify highs, lows, closes all passed (not just closes).
4. Test BB%B case: verify `getParamOrDefault` used for `std_dev` (float param, not `getIntParam`).
5. Test with custom params (non-default): e.g., RSI with `{period: 21}` — verify `period=21` used.
6. Test with nil Params (backward compat): e.g., RSI with `Params: nil` — verify default `period=14` used.

**formatDetails integration:**
1. Test each new indicator type produces expected string format.
2. Test with empty `ParamSummary` — verify graceful handling (no panic).

### 11.3 Missing API Endpoint Tests

**GET /api/automation/indicators/metadata:**
1. Write an HTTP handler test that:
   - Sends a GET request to the endpoint.
   - Verifies 200 status code.
   - Verifies `success: true`.
   - Verifies `data.total == 17`.
   - Verifies each indicator has required fields.
   - Verifies JSON field names use snake_case (`value_range`, `needs_symbol`, `default_value`).

**POST /api/automation/evaluate (preview) with new indicators:**
1. Test evaluating RSI with `params: {period: 14}` and a valid symbol — mock provider response.
2. Test evaluating MACD with default params — verify 3 params extracted.
3. Test evaluating indicator with empty `params` — verify defaults used.

### 11.4 Missing Frontend Tests

**Metadata-driven UI:**
1. Test that `indicatorTypes` computed property returns 17 items when metadata loads.
2. Test fallback: when metadata API fails, returns 5 hardcoded items.
3. Test `formatIndicatorTypeWithParams` with various indicator/param combinations:
   - VIX (no params): returns `"VIX"`.
   - RSI (period=14): returns `"RSI (14)"`.
   - MACD (12/26/9): returns `"MACD (12/26/9)"`.
   - BB%B (20/2.0): returns `"Bollinger %B (20/2.0)"`.
4. Test `addIndicator` populates default params from metadata.
5. Test `addIndicator` for existing type (e.g., "vix") creates indicator with `params: undefined`.

### 11.5 End-to-End Integration Scenarios

**Scenario 1: Create automation with new indicator**
1. Fetch metadata endpoint — verify 17 indicators returned.
2. Create a config with RSI < 70 on SPY, period=14.
3. Save config via POST `/api/automation/configs`.
4. Load config via GET `/api/automation/configs/:id`.
5. Verify `indicators[0].params.period == 14`.
6. Evaluate via POST `/api/automation/evaluate` — verify RSI value returned.

**Scenario 2: Backward compatibility with existing config**
1. Load an existing config that has VIX, Gap, Range, Trend, Calendar (no `params` fields).
2. Verify all indicators render correctly in the UI.
3. Test All — verify all 5 indicators evaluate without errors.
4. Save and reload — verify no data loss.

**Scenario 3: Multi-parameter indicator**
1. Add MACD with custom params: `fast_period=8, slow_period=21, signal_period=5`.
2. Save and reload — verify all 3 params persist.
3. Test — verify MACD evaluates with custom params.
4. Verify display shows `"MACD (8/21/5)"`.

**Scenario 4: Mixed old + new indicators**
1. Create config with VIX + RSI + MACD + Calendar.
2. Test All — verify all 4 evaluate (2 existing + 2 new).
3. Verify `all_pass` logic works correctly (existing AllIndicatorsPass unchanged).

**Scenario 5: Error handling**
1. Configure RSI with a non-existent symbol (e.g., "ZZZZZ").
2. Test — verify error message returned, not a panic.
3. Verify stale/cached value logic works for new indicators.

---

## Test Execution Checklist

| # | Area | Command / Action | Expected Result | Pass? |
|---|------|-----------------|-----------------|-------|
| 1 | Backend build | `cd trade-backend-go && go build ./...` | Exit 0 | |
| 2 | Backend vet | `cd trade-backend-go && go vet ./...` | Exit 0 | |
| 3 | Unit tests | `cd trade-backend-go && go test ./internal/automation/indicators/ -v -count=1` | 55/55 pass | |
| 4 | Race detector | `cd trade-backend-go && go test ./internal/automation/indicators/ -race` | No races | |
| 5 | Metadata count | Call `GetIndicatorMetadata()` — len == 17 | 17 entries | |
| 6 | Route order | Inspect `registerAutomationRoutes()` | Metadata before `:id` | |
| 7 | Frontend build | `cd trade-app && npm run build` | Exit 0 | |
| 8 | API metadata | `curl GET /api/automation/indicators/metadata` | 200, total=17 | |
| 9 | Backward compat | Deserialize old config JSON (no params) | No errors | |
| 10 | New indicator eval | Test RSI/MACD/ADX via preview endpoint | Values returned | |
| 11 | UI metadata fetch | Open AutomationConfigForm, check network | Metadata loaded | |
| 12 | UI add indicator | Click Add Indicator, see 17 options grouped | All types shown | |
| 13 | UI param rendering | Add RSI — verify period input shown | Default 14 | |
| 14 | UI multi-param | Add MACD — verify 3 param inputs | Defaults 12/26/9 | |
| 15 | UI display format | Check indicator row label | "RSI (14)" format | |
