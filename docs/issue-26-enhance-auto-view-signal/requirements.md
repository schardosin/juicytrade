# Requirements: Enhance Auto View Signal Colors (Issue #26)

## 1. Overview

**Request:** Enhance the Automation Dashboard card visual signals to distinguish between strategies that are activated but not ready to trade (indicators not all passing) vs. strategies that are activated and ready to trade (all indicators passing).

**Motivation:** Currently, the Automation Dashboard uses three card color states (gray/blue/green). When a strategy is enabled and activated (running), it always shows green regardless of whether the indicator conditions are met. This makes it impossible to see at a glance which strategies are actually ready to enter a trade. Users need to expand individual cards to check indicator status.

## 2. Current Behavior

Each automation strategy card in the Automation Dashboard (`AutomationDashboard.vue`) has a colored left border indicating its state:

| State | Left Border Color | CSS Class | Condition |
|-------|------------------|-----------|-----------|
| Disabled | Gray (`var(--text-tertiary)`) | `.disabled` | `config.enabled === false` |
| Enabled but not running | Blue (`var(--color-info)`) | `.idle` | `config.enabled === true` AND not running |
| Enabled and running | Green (`var(--color-success)`) | `.running` | `config.enabled === true` AND running |

The determination is made by the `getStatusClass(config)` function, which returns `'disabled'`, `'idle'`, or `'running'`.

## 3. Desired Behavior

Split the "running" state into two distinct visual signals:

| State | Left Border Color | CSS Class | Condition |
|-------|------------------|-----------|-----------|
| Disabled | Gray (unchanged) | `.disabled` | `config.enabled === false` |
| Enabled but not running | Blue (unchanged) | `.idle` | `config.enabled === true` AND not running |
| Running, indicators NOT all passing | Yellow (`var(--color-warning)`) | `.running-not-ready` | Running AND `all_indicators_pass === false` |
| Running, all indicators passing | Green (unchanged) | `.running-ready` | Running AND `all_indicators_pass === true` |

## 4. Functional Requirements

### FR-1: New Card Color State — Yellow (Running, Not Ready)
When an automation is running (started/activated) but not all of its enabled indicators are passing, the card's left border MUST display yellow (`var(--color-warning)` or equivalent amber/yellow tone).

### FR-2: Retain Green for Ready State
When an automation is running AND all enabled indicators are passing (`all_indicators_pass === true`), the card's left border MUST remain green (`var(--color-success)`), same as today.

### FR-3: Use Existing Backend Data
The backend already provides `all_indicators_pass` (boolean) on the `ActiveAutomation` object via:
- **WebSocket** `automation_update` messages (field: `all_indicators_pass`)
- **REST API** `GET /api/automation/status` response (in the `details` spread for each running automation)

The frontend MUST use this existing data field. No backend changes should be required.

### FR-4: Real-Time Updates via WebSocket
The card color MUST update in real-time when a WebSocket `automation_update` message arrives with a changed `all_indicators_pass` value. The user should see the card transition from yellow → green (or green → yellow) without manual refresh.

### FR-5: Status Badge Consistency
The status badge (top-right of each card) should also reflect the new state distinction:
- When running and not ready: badge should use warning/yellow styling (similar to existing `status-waiting` class)
- When running and all indicators pass: badge should use success/green styling (existing `status-running` class)

### FR-6: Gray and Blue States Unchanged
The disabled (gray) and idle/enabled-not-running (blue) states MUST remain exactly as they are today. This change only affects cards in the "running" state.

### FR-7: Graceful Handling of Missing Indicator Data
When an automation is running but indicator results have not yet been evaluated (e.g., just started, no evaluation cycle completed yet), the card SHOULD default to yellow (not ready) until the first evaluation confirms all indicators pass. This is the safer visual — avoid showing green until confirmed.

### FR-8: No Indicators Edge Case
If a running automation has zero enabled indicators, `all_indicators_pass` will be `true` (vacuously true from the backend). The card SHOULD display green in this case, which is correct — there are no conditions blocking entry.

## 5. Acceptance Criteria

- **AC-1:** A disabled automation card displays a gray left border (unchanged).
- **AC-2:** An enabled but not running automation card displays a blue left border (unchanged).
- **AC-3:** A running automation where NOT all indicators pass displays a **yellow** left border.
- **AC-4:** A running automation where ALL indicators pass displays a **green** left border.
- **AC-5:** When a WebSocket update changes `all_indicators_pass` from `false` to `true`, the card transitions from yellow to green in real-time without page refresh.
- **AC-6:** When a WebSocket update changes `all_indicators_pass` from `true` to `false`, the card transitions from green to yellow in real-time without page refresh.
- **AC-7:** A newly started automation (before first indicator evaluation) shows yellow until indicators are evaluated.
- **AC-8:** The status badge in the card header visually matches the card border color state.
- **AC-9:** Mobile view cards follow the same color scheme.
- **AC-10:** All existing tests pass without regressions.

## 6. Scope Boundaries

### In Scope
- Modifying `getStatusClass()` logic in `AutomationDashboard.vue` to differentiate running states
- Adding new CSS classes for the yellow/not-ready state  
- Updating WebSocket handler in dashboard to properly store `all_indicators_pass` in status map
- Updating `getRunningStatusClass()` for status badge consistency
- Unit tests for the new color logic

### Out of Scope
- Backend changes (data is already available)
- Changes to the automation config form or edit views
- Changes to the indicator evaluation logic
- Changes to colors for non-running states (disabled, idle)
- Adding new API endpoints

## 7. Technical Notes

### Data Flow (Already Exists)
1. Backend `engine.go` evaluates indicators and sets `active.AllIndicatorsPass`
2. Backend broadcasts via WebSocket: `{ automation_id, data: { all_indicators_pass, indicator_results, status, ... } }`
3. Frontend `AutomationDashboard.vue` receives via `handleAutomationUpdate()` callback and stores in `statuses` ref
4. REST polling (`loadStatuses()` every 5s) also retrieves this data via `GetAllStatus` API

### Key Frontend Code Locations
- `trade-app/src/components/automation/AutomationDashboard.vue`
  - `getStatusClass(config)` — determines card border color class (line ~498)
  - `getRunningStatusClass(config)` — determines status badge class (line ~503)
  - `handleAutomationUpdate(message)` — WebSocket handler (line ~416)
  - `loadStatuses()` — REST polling (line ~440)
  - CSS `.config-card.running` — green border style (line ~668)

### Frontend Status Data Structure
The `statuses` ref is a map keyed by config ID. For running automations, the value includes `all_indicators_pass` from the WebSocket/REST data. The `handleAutomationUpdate` handler already spreads the full data object, so `all_indicators_pass` is already available in the status map — it just needs to be used by `getStatusClass()`.
