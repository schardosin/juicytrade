# Bug Analysis: Duplicate Candle Creation on Chart

## Issue Summary

When the Chart View opens and displays daily candles, the application loads historical data for the current day correctly (showing OHLC values). However, when live market data arrives, **the chart creates a duplicate candle for the same day** instead of updating the existing daily candle with new close prices.

**Symptom:** A new candle appears on the right side of the existing candle for the current trading day, rather than updating the existing one.

## Root Cause Analysis

### Location
**File:** `/root/juicytrade/trade-app/src/components/LightweightChart.vue`  
**Function:** `updateRealTimeData()` (lines 454–584)  
**Critical Code:** Lines 559–567

### The Problem

The component maintains a **single `currentCandle` variable** that tracks the candle currently being updated:

```javascript
let currentCandle = null; // Line 96
```

The `updateRealTimeData()` function implements this logic (lines 559–567):

```javascript
// Check if this is a new candle or updating existing one
if (!currentCandle || currentCandle.time !== timeInSeconds) {
  // New candle - create it with current price as OHLC
  currentCandle = {
    time: timeInSeconds,
    open: newPrice,
    high: newPrice,
    low: newPrice,
    close: newPrice,
  };
} else {
  // Update existing candle
  // ...
}

// Update the chart with the current candle
candlestickSeries.update(currentCandle);
```

### Why This Creates a Duplicate

**When the Chart View first loads:**
1. Historical data is loaded via `loadHistoricalData()` and set with `candlestickSeries.setData(candlestickData)` (line 415)
2. This includes the **current day's candle with OHLC values from historical data**
3. However, `currentCandle = null` (not tracking what was loaded from history)

**When the first live price update arrives:**
1. `updateRealTimeData()` is called with live price data
2. `currentCandle` is still `null` (because it was never set during historical load)
3. The condition `!currentCandle` is `true` → a **new candle is created**
4. `candlestickSeries.update(currentCandle)` is called with this new candle object
5. **Lightweight Charts treats `.update()` as adding a new candle** (not merging with history)
6. Result: **Two candles for the same time** appear on the chart—one from history, one from live updates

### Why the Logic Fails

The component assumes:
- After historical load, `currentCandle` should reflect the last loaded bar
- When a live price arrives with the same time, it should match and update the existing candle

But it doesn't:
- **The historical load never sets `currentCandle`** to the current day's bar
- The first live update sees `currentCandle = null` and creates a *new* candle
- This new candle is treated as a separate entity by Lightweight Charts

## Why It Happens Immediately

The customer reported: *"Open the chart, and boom, problem is there with the default timeframe."*

**Timeline:**
1. Chart loads → historical data is fetched and rendered (including today's candle)
2. Live stream immediately connects → first price tick arrives almost instantly
3. `currentCandle` is still `null` → duplicate candle is created immediately

This happens so fast that it appears to happen on initial load.

## Affected Scenarios

- **All timeframes** (1m, 5m, 15m, 1h, 4h, D, W, M) because the logic is timeframe-independent
- **Current trading sessions** (the `currentCandle` issue only manifests for the current bar)
- **All symbols** (the bug is in the component logic, not broker-specific)

## The Fix

To fix this bug, we need to **synchronize `currentCandle` with the historical data** after it loads:

1. **After loading historical data**, set `currentCandle` to the last candle (current day/period)
2. **When live updates arrive**, the component will correctly identify that the time matches and update the existing candle
3. **No duplicate candle** will be created

### Implementation approach:
- In `loadHistoricalData()`, after setting the candlestick data, extract the last candle from `candlestickData`
- Store it in `currentCandle` so subsequent live updates will merge correctly
- Reset `currentCandle` when the user changes timeframes or symbols (already done on lines 437, 442, 602)

