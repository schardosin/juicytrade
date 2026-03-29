# Requirements: Change Price Chart Weekly (1W) Range to 2 Years

**Issue:** [#38 — Change Price Chart Range for Weekly](https://github.com/schardosin/juicytrade/issues/38)
**Status:** Draft — Awaiting Customer Approval

---

## Description

The Price Chart in the right-side panel's Overview tab offers three timeframe filters: **1D** (daily), **1W** (weekly), and **1M** (monthly). Currently, the **1W** filter fetches and displays **1 year** of weekly data — the same date range as the **1D** filter. This makes the weekly chart appear as a contracted version of the daily chart with no additional historical context.

The request is to extend the **1W** filter to fetch and display **2 years** of weekly data, giving users meaningful additional historical perspective when switching from daily to weekly.

## Motivation

- When switching from 1D to 1W, users expect to see a broader historical view — that's the purpose of a longer timeframe.
- With both 1D and 1W covering 1 year, the weekly chart provides no additional information, making the 1W button effectively useless.
- The 1M filter already uses a 2-year range, so extending 1W to 2 years aligns it with the pattern: each longer timeframe shows more history.

## Functional Requirements

### FR-1: Extend 1W date range to 2 years
- The `calculateDateRange` function in `RightPanelChart.vue` must set the start date for the `1W` range to **2 years before the current date** (currently 1 year).

### FR-2: Increase 1W data point limit to 104
- The `loadChartData` function in `RightPanelChart.vue` must request **104** weekly data points (52 weeks × 2 years) instead of the current **52**.

### FR-3: No changes to other timeframes
- The **1D** filter must remain at 1 year / 365 data points.
- The **1M** filter must remain at 2 years / 24 data points.

## Affected File

- `trade-app/src/components/RightPanelChart.vue` — Two changes in the `setup()` function:
  1. **Line ~201** (`calculateDateRange`, `case "1W"`): Change `now.getFullYear() - 1` → `now.getFullYear() - 2`
  2. **Line ~241** (`loadChartData`, `limit` calculation): Change the `1W` limit from `52` → `104`

## Acceptance Criteria

| # | Criterion |
|---|-----------|
| AC-1 | When the user selects the **1W** button on the Price Chart, the chart displays **2 years** of weekly price data. |
| AC-2 | The **1D** button continues to display **1 year** of daily price data (no regression). |
| AC-3 | The **1M** button continues to display **2 years** of monthly price data (no regression). |
| AC-4 | The chart fits all 2 years of weekly data in view (auto-fit behavior unchanged). |
| AC-5 | Real-time price updates continue to work correctly on the 1W timeframe. |
| AC-6 | Cached data is correctly invalidated/refreshed when switching between ranges. |

## Scope Boundaries

- **In scope:** Changing the 1W date range and data limit in `RightPanelChart.vue`.
- **Out of scope:** Changes to the `LightweightChart.vue` component (separate chart component), changes to chart styling, changes to the backend historical data API, changes to any other timeframes.

## Non-Functional Requirements

- No performance impact expected — fetching 104 weekly bars instead of 52 is negligible.
- No backend changes required — the existing historical data API already supports arbitrary date ranges and limits.
