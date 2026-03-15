# Architecture Outline — Issue #14: Enhance Payoff Chart with T+0 Line

## Sections

1. **Overview & Design Goals** — High-level summary of the change, design principles, and key trade-offs.

2. **System Architecture & Component Diagram** — Mermaid diagram showing the data flow from OptionsTrading.vue through chartUtils.js to PayoffChart.vue, highlighting where the new T+0 logic plugs in.

3. **New Module: `blackScholes.js`** — API contract, function signatures, mathematical formulas (CND approximation, d1/d2, call/put pricing), and performance considerations.

4. **Changes to `chartUtils.js`** — How `generateMultiLegPayoff()` is extended (or a companion function added) to produce T+0 payoff arrays. New function signatures, inputs, outputs, and the T+0 dataset configuration for Chart.js.

5. **Crosshair Plugin Enhancement** — How the `customCrosshair` plugin is modified to show two intersection circles and two P&L values in the tooltip.

6. **Data Flow Design** — Where IV data is fetched, how it flows into the T+0 calculation, and the prop/event chain from OptionsTrading.vue → RightPanel.vue → PayoffChart.vue.

7. **Chart Update Lifecycle Integration** — How the T+0 dataset participates in the existing update mechanisms (minor, full, price-only updates), dataset index management, and frozen/debounce state handling.

8. **File Structure & Change Summary** — List of all files to create/modify with a summary of changes per file.

9. **Edge Cases & Graceful Degradation** — Handling missing IV, 0 DTE, mixed expirations, negative time, and the fallback behavior.

10. **Trade-offs & Decisions** — Explicit documentation of design decisions and their rationale.
