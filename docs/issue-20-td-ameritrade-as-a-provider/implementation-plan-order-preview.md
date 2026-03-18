# Schwab Order Preview — Step-by-Step Implementation Plan

> **Problem:** The Schwab provider's `PreviewOrder` method is a stub that returns
> `fmt.Errorf("schwab: order preview not supported by Schwab API")`. This causes
> the route handler (`main.go:745-750`) to return HTTP 500, which blocks the
> order confirmation dialog — the `canConfirm` computed property returns false
> when there's a preview error.
>
> **Root cause:** At the time of initial implementation, Schwab's preview endpoint
> was not known to exist. Research confirms the endpoint is
> `POST /trader/v1/accounts/{accountHash}/previewOrder` and accepts the same
> JSON body as the order placement endpoint. Multiple third-party libraries
> (Python `schwab-trader`, PHP `schwab-api-php`, Rust `schwab_api`) confirm this.
>
> **Fix:** Replace the stub with a real implementation that calls the Schwab
> preview endpoint, reusing the existing order-building functions. Also fix a
> bug in `doAuthenticatedRequest` where the POST body is lost on 401 retry.
>
> **Additionally:** The `doAuthenticatedRequest` method has a known bug: on 401
> retry, it passes `nil` as the body (line 62 of `helpers.go`), which silently
> breaks POST/PUT requests. This must be fixed first since `PreviewOrder` is a
> POST request.
>
> Derived from the requirements in `requirements-schwab-order-preview.md`.
> Cross-referenced with: `helpers.go` (178 lines), `schwab.go` (232 lines),
> `orders.go` (684 lines), `qa_edge_cases_test.go` (567 lines), `main.go`
> (1854 lines), and the Tradier/TastyTrade `PreviewOrder` implementations.
>
> Each step is one logical unit that can be implemented, tested, and committed
> independently. Steps are ordered to respect dependencies.

---

## Dependency Graph

```
Step 1 (fix doAuthenticatedRequest body retry + test)
  ↓
Step 2 (implement PreviewOrder + unit tests + update qa_edge_cases_test.go)
  ↓
Step 3 (run full test suite, verify no regressions)
```

- Step 2 depends on Step 1 (PreviewOrder is a POST and needs the body retry fix)
- Step 3 validates the full system after Steps 1 and 2

---

## Step 1 — Fix `doAuthenticatedRequest` POST body retry bug + test

**Goal:** When `doAuthenticatedRequest` receives a 401 and retries the request, the original POST/PUT body must be re-sent instead of `nil`. Currently the retry on line 62 of `helpers.go` passes `nil`, which causes the retried request to have an empty body — silently breaking any POST/PUT that requires a request body (including the preview endpoint).

**Files modified: 1 | Files created: 1 | Tests: 1 new**

### File: `trade-backend-go/internal/providers/schwab/helpers.go`

#### 1a. Change the `doAuthenticatedRequest` signature to accept `[]byte` instead of `io.Reader`

**Current signature** (line 27):
```go
func (s *SchwabProvider) doAuthenticatedRequest(ctx context.Context, method, url string, body io.Reader) ([]byte, int, error)
```

**New signature:**
```go
func (s *SchwabProvider) doAuthenticatedRequest(ctx context.Context, method, url string, body []byte) ([]byte, int, error)
```

**Rationale:** `io.Reader` is consumed on first read and cannot be re-read. By accepting `[]byte`, the caller passes an already-marshaled body that can be wrapped in `bytes.NewReader()` for each HTTP call. This is the simplest fix because:
- All existing callers already marshal JSON to `[]byte` before calling this method (they currently wrap it in `bytes.NewReader()` at the call site)
- The retry path can create a fresh `bytes.NewReader()` from the same `[]byte`
- No need for complex `io.TeeReader` or `bytes.Buffer` buffering

#### 1b. Update the internal calls to `executeHTTPRequest` to wrap `body` in `bytes.NewReader`

**Current** (line 39):
```go
respBody, statusCode, err := s.executeHTTPRequest(ctx, method, url, body)
```

**New:**
```go
var bodyReader io.Reader
if body != nil {
    bodyReader = bytes.NewReader(body)
}
respBody, statusCode, err := s.executeHTTPRequest(ctx, method, url, bodyReader)
```

#### 1c. Fix the retry to re-send the body

**Current** (line 62):
```go
// Retry the request (body may have been consumed; callers sending a body
// will need to handle this, but for GET requests body is nil)
respBody, statusCode, err = s.executeHTTPRequest(ctx, method, url, nil)
```

**New:**
```go
// Retry the request with the same body (bytes can be re-read)
var retryReader io.Reader
if body != nil {
    retryReader = bytes.NewReader(body)
}
respBody, statusCode, err = s.executeHTTPRequest(ctx, method, url, retryReader)
```

**Remove the comment** about body being consumed — it no longer applies.

#### 1d. Update all callers of `doAuthenticatedRequest`

All callers that currently pass `bytes.NewReader(jsonBody)` should now pass `jsonBody` directly (the raw `[]byte`). All callers that pass `nil` remain unchanged (`nil` is valid for both `io.Reader` and `[]byte`).

**Callers to update** (search `orders.go` and any other files calling `doAuthenticatedRequest`):

- `submitOrder` in `orders.go` (around line 398): Change `s.doAuthenticatedRequest(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))` → `s.doAuthenticatedRequest(ctx, http.MethodPost, reqURL, jsonBody)`
- `CancelOrder` in `orders.go`: If it passes a body, update similarly. If it passes `nil`, no change needed.
- Any GET callers passing `nil`: No change needed.
- Search all files in `trade-backend-go/internal/providers/schwab/` for `doAuthenticatedRequest` to find every call site.

### File: `trade-backend-go/internal/providers/schwab/helpers_test.go` — **NEW FILE**

**Test: `TestDoAuthenticatedRequest_RetryPreservesBody`**

This test verifies that when a POST request gets a 401 and is retried, the body is preserved on the retry:

```go
func TestDoAuthenticatedRequest_RetryPreservesBody(t *testing.T) {
    // 1. Create an httptest.Server that:
    //    - Returns 401 on first POST request
    //    - Returns 200 on second POST request
    //    - Records the body of each request
    // 2. Create a SchwabProvider pointed at the test server
    //    - Set up a mock ensureValidToken that always succeeds
    //    - Set a valid accessToken so the first request goes through
    // 3. Call doAuthenticatedRequest with method=POST and a non-nil body
    // 4. Assert:
    //    - The server received exactly 2 requests
    //    - Both requests had the same non-empty body
    //    - The final response is 200
    //    - No error was returned
}
```

**Acceptance criteria:**
- [ ] `doAuthenticatedRequest` signature changed from `io.Reader` to `[]byte`
- [ ] Retry path sends the original body, not `nil`
- [ ] All existing callers compile and pass their existing tests
- [ ] New test `TestDoAuthenticatedRequest_RetryPreservesBody` passes
- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/providers/schwab/...` passes (all existing tests + new test)

---

## Step 2 — Implement `PreviewOrder` method + unit tests

**Goal:** Replace the stub `PreviewOrder` in `schwab.go:116-118` with a real implementation that calls `POST /trader/v1/accounts/{accountHash}/previewOrder`, parses the response, and returns the standardized preview result map. Update the existing QA edge case test.

**Files modified: 2 | Files created: 0 | Tests: 4-5 new, 1 updated**

### File: `trade-backend-go/internal/providers/schwab/schwab.go`

#### 2a. Replace the `PreviewOrder` stub (lines 115-118)

**Current:**
```go
// PreviewOrder previews a trading order to get cost estimates and validation.
func (s *SchwabProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("schwab: order preview not supported by Schwab API")
}
```

**New implementation (~60-80 lines):**

```go
// PreviewOrder previews a trading order to get cost estimates and validation.
// Calls POST /trader/v1/accounts/{accountHash}/previewOrder with the same
// JSON body format used by PlaceOrder. Returns a standardized preview result
// map (matching Tradier/TastyTrade patterns) — never returns a Go error for
// API-level failures; errors are returned as status: "error" with validation_errors.
func (s *SchwabProvider) PreviewOrder(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error) {
    // 1. Build the order request — try multi-leg first if "legs" are present,
    //    otherwise build a single-leg request.
    //    Reuse buildSchwabOrderRequest / buildSchwabMultiLegOrderRequest from orders.go.
    var req *schwabOrderRequest
    var err error

    if legs, ok := orderData["legs"]; ok {
        if legsArr, ok := legs.([]interface{}); ok && len(legsArr) > 0 {
            req, err = buildSchwabMultiLegOrderRequest(orderData)
        } else {
            req, err = buildSchwabOrderRequest(orderData)
        }
    } else {
        req, err = buildSchwabOrderRequest(orderData)
    }

    if err != nil {
        // Build error — return structured error, not Go error
        return map[string]interface{}{
            "status":              "error",
            "validation_errors":   []string{fmt.Sprintf("Failed to build order: %s", err.Error())},
            "commission":          0,
            "cost":                0,
            "fees":                0,
            "order_cost":          0,
            "margin_change":       0,
            "buying_power_effect": 0,
            "day_trades":          0,
            "estimated_total":     0,
        }, nil
    }

    // 2. Marshal to JSON
    jsonBody, err := json.Marshal(req)
    if err != nil {
        return map[string]interface{}{
            "status":              "error",
            "validation_errors":   []string{fmt.Sprintf("Failed to marshal order: %s", err.Error())},
            ... // same zero fields
        }, nil
    }

    // 3. Call the preview endpoint
    reqURL := s.buildTraderURL("/accounts/" + s.accountHash + "/previewOrder")
    body, statusCode, err := s.doAuthenticatedRequest(ctx, http.MethodPost, reqURL, jsonBody)

    // 4. Handle transport/auth errors
    if err != nil {
        return map[string]interface{}{
            "status":              "error",
            "validation_errors":   []string{err.Error()},
            ... // same zero fields
        }, nil
    }

    // 5. Parse the response
    return s.parsePreviewResponse(body, statusCode)
}
```

**Key design decisions** (matching Tradier/TastyTrade patterns):
- **Never return a Go error** for API-level failures. Always return `(map, nil)`.
- Only return `(nil, error)` for truly exceptional infrastructure failures (none expected — even auth failures from `doAuthenticatedRequest` are caught and returned as structured errors).
- This ensures the route handler in `main.go:744-776` correctly routes to either HTTP 422 (validation error) or HTTP 200 (success), never HTTP 500 for normal API errors.

#### 2b. Add `parsePreviewResponse` helper method

Add a new method on `SchwabProvider` (can go in `schwab.go` below `PreviewOrder`, or in a new section of `orders.go`):

```go
// parsePreviewResponse transforms the raw Schwab preview API response into
// the standardized preview result map.
func (s *SchwabProvider) parsePreviewResponse(body []byte, statusCode int) (map[string]interface{}, error) {
    // Empty response
    if len(body) == 0 {
        if statusCode >= 400 {
            return errorPreviewResult(fmt.Sprintf("HTTP %d (empty response)", statusCode)), nil
        }
        // 200 with empty body — unlikely but handle gracefully
        return okPreviewResult(0, 0, 0, 0, 0, 0), nil
    }

    // Parse JSON
    var raw map[string]interface{}
    if err := json.Unmarshal(body, &raw); err != nil {
        return errorPreviewResult(fmt.Sprintf("Failed to parse response: %s", err.Error())), nil
    }

    // Handle error responses (4xx/5xx)
    if statusCode >= 400 {
        return s.extractPreviewErrors(raw, statusCode), nil
    }

    // Success — extract fields from Schwab's preview response format.
    // The Schwab preview response contains:
    //   - orderStrategy.orderLegCollection (same as request)
    //   - commission (object with total, etc.)
    //   - orderActivityCollection (may be present)
    //
    // Extract commission and fees from the response.
    // Fields that Schwab does not return default to 0.
    commission := extractNestedFloat(raw, "commission", "total")  // or however Schwab structures it
    fees := extractNestedFloat(raw, "fees", "total")
    orderCost := extractFloat(raw, "orderValue")
    estimatedTotal := orderCost + commission + fees

    return map[string]interface{}{
        "status":              "ok",
        "commission":          commission,
        "cost":                orderCost,
        "fees":                fees,
        "order_cost":          orderCost,
        "margin_change":       0,   // Not provided by Schwab preview
        "buying_power_effect": 0,   // Not provided by Schwab preview
        "day_trades":          0,   // Not provided by Schwab preview
        "estimated_total":     estimatedTotal,
        "validation_errors":   []string{},
    }, nil
}
```

**Note on response field mapping:** The exact Schwab preview response field names need to be determined during implementation. The Schwab developer portal documentation is behind authentication, but the response format can be discovered by:
1. Making a real preview API call and logging the response
2. Referencing the Python `schwab-trader` library's parsing code
3. The response likely mirrors the order GET response format with added fee/commission fields

If the exact field names differ from the above, adjust the extraction logic accordingly. The important contract is: the method always returns the standardized map shape.

#### 2c. Add `extractPreviewErrors` helper

Extract user-friendly validation error messages from Schwab's error response formats (reusing the same 3 formats already handled by `parseErrorResponse` in `helpers.go:121-170`):

```go
// extractPreviewErrors extracts validation error messages from a Schwab
// error response and returns them in the standardized preview error format.
func (s *SchwabProvider) extractPreviewErrors(raw map[string]interface{}, statusCode int) map[string]interface{} {
    var messages []string

    // Format 1: OAuth error — {"error": "...", "error_description": "..."}
    if errField, ok := raw["error"].(string); ok {
        desc, _ := raw["error_description"].(string)
        if desc != "" {
            messages = append(messages, fmt.Sprintf("%s: %s", errField, desc))
        } else {
            messages = append(messages, errField)
        }
    }

    // Format 2: API errors array — {"errors": [{"message": "...", "detail": "..."}]}
    if errorsField, ok := raw["errors"]; ok {
        if errArray, ok := errorsField.([]interface{}); ok {
            for _, item := range errArray {
                if errObj, ok := item.(map[string]interface{}); ok {
                    for _, key := range []string{"message", "detail", "title"} {
                        if msg, ok := errObj[key].(string); ok && msg != "" {
                            messages = append(messages, msg)
                            break
                        }
                    }
                }
            }
        }
    }

    // Format 3: Simple message — {"message": "..."}
    if msg, ok := raw["message"].(string); ok && msg != "" && len(messages) == 0 {
        messages = append(messages, msg)
    }

    // Fallback
    if len(messages) == 0 {
        messages = []string{fmt.Sprintf("Preview failed with HTTP %d", statusCode)}
    }

    return map[string]interface{}{
        "status":              "error",
        "validation_errors":   messages,
        "commission":          0,
        "cost":                0,
        "fees":                0,
        "order_cost":          0,
        "margin_change":       0,
        "buying_power_effect": 0,
        "day_trades":          0,
        "estimated_total":     0,
    }
}
```

#### 2d. Add convenience helpers (optional, reduces boilerplate)

```go
// errorPreviewResult returns a standardized error preview result.
func errorPreviewResult(message string) map[string]interface{} {
    return map[string]interface{}{
        "status":              "error",
        "validation_errors":   []string{message},
        "commission":          0,
        "cost":                0,
        "fees":                0,
        "order_cost":          0,
        "margin_change":       0,
        "buying_power_effect": 0,
        "day_trades":          0,
        "estimated_total":     0,
    }
}
```

### File: `trade-backend-go/internal/providers/schwab/qa_edge_cases_test.go`

#### 2e. Update `TestPreviewOrder_ReturnsNotSupported` (lines 242-267)

**Rename** to `TestPreviewOrder_BuildsRequestAndCallsAPI` and rewrite to test the new behavior:

```go
// TestPreviewOrder_BuildsRequestAndCallsAPI verifies that PreviewOrder
// constructs the correct request and calls the Schwab preview endpoint.
func TestPreviewOrder_BuildsRequestAndCallsAPI(t *testing.T) {
    // 1. Create an httptest.Server that:
    //    - Accepts POST requests to /trader/v1/accounts/.../previewOrder
    //    - Verifies the request body is valid Schwab order JSON
    //    - Returns a mock preview response with commission/fee data
    // 2. Create a SchwabProvider pointed at the test server
    // 3. Call PreviewOrder with sample order data
    // 4. Assert:
    //    - No Go error returned (err == nil)
    //    - Result is not nil
    //    - Result["status"] == "ok"
    //    - Result has all expected fields (commission, fees, etc.)
}
```

**Key assertions:**
- `err == nil` (no Go error — this is the opposite of the old test)
- `result != nil` (structured result returned)
- `result["status"] == "ok"`
- All standardized fields are present: `commission`, `cost`, `fees`, `order_cost`, `margin_change`, `buying_power_effect`, `day_trades`, `estimated_total`, `validation_errors`

#### 2f. Add new test cases (in `qa_edge_cases_test.go` or a new `preview_order_test.go`)

**Test 1: `TestPreviewOrder_SingleLegEquity`**
- Mock server returns 200 with commission/fee data
- Call `PreviewOrder` with `{symbol: "AAPL", side: "buy", qty: 100, order_type: "limit", limit_price: 200.0}`
- Verify the request sent to the mock server has correct Schwab format (session, duration, orderType, orderLegCollection)
- Verify result is `status: "ok"` with populated fields

**Test 2: `TestPreviewOrder_MultiLegOption`**
- Mock server returns 200
- Call `PreviewOrder` with `{legs: [{symbol: "AAPL251219C00200000", side: "buy_to_open", qty: 1}, {symbol: "AAPL251219C00210000", side: "sell_to_open", qty: 1}], order_type: "limit", limit_price: 2.50}`
- Verify the request has 2 legs with correct Schwab symbol format
- Verify result is `status: "ok"`

**Test 3: `TestPreviewOrder_APIError_ReturnsValidationErrors`**
- Mock server returns 400 with `{"errors": [{"message": "Insufficient funds"}]}`
- Call `PreviewOrder` with valid order data
- Verify `err == nil` (no Go error)
- Verify `result["status"] == "error"`
- Verify `result["validation_errors"]` contains `"Insufficient funds"`
- Verify all cost fields are `0`

**Test 4: `TestPreviewOrder_InvalidOrderData_ReturnsStructuredError`**
- Call `PreviewOrder` with `{symbol: "", side: "buy"}` (missing symbol)
- Verify `err == nil` (no Go error)
- Verify `result["status"] == "error"`
- Verify `result["validation_errors"]` has a meaningful message about the build failure

**Test 5: `TestPreviewOrder_AuthFailure_ReturnsStructuredError`** (optional)
- Mock server always returns 401
- Verify `err == nil`
- Verify `result["status"] == "error"`

**Acceptance criteria:**
- [ ] `PreviewOrder` stub replaced with real implementation calling `/previewOrder`
- [ ] Method reuses `buildSchwabOrderRequest` and `buildSchwabMultiLegOrderRequest`
- [ ] Method never returns `(nil, error)` for API-level failures — always `(map, nil)`
- [ ] Error responses parsed into `status: "error"` with `validation_errors`
- [ ] Success responses parsed into `status: "ok"` with cost/fee fields
- [ ] Old test `TestPreviewOrder_ReturnsNotSupported` updated for new behavior
- [ ] At least 4 new test cases covering: single-leg success, multi-leg success, API error, invalid input
- [ ] `go test ./internal/providers/schwab/... -v` passes all tests
- [ ] Route handler in `main.go:734-777` correctly returns HTTP 200 for success and HTTP 422 for validation errors (no code changes needed in `main.go` — the handler already handles both cases)

---

## Step 3 — Run full test suite and verify no regressions

**Goal:** Confirm that all existing tests across the Go backend and Vue.js frontend continue to pass after the changes in Steps 1 and 2.

**Files modified: 0 | Files created: 0**

### 3a. Go backend tests

```bash
cd trade-backend-go && go test ./... -count=1
```

**Expected results:**
- All existing tests pass (streaming, auth, indicators, handlers, etc.)
- New `helpers_test.go` test passes (Step 1)
- Updated + new `qa_edge_cases_test.go` tests pass (Step 2)
- Total test count increases by 5-6 (1 from Step 1, 4-5 from Step 2)

**Specific areas to verify:**
- `go test ./internal/providers/schwab/... -v` — all Schwab provider tests
- `go test ./internal/providers/... -v` — no other provider affected
- `go test ./internal/api/handlers/... -v` — handler tests unaffected
- `go test ./internal/auth/... -v` — auth tests unaffected

### 3b. Go build verification

```bash
cd trade-backend-go && go build ./...
```

Verifies that the `doAuthenticatedRequest` signature change (Step 1) didn't break any callers.

### 3c. Frontend tests

```bash
cd trade-app && npx vitest run
```

**Expected results:**
- All 28 existing test files pass
- No frontend code was changed, so this is purely a regression check
- The pre-existing 2 failures in `CollapsibleOptionsChain.test.js` are known and excluded from pass criteria (per `requirements-schwab-order-preview.md` AC-6)

### 3d. Frontend build

```bash
cd trade-app && npm run build
```

Verifies the production build succeeds (no frontend changes, but good to confirm).

**Acceptance criteria:**
- [ ] `go test ./...` passes with zero failures (excluding any pre-existing known failures)
- [ ] `go build ./...` compiles successfully
- [ ] `npx vitest run` passes with zero new failures (pre-existing `CollapsibleOptionsChain.test.js` failures excluded)
- [ ] `npm run build` succeeds
- [ ] No regressions in any provider (Tradier, TastyTrade, Alpaca, Schwab)

---

## Summary

| Step | Files Modified | Files Created | Tests | Description |
|------|---------------|---------------|-------|-------------|
| 1 | `helpers.go`, `orders.go` (+ other callers) | `helpers_test.go` | 1 new | Fix POST body retry bug in `doAuthenticatedRequest` |
| 2 | `schwab.go`, `qa_edge_cases_test.go` | — | 4-5 new, 1 updated | Implement `PreviewOrder` with response parsing and error handling |
| 3 | — | — | — | Full test suite validation (Go + frontend) |

**Total: 3-4 files modified, 1 file created, 5-6 tests added, 1 test updated**

### Key File Reference

| File | Lines | Role |
|------|-------|------|
| `trade-backend-go/internal/providers/schwab/helpers.go` | 27-84 | `doAuthenticatedRequest` — body retry fix |
| `trade-backend-go/internal/providers/schwab/schwab.go` | 115-118 | `PreviewOrder` stub → real implementation |
| `trade-backend-go/internal/providers/schwab/orders.go` | 334-356 | `schwabOrderRequest` types (reused) |
| `trade-backend-go/internal/providers/schwab/orders.go` | 364-371 | `PlaceOrder` — pattern to follow |
| `trade-backend-go/internal/providers/schwab/orders.go` | 385-450 | `submitOrder` — caller update for new signature |
| `trade-backend-go/internal/providers/schwab/orders.go` | 457-521 | `buildSchwabOrderRequest` (reused) |
| `trade-backend-go/internal/providers/schwab/orders.go` | 524-594 | `buildSchwabMultiLegOrderRequest` (reused) |
| `trade-backend-go/internal/providers/schwab/helpers.go` | 121-170 | `parseErrorResponse` — reference for error format handling |
| `trade-backend-go/internal/providers/schwab/qa_edge_cases_test.go` | 247-267 | Existing test to update |
| `trade-backend-go/cmd/server/main.go` | 734-777 | Route handler (no changes needed) |
| `trade-backend-go/internal/providers/manager.go` | 415-424 | `ProviderManager.PreviewOrder` routing (no changes needed) |
| `trade-backend-go/internal/providers/base/provider.go` | 82-84 | `Provider` interface (no changes needed) |
