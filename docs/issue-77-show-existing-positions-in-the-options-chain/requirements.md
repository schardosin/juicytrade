# Requirements: Show Existing Positions in the Options Chain

**Issue:** [#77 - Show existing positions in the Options Chain](https://github.com/schardosin/juicytrade/issues/77)  
**Author:** @schardosin  
**Date:** 2026-07-14

## Overview

When a user expands an expiration date in the options chain (`CollapsibleOptionsChain` component), any strikes that correspond to existing open positions should display a visual indicator showing:
1. That a position exists at that strike
2. Whether the position is **long** or **short**
3. The **size** (quantity) of the position

This feature allows traders to quickly identify where their existing positions are located relative to the current market, improving situational awareness during trading decisions.

## Context & Motivation

Currently, the options chain displays market data (bid/ask, Greeks, volume) but provides no indication of whether the user already holds a position at any given strike. Traders must mentally cross-reference the separate Positions view to identify their exposure within the chain. This is a common workflow friction point that most professional trading platforms solve by annotating positions directly in the options chain.

## Data Sources

### Positions Data (Already Available)
- Positions are loaded periodically via `smartMarketDataStore` using the `getPositionsForSymbol(symbol)` method
- The `EnhancedPositionsResponse` structure contains `symbol_groups` → `strategies` → `legs`
- Each `PositionLeg` contains:
  - `symbol` (option symbol, e.g., `SPX250718C05500000`)
  - `qty` (positive = long, negative = short)
  - `strike_price` (float)
  - `expiry_date` (string, `YYYY-MM-DD`)
  - `option_type` (`"call"` or `"put"`)
  - `underlying_symbol`

### Options Chain Data (Already Available)
- Each option row has a `symbol`, `strike_price`, `type` (call/put), and belongs to a specific expiration date
- The component already has access to `byStrike` maps keyed by strike price

## Functional Requirements

### FR-1: Position Indicator Display
When an expiration date is expanded in the options chain, each option row (call side and/or put side) that matches an existing open position SHALL display a position indicator badge.

- The badge SHALL appear on the **call side** if the user has an open call position at that strike/expiry
- The badge SHALL appear on the **put side** if the user has an open put position at that strike/expiry
- The badge SHALL be visible without interfering with the existing bid/ask/Greeks data

### FR-2: Long/Short Indication
The position indicator SHALL clearly distinguish between long and short positions:
- **Long positions** (qty > 0): Display with a color/style associated with buying (green tint or similar positive indicator)
- **Short positions** (qty < 0): Display with a color/style associated with selling (red tint or similar negative indicator)

### FR-3: Position Size Display
The position indicator SHALL display the quantity (size) of the position:
- Show the absolute quantity value (e.g., `1`, `5`, `10`)
- Prefix with a direction indicator: `+` for long, `-` for short (e.g., `+2`, `-5`)

### FR-4: Multiple Positions at Same Strike
If multiple position legs exist at the same strike/expiry/type (e.g., from different strategies), the indicator SHALL show the **net aggregate quantity** across all legs.

### FR-5: Position Data Integration
- The `CollapsibleOptionsChain` component SHALL accept positions data (either as a prop or by directly accessing the positions store/composable)
- Position matching SHALL use the option symbol OR the combination of (expiry_date + strike_price + option_type) to identify matches
- The component SHALL react to position data changes (e.g., if a position is closed while the chain is open, the indicator should disappear)

### FR-6: Compact Badge Design
The position indicator SHALL be a small, compact badge that:
- Does not significantly increase the row height
- Does not obscure or push existing data (bid/ask/delta/theta/volume) out of alignment
- Is positioned in a consistent location within the call-side or put-side cell (e.g., a small badge overlaid on the edge of the option-data area, or an additional narrow column)

### FR-7: Responsiveness
- The position indicators SHALL work on both desktop and mobile layouts
- On mobile (where some columns like Vol and Theta are hidden), the indicator SHALL still be visible and not cause layout issues

## Non-Functional Requirements

### NFR-1: Performance
- Position lookups per row SHALL be O(1) using a pre-computed map (keyed by `expiryDate-strike-type` or by option symbol)
- Position data should be computed once per expiration expansion (not recalculated per row render)
- No additional API calls are needed — positions data is already loaded and reactive in the store

### NFR-2: Visual Consistency
- Colors and styles SHALL use existing CSS custom properties from the design system (e.g., `--color-success`, `--color-danger`)
- The badge style should feel native to the existing UI (dark theme, compact, using existing spacing variables)

## Acceptance Criteria

1. **AC-1:** When an expiration is expanded and the user has a long call position at strike $550, the call side of the $550 row displays a green badge showing `+N` (where N is the quantity)
2. **AC-2:** When an expiration is expanded and the user has a short put position at strike $540, the put side of the $540 row displays a red badge showing `-N`
3. **AC-3:** If no positions exist for the expanded expiration date, no indicators are shown (no visual changes)
4. **AC-4:** If a position spans multiple legs at the same strike/expiry/type (net qty = 3), the badge shows `+3` or `-3`
5. **AC-5:** The indicator updates reactively — if positions data refreshes (position closed), the badge disappears without page reload
6. **AC-6:** On mobile layout, the position badge is still visible and does not break the grid alignment
7. **AC-7:** The options chain rendering performance is not noticeably degraded (position map is pre-computed, not searched per row)

## Scope Boundaries

### In Scope
- Visual indicator on the options chain rows showing existing positions
- Long/short distinction via color coding
- Position size display
- Works with the existing `CollapsibleOptionsChain` component
- Works with all position data formats (enhanced hierarchical structure)

### Out of Scope
- Clicking on the position badge to navigate to/manage the position
- Showing P&L information within the options chain row
- Any changes to the backend API (all data already available)
- Changes to the Positions view or RightPanel
- Position grouping/strategy identification within the badge (just show net qty)

## Technical Notes

- The `CollapsibleOptionsChain` component currently imports `useMarketData` and `useSelectedLegs`. Adding `getPositionsForSymbol` from `useMarketData` is the natural integration point.
- Position matching can use the option `symbol` field directly (each option row exposes `getCallOption(expiration, strike)?.symbol` and `getPutOption(expiration, strike)?.symbol`)
- A reactive `positionsMap` computed property (keyed by option symbol → qty) would enable O(1) lookups per row
- The component already uses `reactive` and `computed` from Vue, so integrating reactive position data fits naturally
