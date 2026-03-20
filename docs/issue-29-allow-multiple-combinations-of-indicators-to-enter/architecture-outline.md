# Architecture Document Outline — Indicator Groups (Issue #29)

## 1. System Overview
- Problem recap, solution summary, and high-level component diagram showing which services are affected.

## 2. Data Model Changes
- New `IndicatorGroup` struct, changes to `AutomationConfig`, changes to `ActiveAutomation` (new `GroupResult`), and JSON storage schema.

## 3. Evaluation Logic Changes
- New `EvaluateAllIndicatorGroups` / `AnyGroupPasses` functions, per-group AND logic, cross-group OR logic, stale data handling per group, no short-circuiting.

## 4. Storage Migration
- Auto-migration of flat `indicators` to a single "Default" group on load, following the `migrateIndicatorIDs` pattern. Version bump.

## 5. API Changes
- CRUD endpoint changes for configs, test-indicators endpoint changes, WebSocket broadcast changes.

## 6. Frontend Data Flow
- AutomationConfigForm.vue grouped indicator UI data model, AutomationDashboard.vue group-aware display, automationService/api.js changes.

## 7. File Change Inventory
- Exhaustive list of files to create/modify with descriptions of changes.

## 8. Component Interaction Diagram
- Mermaid sequence diagram showing the evaluation cycle with groups.

## 9. Trade-offs & Decisions
- Design decisions made and their rationale.

## 10. Testing Strategy
- Unit tests for evaluation logic, migration tests, integration/E2E considerations.
