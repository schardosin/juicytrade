# Architecture Outline — Issue #34: Easy Broker Account Switch

## Sections

1. **Overview & Scope** — Summary of the change, files affected, and the "no backend changes" constraint.
2. **Current Architecture** — How the TopBar tooltip, provider data, and config update flow work today.
3. **Component Architecture** — Mermaid diagram showing data flow between TopBar, composable, store, and API.
4. **Detailed Design: TopBar.vue Changes** — Template changes (Dropdown integration), script changes (new refs, computed, methods), tooltip interaction model.
5. **Tooltip Interaction Model** — The trickiest part: how to keep the tooltip open while the PrimeVue Dropdown overlay is open, and when to close it.
6. **Data Flow & Filtering** — How to filter `providers.available` for trade-capable providers, build dropdown options, and handle the config update.
7. **Loading, Error & Edge Case Handling** — Loading state during switch, error rollback, single-provider case.
8. **Styling Guidelines** — PrimeVue Dropdown dark theme integration, scoped CSS overrides, sizing constraints.
9. **File Change Summary** — Exact list of files to modify, with change descriptions.
10. **Testing Guidance** — Key scenarios to verify against acceptance criteria.
