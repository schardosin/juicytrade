# Requirements: Change Price Chart Range for Weekly and Monthly

**Issue:** [#38 — Change Price Chart Range for Weekly](https://github.com/schardosin/juicytrade/issues/38)
**Status:** Draft — Awaiting Customer Approval

---

## Description

The Price Chart in the right-side panel's Overview tab offers three timeframe filters: **1D** (daily), **1W** (weekly), and **1M** (monthly). Two changes are requested:

1. **1W (Weekly):** Currently fetches **1 year** of weekly data — the same date range as the **1D** filter. This makes the weekly chart appear as a contracted version of the daily chart with no additional historical context. The request is to extend it to **2 years**.

2. **1M (Monthly):** Currently fetches **2 years** of monthly data. The request is to extend it to **5 years**, providing a much broader historical perspective for the longest timeframe.

## Motivation

- When switching from 1D to 1W, users expect to see a broader historical view — that's the purpose of a longer timeframe. With both covering 1 year, the weekly chart provides no additional information.
- The monthly chart at 2 years only covers 24 data points, which is limited context for monthly analysis. Extending to 5 years (60 data points) gives users meaningful long-term trend visibility.
- After these changes, the timeframes follow a clear progression: **1D → 1 year**, **1W → 2 years**, **1M → 5 years** — each longer timeframe reveals more history.

## Functional Requirements

### FR-1: Extend 1W date range to 2 years
- The `calculateDateRange` function in `RightPanelChart.vue` must set the start date for the `1W` range to **2 years before the current date** (currently 1 year).

### FR-2: Increase 1W data point limit to 104
- The `loadChartData` function in `RightPanelChart.vue` must request **104** weekly data points (52 weeks × 2 years) instead of the current **52**.

### FR-3: Extend 1M date range to 5 years
- The `calculateDateRange` function in `RightPanelChart.vue` must set the start date for the `1M` range to **5 years before the current date** (currently 2 years).

### FR-4: Increase 1M data point limit to 60
- The `loadChartData` function in `RightPanelChart.vue` must request **60** monthly data points (12 months × 5 years) instead of the current **24**.

### FR-5: No changes to 1D timeframe
- The **1D** filter must remain at 1 year / 365 data points.

## Affected File

- `trade-app/src/components/RightPanelChart.vue` — Four changes in the `setup()` function:
  1. **`calculateDateRange`, `case "1W"`:** Change `now.getFullYear() - 1` → `now.getFullYear() - 2`
  2. **`calculateDateRange`, `case "1M"`:** Change `now.getFullYear() - 2` → `now.getFullYear() - 5`
  3. **`loadChartData`, `limit` calculation:** Change the `1W` limit from `52` → `104`
  4. **`loadChartData`, `limit` calculation:** Change the `1M` limit from `24` → `60`

## Acceptance Criteria

| # | Criterion |
|---|-----------|
| AC-1 | When the user selects the **1W** button on the Price Chart, the chart displays **2 years** of weekly price data. |
| AC-2 | When the user selects the **1M** button on the Price Chart, the chart displays **5 years** of monthly price data. |
| AC-3 | The **1D** button continues to display **1 year** of daily price data (no regression). |
| AC-4 | The chart fits all data in view for each timeframe (auto-fit behavior unchanged). |
| AC-5 | Real-time price updates continue to work correctly on all timeframes (1D, 1W, 1M). |
| AC-6 | Cached data is correctly invalidated/refreshed when switching between ranges. |

## Scope Boundaries

- **In scope:** Changing the 1W and 1M date ranges and data limits in `RightPanelChart.vue`.
- **Out of scope:** Changes to the `LightweightChart.vue` component (separate chart component), changes to chart styling, changes to the backend historical data API, changes to the 1D timeframe.

## Non-Functional Requirements

- No significant performance impact expected — fetching 104 weekly bars or 60 monthly bars is negligible.
- No backend changes required — the existing historical data API already supports arbitrary date ranges and limits.
