# Schwab OAuth Authorization Flow — Step-by-Step Implementation Plan

> Derived from [requirements-oauth-flow.md](./requirements-oauth-flow.md) and
> [architecture-oauth-flow.md](./architecture-oauth-flow.md), cross-referenced with
> the current codebase: `schwab.go` (206 lines), `auth.go` (126 lines),
> `provider_types.go` (362 lines), `credential_store.go` (277 lines),
> `manager.go` (670 lines), `main.go` (1822 lines), `ProvidersTab.vue` (1700+ lines),
> and 11 existing Schwab test files.
>
> Each step is one logical unit that can be implemented, tested, and committed
> independently. Steps are ordered to respect dependencies — each step builds on
> the previous ones. Parallel implementation opportunities are noted.

---

## Dependency Graph

```
Step 1 ──→ Step 2 ──→ Step 3 ──→ Step 6 ──→ Step 9 ──→ Step 10
                 ↘                   ↗
Step 4 ──→ Step 5 ──→ Step 7 ──→ Step 8
```

- Steps 1 and 4 can be developed in parallel (no dependencies on each other)
- Steps 2–3 depend on Step 1
- Steps 5–7 depend on Step 4; Step 7 also depends on Steps 3 and 5
- Step 8 depends on Step 7
- Step 9 (frontend) depends on Steps 6 and 8 (all backend routes registered)
- Step 10 (frontend reconnect) depends on Step 9

---

## Phase 1: Backend Foundation (Steps 1–3)

### Step 1 — CredentialStore: Add `UpdateCredentialFields` Method

**Goal:** Enable targeted credential sub-map updates so providers can persist token rotations without overwriting the entire credentials object.

**Files modified: 1 | Files created: 0 | Tests: 3–5**

**File: `trade-backend-go/internal/providers/credential_store.go`** (~+35 lines)

Add a new method to `CredentialStore`:

```go
// UpdateCredentialFields updates specific fields within an instance's credentials sub-map.
// Performs a deep merge — only the specified fields are updated, others are preserved.
//
// Example: UpdateCredentialFields("schwab_live_1", map[string]interface{}{"refresh_token": "new_token"})
// This updates only refresh_token within the credentials map, leaving app_key, app_secret, etc. untouched.
func (cs *CredentialStore) UpdateCredentialFields(instanceID string, fieldUpdates map[string]interface{}) error
```

Implementation:
1. Check if `instanceID` exists in `cs.data` — return error if not found
2. Extract the `"credentials"` sub-map from the instance data (create if nil)
3. Merge each key from `fieldUpdates` into the credentials sub-map
4. Set `"updated_at"` timestamp
5. Call `cs.saveCredentials()` to persist

**Tests** (add to a new file or extend existing):

| # | Test | Description |
|---|------|-------------|
| 1 | `TestUpdateCredentialFields_Success` | Update one field in an existing instance; verify only that field changed |
| 2 | `TestUpdateCredentialFields_MultipleFields` | Update 2+ fields at once; verify all updated, others preserved |
| 3 | `TestUpdateCredentialFields_NonExistentInstance` | Call with unknown instance ID; verify error returned |
| 4 | `TestUpdateCredentialFields_NilCredentials` | Instance exists but has no credentials sub-map; verify it's created |
| 5 | `TestUpdateCredentialFields_Persistence` | Verify the file on disk reflects the update |

**Verify:** `go test ./internal/providers/... -run TestUpdateCredentialFields -v`

---

### Step 2 — SchwabProvider: Add `instanceID`, `credentialUpdater`, `authExpired` Fields

**Goal:** Give the Schwab provider the ability to identify itself and persist credential changes back to the store. Add auth health signaling.

**Files modified: 3 | Files created: 0 | Tests: ~5 new + fix ~20 existing constructor calls**

#### 2a. Define `CredentialUpdater` type

**File: `trade-backend-go/internal/providers/schwab/schwab.go`** (~+5 lines at top)

Add the type definition within the `schwab` package:

```go
// CredentialUpdater is a callback function that the provider uses to persist
// credential changes back to the credential store. Injected at construction time.
// Pass nil in tests or when persistence is not needed.
type CredentialUpdater func(instanceID string, updates map[string]interface{}) error
```

> **Design note:** Defined in the `schwab` package (not `providers`) to avoid circular imports. The `manager.go` factory creates a closure that calls `CredentialStore.UpdateCredentialFields` and passes it as this type.

#### 2b. Update `SchwabProvider` struct

**File: `trade-backend-go/internal/providers/schwab/schwab.go`** (~+10 lines)

Add three new fields to the struct:

```go
// NEW: Instance identity and credential persistence
instanceID        string              // Provider instance ID (e.g., "schwab_live_MyAccount")
credentialUpdater CredentialUpdater   // Callback to persist credential changes (may be nil)

// NEW: Auth health status for re-authentication signaling
authExpired       bool                // Set to true when refresh token is confirmed expired
```

#### 2c. Update constructor signature

**File: `trade-backend-go/internal/providers/schwab/schwab.go`** (~+5 lines)

Update `NewSchwabProvider` to accept two additional trailing parameters:

```go
func NewSchwabProvider(
    appKey, appSecret, callbackURL, refreshToken, accountHash, baseURL, accountType string,
    instanceID string,                   // NEW: pass "" in tests
    credentialUpdater CredentialUpdater,  // NEW: pass nil in tests
) *SchwabProvider
```

Store both new params on the struct in the constructor body.

#### 2d. Update `refreshAccessToken` to persist rotated tokens

**File: `trade-backend-go/internal/providers/schwab/auth.go`** (~+20 lines, ~-5 lines)

Replace the existing token rotation warning block (lines 110–115) with:

```go
if tokenResp.RefreshToken != "" && tokenResp.RefreshToken != s.refreshToken {
    s.refreshToken = tokenResp.RefreshToken // Always update in-memory

    if s.credentialUpdater != nil && s.instanceID != "" {
        if err := s.credentialUpdater(s.instanceID, map[string]interface{}{
            "refresh_token": tokenResp.RefreshToken,
        }); err != nil {
            s.logger.Error("failed to persist rotated refresh token", "error", err)
        } else {
            s.logger.Info("persisted rotated refresh token to credential store")
        }
    } else {
        s.logger.Warn("refresh token rotated but persistence not configured",
            "old_prefix", truncateToken(s.refreshToken),
        )
    }
}
```

Also add `authExpired` signaling on 401 (line 82–84):

```go
if resp.StatusCode == http.StatusUnauthorized {
    s.authExpired = true  // NEW: signal to frontend
    return ErrRefreshTokenExpired
}
```

#### 2e. Update `TestCredentials` to report `authExpired`

**File: `trade-backend-go/internal/providers/schwab/schwab.go`** (~+10 lines)

Add an early return at the top of `TestCredentials`:

```go
if s.authExpired {
    return map[string]interface{}{
        "success":      false,
        "message":      "Refresh token expired. Please reconnect to Schwab.",
        "auth_expired": true,
    }, nil
}
```

#### 2f. Fix all existing test files

**Files modified:** All 11 `*_test.go` files in `schwab/` package:
- `schwab_test.go` — update `TestNewSchwabProvider`, `TestNewSchwabProvider_Defaults`, all other `NewSchwabProvider(...)` calls
- `auth_test.go` — update `newTestProvider()` helper (line 17–27): add `"", nil` params
- `helpers_test.go` — update any `NewSchwabProvider(...)` calls
- `account_test.go` — update any constructor calls
- `account_stream_test.go` — update any constructor calls
- `market_data_test.go` — update any constructor calls
- `orders_test.go` — update any constructor calls
- `streaming_test.go` — update any constructor calls
- `symbols_test.go` — update any constructor calls
- `rate_limiter_test.go` — update any constructor calls (if present)
- `qa_edge_cases_test.go` — update any constructor calls

The key change in each file: append `"", nil` to every `NewSchwabProvider(...)` call.

**New tests:**

| # | Test | File | Description |
|---|------|------|-------------|
| 1 | `TestNewSchwabProvider_WithUpdater` | `schwab_test.go` | Construct with instanceID + updater; verify fields stored |
| 2 | `TestNewSchwabProvider_NilUpdater` | `schwab_test.go` | Construct with nil updater; verify no panic |
| 3 | `TestRefreshAccessToken_RotatesAndPersists` | `auth_test.go` | Mock server returns new refresh_token; verify updater called with correct args |
| 4 | `TestRefreshAccessToken_RotatesPersistFailure` | `auth_test.go` | Updater returns error; verify in-memory token still updated, no panic |
| 5 | `TestRefreshAccessToken_SetsAuthExpiredOn401` | `auth_test.go` | Mock server returns 401; verify `authExpired` is `true` |
| 6 | `TestTestCredentials_AuthExpired` | `schwab_test.go` | Set `authExpired = true`; verify response includes `auth_expired: true` |

**Verify:** `go test ./internal/providers/schwab/... -v` — ALL existing + new tests pass.

---

### Step 3 — ProviderTypes & Manager: Update Schwab Definition and Factory

**Goal:** Update the Schwab credential fields to remove manual-entry fields, add OAuth signal, and wire the credential updater callback in the factory.

**Files modified: 2 | Files created: 0 | Tests: 3–5**

#### 3a. Add `AuthMethod` to `ProviderType` struct

**File: `trade-backend-go/internal/providers/provider_types.go`** (~+3 lines)

Add field to `ProviderType` struct:

```go
type ProviderType struct {
    Name                 string                       `json:"name"`
    Description          string                       `json:"description"`
    SupportsAccountTypes []string                     `json:"supports_account_types"`
    Capabilities         ProviderCapabilities         `json:"capabilities"`
    CredentialFields     map[string][]CredentialField `json:"credential_fields"`
    AuthMethod           string                       `json:"auth_method,omitempty"` // NEW: "oauth" or "" (default)
}
```

Add `HelpText` to `CredentialField` struct:

```go
type CredentialField struct {
    Name        string `json:"name"`
    Label       string `json:"label"`
    Type        string `json:"type"`
    Required    bool   `json:"required"`
    Placeholder string `json:"placeholder,omitempty"`
    Default     string `json:"default,omitempty"`
    HelpText    string `json:"help_text,omitempty"` // NEW: tooltip/help for the field
}
```

#### 3b. Update Schwab entry in `ProviderTypes` map

**File: `trade-backend-go/internal/providers/provider_types.go`** (~+10, -15 lines)

Replace the `"schwab"` entry with updated credential fields:

- **Add:** `AuthMethod: "oauth"` on the Schwab entry
- **Remove** from both live and paper field lists:
  - `refresh_token` field (obtained via OAuth)
  - `account_hash` field (obtained via OAuth)
- **Update** `callback_url` field: change `Default` from `"https://127.0.0.1"` to `"https://127.0.0.1/callback"`; add `HelpText`
- **Add** `HelpText` to `app_key`, `app_secret`, and `base_url` fields
- **Result:** Each account type has 4 fields: `app_key`, `app_secret`, `callback_url`, `base_url`

#### 3c. Update Schwab factory case in `manager.go`

**File: `trade-backend-go/internal/providers/manager.go`** (~+20 lines)

Update the `case "schwab"` block in `createProviderInstance()`:

```go
case "schwab":
    appKey, _ := credentials["app_key"].(string)
    appSecret, _ := credentials["app_secret"].(string)
    callbackURL, _ := credentials["callback_url"].(string)
    refreshToken, _ := credentials["refresh_token"].(string)
    accountHash, _ := credentials["account_hash"].(string)
    baseURL, _ := credentials["base_url"].(string)
    if baseURL == "" {
        baseURL = "https://api.schwabapi.com"
    }
    if callbackURL == "" {
        callbackURL = "https://127.0.0.1/callback"
    }

    // Create credential updater callback for this instance
    instID := instanceID  // capture for closure (instanceID param added to createProviderInstance)
    credUpdater := schwab.CredentialUpdater(func(id string, updates map[string]interface{}) error {
        cs := NewCredentialStore()
        return cs.UpdateCredentialFields(id, updates)
    })

    return schwab.NewSchwabProvider(
        appKey, appSecret, callbackURL, refreshToken,
        accountHash, baseURL, accountType,
        instID, credUpdater,
    )
```

> **Note:** The `createProviderInstance` method signature needs to be updated to accept `instanceID string` as a parameter. The caller in `initializeActiveProviders` already has the `instanceID` available (loop variable at line 59).

#### 3d. Add `ReinitializeInstance` method stub

**File: `trade-backend-go/internal/providers/manager.go`** (~+30 lines)

```go
// ReinitializeInstance destroys the current provider instance and creates a new one
// from updated credentials in the store. Used after OAuth re-authentication.
func (pm *ProviderManager) ReinitializeInstance(instanceID string) error {
    pm.mutex.Lock()
    defer pm.mutex.Unlock()

    // 1. Get updated instance data from credential store
    credStore := NewCredentialStore()
    instanceData := credStore.GetInstance(instanceID)
    if instanceData == nil {
        return fmt.Errorf("instance %s not found in credential store", instanceID)
    }

    // 2. Remove old provider
    delete(pm.providers, instanceID)

    // 3. Create new provider from updated credentials
    providerType, _ := instanceData["provider_type"].(string)
    accountType, _ := instanceData["account_type"].(string)
    credentials, _ := instanceData["credentials"].(map[string]interface{})
    credentials = ApplyDefaults(providerType, accountType, credentials)

    provider := pm.createProviderInstance(providerType, accountType, credentials, instanceID)
    if provider == nil {
        return fmt.Errorf("failed to create provider instance %s", instanceID)
    }

    pm.providers[instanceID] = provider
    slog.Info(fmt.Sprintf("🔄 Reinitialized provider instance: %s", instanceID))
    return nil
}
```

> **Note:** `createProviderInstance` needs its signature updated to `createProviderInstance(providerType, accountType string, credentials map[string]interface{}, instanceID string) base.Provider`. All call sites must be updated.

**Tests:**

| # | Test | Description |
|---|------|-------------|
| 1 | `TestProviderTypes_SchwabAuthMethod` | Verify `ProviderTypes["schwab"].AuthMethod == "oauth"` |
| 2 | `TestProviderTypes_SchwabCredentialFields` | Verify live/paper have 4 fields each, no `refresh_token` or `account_hash` |
| 3 | `TestProviderTypes_SchwabHelpText` | Verify `HelpText` is set on Schwab fields |
| 4 | `TestCredentialField_HelpText` | Verify `HelpText` JSON serialization (`help_text` key, omitempty) |
| 5 | `TestProviderTypes_OtherProvidersUnchanged` | Verify Alpaca, Tradier, TastyTrade, Public types are unmodified |

**Verify:** `go test ./internal/providers/... -v` and `go build ./...`

---

## Phase 2: OAuth Backend — State Management (Steps 4–5)

### Step 4 — OAuth State Store

**Goal:** Implement the in-memory state store that tracks OAuth flows from initiation through completion.

**Files created: 1 | Tests: 8–10**

**New file: `trade-backend-go/internal/providers/schwab/oauth.go`** (~150 lines — state store portion only)

Implement:

1. **`OAuthFlowState` struct** — holds all state for one OAuth flow:
   - `AppKey`, `AppSecret`, `CallbackURL`, `BaseURL` (set at creation)
   - `CreatedAt`, `Status` (pending → exchanging → completed → finalized | failed)
   - `RefreshToken`, `AccessToken`, `TokenExpiry` (set after callback)
   - `Accounts []SchwabAccountInfo` (set after account fetch)
   - `Error string` (set on failure)
   - `ExistingInstanceID string` (set for re-auth flows)
   - `mu sync.Mutex` (protects state transitions)
   - JSON tags: `json:"-"` on all sensitive fields

2. **`SchwabAccountInfo` struct** — `AccountNumber string`, `HashValue string`

3. **`SchwabOAuthStore` struct** — wraps `sync.Map`

4. **State lifecycle methods:**
   - `NewSchwabOAuthStore() *SchwabOAuthStore`
   - `CreateState(appKey, appSecret, callbackURL, baseURL string) (stateToken string, err error)` — generates 32-byte crypto-random token, stores state as "pending"
   - `GetState(stateToken string) *OAuthFlowState` — returns nil if not found or expired (>10 min)
   - `UpdateState(stateToken string, updateFn func(*OAuthFlowState)) bool` — locks state mutex, calls updateFn, returns false if state not found/expired
   - `DeleteState(stateToken string)` — removes state entry
   - `StartCleanup(ctx context.Context)` — goroutine that runs every 60s, removes expired entries

5. **`generateStateToken() (string, error)`** — 32 bytes crypto/rand → base64url

**Tests** (in `oauth_test.go` — state store section):

| # | Test | Description |
|---|------|-------------|
| 1 | `TestCreateState_GeneratesUniqueTokens` | Call CreateState twice; verify different tokens returned |
| 2 | `TestCreateState_StoresCorrectData` | Create state; GetState; verify all fields match |
| 3 | `TestGetState_NotFound` | GetState with random token; verify nil returned |
| 4 | `TestGetState_Expired` | Create state; manually set CreatedAt to 11 min ago; verify GetState returns nil |
| 5 | `TestUpdateState_TransitionsStatus` | Create pending state; update to "exchanging"; verify status changed |
| 6 | `TestUpdateState_NotFound` | UpdateState with unknown token; verify returns false |
| 7 | `TestDeleteState` | Create state; delete; verify GetState returns nil |
| 8 | `TestStartCleanup_RemovesExpired` | Create state with old timestamp; run cleanup; verify removed |
| 9 | `TestStateToken_Length` | Verify generated tokens are 43 chars (32 bytes base64url) |
| 10 | `TestConcurrentAccess` | Goroutines creating/reading/updating states concurrently; verify no races |

**Verify:** `go test ./internal/providers/schwab/... -run TestOAuthStore -v -race`

---

### Step 5 — OAuth Helper Functions: Token Exchange & Account Fetch

**Goal:** Implement the standalone functions that exchange an authorization code for tokens and fetch account numbers from Schwab's API.

**Files modified: 1 (oauth.go) | Tests: 8–10**

**File: `trade-backend-go/internal/providers/schwab/oauth.go`** (~+120 lines)

Implement:

1. **`tokenExchangeResult` struct** — `AccessToken`, `RefreshToken`, `ExpiresIn int`

2. **`exchangeCodeForTokens(baseURL, appKey, appSecret, callbackURL, code string) (*tokenExchangeResult, error)`**
   - POST to `{baseURL}/v1/oauth/token` with `grant_type=authorization_code`
   - Set `Content-Type: application/x-www-form-urlencoded`
   - Set Basic Auth: `base64(appKey:appSecret)`
   - 30-second timeout
   - Parse JSON response; validate `access_token` present
   - Handle 400/401 with descriptive error messages

3. **`fetchAccountNumbers(baseURL, accessToken string) ([]SchwabAccountInfo, error)`**
   - GET `{baseURL}/trader/v1/accounts/accountNumbers`
   - Set `Authorization: Bearer {accessToken}`
   - 15-second timeout
   - Parse JSON array of `{accountNumber, hashValue}` objects
   - Mask account numbers for display (last 4 digits visible)

4. **`maskAccountNumber(number string) string`** — returns `"*****6789"` format

5. **`renderCallbackPage(status, errorMessage string) []byte`**
   - Returns minimal HTML page with inline CSS
   - Three states: `"success"` (green), `"error"` (red + message), `"cancelled"` (yellow)
   - Instructs user to close the tab and return to JuicyTrade
   - No external dependencies (no JS frameworks, no external CSS)

**Tests** (in `oauth_test.go` — helpers section):

| # | Test | Description |
|---|------|-------------|
| 1 | `TestExchangeCodeForTokens_Success` | httptest server returns valid tokens; verify parsed correctly |
| 2 | `TestExchangeCodeForTokens_InvalidCode` | Server returns 400; verify descriptive error |
| 3 | `TestExchangeCodeForTokens_InvalidCredentials` | Server returns 401; verify error message |
| 4 | `TestExchangeCodeForTokens_BasicAuthHeader` | Verify correct Basic Auth header sent |
| 5 | `TestExchangeCodeForTokens_FormBody` | Verify correct form body (grant_type, code, redirect_uri) |
| 6 | `TestFetchAccountNumbers_SingleAccount` | Server returns 1 account; verify parsed and masked |
| 7 | `TestFetchAccountNumbers_MultipleAccounts` | Server returns 3 accounts; verify all parsed |
| 8 | `TestFetchAccountNumbers_EmptyAccounts` | Server returns empty array; verify empty slice returned (no error) |
| 9 | `TestMaskAccountNumber` | Test various lengths: "123456789" → "*****6789", "1234" → "1234", "" → "" |
| 10 | `TestRenderCallbackPage_Success` | Verify success HTML contains "successful" text |
| 11 | `TestRenderCallbackPage_Error` | Verify error HTML contains the error message |
| 12 | `TestRenderCallbackPage_Cancelled` | Verify cancelled HTML contains "cancelled" text |

**Verify:** `go test ./internal/providers/schwab/... -run "TestExchange|TestFetch|TestMask|TestRender" -v`

---

## Phase 3: OAuth Backend — Handlers & Route Registration (Steps 6–8)

### Step 6 — OAuth Handler Struct & `HandleAuthorize`

**Goal:** Implement the handler struct and the first endpoint that generates the Schwab authorization URL.

**Files modified: 1 (oauth.go) | Tests: 5–7**

**File: `trade-backend-go/internal/providers/schwab/oauth.go`** (~+60 lines)

Implement:

1. **`SchwabOAuthHandler` struct:**
   ```go
   type SchwabOAuthHandler struct {
       oauthStore      *SchwabOAuthStore
       credentialStore *CredentialStore  // import from providers package — see note
       providerManager *ProviderManager  // import from providers package — see note
   }
   ```

   > **Package note:** `SchwabOAuthHandler` references `CredentialStore` and `ProviderManager` from the parent `providers` package. Since `schwab` is a sub-package of `providers`, there's a potential circular import. To resolve this, the handler struct accepts **interfaces** or **function closures** instead of concrete types:
   >
   > ```go
   > type SchwabOAuthHandler struct {
   >     oauthStore       *SchwabOAuthStore
   >     addInstance       func(instanceID, providerType, accountType, displayName string, credentials map[string]interface{}) bool
   >     updateCredFields  func(instanceID string, updates map[string]interface{}) error
   >     generateInstID    func(providerType, accountType, displayName string) string
   >     reinitInstance    func(instanceID string) error
   >     getInstance       func(instanceID string) map[string]interface{}
   > }
   > ```
   >
   > These closures are wired up in `main.go` when constructing the handler.

2. **`NewSchwabOAuthHandler(...)` constructor** — accepts the closure functions, creates internal `SchwabOAuthStore`, starts cleanup goroutine

3. **`HandleAuthorize(c *gin.Context)`:**
   - Parse JSON body: `app_key`, `app_secret`, `callback_url`, `base_url` (optional), `instance_id` (optional for re-auth)
   - Validate required fields
   - Default `base_url` to `https://api.schwabapi.com`
   - Call `oauthStore.CreateState(...)` to generate state token
   - If `instance_id` provided, store it in state via `UpdateState`
   - Build Schwab authorization URL: `{base_url}/v1/oauth/authorize?client_id={app_key}&redirect_uri={callback_url}&response_type=code&state={state_token}`
   - Return JSON: `{auth_url, state}`

**Tests** (in `oauth_test.go` — handler section):

| # | Test | Description |
|---|------|-------------|
| 1 | `TestHandleAuthorize_Success` | POST with valid fields; verify 200, auth_url contains correct params, state is 43 chars |
| 2 | `TestHandleAuthorize_MissingAppKey` | POST without app_key; verify 400 error |
| 3 | `TestHandleAuthorize_MissingAppSecret` | POST without app_secret; verify 400 error |
| 4 | `TestHandleAuthorize_DefaultBaseURL` | POST without base_url; verify auth_url uses default |
| 5 | `TestHandleAuthorize_WithInstanceID` | POST with instance_id; verify state stores ExistingInstanceID |
| 6 | `TestHandleAuthorize_AuthURLFormat` | Verify auth_url is correctly URL-encoded |

**Verify:** `go test ./internal/providers/schwab/... -run TestHandleAuthorize -v`

---

### Step 7 — `HandleCallback`: Token Exchange & Account Fetch

**Goal:** Implement the callback handler that Schwab redirects to after user authorization.

**Files modified: 1 (oauth.go) | Tests: 8–10**

**File: `trade-backend-go/internal/providers/schwab/oauth.go`** (~+80 lines)

Implement `HandleCallback(c *gin.Context)`:

1. Extract query params: `code`, `state`, `error`
2. If `error` param present → update state to "failed", render cancelled HTML page
3. If `state` missing or not found → render error HTML page ("Invalid or expired authorization request")
4. If state not in "pending" status → render error HTML page ("Already processed") — prevents duplicate processing
5. Transition state to "exchanging"
6. Call `exchangeCodeForTokens(...)` — on failure, update state to "failed", render error HTML
7. Call `fetchAccountNumbers(...)` — on failure, update state to "failed", render error HTML
8. Validate at least one account returned — on empty, update state to "failed", render error HTML
9. Update state to "completed" with tokens and accounts
10. Render success HTML page

**Tests** (in `oauth_test.go`):

| # | Test | Description |
|---|------|-------------|
| 1 | `TestHandleCallback_Success_SingleAccount` | Mock Schwab token + account endpoints; verify state → "completed", success HTML |
| 2 | `TestHandleCallback_Success_MultipleAccounts` | Mock endpoints returning 3 accounts; verify all stored in state |
| 3 | `TestHandleCallback_UserCancelled` | GET with `?error=access_denied`; verify state → "failed", cancelled HTML |
| 4 | `TestHandleCallback_InvalidState` | GET with unknown state token; verify error HTML |
| 5 | `TestHandleCallback_ExpiredState` | Create state, set old timestamp; GET callback; verify error HTML |
| 6 | `TestHandleCallback_DuplicateCallback` | Call callback twice with same state; verify second returns "already processed" |
| 7 | `TestHandleCallback_TokenExchangeFails` | Mock token endpoint returns 401; verify state → "failed" |
| 8 | `TestHandleCallback_AccountFetchFails` | Mock account endpoint returns 500; verify state → "failed" |
| 9 | `TestHandleCallback_NoAccounts` | Mock account endpoint returns empty array; verify state → "failed" |
| 10 | `TestHandleCallback_MissingCode` | GET with state but no code and no error; verify error handling |

**Verify:** `go test ./internal/providers/schwab/... -run TestHandleCallback -v`

---

### Step 8 — `HandleOAuthStatus` & `HandleSelectAccount`

**Goal:** Implement the polling endpoint and the finalization endpoint that creates/updates the provider instance.

**Files modified: 1 (oauth.go) | Tests: 12–15**

**File: `trade-backend-go/internal/providers/schwab/oauth.go`** (~+120 lines)

#### 8a. `HandleOAuthStatus(c *gin.Context)`

- Extract `:state` path param
- Call `oauthStore.GetState(stateToken)`
- If nil → return 404 JSON: `{"error": "OAuth flow not found or expired"}`
- Return JSON based on status:
  - `"pending"` / `"exchanging"` → `{"status": "pending"}` / `{"status": "exchanging"}`
  - `"completed"` → `{"status": "completed", "accounts": [...]}`
  - `"failed"` → `{"status": "failed", "error": "..."}`

#### 8b. `HandleSelectAccount(c *gin.Context)`

- Parse JSON body: `state` (required), `account_hash` (required), `provider_name` (required for new), `account_type` (optional, default "live")
- Lookup state — must be in "completed" status
- Validate `account_hash` is in the state's accounts list
- **New provider path** (no `ExistingInstanceID`):
  1. Validate `provider_name` is non-empty
  2. Build credentials map: `{app_key, app_secret, callback_url, base_url, refresh_token, account_hash}`
  3. Call `generateInstID(providerType="schwab", accountType, providerName)` closure
  4. Call `addInstance(instanceID, "schwab", accountType, providerName, credentials)` closure
  5. Call `reinitInstance(instanceID)` closure — this is a no-op if the manager initializes on next startup, or eagerly initializes the provider
  6. Delete state from store
  7. Return `{"success": true, "instance_id": "...", "message": "..."}`
- **Re-auth path** (`ExistingInstanceID` set):
  1. Call `getInstance(existingInstanceID)` — verify exists
  2. Call `updateCredFields(existingInstanceID, {"refresh_token": ..., "account_hash": ...})`
  3. Call `reinitInstance(existingInstanceID)`
  4. Delete state from store
  5. Return `{"success": true, "instance_id": "...", "message": "Provider re-authenticated successfully"}`

**Tests:**

| # | Test | Description |
|---|------|-------------|
| 1 | `TestHandleOAuthStatus_Pending` | Create pending state; GET status; verify `{"status": "pending"}` |
| 2 | `TestHandleOAuthStatus_Completed` | Create completed state with accounts; GET status; verify accounts in response |
| 3 | `TestHandleOAuthStatus_Failed` | Create failed state; GET status; verify error in response |
| 4 | `TestHandleOAuthStatus_NotFound` | GET status with unknown state; verify 404 |
| 5 | `TestHandleOAuthStatus_Expired` | Create state, set old timestamp; verify 404 |
| 6 | `TestHandleSelectAccount_NewProvider` | Completed state; POST select-account; verify addInstance called with correct credentials |
| 7 | `TestHandleSelectAccount_ReAuth` | Completed state with ExistingInstanceID; POST select-account; verify updateCredFields called |
| 8 | `TestHandleSelectAccount_InvalidState` | POST with unknown state; verify 400 error |
| 9 | `TestHandleSelectAccount_WrongStatus` | State is "pending" (not completed); verify 400 error |
| 10 | `TestHandleSelectAccount_InvalidAccountHash` | Completed state; POST with hash not in accounts list; verify 400 error |
| 11 | `TestHandleSelectAccount_MissingProviderName` | New provider but no provider_name; verify 400 error |
| 12 | `TestHandleSelectAccount_DefaultAccountType` | Omit account_type; verify defaults to "live" |
| 13 | `TestHandleSelectAccount_StateDeletedAfterSuccess` | Verify state is removed from store after successful finalization |
| 14 | `TestHandleSelectAccount_ReAuthInstanceNotFound` | Re-auth with nonexistent instance; verify 404 |

**Verify:** `go test ./internal/providers/schwab/... -run "TestHandleOAuthStatus|TestHandleSelectAccount" -v`

---

## Phase 4: Route Registration (Step 9)

### Step 9 — Register OAuth Routes in `main.go`

**Goal:** Wire up the OAuth handler and register all routes in the correct positions.

**Files modified: 1 | Tests: 3–5 (integration-level)**

**File: `trade-backend-go/cmd/server/main.go`** (~+30 lines)

#### 9a. Import schwab package

Add import: `"trade-backend-go/internal/providers/schwab"`

#### 9b. Instantiate `SchwabOAuthHandler`

After `providerManager` is created (around line 170), add:

```go
// Create Schwab OAuth handler with credential store closures
schwabOAuthHandler := schwab.NewSchwabOAuthHandler(
    schwab.OAuthHandlerDeps{
        AddInstance: func(instanceID, providerType, accountType, displayName string, credentials map[string]interface{}) bool {
            cs := providers.NewCredentialStore()
            return cs.AddInstance(instanceID, providerType, accountType, displayName, credentials)
        },
        UpdateCredFields: func(instanceID string, updates map[string]interface{}) error {
            cs := providers.NewCredentialStore()
            return cs.UpdateCredentialFields(instanceID, updates)
        },
        GenerateInstID: func(providerType, accountType, displayName string) string {
            cs := providers.NewCredentialStore()
            return cs.GenerateInstanceID(providerType, accountType, displayName)
        },
        ReinitInstance: func(instanceID string) error {
            return providerManager.ReinitializeInstance(instanceID)
        },
        GetInstance: func(instanceID string) map[string]interface{} {
            cs := providers.NewCredentialStore()
            return cs.GetInstance(instanceID)
        },
    },
)
```

#### 9c. Register root-level callback route

Add right after the existing root routes (around line 359–363):

```go
// Schwab OAuth callback — root level to match registered callback URL
router.GET("/callback", schwabOAuthHandler.HandleCallback)
```

#### 9d. Register Schwab OAuth API routes

Add within the provider management routes block (before the auth middleware at line 356):

```go
// Schwab OAuth routes (under /api/providers/schwab/) — no auth required
schwabOAuth := api.Group("/providers/schwab")
{
    schwabOAuth.POST("/authorize", schwabOAuthHandler.HandleAuthorize)
    schwabOAuth.GET("/oauth/status/:state", schwabOAuthHandler.HandleOAuthStatus)
    schwabOAuth.POST("/select-account", schwabOAuthHandler.HandleSelectAccount)
}
```

**Tests:**

| # | Test | Description |
|---|------|-------------|
| 1 | `TestCallbackRouteAccessible` | HTTP GET `/callback` returns 200 (even if state is invalid, it returns HTML not 404) |
| 2 | `TestOAuthRoutesBeforeAuthMiddleware` | Verify `/api/providers/schwab/authorize` is accessible without auth token |
| 3 | `TestOAuthStatusRouteAccessible` | Verify `/api/providers/schwab/oauth/status/test` returns 404 JSON (not auth error) |
| 4 | `TestExistingRoutesUnaffected` | Verify `/api/providers/types` and `/api/providers/instances` still work |

**Verify:** `go build ./cmd/server/...` compiles; `go test ./cmd/server/... -v` (if integration tests exist); manual smoke test with `curl http://localhost:8008/callback`

---

## Phase 5: Frontend (Steps 10–11)

### Step 10 — ProvidersTab: OAuth Connect Flow

**Goal:** Add the "Connect to Schwab" button, polling logic, account selection, and finalization to the provider setup dialog.

**Files modified: 2 | Tests: manual**

#### 10a. Add API helper methods

**File: `trade-app/src/services/api.js`** (~+30 lines)

Add three new methods to the API service object (in the Provider Instance Management section, after line ~697):

```javascript
// === Schwab OAuth Flow APIs ===

async initiateSchwabOAuth(data) {
    const response = await axios.post(`${BASE_URL}/api/providers/schwab/authorize`, data);
    return response.data;
},

async getSchwabOAuthStatus(stateToken) {
    const response = await axios.get(`${BASE_URL}/api/providers/schwab/oauth/status/${stateToken}`);
    return response.data;
},

async selectSchwabAccount(data) {
    const response = await axios.post(`${BASE_URL}/api/providers/schwab/select-account`, data);
    return response.data;
},
```

#### 10b. Update `ProvidersTab.vue` — Template changes

**File: `trade-app/src/components/settings/ProvidersTab.vue`**

**In the dialog template** (around line 325–375, Step 3 credentials form):

1. **Detect OAuth provider:** After the credential fields loop, add a conditional block:

```html
<!-- OAuth Connect Button (for Schwab and other OAuth providers) -->
<div v-if="isOAuthProvider && !editingInstance" class="oauth-connect-section">
    <Button
        label="Connect to Schwab"
        icon="pi pi-external-link"
        @click="startOAuth"
        class="p-button-brand oauth-connect-btn"
        :loading="oauthLoading"
        :disabled="!canStartOAuth"
        type="button"
    />
    <small class="oauth-help-text">
        Opens Schwab's login page in a new tab. After you authorize, return here.
    </small>
</div>
```

2. **Add OAuth status display** (below the connect section):

```html
<!-- OAuth Polling Status -->
<div v-if="oauthStatus === 'pending' || oauthStatus === 'exchanging'" class="oauth-status">
    <div class="loading-spinner"></div>
    <span>Waiting for Schwab authorization...</span>
    <Button label="Cancel" size="small" text @click="cancelOAuth" />
</div>

<!-- OAuth Error -->
<div v-if="oauthError" class="oauth-error">
    <i class="pi pi-times-circle"></i>
    <span>{{ oauthError }}</span>
    <Button label="Retry" size="small" text @click="startOAuth" />
</div>
```

3. **Add account selection UI** (shows when multiple accounts returned):

```html
<!-- Account Selection (when OAuth completed with multiple accounts) -->
<div v-if="oauthStatus === 'completed' && oauthAccounts.length > 1" class="account-selection">
    <h4>Select Account</h4>
    <p>Multiple Schwab accounts found. Select which account to use:</p>
    <div
        v-for="account in oauthAccounts"
        :key="account.hash_value"
        class="account-card"
        :class="{ selected: selectedAccountHash === account.hash_value }"
        @click="selectedAccountHash = account.hash_value"
    >
        <i class="pi pi-wallet"></i>
        <span class="account-number">{{ account.account_number }}</span>
    </div>
</div>
```

4. **Update dialog footer:** Replace the Create/Update button logic for OAuth providers:

```html
<Button
    v-if="dialogStep === 3 && isOAuthProvider && !editingInstance"
    label="Create Provider"
    icon="pi pi-check"
    @click="finalizeOAuth(selectedAccountHash || oauthAccounts[0]?.hash_value)"
    :loading="savingProvider"
    :disabled="!canFinalizeOAuth"
/>
```

#### 10c. Update `ProvidersTab.vue` — Script changes

Add reactive state and methods in the `setup()` function:

**New reactive state:**

```javascript
// OAuth flow state
const oauthState = ref(null);
const oauthStatus = ref(null);
const oauthAccounts = ref([]);
const oauthError = ref(null);
const oauthLoading = ref(false);
const selectedAccountHash = ref(null);
let oauthPollInterval = null;
```

**New computed properties:**

```javascript
const isOAuthProvider = computed(() => {
    if (!newProvider.value.provider_type) return false;
    const pt = providerTypes.value[newProvider.value.provider_type];
    return pt && pt.auth_method === 'oauth';
});

const canStartOAuth = computed(() => {
    const creds = newProvider.value.credentials;
    return creds.app_key && creds.app_secret && creds.callback_url && newProvider.value.display_name;
});

const canFinalizeOAuth = computed(() => {
    return oauthStatus.value === 'completed' &&
           (oauthAccounts.value.length === 1 || selectedAccountHash.value);
});
```

**New methods:**

```javascript
async function startOAuth() {
    oauthLoading.value = true;
    oauthError.value = null;
    oauthStatus.value = 'pending';

    try {
        const result = await api.initiateSchwabOAuth({
            app_key: newProvider.value.credentials.app_key,
            app_secret: newProvider.value.credentials.app_secret,
            callback_url: newProvider.value.credentials.callback_url,
            base_url: newProvider.value.credentials.base_url,
        });

        oauthState.value = result.state;
        window.open(result.auth_url, '_blank');

        // Start polling every 2 seconds
        oauthPollInterval = setInterval(pollOAuthStatus, 2000);
    } catch (err) {
        oauthError.value = err.response?.data?.error || 'Failed to initiate OAuth flow';
        oauthStatus.value = null;
    } finally {
        oauthLoading.value = false;
    }
}

async function pollOAuthStatus() {
    try {
        const result = await api.getSchwabOAuthStatus(oauthState.value);
        oauthStatus.value = result.status;

        if (result.status === 'completed') {
            clearInterval(oauthPollInterval);
            oauthAccounts.value = result.accounts;

            // Auto-select if single account
            if (result.accounts.length === 1) {
                selectedAccountHash.value = result.accounts[0].hash_value;
            }
        } else if (result.status === 'failed') {
            clearInterval(oauthPollInterval);
            oauthError.value = result.error || 'Authorization failed';
            oauthStatus.value = null;
        }
    } catch (err) {
        clearInterval(oauthPollInterval);
        oauthError.value = 'Lost connection to server';
        oauthStatus.value = null;
    }
}

async function finalizeOAuth(accountHash) {
    if (!accountHash) return;
    savingProvider.value = true;

    try {
        const result = await api.selectSchwabAccount({
            state: oauthState.value,
            account_hash: accountHash,
            provider_name: newProvider.value.display_name,
            account_type: newProvider.value.account_type,
        });

        if (result.success) {
            showSuccess('Schwab provider created successfully');
            await loadProviderInstances();
            await refreshProviderData();
            closeDialog();
        }
    } catch (err) {
        oauthError.value = err.response?.data?.error || 'Failed to create provider';
    } finally {
        savingProvider.value = false;
    }
}

function cancelOAuth() {
    clearInterval(oauthPollInterval);
    oauthStatus.value = null;
    oauthState.value = null;
    oauthAccounts.value = [];
    oauthError.value = null;
    selectedAccountHash.value = null;
}
```

**Update `isSensitiveField`** (line 1093–1096):

```javascript
const isSensitiveField = (fieldName) => {
    const sensitiveFields = ['password', 'api_key', 'api_secret', 'app_key', 'app_secret', 'client_secret', 'refresh_token'];
    return sensitiveFields.includes(fieldName);
};
```

**Update `closeDialog`** to reset OAuth state:

```javascript
const closeDialog = () => {
    // ... existing reset ...
    cancelOAuth(); // NEW: clean up OAuth polling
};
```

**Update `getProviderIcon`** to include Schwab in SVG providers:

```javascript
const svgProviders = ['alpaca', 'tradier', 'public', 'tastytrade', 'schwab'];
```

#### 10d. Add CSS styles

Add styles for the OAuth-specific UI elements:

```css
.oauth-connect-section { ... }
.oauth-connect-btn { ... }
.oauth-help-text { ... }
.oauth-status { ... }
.oauth-error { ... }
.account-selection { ... }
.account-card { ... }
.account-card.selected { ... }
```

**Tests:** Manual testing with Schwab developer account:
- New provider flow: Enter App Key + Secret → Connect → Authorize → Account selection → Create
- Cancellation: Start OAuth → Cancel in Schwab → Verify error displayed
- Timeout: Start OAuth → Wait 10+ minutes → Verify expired message
- Invalid credentials: Enter wrong App Key → Connect → Verify error after token exchange

**Verify:** `cd trade-app && npm run dev` — navigate to Settings → Providers → Add Provider → Schwab

---

### Step 11 — ProvidersTab: Reconnect Flow for Expired Auth

**Goal:** Add the "Reconnect" button for Schwab provider instances with expired auth tokens.

**Files modified: 1 (ProvidersTab.vue) | Tests: manual**

#### 11a. Template changes

**In the provider instance list** (around line 56–116, within each provider instance card):

Add a conditional auth-expired warning after the status badge:

```html
<!-- Auth Expired Warning (Schwab only) -->
<div v-if="instance.provider_type === 'schwab' && isAuthExpired(instanceId)" class="auth-expired-warning">
    <i class="pi pi-exclamation-triangle"></i>
    <span>Auth expired</span>
    <Button
        label="Reconnect"
        icon="pi pi-refresh"
        severity="warning"
        size="small"
        text
        @click.stop="startReconnect(instanceId)"
        :loading="reconnectingInstances.has(instanceId)"
    />
</div>
```

#### 11b. Script changes

**New reactive state:**

```javascript
const reconnectingInstances = ref(new Set());
```

**New methods:**

```javascript
function isAuthExpired(instanceId) {
    // Check if the instance has auth_expired flag
    // This comes from TestCredentials() response when authExpired is true
    const instance = providerInstances.value[instanceId];
    return instance?.auth_expired === true;
}

async function startReconnect(instanceId) {
    const instance = providerInstances.value[instanceId];
    if (!instance) return;

    reconnectingInstances.value.add(instanceId);

    try {
        // Get existing credentials for this instance
        const creds = instance.visible_credentials || {};

        const result = await api.initiateSchwabOAuth({
            app_key: creds.app_key || '',
            app_secret: '', // User must re-enter — it's sensitive
            callback_url: creds.callback_url || 'https://127.0.0.1/callback',
            base_url: creds.base_url || 'https://api.schwabapi.com',
            instance_id: instanceId, // Flag for re-auth flow
        });

        // Open auth URL and set up polling
        oauthState.value = result.state;
        window.open(result.auth_url, '_blank');

        // For reconnect, we need to handle the flow differently
        // Show a reconnect dialog or inline UI
        editingInstance.value = instanceId;
        newProvider.value = {
            provider_type: instance.provider_type,
            account_type: instance.account_type,
            display_name: instance.display_name,
            credentials: creds,
        };

        oauthStatus.value = 'pending';
        showAddProviderDialog.value = true;
        dialogStep.value = 3;

        oauthPollInterval = setInterval(async () => {
            await pollOAuthStatus();
            // Auto-finalize for reconnect when completed
            if (oauthStatus.value === 'completed' && oauthAccounts.value.length > 0) {
                const hash = selectedAccountHash.value || oauthAccounts.value[0].hash_value;
                try {
                    const selectResult = await api.selectSchwabAccount({
                        state: oauthState.value,
                        account_hash: hash,
                    });
                    if (selectResult.success) {
                        showSuccess('Schwab provider reconnected successfully');
                        await loadProviderInstances();
                        closeDialog();
                    }
                } catch (err) {
                    oauthError.value = 'Failed to reconnect';
                }
            }
        }, 2000);

    } catch (err) {
        showError('Failed to start reconnection', 'Reconnect Error');
    } finally {
        reconnectingInstances.value.delete(instanceId);
    }
}
```

> **Design note:** The reconnect flow opens the dialog in edit mode (step 3) and automatically handles the OAuth polling + finalization. For re-auth, the `app_secret` is sensitive and won't be available in `visible_credentials`. The backend's `HandleAuthorize` endpoint receives it from the request body, so the user may need to re-enter it. An alternative is to read it from the credential store server-side — but the architecture doc specifies reusing existing credentials from the store. This needs a backend endpoint or the `HandleAuthorize` can read from the credential store when `instance_id` is provided.

#### 11c. CSS for auth-expired state

```css
.auth-expired-warning {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    color: var(--color-warning);
    font-size: var(--font-size-sm);
}

.auth-expired-warning i {
    color: var(--color-warning);
}
```

**Tests:** Manual testing:
- Set `authExpired = true` on a Schwab provider (or simulate expired refresh token)
- Verify "Reconnect" button appears on the instance card
- Click Reconnect → complete OAuth flow → verify provider works again

**Verify:** `cd trade-app && npm run dev` — test with an expired Schwab provider instance

---

## Summary

### Total File Inventory

| Category | New Files | Modified Files | Est. New Lines |
|----------|-----------|----------------|----------------|
| Backend — Go | 1 (`oauth.go`) | 5 (`schwab.go`, `auth.go`, `provider_types.go`, `credential_store.go`, `manager.go`) | ~600 |
| Backend — Go routes | 0 | 1 (`main.go`) | ~30 |
| Backend — Go tests | 1 (`oauth_test.go`) | 11 (all existing `*_test.go`) | ~500 |
| Frontend — Vue | 0 | 1 (`ProvidersTab.vue`) | ~200 |
| Frontend — JS | 0 | 1 (`api.js`) | ~15 |
| **Total** | **2** | **19** | **~1,345** |

### Test Inventory

| Step | New Tests | Modified Tests | Test Type |
|------|-----------|----------------|-----------|
| 1 — CredentialStore | 5 | 0 | Unit |
| 2 — Provider struct + auth | 6 | ~20 constructor fixes | Unit |
| 3 — ProviderTypes + Manager | 5 | 0 | Unit |
| 4 — OAuth State Store | 10 | 0 | Unit |
| 5 — Token exchange + helpers | 12 | 0 | Unit (httptest) |
| 6 — HandleAuthorize | 6 | 0 | Unit (httptest + gin) |
| 7 — HandleCallback | 10 | 0 | Unit (httptest + gin) |
| 8 — HandleOAuthStatus + SelectAccount | 14 | 0 | Unit (httptest + gin) |
| 9 — Route registration | 4 | 0 | Integration |
| 10 — Frontend OAuth flow | Manual | 0 | Manual / E2E |
| 11 — Frontend reconnect | Manual | 0 | Manual / E2E |
| **Total** | **~72 automated + manual** | **~20 fixes** | |

### Estimated Effort

| Phase | Steps | Est. Time | Risk |
|-------|-------|-----------|------|
| Phase 1: Backend Foundation | 1–3 | 2–3 hours | Low — struct/method additions, constructor signature changes |
| Phase 2: OAuth State | 4–5 | 2–3 hours | Low — self-contained state store + HTTP helpers |
| Phase 3: OAuth Handlers | 6–8 | 3–4 hours | Medium — multiple handler interactions, gin context testing |
| Phase 4: Route Registration | 9 | 30–60 min | Low — wiring only |
| Phase 5: Frontend | 10–11 | 3–4 hours | Medium — Vue dialog flow, polling, UX polish |
| **Total** | **11 steps** | **11–15 hours** | |

### Acceptance Criteria Mapping

| AC | Step | Verification |
|----|------|-------------|
| AC-1: Credential form shows only App Key, App Secret, Callback URL, Base URL | Step 3 + Step 10 | Unit test + manual |
| AC-2: "Connect to Schwab" button available | Step 10 | Manual |
| AC-3: Opens Schwab auth page in new tab | Step 10 | Manual |
| AC-4: Backend exchanges code for tokens | Step 7 | Unit tests (httptest) |
| AC-5: Account selection UI for multiple accounts | Step 10 | Manual |
| AC-6: Credentials persisted after authorization | Step 8 | Unit test |
| AC-7: Token rotation persisted | Step 2 | Unit test |
| AC-8: Auth expired surfaced, "Reconnect" shown | Steps 2 + 11 | Unit test + manual |
| AC-9: Re-auth reuses existing credentials | Step 11 | Manual |
| AC-10: `GET /callback` route exists | Step 9 | Integration test |
| AC-11: Callback handles error cases | Step 7 | Unit tests |
| AC-12: Callback excluded from auth middleware | Step 9 | Integration test |
| AC-13: Credentials in `provider_credentials.json` | Step 8 | Unit test |
| AC-14: Atomic credential writes | Step 1 | Unit test |
| AC-15: Cancellation handled gracefully | Step 7 + 10 | Unit test + manual |
| AC-16: Token exchange failure shows error | Step 7 + 10 | Unit test + manual |
| AC-17: No accounts shows error | Step 7 + 10 | Unit test + manual |

---

## End of Implementation Plan
