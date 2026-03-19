# Schwab OAuth Callback Bug Fix — Step-by-Step Implementation Plan

> **Bug:** The Schwab OAuth callback URL (e.g., `https://juicytrade.muxpie.com/callback`)
> hits the frontend (port 443) instead of the Go backend (port 8008). The Vue router
> has no `/callback` route, so the user sees a blank screen. The backend never receives
> the authorization code, tokens are never exchanged, and the polling loop in
> `ProvidersTab.vue` never sees completion.
>
> **Root cause:** `router.GET("/callback", ...)` is registered at root level on the Go
> backend (port 8008), but in production, Schwab redirects to the configured callback
> URL which resolves to the frontend (port 443 via nginx/ingress). The frontend has no
> route to handle `/callback`.
>
> **Fix:** Route the callback through the frontend — add a Vue `/callback` route that
> captures the authorization code and relays it to a new backend API endpoint. Change
> the OAuth popup from a new tab to a popup window that communicates back to the opener
> via `postMessage`.
>
> Derived from the bug report in Issue #20 comments 29–34 and the PO's fix specification.
> Cross-referenced with the current codebase: `oauth.go` (782 lines), `oauth_test.go`
> (1488 lines), `qa_oauth_edge_test.go` (959 lines), `main.go` (1856 lines),
> `ProvidersTab.vue` (2432 lines), `api.js` (1448 lines), `router/index.js` (229 lines).
>
> Each step is one logical unit that can be implemented, tested, and committed
> independently. Steps are ordered to respect dependencies.

---

## Dependency Graph

```
Step 1 (backend) ──→ Step 2 (backend tests) ──→ Step 5 (run all tests)
                                                       ↑
Step 3 (frontend view + route + api) ──→ Step 4 (frontend popup + postMessage)
```

- Step 1 (backend) and Step 3 (frontend) can be developed in parallel
- Step 2 depends on Step 1 (tests validate the new JSON responses)
- Step 4 depends on Step 3 (popup communicates with the new callback view)
- Step 5 is the final validation — all backend and frontend tests must pass

---

## Step 1 — Backend: Move callback route to `/api` and return JSON instead of HTML

**Goal:** The callback endpoint becomes a standard JSON API that the frontend can call programmatically, instead of a browser-facing HTML page that Schwab redirects to directly.

**Files modified: 2 | Files created: 0**

### File: `trade-backend-go/cmd/server/main.go`

1. **Remove** the root-level callback route (current line 397):
   ```go
   // REMOVE this line:
   router.GET("/callback", schwabOAuthHandler.HandleCallback)
   ```

2. **Add** the callback as an API route, placed alongside the other Schwab OAuth routes (after line 384, before the auth middleware):
   ```go
   api.GET("/providers/schwab/oauth/callback", schwabOAuthHandler.HandleCallback)
   ```
   This places it in the `/api` group, before the auth middleware (line 387), so it does not require authentication — which is correct because the frontend callback page needs to call this endpoint during the OAuth flow before the user may be fully authenticated.

3. **Remove** the comment on line 396 (`// Schwab OAuth callback — root level to match registered callback URL`) since it no longer applies.

### File: `trade-backend-go/internal/providers/schwab/oauth.go`

1. **Modify `HandleCallback`** (lines 525–635) to return JSON responses instead of HTML. Replace every `c.Data(http.StatusOK, "text/html; charset=utf-8", renderCallbackPage(...))` with the appropriate `c.JSON(...)` call:

   | Scenario | Current response | New response |
   |----------|-----------------|--------------|
   | User cancelled (`errorParam != ""`) | `renderCallbackPage("cancelled", "")` | `c.JSON(200, gin.H{"status": "cancelled", "message": "Authorization cancelled by user"})` |
   | Missing/invalid state token | `renderCallbackPage("error", "Invalid or expired authorization request")` | `c.JSON(400, gin.H{"status": "error", "error": "Invalid or expired authorization request"})` |
   | Already processed (duplicate callback) | `renderCallbackPage("error", "This authorization has already been processed")` | `c.JSON(409, gin.H{"status": "error", "error": "This authorization has already been processed"})` |
   | Missing authorization code | `renderCallbackPage("error", "Missing authorization code")` | `c.JSON(400, gin.H{"status": "error", "error": "Missing authorization code"})` |
   | Token exchange failed | `renderCallbackPage("error", "Token exchange failed: "+err.Error())` | `c.JSON(502, gin.H{"status": "error", "error": "Token exchange failed: "+err.Error()})` |
   | Account fetch failed | `renderCallbackPage("error", "Account fetch failed: "+err.Error())` | `c.JSON(502, gin.H{"status": "error", "error": "Account fetch failed: "+err.Error()})` |
   | No accounts found | `renderCallbackPage("error", "No accounts found")` | `c.JSON(400, gin.H{"status": "error", "error": "No accounts found"})` |
   | Success | `renderCallbackPage("success", "")` | `c.JSON(200, gin.H{"status": "success"})` |

   **HTTP status code rationale:**
   - `200` for success and cancellation (the request was handled correctly)
   - `400` for client errors (invalid state, missing code, no accounts)
   - `409` for duplicate/already-processed callbacks (conflict)
   - `502` for upstream Schwab API failures (token exchange, account fetch)

2. **Delete the `renderCallbackPage` function** (lines 342–420) entirely. It is no longer needed since the frontend handles all UI rendering.

3. **Update the doc comment** on `HandleCallback` (lines 520–524) to reflect the new route path and JSON response format:
   ```go
   // HandleCallback receives the OAuth authorization code relayed by the frontend
   // callback page. It exchanges the code for tokens, fetches account numbers,
   // and updates the flow state. The frontend polls HandleOAuthStatus to detect
   // completion.
   //
   // GET /api/providers/schwab/oauth/callback?code=...&state=...  (or ?error=...&state=...)
   // Response: { "status": "success" } or { "status": "error", "error": "..." }
   ```

**Verification:** `go build ./...` should succeed. Manual verification that the removed root-level route is gone and the new API route is registered.

---

## Step 2 — Backend: Update all HandleCallback tests for JSON responses

**Goal:** All 16 existing `HandleCallback` tests pass with the new JSON response format. No behavioral changes to the handler logic — only the response format is different.

**Files modified: 2 | Files created: 0 | Tests: 16 updated**

### File: `trade-backend-go/internal/providers/schwab/oauth_test.go`

Update the following 13 tests. Each test currently asserts against HTML content (`strings.Contains(body, "successful")`, etc.) and needs to be changed to parse JSON and assert against the JSON fields.

**Helper change:** The `getCallback` helper (line 812) currently uses path `/api/schwab/oauth/callback` — update to `/api/providers/schwab/oauth/callback` for accuracy (though the path doesn't affect Gin test context behavior, it's good for documentation).

**Test-by-test changes:**

1. **`TestHandleCallback_Success_SingleAccount`** (line 826)
   - Current: `strings.Contains(body, "successful")`
   - New: Parse JSON, assert `resp["status"] == "success"`, verify HTTP 200

2. **`TestHandleCallback_Success_MultipleAccounts`** (line 867)
   - Current: Checks HTTP 200 (already correct)
   - New: Parse JSON, assert `resp["status"] == "success"`

3. **`TestHandleCallback_UserCancelled`** (line 905)
   - Current: `strings.Contains(body, "Cancelled")`
   - New: Parse JSON, assert `resp["status"] == "cancelled"`, verify HTTP 200

4. **`TestHandleCallback_InvalidState`** (line 931)
   - Current: `strings.Contains(body, "Invalid or expired")`
   - New: Parse JSON, assert `resp["status"] == "error"` and `resp["error"]` contains `"Invalid or expired"`, verify HTTP 400

5. **`TestHandleCallback_DuplicateCallback`** (line 945)
   - Current: `strings.Contains(body, "already been processed")`
   - New: Parse JSON on second call, assert `resp["error"]` contains `"already been processed"`, verify HTTP 409

6. **`TestHandleCallback_TokenExchangeFails`** (line 968)
   - Current: `strings.Contains(body, "Token exchange failed")`
   - New: Parse JSON, assert `resp["error"]` contains `"Token exchange failed"`, verify HTTP 502

7. **`TestHandleCallback_AccountFetchFails`** (line 994)
   - Current: `strings.Contains(body, "Account fetch failed")`
   - New: Parse JSON, assert `resp["error"]` contains `"Account fetch failed"`, verify HTTP 502

8. **`TestHandleCallback_NoAccounts`** (line 1015)
   - Current: `strings.Contains(body, "No accounts found")`
   - New: Parse JSON, assert `resp["error"]` contains `"No accounts found"`, verify HTTP 400

9. **`TestHandleCallback_MissingCode`** (line 1036)
   - Current: `strings.Contains(body, "Missing authorization code")`
   - New: Parse JSON, assert `resp["error"]` contains `"Missing authorization code"`, verify HTTP 400

10. **`TestHandleCallback_MissingState`** (line 1054)
    - Current: `strings.Contains(body, "Invalid or expired")`
    - New: Parse JSON, assert `resp["error"]` contains `"Invalid or expired"`, verify HTTP 400

11. **`TestRenderCallbackPage_Success`** (line 531) — **DELETE** this test entirely (function removed)

12. **`TestRenderCallbackPage_Error`** (line 548) — **DELETE** this test entirely

13. **`TestRenderCallbackPage_Cancelled`** (line 562) — **DELETE** this test entirely

### File: `trade-backend-go/internal/providers/schwab/qa_oauth_edge_test.go`

Update the following 3 tests:

1. **`TestHandleCallback_ErrorTakesPrecedenceOverCode`** (line 495)
   - Current: `strings.Contains(body, "Cancelled")`
   - New: Parse JSON, assert `resp["status"] == "cancelled"`, verify `!strings.Contains(body, "success")`

2. **`TestHandleCallback_ExpiredState`** (line 527)
   - Current: `strings.Contains(body, "Invalid or expired")`
   - New: Parse JSON, assert `resp["error"]` contains `"Invalid or expired"`, verify HTTP 400

3. **`TestHandleCallback_HTMLContentType`** (line 556)
   - **Rename** to `TestHandleCallback_JSONContentType`
   - Current: Asserts `Content-Type == "text/html; charset=utf-8"` for 3 scenarios
   - New: Assert `Content-Type` starts with `"application/json"` for all 3 scenarios

**Verification:** `cd trade-backend-go && go test ./internal/providers/schwab/ -v -run TestHandleCallback` — all 16 callback tests should pass. Then `go test ./...` for full suite.

---

## Step 3 — Frontend: Add `/callback` route, `OAuthCallback.vue` view, and API method

**Goal:** When Schwab redirects to `https://juicytrade.muxpie.com/callback?code=...&state=...`, the Vue router captures the request, renders a callback view that relays the code to the backend API, and communicates the result back to the opener window.

**Files modified: 2 | Files created: 1**

### File: `trade-app/src/services/api.js` (~+8 lines)

Add a new method to the `api` object, in the `// === Schwab OAuth Flow APIs ===` section (after `selectSchwabAccount`, around line 714):

```js
async relaySchwabOAuthCallback(code, state) {
  const params = new URLSearchParams();
  if (code) params.append('code', code);
  if (state) params.append('state', state);
  const response = await axios.get(`${API_BASE_URL}/providers/schwab/oauth/callback?${params.toString()}`);
  return response.data;
},

async relaySchwabOAuthCallbackError(error, state) {
  const params = new URLSearchParams();
  params.append('error', error);
  if (state) params.append('state', state);
  const response = await axios.get(`${API_BASE_URL}/providers/schwab/oauth/callback?${params.toString()}`);
  return response.data;
},
```

**Note:** Uses `GET` to match the existing `HandleCallback` method signature (it reads query params). The `axios.get` call includes query params in the URL string directly.

### File: `trade-app/src/views/OAuthCallback.vue` — **NEW FILE** (~120 lines)

This view is displayed in the popup window after Schwab redirects to the callback URL.

**Component structure:**

```
<template>
  - Dark-themed full-page centered card (matching app's #1a1a2e background)
  - Loading state: spinner + "Completing authorization..."
  - Success state: checkmark icon + "Authorization Successful" + "This window will close automatically"
  - Error state: error icon + error message + "Close" button
</template>

<script setup>
  import { ref, onMounted } from 'vue'
  import { useRoute } from 'vue-router'
  import { api } from '@/services/api.js'
</script>
```

**`onMounted` logic:**

1. Extract query params from the route: `code`, `state`, `error`
2. If `error` param is present (user cancelled on Schwab):
   - Call `api.relaySchwabOAuthCallbackError(error, state)` to notify the backend
   - Set status to `cancelled`
   - Notify opener via `postMessage` (see below)
   - Auto-close after 2 seconds
3. If `code` and `state` are present:
   - Set status to `loading`
   - Call `api.relaySchwabOAuthCallback(code, state)`
   - On success (response `status === 'success'`):
     - Set status to `success`
     - Notify opener: `window.opener?.postMessage({ type: 'schwab-oauth-callback', status: 'success' }, window.location.origin)`
     - Auto-close after 2 seconds: `setTimeout(() => window.close(), 2000)`
   - On error (catch block or response `status === 'error'`):
     - Set status to `error` with error message from response
     - Notify opener: `window.opener?.postMessage({ type: 'schwab-oauth-callback', status: 'error', error: errorMsg }, window.location.origin)`
     - Show "Close" button (do not auto-close on error)
4. If neither `code` nor `error` is present:
   - Set status to `error` with message "Missing authorization parameters"

**Styling:** Use inline `<style scoped>` matching the existing dark theme CSS variables from `theme.css`. The card should look similar to the removed `renderCallbackPage` HTML but using the app's design language.

**Security:** Use `window.location.origin` as the `targetOrigin` in `postMessage` (not `*`), so the message is only delivered to the same origin.

### File: `trade-app/src/router/index.js` (~+12 lines)

1. Add the `/callback` route to the `routes` array. Place it after the `/login` route (since both are auth-exempt):

   ```js
   {
     path: '/callback',
     name: 'OAuthCallback',
     component: () => import('../views/OAuthCallback.vue'),
     meta: {
       title: 'Authorization - JuicyTrade',
       requiresAuth: false,
       requiresSetup: false,
     },
   },
   ```

   **Key meta fields:**
   - `requiresAuth: false` — the callback is hit during OAuth before the user may be authenticated in JuicyTrade
   - `requiresSetup: false` — the callback is hit during provider setup, which is the setup flow itself

2. Using lazy import (`() => import(...)`) since this view is only needed during the OAuth flow.

**Verification:** `cd trade-app && npm run build` should succeed. Manually verify that navigating to `http://localhost:3001/callback?code=test&state=test` renders the OAuthCallback view (it will show an error since the state token is invalid, but the route should work).

---

## Step 4 — Frontend: Change `window.open` to popup and add `postMessage` listener

**Goal:** The OAuth flow opens in a popup window (not a new tab) that can communicate back to the opener via `postMessage`. The opener listens for the completion message and updates the UI immediately.

**Files modified: 1**

### File: `trade-app/src/components/settings/ProvidersTab.vue` (~+30 lines, ~-2 lines)

#### 4a. Change `window.open` call in `startOAuth()` (line 1033)

**Current:**
```js
window.open(result.auth_url, '_blank', 'noopener');
```

**New:**
```js
window.open(result.auth_url, 'schwab-oauth', 'width=600,height=700,popup=yes');
```

**Changes:**
- Window name `'schwab-oauth'` instead of `'_blank'` — reuses the same popup if one is already open
- `'width=600,height=700,popup=yes'` — opens as a popup window with specified dimensions, not a tab
- **Removed `noopener`** — this is critical so that `window.opener` is available in the popup, enabling `postMessage` communication back to the opener

#### 4b. Add `postMessage` event listener

Add a `message` event handler that listens for `schwab-oauth-callback` messages from the popup. Add this inside `startOAuth()`, right after the `window.open()` call:

```js
// Listen for completion message from popup
const handleOAuthMessage = (event) => {
  if (event.origin !== window.location.origin) return;
  if (event.data?.type !== 'schwab-oauth-callback') return;
  
  window.removeEventListener('message', handleOAuthMessage);
  
  // The popup has relayed the code to the backend and it was processed.
  // Force an immediate poll instead of waiting for the next 2-second cycle.
  // The poll will pick up the 'completed' or 'failed' status.
  if (oauthPollTimer) {
    clearInterval(oauthPollTimer);
  }
  pollOAuthStatus();
};
window.addEventListener('message', handleOAuthMessage);
```

**Design decision:** The `postMessage` listener does NOT directly update `oauthStatus` based on the message. Instead, it triggers an immediate poll cycle. This keeps the existing polling logic as the single source of truth for status updates, avoiding duplicated logic and race conditions. The `postMessage` simply eliminates the up-to-2-second delay between the popup closing and the opener detecting the status change.

#### 4c. Clean up listener on cancel/close

In the `cancelOAuth()` function (line 1138), add cleanup for the message listener. Store the listener reference as a module-scoped variable:

```js
let oauthMessageHandler = null;  // alongside the existing oauthPollTimer
```

Update `startOAuth()` to store the handler, and `cancelOAuth()` to remove it:
```js
// In cancelOAuth():
if (oauthMessageHandler) {
  window.removeEventListener('message', oauthMessageHandler);
  oauthMessageHandler = null;
}
```

#### 4d. Update the OAuth hint text (line 403)

**Current:**
```html
<p class="oauth-hint">Opens Schwab login in a new window. Authorize access, then return here.</p>
```

**New:**
```html
<p class="oauth-hint">Opens Schwab login in a popup. Authorize access — this page updates automatically.</p>
```

**Note:** `startReconnect()` (line 1107) does NOT call `window.open` directly. It pre-fills the provider dialog and shows it at step 3, which then lets the user click "Connect to Schwab" → `startOAuth()`. So the popup change in `startOAuth()` automatically covers the reconnect flow too.

**Verification:** `cd trade-app && npm run build` should succeed. Manual test: clicking "Connect to Schwab" should open a popup window (not a new tab).

---

## Step 5 — Tests: Frontend unit tests and full test suite validation

**Goal:** Add frontend tests for the new `OAuthCallback.vue` view and verify all existing tests pass across both backend and frontend.

**Files modified: 0 | Files created: 1 | Tests: 5–8 new**

### File: `trade-app/tests/OAuthCallback.test.js` — **NEW FILE**

**Test cases:**

1. **`renders loading state and calls API with code and state params`**
   - Mount `OAuthCallback` with route query `{ code: 'test-code', state: 'test-state' }`
   - Mock `api.relaySchwabOAuthCallback` to return `{ status: 'success' }`
   - Assert the component calls the API with the correct params
   - Assert the component shows success state after the API resolves

2. **`shows error when API returns error response`**
   - Mount with route query `{ code: 'test-code', state: 'test-state' }`
   - Mock `api.relaySchwabOAuthCallback` to throw with `{ response: { data: { error: 'Token exchange failed' } } }`
   - Assert the component shows the error message

3. **`handles user cancellation (error param)`**
   - Mount with route query `{ error: 'access_denied', state: 'test-state' }`
   - Mock `api.relaySchwabOAuthCallbackError`
   - Assert the component shows cancellation message

4. **`sends postMessage to opener on success`**
   - Mount with `window.opener` set to a mock object
   - Mock `api.relaySchwabOAuthCallback` to return `{ status: 'success' }`
   - Assert `window.opener.postMessage` was called with `{ type: 'schwab-oauth-callback', status: 'success' }`

5. **`shows error when no code or error params`**
   - Mount with empty route query `{}`
   - Assert the component shows "Missing authorization parameters" error
   - Assert no API call was made

6. **`calls window.close after success`**
   - Mount with valid params, mock `window.close`
   - Assert `window.close` is called after a timeout (use `vi.useFakeTimers`)

**Test setup:**
- Use `@vue/test-utils` `mount` with `global.plugins: [router]` or mock `useRoute`
- Mock `api.relaySchwabOAuthCallback` and `api.relaySchwabOAuthCallbackError` via `vi.mock('@/services/api.js')`
- Mock `window.opener` and `window.close` using `vi.stubGlobal`

### Full test suite validation

Run all tests to verify zero regressions:

```bash
# Backend (292 existing tests + updated callback tests)
cd trade-backend-go && go test ./... -count=1

# Frontend (28 existing test files + 1 new)
cd trade-app && npx vitest run
```

**Expected results:**
- All 292+ backend tests pass (the 16 callback tests now validate JSON instead of HTML, 3 renderCallbackPage tests are deleted)
- All frontend tests pass including the new OAuthCallback tests
- `npm run build` succeeds with no errors

---

## Summary

| Step | Files | Type | Description |
|------|-------|------|-------------|
| 1 | `main.go`, `oauth.go` | Backend | Move callback route to `/api`, return JSON, remove `renderCallbackPage` |
| 2 | `oauth_test.go`, `qa_oauth_edge_test.go` | Backend tests | Update 16 tests from HTML to JSON assertions, delete 3 `renderCallbackPage` tests |
| 3 | `api.js`, `OAuthCallback.vue` (new), `router/index.js` | Frontend | Add callback relay API, callback view, and `/callback` route |
| 4 | `ProvidersTab.vue` | Frontend | Change to popup window, add `postMessage` listener |
| 5 | `OAuthCallback.test.js` (new) | Tests | Frontend tests for callback view, full suite validation |

**Total: 7 files modified, 2 files created, ~19 tests updated/added, 3 tests deleted**
