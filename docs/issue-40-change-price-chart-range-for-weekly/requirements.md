# Requirements: Change Price Chart Date Ranges for Weekly and Monthly Views

**Issue:** [#40 — Change Price Chart Range for Weekly](https://github.com/schardosin/juicytrade/issues/40)
**Component:** `trade-app/src/components/RightPanelChart.vue`
**Type:** Enhancement

---

## Context & Motivation

The Price Chart in the right-side panel's Overview tab offers three timeframe buttons: **1D** (daily), **1W** (weekly), and **1M** (monthly). Currently, the **1W** view fetches only 1 year of weekly data — the same date range as the **1D** view. This means switching from 1D to 1W produces a very similar but more contracted chart with no additional historical context. The 1M view similarly could benefit from a longer lookback.

The goal is to make each timeframe button show a meaningfully different amount of historical data, so the user gains additional context when switching to longer timeframes.

## Functional Requirements

### FR-1: Update 1W (Weekly) date range from 1 year to 2 years
- When the user selects the **1W** button, the chart must request and display **2 years** of weekly data (instead of the current 1 year).
- The `startDate` in `calculateDateRange()` for the `"1W"` case must be set to `now.getFullYear() - 2` (2 years ago from today).
- The `limit` for the `"1W"` case must be updated from `52` to `104` (approximately 52 weeks/year × 2 years).

### FR-2: Update 1M (Monthly) date range from 2 years to 5 years
- When the user selects the **1M** button, the chart must request and display **5 years** of monthly data (instead of the current 2 years).
- The `startDate` in `calculateDateRange()` for the `"1M"` case must be set to `now.getFullYear() - 5` (5 years ago from today).
- The `limit` for the `"1M"` case must be updated from `24` to `60` (approximately 12 months/year × 5 years).

### FR-3: No change to 1D (Daily)
- The **1D** button must continue to show 1 year of daily data (365 bars). No changes required.

## Code Changes Summary

Only **one file** needs to be modified: `trade-app/src/components/RightPanelChart.vue`

**Change 1 — `calculateDateRange()` function (around line 196–213):**
- `case "1W":` — change `now.getFullYear() - 1` → `now.getFullYear() - 2`
- `case "1M":` — change `now.getFullYear() - 2` → `now.getFullYear() - 5`
- Update the corresponding comments to reflect the new ranges.

**Change 2 — `limit` calculation (around line 241):**
- Change from: `range === "1W" ? 52 : 24`
- Change to: `range === "1W" ? 104 : 60`
- Update the inline comment to reflect: `1 year daily, 2 years weekly, 5 years monthly`

## Acceptance Criteria

1. **AC-1:** Selecting **1D** shows ~1 year of daily bars (unchanged behavior).
2. **AC-2:** Selecting **1W** shows ~2 years of weekly bars. The chart should display noticeably more historical data than the 1D view.
3. **AC-3:** Selecting **1M** shows ~5 years of monthly bars. The chart should display noticeably more historical data than the 1W view.
4. **AC-4:** The cached data (`chartData` Map) correctly stores and retrieves data for each range with the updated limits.
5. **AC-5:** Real-time price updates continue to work correctly for all three timeframes (no regression in `updateRealTimeData`).
6. **AC-6:** No changes to the `LightweightChart.vue` component (the main chart has its own separate range/timeframe system that is unaffected).

## Scope Boundaries

- **In scope:** Adjusting date range and limit values for 1W and 1M in `RightPanelChart.vue` only.
- **Out of scope:** Changes to `LightweightChart.vue`, backend API changes, adding new timeframe options, or modifying the chart's visual appearance.

## Risks & Notes

- The backend historical data API must support returning 5 years of monthly data and 2 years of weekly data. Based on the existing `LightweightChart.vue` which already supports up to 20-year ranges, the backend should handle this without issues.
- No backend changes are expected since the API already accepts `start_date` and `limit` parameters for arbitrary ranges.
