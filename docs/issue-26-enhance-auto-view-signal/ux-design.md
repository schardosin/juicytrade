# UX Design Spec: Enhance Auto View Signal Colors (Issue #26)

## Overview

This spec covers the visual design for splitting the "running" card state into two distinct states: **running-not-ready** (yellow) and **running-ready** (green). The change affects the Automation Dashboard strategy cards in `AutomationDashboard.vue`.

**Scope:** Only the running state visual treatment changes. Disabled (gray) and idle (blue) states remain exactly as they are.

---

## 1. Card Left Border — New State Colors

The card left border is the primary state indicator. Currently 4px solid with three colors. We add a fourth:

| State | CSS Class | Border Color | Value |
|-------|-----------|-------------|-------|
| Disabled | `.disabled` | Gray | `var(--text-tertiary)` — `#888888` *(unchanged)* |
| Idle | `.idle` | Blue | `var(--color-info)` — `#007bff` *(unchanged)* |
| **Running, not ready** | **`.running-not-ready`** | **Yellow** | **`var(--color-warning)` — `#ffbb33`** |
| Running, ready | `.running-ready` | Green | `var(--color-success)` — `#00c851` *(unchanged)* |

### CSS for new `.running-not-ready` class

```css
.config-card.running-not-ready {
  border-left: 4px solid var(--color-warning);
}
```

### CSS update for existing running state

Rename `.config-card.running` to `.config-card.running-ready` (same styles, just renamed for clarity):

```css
.config-card.running-ready {
  border-left: 4px solid var(--color-success);
}
```

### What NOT to change

- **No glow or box-shadow per state.** The existing hover effect (`box-shadow: var(--shadow-md)`) is the same for all states. Keep it that way — don't add colored glows.
- **No change to card background color.** All cards keep `var(--bg-secondary)` background regardless of state.
- **No change to disabled opacity.** The `.disabled` class keeps `opacity: 0.7`.
- **No change to card padding, border-radius, or overall card structure.**

---

## 2. Status Badge — New Variant

The status badge (top-right of each card header) already has a `.status-waiting` class that uses warning yellow styling. This is a good fit for the "running but not ready" state.

### Current badge classes and their mapping

The `getRunningStatusClass()` function currently maps automation states to badge styles. The logic needs adjustment so the badge reflects the `all_indicators_pass` data:

| Automation State | `all_indicators_pass` | Badge Class | Badge Color |
|-----------------|----------------------|-------------|-------------|
| `waiting` / `evaluating` | `false` (or not yet evaluated) | `.status-waiting` | Yellow (`var(--color-warning)`) — already exists |
| `waiting` / `evaluating` | `true` | `.status-running` | Green (`var(--color-success)`) — already exists |
| `trading` / `monitoring` | (any) | `.status-running` | Green (`var(--color-success)`) — already exists |
| `completed` | N/A | `.status-completed` | Green *(unchanged)* |
| `failed` | N/A | `.status-failed` | Red *(unchanged)* |

**Key insight:** The existing `.status-waiting` and `.status-running` badge classes already have the correct yellow and green styling respectively. No new badge CSS classes are needed — only the JS logic that selects which class to apply needs to change.

### No new CSS needed for badges

The existing styles are already correct:

```css
/* Already exists — yellow badge */
.status-waiting {
  background: rgba(251, 191, 36, 0.1);
  color: var(--color-warning);
}

/* Already exists — green badge */
.status-running {
  background: rgba(34, 197, 94, 0.1);
  color: var(--color-success);
}
```

---

## 3. Transition Behavior

### Card border color transitions

The existing `.config-card` already has `transition: var(--transition-normal)` (0.2s ease). However, `border-left-color` transitions can sometimes appear abrupt since the border is narrow.

**Recommendation:** Add `border-left-color` explicitly to the transition to ensure smooth yellow ↔ green transitions when WebSocket updates arrive:

```css
.config-card {
  /* existing transition already covers this via var(--transition-normal), 
     but ensure border-color is included in what gets transitioned */
  transition: var(--transition-normal);
  /* This is already set — no change needed. The shorthand 'all 0.2s ease' 
     covers border-left-color automatically. */
}
```

**No additional transition CSS is needed.** The existing `transition: var(--transition-normal)` already applies to all properties including `border-left-color`. The 0.2s ease timing is appropriate — fast enough to feel responsive but smooth enough to notice the color change.

### Badge transitions

The badge text/class may change (e.g., "Waiting" with yellow → "Waiting" with green when indicators pass). Since the badge is re-rendered by Vue's class binding, there is no CSS transition to add — it will swap instantly. This is fine; badge changes should feel immediate.

---

## 4. Status Text & Icon Behavior

### Status text
The `getStatusText()` function should remain unchanged — it shows the automation engine state (`Waiting`, `Evaluating`, `Trading`, etc.), not the indicator readiness. The card border color already communicates readiness; the text communicates the engine phase.

### Status icon
The `getStatusIcon()` function should also remain unchanged. The icons represent engine activity (clock for waiting, search for evaluating, spinner for trading). Do NOT change icons based on `all_indicators_pass` — that would overload the icon's meaning.

**The border color is the single visual channel for indicator readiness. Don't duplicate this signal in text or icons.**

---

## 5. Mobile Considerations

### Mobile card layout
On mobile (≤768px), cards use a single-column grid with collapsible details. The left border is equally visible in this layout. No mobile-specific adjustments are needed for the new color state.

### Mobile badge
On mobile, the badge shows icon-only (text is hidden via `v-if="!isMobile"`). The badge background color is still visible around the icon, so the yellow/green distinction works in icon-only mode.

### Collapsed cards on mobile
When a card is collapsed (`mobile-collapsed` class), the left border remains fully visible. The color change from yellow to green will be visible even in collapsed state — this is correct and desirable, as it gives at-a-glance status.

**No mobile-specific CSS changes needed.**

---

## 6. Color Accessibility Notes

The four states use colors with sufficient perceptual contrast on the dark background (`#141519`):

| Color | Hex | Purpose |
|-------|-----|---------|
| Gray | `#888888` | Disabled — low contrast signals "inactive" |
| Blue | `#007bff` | Idle — cool tone, clearly different from warm tones |
| Yellow | `#ffbb33` | Running, not ready — warm tone, high visibility |
| Green | `#00c851` | Running, ready — distinct hue from yellow |

Yellow (`#ffbb33`) and green (`#00c851`) are distinguishable even for common forms of color blindness (deuteranopia/protanopia) because they differ significantly in luminance — yellow is much brighter than green. The 4px border width is sufficient to perceive the color.

---

## 7. Implementation Guidance Summary

### Changes needed (all in `AutomationDashboard.vue`):

**JavaScript changes:**

1. **`getStatusClass(config)`** — Update to return four possible classes:
   - `'disabled'` — when `config.enabled === false`
   - `'idle'` — when enabled but not running
   - `'running-not-ready'` — when running AND `all_indicators_pass` is falsy (false, undefined, or null)
   - `'running-ready'` — when running AND `all_indicators_pass === true`
   
   Access `all_indicators_pass` from `statuses.value[config.id]?.all_indicators_pass`.

2. **`getRunningStatusClass(config)`** — Update the `waiting`/`evaluating` cases to check `all_indicators_pass`:
   - If `all_indicators_pass === true` → return `'status-running'` (green badge)
   - If `all_indicators_pass` is falsy → return `'status-waiting'` (yellow badge)
   - `trading`/`monitoring` states keep `'status-running'` (green badge)

**CSS changes:**

3. **Rename** `.config-card.running` → `.config-card.running-ready` (keep same styles)
4. **Add** `.config-card.running-not-ready`:
   ```css
   .config-card.running-not-ready {
     border-left: 4px solid var(--color-warning);
   }
   ```

### What NOT to change:
- No changes to `theme.css` — the `--color-warning` variable already has the right value
- No changes to card structure, padding, or layout
- No changes to the template HTML structure
- No changes to `getStatusIcon()` or `getStatusText()`
- No changes to disabled/idle states
- No changes to indicator chips, evaluation dialog, or log dialog
- No backend changes
