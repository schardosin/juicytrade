# Requirements: Schwab Order Preview Implementation

## 1. Overview

**Task:** Implement the Schwab order preview functionality so that users can preview orders (see estimated costs, fees, commissions, and validation) before submitting them, just like they can with Tradier and TastyTrade providers.

**Context:** The Schwab provider's `PreviewOrder` method currently returns a hard-coded error: `"schwab: order preview not supported by Schwab API"`. Research reveals that the Schwab Trader API **does** have a preview endpoint at `POST /trader/v1/accounts/{accountHash}/previewOrder`. Multiple third-party libraries (Python `schwab-trader`, PHP `schwab-api-php`, Rust `schwab_api`) have successfully implemented it. The endpoint accepts the same order JSON body as the `placeOrder` endpoint and returns estimated commission, fees, and order validation details.

**Motivation:** When a user tries to submit a trade using the Schwab provider, the order confirmation dialog shows an error "⚠️ Order Preview Failed — schwab: order preview not supported by Schwab API" instead of showing cost estimates. This blocks the user from confirming orders because the preview error prevents confirmation (the `canConfirm` computed property returns false when there's a preview error). The user must work around this by modifying their workflow, which is a poor experience.

## 2. Functional Requirements

### FR-1: Implement Schwab PreviewOrder API Call

The Schwab provider's `PreviewOrder` method must call the Schwab API's preview endpoint instead of returning a hard-coded error.

- **Endpoint:** `POST https://api.schwabapi.com/trader/v1/accounts/{accountHash}/previewOrder`
- **Request body:** Same JSON format as the order placement endpoint (the `schwabOrderRequest` struct)
- **Authentication:** Bearer token (same as all other Schwab API calls, using `doAuthenticatedRequest`)
- **The method must support:**
  - Single-leg equity orders (buy/sell stocks)
  - Single-leg option orders (buy_to_open, sell_to_close, etc.)
  - Multi-leg option orders (spreads, straddles, iron condors, etc.)

### FR-2: Transform Order Data for Preview

The preview method must reuse the existing order-building logic (`buildSchwabOrderRequest` and `buildSchwabMultiLegOrderRequest`) to transform JuicyTrade's generic order data format into Schwab's order JSON format. This ensures consistency between what is previewed and what will be placed.

### FR-3: Parse Preview Response

The preview response from Schwab must be parsed and transformed into the standardized preview result format used by the application:

```go
map[string]interface{}{
    "status":              "ok",      // or "error"
    "commission":          float64,   // estimated commission
    "cost":                float64,   // order cost
    "fees":                float64,   // regulatory/exchange fees
    "order_cost":          float64,   // total order cost
    "margin_change":       float64,   // margin impact (if available)
    "buying_power_effect": float64,   // buying power change (if available)
    "day_trades":          int,       // day trade impact (if available)
    "estimated_total":     float64,   // estimated total cost/proceeds
    "validation_errors":   []string,  // any validation warnings/errors
}
```

Fields not returned by Schwab should default to `0` / empty.

### FR-4: Handle Preview Errors Gracefully

- **API errors (4xx/5xx):** Parse the error response body for validation messages and return them in `validation_errors`. Return `status: "error"`.
- **Authentication failures:** If the token is expired or invalid, the method should attempt to refresh and retry (consistent with existing `doAuthenticatedRequest` behavior).
- **Network errors:** Return a meaningful error message in `validation_errors`.
- **Schwab-specific validation errors:** Extract and surface user-friendly messages from Schwab's error response format (e.g., insufficient funds, invalid symbol, market closed).

### FR-5: Remove Hard-Coded Error

Remove the current stub implementation:
```go
func (s *SchwabProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("schwab: order preview not supported by Schwab API")
}
```

Replace it with the real implementation.

### FR-6: Update Edge Case Test

The existing QA edge case test (`qa_edge_cases_test.go`) that asserts the "not supported" error message must be updated to reflect the new behavior.

## 3. Acceptance Criteria

### AC-1: Preview Succeeds for Valid Orders
Given a user has an authenticated Schwab provider configured,
when they preview a valid single-leg equity order (e.g., buy 1 share of AAPL at limit $200),
then the preview returns `status: "ok"` with numeric cost/fee fields populated.

### AC-2: Preview Succeeds for Option Orders
Given a user has an authenticated Schwab provider configured,
when they preview a valid multi-leg option order (e.g., a vertical spread),
then the preview returns `status: "ok"` with cost/fee estimates.

### AC-3: Preview Returns Validation Errors
Given a user submits an invalid order for preview (e.g., invalid symbol, zero quantity),
when the Schwab API returns an error response,
then the preview returns `status: "error"` with human-readable messages in `validation_errors`.

### AC-4: No Regression in Order Placement
Given the preview implementation reuses order-building logic,
when the user places an order after previewing,
then the order placement continues to work exactly as before.

### AC-5: Frontend Displays Preview Data
Given the backend returns a successful preview result,
when the user views the order confirmation dialog,
then it displays the cost estimates (commission, fees, total) instead of an error message.

### AC-6: All Existing Tests Pass
All existing backend tests (Go) and frontend tests (Vitest) must continue to pass with zero regressions, excluding the pre-existing 2 failures in `CollapsibleOptionsChain.test.js`.

## 4. Scope Boundaries

### In Scope
- Implementing the `PreviewOrder` method for the Schwab provider
- Parsing the Schwab preview response into the standardized format
- Unit tests for the new preview functionality
- Updating the existing edge case test that checks for the "not supported" error

### Out of Scope
- Changes to the frontend order confirmation dialog (it already handles preview data correctly for other providers — once the backend returns valid data, it will work)
- Changes to the `/api/orders/preview` route handler (it already works correctly)
- Changes to other providers (Tradier, TastyTrade, Alpaca)
- Changes to the order placement flow

## 5. Technical Notes

- The Schwab preview endpoint URL pattern: `POST /trader/v1/accounts/{accountHash}/previewOrder`
- The request body is identical to the order placement body (same `schwabOrderRequest` JSON)
- The existing `buildSchwabOrderRequest` and `buildSchwabMultiLegOrderRequest` functions should be reused
- The existing `doAuthenticatedRequest` method handles authentication, token refresh, and rate limiting
- Reference implementations exist in:
  - Python `schwab-trader` library: `POST /accounts/{account_number}/previewOrder`
  - PHP `schwab-api-php`: `previewOrder($encryptedAccountHash, $orderJson)`
  - Note: The Schwab developer portal documentation is behind authentication, but the endpoint is confirmed by multiple independent implementations
- The Alpaca provider uses a stub pattern (`preview_not_available: true`) — the Schwab provider should NOT use this pattern since the API actually supports preview
- Schwab charges $0 commission for equities and $0.65/contract for options — the preview should reflect these real fees

## 6. Dependencies

- Schwab OAuth authentication must be working (confirmed by customer)
- Account hash must be available (already stored in `s.accountHash`)
- Valid access token must be available (handled by existing auth infrastructure)
