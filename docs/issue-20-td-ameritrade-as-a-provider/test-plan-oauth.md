# QA Test Plan: Schwab OAuth Authorization Flow

**Issue:** #20 (TD Ameritrade as a Provider) — OAuth Enhancement
**Requirements:** [requirements-oauth-flow.md](./requirements-oauth-flow.md)
**Architecture:** [architecture-oauth-flow.md](./architecture-oauth-flow.md)
**Implementation Plan:** [implementation-plan-oauth.md](./implementation-plan-oauth.md)
**Date:** 2026-03-17
**Status:** Draft

---

## Overview

This test plan covers all 17 acceptance criteria (AC-1 through AC-17) and all 4 non-functional requirements (NFR-1 through NFR-4) for the Schwab OAuth Authorization Flow enhancement. Each step is a focused, independently executable QA task.

**Test execution approach:** For each step, run the existing developer tests first, review the implementation code against the architecture specification, then write and run any additional QA tests needed to close coverage gaps.

---

## Step 1 — CredentialStore.UpdateCredentialFields

**Acceptance Criteria:** AC-13 (credentials stored in `provider_credentials.json`), AC-14 (atomic credential writes)

**Source Files:**
- `trade-backend-go/internal/providers/credential_store.go` (lines 200–226)
- `trade-backend-go/internal/providers/credential_store_test.go`

**Checks to Perform:**

1. **Run existing tests:**
   ```
   cd trade-backend-go && go test ./internal/providers/... -run TestUpdateCredentialFields -v
   ```
2. **Code review — verify method signature matches architecture** (arch §6.3):
   - Accepts `instanceID string` and `fieldUpdates map[string]interface{}`
   - Returns `error`
   - Performs shallow merge into the `"credentials"` sub-map, not top-level keys
   - Sets `"updated_at"` timestamp after merge
   - Calls `saveCredentials()` to persist
3. **Verify existing test coverage** — the following 5 tests exist and must pass:
   - `TestUpdateCredentialFields_Success` — single field update preserves others
   - `TestUpdateCredentialFields_MultipleFields` — 2 field update, others preserved
   - `TestUpdateCredentialFields_NonExistentInstance` — returns error
   - `TestUpdateCredentialFields_NilCredentials` — creates sub-map if absent
   - `TestUpdateCredentialFields_Persistence` — verifies on-disk JSON and reload

**Edge Cases to Verify:**
- Updating with an empty `fieldUpdates` map — should succeed without error and still update `updated_at`
- Updating a field to an empty string `""` — should store the empty string (not remove the key)
- Concurrent calls to `UpdateCredentialFields` for different instances — no data corruption

**New QA Tests to Write:**
- `TestUpdateCredentialFields_EmptyUpdates` — call with `map[string]interface{}{}`, verify no error and `updated_at` changes
- `TestUpdateCredentialFields_OverwriteToEmpty` — set a field to `""`, verify it persists as `""`
- `TestUpdateCredentialFields_DoesNotClobberTopLevel` — verify that calling `UpdateCredentialFields` does NOT modify top-level keys like `provider_type`, `display_name`, `active` (only touches the `credentials` sub-map)

---

## Step 2 — Provider Struct Changes (instanceID, credentialUpdater, authExpired)

**Acceptance Criteria:** AC-7 (token rotation persisted), AC-8 (auth expired surfaced)

**Source Files:**
- `trade-backend-go/internal/providers/schwab/schwab.go` (lines 21, 28–104)
- `trade-backend-go/internal/providers/schwab/auth.go` (lines 50–129)
- `trade-backend-go/internal/providers/schwab/schwab_test.go`
- `trade-backend-go/internal/providers/schwab/auth_test.go`

**Checks to Perform:**

1. **Run existing tests:**
   ```
   cd trade-backend-go && go test ./internal/providers/schwab/... -v
   ```
2. **Code review — CredentialUpdater type** (arch §6.2):
   - Defined as `type CredentialUpdater func(instanceID string, updates map[string]interface{}) error` in `schwab.go:21`
   - Not an interface — a function type (lightweight, mockable)
3. **Code review — struct fields** (arch §7.1):
   - `instanceID string` on `SchwabProvider` struct (line 41)
   - `credentialUpdater CredentialUpdater` (line 42, may be nil)
   - `authExpired bool` (line 45)
4. **Code review — constructor** (arch §7.2):
   - `NewSchwabProvider` accepts `instanceID string` and `credentialUpdater CredentialUpdater` as trailing params
   - Both stored on the struct
   - Nil `credentialUpdater` is valid (no panic on construction)
5. **Code review — `refreshAccessToken` token rotation** (arch §6.6, auth.go:110–126):
   - On rotation: always updates in-memory `s.refreshToken`
   - If `credentialUpdater != nil && instanceID != ""`: calls updater with `{"refresh_token": newToken}`
   - If updater is nil: logs warning, does NOT panic
   - If updater returns error: logs error, does NOT return error (best-effort persistence)
6. **Code review — `refreshAccessToken` auth expired** (auth.go:82–84):
   - On 401 from token endpoint: sets `s.authExpired = true`
   - Returns `ErrRefreshTokenExpired`
7. **Code review — `TestCredentials` auth expired** (schwab.go:141–148):
   - If `s.authExpired == true`: returns `{"success": false, "auth_expired": true, "message": "Refresh token expired..."}`
   - Short-circuits before any HTTP call

**Existing Test Coverage (must all pass):**
- `TestNewSchwabProvider` — basic constructor
- `TestNewSchwabProvider_Defaults` — default baseURL and accountType
- `TestNewSchwabProvider_WithUpdater` — instanceID + updater stored
- `TestNewSchwabProvider_NilUpdater` — nil updater, no panic
- `TestRefreshAccessToken_TokenRotation` — in-memory rotation (nil updater)
- `TestRefreshAccessToken_RotatesAndPersists` — updater called with correct args
- `TestRefreshAccessToken_RotatesPersistFailure` — updater error, in-memory still updated
- `TestRefreshAccessToken_SetsAuthExpiredOn401` — authExpired set to true
- `TestRefreshAccessToken_ExpiredRefreshToken` — ErrRefreshTokenExpired returned
- `TestTestCredentials_AuthExpired` — short-circuit on authExpired

**Edge Cases to Verify:**
- Token rotation when `instanceID` is empty but `credentialUpdater` is non-nil — should NOT call updater (guarded by `s.instanceID != ""`)
- Token rotation when `instanceID` is set but `credentialUpdater` is nil — should NOT call updater
- `authExpired` remains `true` across multiple calls — never auto-clears

**New QA Tests to Write:**
- `TestRefreshAccessToken_EmptyInstanceID_SkipsUpdater` — set `credentialUpdater` to a non-nil function but `instanceID` to `""` → verify updater is NOT called
- `TestAuthExpired_DoesNotAutoClear` — set `authExpired = true`, call `ensureValidToken()`, verify it still returns `ErrRefreshTokenExpired` and `authExpired` remains `true`
- `TestAllConstructorCallSites_Compile` — verify `go build ./...` succeeds (all 12 test files updated with new constructor args)

---

## Step 3 — Provider Types and Credential Field Changes

**Acceptance Criteria:** AC-1 (credential form shows only App Key, App Secret, Callback URL, Base URL)

**Source Files:**
- `trade-backend-go/internal/providers/provider_types.go` (lines 7–15, 26–33, 130–153)
- `trade-backend-go/internal/providers/provider_types_test.go`

**Checks to Perform:**

1. **Run existing tests:**
   ```
   cd trade-backend-go && go test ./internal/providers/... -run TestProviderTypes -v
   ```
2. **Code review — ProviderType struct** (arch §7.6):
   - `AuthMethod string \`json:"auth_method,omitempty"\`` field exists (line 32)
   - Field omits from JSON when empty — verify with JSON marshaling
3. **Code review — CredentialField struct**:
   - `HelpText string \`json:"help_text,omitempty"\`` field exists (line 14)
4. **Code review — Schwab provider type entry** (arch §7.5):
   - `AuthMethod: "oauth"` set on schwab entry (line 134)
   - Live and paper each have exactly 4 fields: `app_key`, `app_secret`, `callback_url`, `base_url`
   - `refresh_token` and `account_hash` are NOT in the field list
   - `callback_url` has `Default: "https://127.0.0.1/callback"`
   - All 4 fields have `HelpText` set
5. **Code review — other providers unchanged**:
   - Alpaca, Tradier, TastyTrade, Public — `AuthMethod` is empty string (default)
   - Their credential fields are identical to before this enhancement

**Existing Test Coverage (must all pass):**
- `TestProviderTypes_SchwabAuthMethod` — AuthMethod == "oauth"
- `TestProviderTypes_SchwabCredentialFields` — 4 fields, no refresh_token/account_hash
- `TestProviderTypes_SchwabCallbackDefault` — callback_url default value
- `TestProviderTypes_SchwabHelpText` — HelpText on all 4 fields
- `TestProviderTypes_OtherProvidersUnchanged` — other providers have empty AuthMethod

**Edge Cases to Verify:**
- `ValidateCredentials("schwab", "live", creds)` — should only require `app_key` and `app_secret` and `callback_url` (not `refresh_token` or `account_hash`)
- `ApplyDefaults("schwab", "live", {})` — should fill `callback_url` and `base_url` defaults
- `IsSensitiveField("app_key")` — should return true (contains "api_key" pattern? No, but `app_key` may not match). Verify `app_secret` is sensitive.
- JSON serialization of `AuthMethod: ""` — should NOT emit `"auth_method"` key (omitempty)

**New QA Tests to Write:**
- `TestValidateCredentials_SchwabLive` — pass empty map → expect errors for app_key, app_secret, callback_url only (NOT for refresh_token or account_hash)
- `TestApplyDefaults_SchwabLive` — verify callback_url and base_url defaults applied
- `TestProviderType_AuthMethod_JSONOmitEmpty` — marshal Alpaca ProviderType to JSON → verify no `auth_method` key; marshal Schwab → verify `"auth_method":"oauth"` present
- `TestIsSensitiveField_SchwabFields` — verify `app_secret` returns true; `app_key` behavior documented

---

## Step 4 — OAuth State Store

**Acceptance Criteria:** NFR-1 (security — CSRF state tokens), NFR-2 (timeout — 10-minute TTL), NFR-3 (idempotency — duplicate state handling)

**Source Files:**
- `trade-backend-go/internal/providers/schwab/oauth.go` (lines 20–191)
- `trade-backend-go/internal/providers/schwab/oauth_test.go` (lines 24–302)

**Checks to Perform:**

1. **Run existing tests with race detector:**
   ```
   cd trade-backend-go && go test ./internal/providers/schwab/... -run "TestOAuthStore|TestStateToken|TestConcurrent" -v -race
   ```
2. **Code review — OAuthFlowState struct** (arch §4.2):
   - Sensitive fields (`AppKey`, `AppSecret`, `RefreshToken`, `AccessToken`, `TokenExpiry`) tagged `json:"-"`
   - `mu sync.Mutex` tagged `json:"-"`
   - Status flow: pending → exchanging → completed → finalized | failed
3. **Code review — state token generation** (arch §4.4):
   - 32 bytes from `crypto/rand` → base64 RawURLEncoding → 43 characters
   - URL-safe (no padding, no `+` or `/`)
4. **Code review — TTL enforcement** (arch §4.5):
   - `oauthStateTTL = 10 * time.Minute` (line 66)
   - `GetState()` returns nil for expired states and deletes them
   - Cleanup goroutine runs every 60 seconds, removes expired entries
5. **Code review — concurrency** (arch §4.6):
   - `sync.Map` for outer store — thread-safe
   - `sync.Mutex` per state — protects state transitions in `UpdateState`

**Existing Test Coverage (must all pass):**
- `TestOAuthStore_CreateState_GeneratesUniqueTokens`
- `TestOAuthStore_CreateState_StoresCorrectData`
- `TestOAuthStore_GetState_NotFound`
- `TestOAuthStore_GetState_Expired`
- `TestOAuthStore_UpdateState_TransitionsStatus`
- `TestOAuthStore_UpdateState_NotFound`
- `TestOAuthStore_DeleteState`
- `TestOAuthStore_StartCleanup_RemovesExpired`
- `TestOAuthStore_StartCleanup_ContextCancel`
- `TestOAuthStore_StateToken_Length`
- `TestOAuthStore_ConcurrentAccess`

**Edge Cases to Verify:**
- State at exactly 10 minutes old — boundary condition (should still be valid or expired?)
- State at 10 minutes + 1 second — must be expired
- `GetState` after `DeleteState` — must return nil
- `UpdateState` on expired state — must return false
- Empty string as state token — `GetState("")` must return nil

**New QA Tests to Write:**
- `TestOAuthFlowState_JSONExcludesSensitive` — marshal `OAuthFlowState` to JSON, verify `AppKey`, `AppSecret`, `RefreshToken`, `AccessToken` are NOT in output (security NFR-1)
- `TestOAuthStore_GetState_EmptyToken` — `GetState("")` returns nil
- `TestOAuthStore_TTLBoundary` — create state, set `CreatedAt` to exactly 10 min ago, verify `GetState` returns nil (TTL is exclusive: `time.Since > oauthStateTTL`)
- `TestOAuthStore_UpdateState_Expired` — create state, backdate to 15 min, call `UpdateState` → returns false

---

## Step 5 — Token Exchange and Account Fetch Helpers

**Acceptance Criteria:** AC-4 (backend exchanges code for tokens), AC-17 (no accounts shows error)

**Source Files:**
- `trade-backend-go/internal/providers/schwab/oauth.go` (lines 196–420)
- `trade-backend-go/internal/providers/schwab/oauth_test.go` (lines 308–571)

**Checks to Perform:**

1. **Run existing tests:**
   ```
   cd trade-backend-go && go test ./internal/providers/schwab/... -run "TestExchange|TestFetch|TestMask|TestRender" -v
   ```
2. **Code review — `exchangeCodeForTokens`** (arch §5.3):
   - POST to `{baseURL}/v1/oauth/token`
   - Content-Type: `application/x-www-form-urlencoded`
   - Basic Auth: `base64(appKey:appSecret)` via `req.SetBasicAuth()`
   - Form body: `grant_type=authorization_code`, `code=...`, `redirect_uri=...`
   - 30-second timeout
   - Returns descriptive errors for 400, 401, non-200 status codes
   - Validates `access_token` is non-empty
3. **Code review — `fetchAccountNumbers`** (arch §5.4):
   - GET `{baseURL}/trader/v1/accounts/accountNumbers`
   - Authorization: Bearer token
   - 15-second timeout
   - Masks account numbers via `maskAccountNumber`
   - Returns empty slice (not nil) for empty array response
4. **Code review — `maskAccountNumber`**:
   - `""` → `""`
   - `"1234"` → `"1234"` (4 or fewer chars: no masking)
   - `"123456789"` → `"*****6789"`
5. **Code review — `renderCallbackPage`**:
   - Returns self-contained HTML (no external dependencies)
   - Three states: success (green), error (red), cancelled (yellow)
   - Contains page title "JuicyTrade - Schwab Authorization"

**Existing Test Coverage:**
- `TestExchangeCodeForTokens_Success` — valid tokens parsed
- `TestExchangeCodeForTokens_InvalidCode` — 400 error
- `TestExchangeCodeForTokens_InvalidCredentials` — 401 error
- `TestExchangeCodeForTokens_BasicAuthHeader` — Basic auth sent
- `TestExchangeCodeForTokens_FormBody` — correct form fields
- `TestFetchAccountNumbers_SingleAccount` — 1 account, masked
- `TestFetchAccountNumbers_MultipleAccounts` — 3 accounts, all masked
- `TestFetchAccountNumbers_EmptyAccounts` — empty slice returned
- `TestMaskAccountNumber` — 7 edge cases
- `TestRenderCallbackPage_Success/Error/Cancelled` — HTML content

**Edge Cases to Verify:**
- Token response with `refresh_token` empty string — should still succeed
- Token response with `expires_in: 0` — should succeed (expiry = now)
- Account numbers API returning 401 — should return error
- Account numbers API returning malformed JSON — should return error
- Very long error messages in `renderCallbackPage` — no XSS (error message is not escaped in current template — verify)

**New QA Tests to Write:**
- `TestExchangeCodeForTokens_Timeout` — use a server that delays 35s → verify timeout error
- `TestFetchAccountNumbers_Unauthorized` — server returns 401 → verify descriptive error
- `TestFetchAccountNumbers_MalformedJSON` — server returns `{broken` → verify error
- `TestRenderCallbackPage_ErrorMessageEscaping` — pass `<script>alert(1)</script>` as error message → verify it's rendered literally in HTML (check for XSS vulnerability — if unescaped, flag as bug)
- `TestFetchAccountNumbers_BearerTokenSent` — verify Authorization header contains `Bearer {token}`

---

## Step 6 — HandleAuthorize Endpoint

**Acceptance Criteria:** AC-2 ("Connect to Schwab" initiates flow), AC-3 (opens Schwab auth page)

**Source Files:**
- `trade-backend-go/internal/providers/schwab/oauth.go` (lines 457–518)
- `trade-backend-go/internal/providers/schwab/oauth_test.go` (lines 610–777)

**Checks to Perform:**

1. **Run existing tests:**
   ```
   cd trade-backend-go && go test ./internal/providers/schwab/... -run TestHandleAuthorize -v
   ```
2. **Code review — request validation** (arch §5.1):
   - `app_key`, `app_secret`, `callback_url` are required (binding:"required")
   - `callback_url` must start with `https://`
   - `base_url` defaults to `https://api.schwabapi.com` if omitted
   - `instance_id` is optional (for re-auth)
3. **Code review — authorization URL construction**:
   - Format: `{base_url}/v1/oauth/authorize?client_id={app_key}&redirect_uri={callback_url}&response_type=code&state={state_token}`
   - All parameters URL-encoded
   - State token is 43 characters (32 bytes base64url)
4. **Code review — response format**:
   - 200 JSON: `{"auth_url": "...", "state": "..."}`
5. **Code review — re-auth support** (arch §4.7):
   - If `instance_id` provided, stored in state's `ExistingInstanceID`

**Existing Test Coverage:**
- `TestHandleAuthorize_Success` — 200, auth_url + state present
- `TestHandleAuthorize_MissingAppKey` — 400
- `TestHandleAuthorize_MissingAppSecret` — 400
- `TestHandleAuthorize_DefaultBaseURL` — default api.schwabapi.com
- `TestHandleAuthorize_WithInstanceID` — ExistingInstanceID stored
- `TestHandleAuthorize_AuthURLFormat` — URL parsing, encoding, query params

**Edge Cases to Verify:**
- `callback_url` with `http://` (not https) — should return 400
- Missing `callback_url` — should return 400 (binding required)
- `base_url` with trailing slash — verify auth URL is well-formed
- Very long `app_key` — should be URL-encoded correctly
- JSON body with extra unknown fields — should be ignored (gin default behavior)

**New QA Tests to Write:**
- `TestHandleAuthorize_HttpCallbackURL` — pass `http://localhost/callback` → verify 400 error about https
- `TestHandleAuthorize_MissingCallbackURL` — omit callback_url → verify 400
- `TestHandleAuthorize_StateStoredCorrectly` — after successful authorize, retrieve state from store → verify all fields match request
- `TestHandleAuthorize_TrailingSlashBaseURL` — pass `https://api.schwabapi.com/` → verify auth_url is not double-slashed

---

## Step 7 — HandleCallback Endpoint

**Acceptance Criteria:** AC-4 (code exchanged for tokens), AC-10 (`GET /callback` exists), AC-11 (error cases handled), AC-15 (cancellation handled), AC-16 (exchange failure shows error)

**Source Files:**
- `trade-backend-go/internal/providers/schwab/oauth.go` (lines 520–635)
- `trade-backend-go/internal/providers/schwab/oauth_test.go` (lines 782–1064)

**Checks to Perform:**

1. **Run existing tests:**
   ```
   cd trade-backend-go && go test ./internal/providers/schwab/... -run TestHandleCallback -v
   ```
2. **Code review — flow steps** (arch §5.2):
   - Extract `code`, `state`, `error` from query params
   - If `error` param present → state="failed", render cancelled HTML
   - If `state` missing or invalid → render error HTML
   - Atomically transition `pending → exchanging` (prevents duplicates — NFR-3)
   - If `code` empty → state="failed", render error HTML
   - Exchange code for tokens → on failure: state="failed", render error HTML
   - Fetch account numbers → on failure: state="failed", render error HTML
   - If 0 accounts → state="failed", render error HTML (AC-17)
   - On success → state="completed" with tokens + accounts, render success HTML
3. **Code review — response type**:
   - Returns HTML (not JSON) — `c.Data(200, "text/html; charset=utf-8", ...)`
   - Always returns 200 status (even on error) — appropriate since this is a browser redirect target
4. **Code review — duplicate processing prevention** (NFR-3):
   - Checks `s.Status != "pending"` → `alreadyProcessed = true`
   - Second callback with same state → "already been processed" error

**Existing Test Coverage:**
- `TestHandleCallback_Success_SingleAccount` — full success flow
- `TestHandleCallback_Success_MultipleAccounts` — 3 accounts stored
- `TestHandleCallback_UserCancelled` — `?error=access_denied`
- `TestHandleCallback_InvalidState` — unknown state token
- `TestHandleCallback_DuplicateCallback` — second call rejected (NFR-3)
- `TestHandleCallback_TokenExchangeFails` — 401 from token endpoint
- `TestHandleCallback_AccountFetchFails` — 500 from accounts endpoint
- `TestHandleCallback_NoAccounts` — empty accounts array (AC-17)
- `TestHandleCallback_MissingCode` — state valid but no code
- `TestHandleCallback_MissingState` — no state parameter at all

**Edge Cases to Verify:**
- Both `error` and `code` query params present — `error` should take precedence
- State in "exchanging" status (in-progress) and callback received again — should show "already processed"
- State in "completed" status and callback received — should show "already processed"
- Expired state token in callback — should show "Invalid or expired"
- Empty `error` query param (`?error=&state=...`) — should be treated as error present

**New QA Tests to Write:**
- `TestHandleCallback_ErrorTakesPrecedenceOverCode` — send `?code=xyz&error=denied&state=...` → verify "cancelled" HTML (not success)
- `TestHandleCallback_ExpiredState` — create state, backdate to 15 min, call callback → verify "Invalid or expired" HTML
- `TestHandleCallback_EmptyErrorParam` — send `?error=&state=...` → verify behavior (should this be treated as error or normal flow? Document and test)
- `TestHandleCallback_HTMLContentType` — verify response Content-Type is `text/html; charset=utf-8`

---

## Step 8 — HandleOAuthStatus and HandleSelectAccount

**Acceptance Criteria:** AC-5 (account selection UI), AC-6 (credentials persisted), AC-9 (re-auth reuses credentials)

**Source Files:**
- `trade-backend-go/internal/providers/schwab/oauth.go` (lines 637–782)
- `trade-backend-go/internal/providers/schwab/oauth_test.go` (lines 1066–1488)

**Checks to Perform:**

1. **Run existing tests:**
   ```
   cd trade-backend-go && go test ./internal/providers/schwab/... -run "TestHandleOAuthStatus|TestHandleSelectAccount" -v
   ```
2. **Code review — HandleOAuthStatus** (arch §5.5):
   - GET with `:state` path param
   - State not found → 404 JSON
   - Returns JSON: `{"status": "pending|exchanging|completed|failed"}`
   - Completed → includes `"accounts"` array
   - Failed → includes `"error"` string
3. **Code review — HandleSelectAccount** (arch §5.6):
   - POST with JSON body: `state` (required), `account_hash` (required), `provider_name`, `account_type`
   - State must be in "completed" status
   - Account hash must match one of the state's accounts
   - **New provider path:** calls `GenerateInstID`, `AddInstance`, `ReinitInstance`; requires `provider_name`
   - **Re-auth path:** calls `GetInstance`, `UpdateCredFields`, `ReinitInstance`; `provider_name` not required
   - State is deleted after successful finalization
   - Credentials map includes: `app_key`, `app_secret`, `callback_url`, `base_url`, `refresh_token`, `account_hash`
4. **Code review — dependency injection**:
   - All 5 deps (`AddInstance`, `UpdateCredFields`, `GenerateInstID`, `ReinitInstance`, `GetInstance`) are closures — verify mock tests use `mockDeps` struct for assertions

**Existing Test Coverage:**
- Status: `TestHandleOAuthStatus_Pending`, `_Completed`, `_Failed`, `_NotFound`, `_Expired`
- SelectAccount: `TestHandleSelectAccount_NewProvider`, `_ReAuth`, `_InvalidState`, `_WrongStatus`, `_InvalidAccountHash`, `_MissingProviderName`, `_DefaultAccountType`, `_StateDeletedAfterSuccess`, `_ReAuthInstanceNotFound`

**Edge Cases to Verify:**
- `HandleSelectAccount` with `AddInstance` returning `false` — should return 500
- `HandleSelectAccount` with `ReinitInstance` returning error on new provider — should still return success (per code: logged as warning only, provider available after restart)
- `HandleSelectAccount` with `UpdateCredFields` returning error on re-auth — should return 500
- `HandleSelectAccount` called twice with same state — second call should fail (state was deleted)
- Account hash containing special characters — should match exactly

**New QA Tests to Write:**
- `TestHandleSelectAccount_AddInstanceFails` — mock `AddInstance` returns false → verify 500 response
- `TestHandleSelectAccount_ReinitFailsOnNew` — mock `ReinitInstance` returns error for new provider → verify 200 success still returned (non-fatal)
- `TestHandleSelectAccount_DoubleFinalize` — finalize once (success), try to finalize again with same state → verify 400 (state deleted)
- `TestHandleSelectAccount_CredentialsMapComplete` — verify `addInstanceCredentials` contains all 6 expected keys: `app_key`, `app_secret`, `callback_url`, `base_url`, `refresh_token`, `account_hash`
- `TestHandleOAuthStatus_Exchanging` — create state in "exchanging" status → verify response `{"status": "exchanging"}`

---

## Step 9 — Route Registration in main.go

**Acceptance Criteria:** AC-10 (`GET /callback` route exists), AC-12 (callback excluded from auth middleware)

**Source Files:**
- `trade-backend-go/cmd/server/main.go` (lines 64–87 handler creation, lines 381–384 API routes, lines 386–387 auth middleware, lines 396–397 root callback)

**Checks to Perform:**

1. **Build verification:**
   ```
   cd trade-backend-go && go build ./cmd/server/...
   ```
2. **Code review — handler instantiation** (main.go:64–87):
   - `schwab.NewSchwabOAuthHandler(schwab.OAuthHandlerDeps{...})` called with 5 closures
   - Each closure creates a fresh `NewCredentialStore()` per call (safe for concurrent use)
   - `ReinitInstance` closure calls `providerManager.ReinitializeInstance(instanceID)`
   - Cleanup goroutine started: `go schwabOAuthHandler.StartCleanup(context.Background())`
3. **Code review — route placement** (arch §3.2):
   - Schwab OAuth API routes registered at lines 381–384, BEFORE auth middleware at line 386–387:
     - `POST /api/providers/schwab/authorize`
     - `GET /api/providers/schwab/oauth/status/:state`
     - `POST /api/providers/schwab/select-account`
   - Root callback route at line 396–397: `GET /callback`
   - These routes do NOT require JuicyTrade authentication (AC-12)
4. **Code review — no conflicts with existing routes**:
   - `/callback` does not conflict with any existing route
   - `/api/providers/schwab/*` is a new sub-group under existing `/api/providers` group

**Edge Cases to Verify:**
- `GET /callback` without any query params — should return HTML (error page), not 404
- `POST /api/providers/schwab/authorize` without auth header — should succeed (not blocked by auth middleware)
- `GET /api/providers/schwab/oauth/status/test-token` without auth — should return 404 JSON (not auth error)
- Existing routes still functional: `GET /api/providers/types`, `GET /api/providers/instances`
- `GET /callback` vs `POST /callback` — only GET should be registered

**New QA Tests to Write (integration):**
- `TestCallbackRoute_Accessible` — HTTP GET `/callback` returns 200 with HTML content-type
- `TestOAuthRoutes_NoAuthRequired` — POST `/api/providers/schwab/authorize` returns 400 (bad request, not 401 unauthorized)
- `TestOAuthStatusRoute_NoAuthRequired` — GET `/api/providers/schwab/oauth/status/fake` returns 404 (not 401)
- `TestExistingRoutes_Unaffected` — GET `/api/providers/types` still returns provider types including schwab
- Note: These integration tests may require spinning up a full Gin router. If not feasible, verify by code review + curl during manual testing.

---

## Step 10 — Frontend OAuth Flow

**Acceptance Criteria:** AC-2 ("Connect to Schwab" button available), AC-3 (opens Schwab auth in new tab), AC-5 (account selection UI), AC-8 (auth expired → "Reconnect")

**Source Files:**
- `trade-app/src/components/settings/ProvidersTab.vue` (lines 88–101, 374, 391–487, 575, 615–621, 930–942, 1010–1160, 1511–1526)
- `trade-app/src/services/api.js` (lines 699–714)

**Checks to Perform:**

1. **Code review — api.js methods** (arch §9, step 10a):
   - `initiateSchwabOAuth(data)` — POST to `/providers/schwab/authorize`
   - `getSchwabOAuthStatus(stateToken)` — GET to `/providers/schwab/oauth/status/${stateToken}`
   - `selectSchwabAccount(data)` — POST to `/providers/schwab/select-account`
   - All use correct HTTP methods and URL paths
2. **Code review — isOAuthProvider** (arch §8.3):
   - Returns true when `providerTypes[selectedType].auth_method === 'oauth'`
   - Only Schwab should match
3. **Code review — dialog flow** (arch §8.2):
   - Non-OAuth providers: Step 3 shows "Test & Save" button (`v-if="!isOAuthProvider"`)
   - OAuth providers: Step 3 shows "Connect to Schwab" button (`v-if="isOAuthProvider"`)
4. **Code review — startOAuth()** (ProvidersTab.vue:1010–1044):
   - Calls `api.initiateSchwabOAuth()` with credentials
   - Opens `auth_url` in new tab via `window.open()`
   - Sets `oauthStatus = 'pending'`
   - Starts 2-second polling interval
   - Passes `instance_id` for re-auth flows
5. **Code review — pollOAuthStatus()** (ProvidersTab.vue:1046–1078):
   - Polls every 2 seconds
   - On completed: stops polling, stores accounts, auto-selects if single account
   - On failed: stops polling, stores error
   - On 404: stops polling (expired state)
6. **Code review — finalizeOAuth()** (ProvidersTab.vue:1080–1105):
   - Calls `api.selectSchwabAccount()` with state, hash, name, type
   - On success: reload instances, close dialog
7. **Code review — cancelOAuth()** (ProvidersTab.vue:1138–1146):
   - Clears polling interval, resets all OAuth state
   - Called by closeDialog() (line 1148–1160)
8. **Code review — account selection UI** (ProvidersTab.vue:422–435):
   - Shows when `oauthStatus === 'completed' && oauthAccounts.length > 0`
   - Cards keyed by `hash_value`, display `account_number`
   - Click sets `selectedAccountHash`
9. **Code review — auth expired badge** (ProvidersTab.vue:88–101):
   - Shows when `instance.auth_expired === true` on Schwab instances
   - "Reconnect" button calls `startReconnect(instanceId, instance)`
10. **Code review — startReconnect()** (ProvidersTab.vue:1107–1136):
    - Pre-fills dialog with existing credentials (visible only — app_secret left empty)
    - Sets `editingInstance` to existing instance ID
    - Opens dialog at step 3

**Edge Cases to Verify:**
- Dialog closed during active polling — `cancelOAuth()` must clear interval (memory leak prevention)
- `window.open()` blocked by popup blocker — user should still see instructions
- Network error during polling — should show error and stop polling
- Multiple rapid clicks on "Connect to Schwab" — should not create multiple states
- Re-auth flow with `app_secret` empty — user must re-enter it before clicking Connect

**New QA Tests to Write:**
- Frontend unit tests are manual per the implementation plan. Recommend the following manual test scenarios:
  1. Add Schwab provider: Enter App Key + Secret + Callback URL → Click Connect → Verify new tab opens with Schwab URL
  2. Cancel flow: Start OAuth → Click Cancel → Verify polling stops and UI resets
  3. Account selection: Mock backend returning 3 accounts → Verify cards displayed, click selects correctly
  4. Auto-select: Mock backend returning 1 account → Verify auto-selected, "Create Provider" button enabled
  5. Error display: Mock backend returning failed status → Verify error message shown, "Try Again" button works
  6. Reconnect: Mock Schwab instance with `auth_expired: true` → Verify badge appears, click Reconnect opens dialog

---

## Step 11 — Manager.go Factory and ReinitializeInstance

**Acceptance Criteria:** AC-7 (token rotation persisted via credential updater wired in factory), AC-9 (re-auth reinitializes instance)

**Source Files:**
- `trade-backend-go/internal/providers/manager.go` (lines 90–158 factory, lines 553–581 reinitialize)

**Checks to Perform:**

1. **Run build:**
   ```
   cd trade-backend-go && go build ./...
   ```
2. **Code review — Schwab factory case** (manager.go:121–148):
   - Extracts `app_key`, `app_secret`, `callback_url`, `refresh_token`, `account_hash`, `base_url` from credentials
   - Defaults `baseURL` to `https://api.schwabapi.com`, `callbackURL` to `https://127.0.0.1/callback`
   - Creates `credUpdater` closure only when `instanceID != ""`
   - Closure creates fresh `NewCredentialStore()` and calls `UpdateCredentialFields`
   - Passes `instanceID` and `credUpdater` to `NewSchwabProvider`
3. **Code review — `createProviderInstance` signature** (manager.go:90):
   - Accepts `instanceID string` as 4th parameter
   - All callers pass instanceID correctly
4. **Code review — `ReinitializeInstance`** (manager.go:553–581):
   - Gets instance data from fresh `NewCredentialStore()`
   - Extracts provider type, account type, credentials
   - Applies defaults
   - Creates new provider via `createProviderInstance`
   - Swaps old provider under mutex lock
   - Logs success

**Edge Cases to Verify:**
- `ReinitializeInstance` with unknown instanceID — should return error
- `ReinitializeInstance` when provider type is not "schwab" — should work for any provider type
- Factory with empty `refresh_token` and `account_hash` (new OAuth flow, tokens not yet obtained) — provider should still construct (needed for `TestCredentials`)
- `credUpdater` closure creates a NEW `CredentialStore` each call — avoids stale data issues

**New QA Tests to Write:**
- `TestCreateProviderInstance_SchwabWithUpdater` — call factory with instanceID → verify returned provider has non-nil credentialUpdater (requires type assertion or behavioral test)
- `TestCreateProviderInstance_SchwabWithoutInstanceID` — call factory with empty instanceID → verify returned provider has nil credentialUpdater
- `TestReinitializeInstance_NotFound` — call with fake ID → verify error returned
- `TestReinitializeInstance_Success` — seed credential store with Schwab instance, create manager, call reinitialize → verify provider is replaced (no panic, no error)

---

## Step 12 — Code Quality, Regression, and Cross-Cutting Concerns

**Acceptance Criteria:** NFR-1 (security), NFR-2 (timeout), NFR-3 (idempotency), NFR-4 (backward compatibility)

**Source Files:** All modified files

**Checks to Perform:**

1. **Full test suite — no regressions:**
   ```
   cd trade-backend-go && go test ./... -v -race
   ```
2. **Build verification:**
   ```
   cd trade-backend-go && go build ./...
   cd trade-app && npm run build
   ```
3. **NFR-1: Security**
   - Tokens never logged in plaintext:
     - `truncateToken()` used for all token logging (auth.go:132–137)
     - `slog.Error` and `slog.Info` in auth.go never pass full tokens
     - `json:"-"` tags on all sensitive fields in `OAuthFlowState`
   - CSRF protection: state token validated in callback
   - Callback URL validated to start with `https://`
4. **NFR-2: Timeout**
   - OAuth state TTL = 10 minutes (oauth.go:66)
   - Token exchange HTTP timeout = 30 seconds (oauth.go:222)
   - Account fetch HTTP timeout = 15 seconds (oauth.go:283)
   - Cleanup goroutine runs every 60 seconds (oauth.go:67)
5. **NFR-3: Idempotency**
   - Duplicate callback: `pending → exchanging` transition is atomic; second callback gets "already processed"
   - State deleted after finalization; second finalize gets "not found or expired"
6. **NFR-4: Backward Compatibility**
   - Existing Schwab instances with manually provided `refresh_token` and `account_hash` in credentials — verify `createProviderInstance` still reads these from credentials map
   - Constructor accepts `"", nil` for new params — tests prove this
   - `AuthMethod: ""` on non-Schwab providers — omitted from JSON (omitempty)
   - Existing `TestCredentials` still works with manual tokens (no regression)

**Edge Cases to Verify:**
- Build with `CGO_ENABLED=0` (Docker builds) — verify static binary compiles
- Frontend `npm run build` succeeds — no TypeScript/template errors
- All 12 Schwab test files compile and pass

**New QA Tests to Write:**
- `TestBackwardCompatibility_ManualTokens` — create Schwab provider with `refresh_token` and `account_hash` in credentials, empty `instanceID`, nil `credentialUpdater` → verify `TestCredentials()` still works against mock server
- `TestSecurityNoTokenLeaks` — grep all slog/log calls in `schwab/` package → verify no raw token values logged (manual code review check)
- `TestRaceCondition_FullSuite` — run `go test ./internal/providers/schwab/... -race` → verify 0 race conditions

---

## Step 13 — Edge Cases and Security (CSRF, State Expiry, Concurrent Access)

**Acceptance Criteria:** NFR-1 (CSRF), NFR-2 (state expiry), NFR-3 (concurrent access)

**Source Files:**
- `trade-backend-go/internal/providers/schwab/oauth.go`
- `trade-backend-go/internal/providers/schwab/oauth_test.go`

**Checks to Perform:**

1. **CSRF Protection (NFR-1):**
   - State token is 32 bytes of `crypto/rand` → unguessable
   - Callback validates state before processing
   - Invalid/missing state → error page (no code exchange attempted)
   - State is single-use: transitions from `pending` to `exchanging` atomically
2. **State Expiry (NFR-2):**
   - States expire after 10 minutes
   - Both `GetState` and cleanup goroutine enforce TTL
   - Expired states are deleted from the `sync.Map`
3. **Concurrent Access:**
   - `sync.Map` for store-level thread safety
   - `sync.Mutex` per state for transition safety
   - `UpdateState` holds mutex during `updateFn` execution
4. **XSS in callback HTML:**
   - `renderCallbackPage` uses `fmt.Sprintf` to inject error messages into HTML
   - Error messages from Schwab API responses may contain HTML-special characters
   - Verify whether error messages are HTML-escaped

**Edge Cases to Verify:**
- Forged state token (attacker sends callback with guessed state) — should fail lookup
- Replay attack: capture valid callback URL, replay after original processed — should get "already processed"
- Replay attack: capture valid callback URL, replay after state expired — should get "expired"
- Concurrent callbacks with same state from two browser tabs — only one should succeed
- Cleanup goroutine exit: cancel context → goroutine stops cleanly (no leak)

**New QA Tests to Write:**
- `TestCSRF_ForgedStateRejected` — call callback with random 43-char token → verify error HTML
- `TestCSRF_ReplayAfterExpiry` — create state, backdate, call callback → verify error HTML
- `TestConcurrentCallbacks_OnlyOneSucceeds` — create state, launch 10 goroutines calling callback concurrently → verify exactly 1 sees "success" HTML, rest see "already processed"
- `TestCallbackPage_XSSPrevention` — render callback page with `<img onerror=alert(1)>` in error message → verify the output either escapes the HTML or is safe
- `TestCleanupGoroutine_NoLeak` — start cleanup with context, cancel it, verify goroutine exits (use `runtime.NumGoroutine()` before/after)

---

## Execution Summary

| Step | Area | AC Coverage | Test Command | Est. Duration |
|------|------|-------------|-------------|---------------|
| 1 | CredentialStore.UpdateCredentialFields | AC-13, AC-14 | `go test ./internal/providers/... -run TestUpdateCredentialFields -v` | 15 min |
| 2 | Provider struct (instanceID, credentialUpdater, authExpired) | AC-7, AC-8 | `go test ./internal/providers/schwab/... -v` | 20 min |
| 3 | Provider types and credential fields | AC-1 | `go test ./internal/providers/... -run TestProviderTypes -v` | 15 min |
| 4 | OAuth state store | NFR-1, NFR-2, NFR-3 | `go test ./internal/providers/schwab/... -run "TestOAuthStore\|TestStateToken\|TestConcurrent" -v -race` | 20 min |
| 5 | Token exchange and account fetch helpers | AC-4, AC-17 | `go test ./internal/providers/schwab/... -run "TestExchange\|TestFetch\|TestMask\|TestRender" -v` | 20 min |
| 6 | HandleAuthorize endpoint | AC-2, AC-3 | `go test ./internal/providers/schwab/... -run TestHandleAuthorize -v` | 15 min |
| 7 | HandleCallback endpoint | AC-4, AC-10, AC-11, AC-15, AC-16 | `go test ./internal/providers/schwab/... -run TestHandleCallback -v` | 20 min |
| 8 | HandleOAuthStatus and HandleSelectAccount | AC-5, AC-6, AC-9 | `go test ./internal/providers/schwab/... -run "TestHandleOAuthStatus\|TestHandleSelectAccount" -v` | 20 min |
| 9 | Route registration in main.go | AC-10, AC-12 | `go build ./cmd/server/...` + code review | 15 min |
| 10 | Frontend OAuth flow | AC-2, AC-3, AC-5, AC-8 | Code review + `npm run build` | 25 min |
| 11 | Manager.go factory and ReinitializeInstance | AC-7, AC-9 | `go build ./...` + code review | 15 min |
| 12 | Code quality, regression, cross-cutting | NFR-1 through NFR-4 | `go test ./... -v -race` + `npm run build` | 20 min |
| 13 | Edge cases and security | NFR-1, NFR-2, NFR-3 | New QA tests + `-race` flag | 20 min |

**Total estimated time:** ~4 hours

---

## Acceptance Criteria Traceability Matrix

| AC | Description | Steps Covering | Primary Verification |
|----|-------------|---------------|---------------------|
| AC-1 | Credential form: only App Key, App Secret, Callback URL, Base URL | 3, 10 | Unit test + code review |
| AC-2 | "Connect to Schwab" button available | 6, 10 | Code review (frontend) |
| AC-3 | Opens Schwab auth page in new tab | 6, 10 | Code review (window.open) |
| AC-4 | Backend exchanges code for tokens | 5, 7 | Unit tests (httptest) |
| AC-5 | Account selection UI for multiple accounts | 8, 10 | Unit test + code review |
| AC-6 | Credentials persisted after authorization | 8 | Unit test (mock deps) |
| AC-7 | Token rotation persisted | 2, 11 | Unit test (credentialUpdater mock) |
| AC-8 | Auth expired surfaced, "Reconnect" shown | 2, 10 | Unit test + code review |
| AC-9 | Re-auth reuses existing credentials | 8, 10, 11 | Unit test (re-auth path) |
| AC-10 | `GET /callback` route exists | 7, 9 | Build + code review |
| AC-11 | Callback handles error cases | 7 | Unit tests (10 scenarios) |
| AC-12 | Callback excluded from auth middleware | 9 | Code review (route placement) |
| AC-13 | Credentials in `provider_credentials.json` | 1, 8 | Unit test (persistence) |
| AC-14 | Atomic credential writes | 1 | Unit test (file verification) |
| AC-15 | Cancellation handled gracefully | 7, 10 | Unit test + code review |
| AC-16 | Token exchange failure shows error | 7, 10 | Unit test + code review |
| AC-17 | No accounts shows error | 5, 7 | Unit test |
| NFR-1 | Security (no plaintext tokens, CSRF) | 4, 12, 13 | Code review + unit tests |
| NFR-2 | Timeout (10-min state TTL) | 4, 12, 13 | Unit tests |
| NFR-3 | Idempotency (duplicate callback) | 4, 7, 13 | Unit tests |
| NFR-4 | Backward compatibility | 2, 3, 12 | Unit tests + build |
