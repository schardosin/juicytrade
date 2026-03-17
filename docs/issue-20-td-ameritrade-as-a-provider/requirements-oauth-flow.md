# Requirements: Schwab OAuth Authorization Flow

## Issue Reference
**Issue:** #20 (TD Ameritrade as a Provider) — Follow-up Enhancement
**Branch:** `fleet/issue-20-td-ameritrade-as-a-provider`
**Status:** Draft — Awaiting Customer Approval

---

## 1. Problem Statement

The current Schwab provider implementation requires users to manually provide a **Refresh Token** and **Account Hash** as credential fields when configuring the provider. These values are difficult to obtain manually — the user would need to manually perform OAuth flows outside the application and paste in tokens they obtained from developer tools or external scripts.

**The correct flow is:**
1. User provides only **App Key** and **App Secret** (obtained from the Schwab Developer Portal)
2. The application handles the entire OAuth authorization flow automatically
3. The application obtains the Refresh Token, Access Token, and Account Hash on behalf of the user
4. The application persists these values and maintains them going forward

This enhancement replaces the manual credential entry with a proper browser-based OAuth authorization flow, making the Schwab provider setup as seamless as adding any other provider.

---

## 2. Context & Motivation

- **User Experience:** Currently, a user would need to manually navigate the Schwab OAuth flow in their browser, extract the authorization code from the redirect URL, exchange it for tokens using curl/Postman, then paste the refresh token and separately look up their account hash. This is a terrible user experience.
- **Security:** Users should not need to copy/paste sensitive tokens through external tools.
- **Consistency:** Other providers in JuicyTrade (Alpaca, Tradier) handle authentication through simple API key entry. While Schwab requires OAuth, the app should abstract that complexity away from the user.
- **Token Lifecycle:** The Schwab refresh token expires after ~7 days. The application needs to handle token rotation and re-authentication gracefully.

---

## 3. Schwab OAuth Flow (Technical Background)

The Schwab API uses OAuth 2.0 Authorization Code Grant:

1. **Authorization URL:** `https://api.schwabapi.com/v1/oauth/authorize?client_id={app_key}&redirect_uri={callback_url}&response_type=code`
2. **User logs in** via Schwab's login page in their browser
3. **Schwab redirects** to the callback URL with an authorization code: `{callback_url}?code={authorization_code}&session={session_id}`
4. **Backend exchanges** the authorization code for tokens via `POST https://api.schwabapi.com/v1/oauth/token` with `grant_type=authorization_code`
5. **Backend receives** Access Token (~30 min), Refresh Token (~7 days), and other metadata
6. **Backend fetches** account numbers via `GET https://api.schwabapi.com/trader/v1/accounts/accountNumbers` to get the Account Hash

**Registered Callback URLs (in Schwab Developer Portal):**
- `https://juicytrade.muxpie.com/callback`
- `https://127.0.0.1/callback`

---

## 4. Functional Requirements

### FR-1: Simplified Credential Fields
The Schwab provider credential configuration shall only require users to provide:
- **App Key** (required) — from Schwab Developer Portal
- **App Secret** (required) — from Schwab Developer Portal
- **Callback URL** (required, with default) — the registered OAuth redirect URI

The following fields shall be **removed from the user-facing credential form:**
- ~~Refresh Token~~ (obtained automatically via OAuth flow)
- ~~Account Hash~~ (obtained automatically via account numbers API)

These values shall still be stored internally in the credential store but are never entered by the user.

### FR-2: OAuth Authorization Initiation
When a user is configuring a new Schwab provider instance:
1. After entering App Key, App Secret, and Callback URL, a **"Connect to Schwab"** button shall be displayed
2. Clicking this button shall generate the Schwab OAuth authorization URL and open it in a new browser tab/window
3. The authorization URL shall include: `client_id` (app_key), `redirect_uri` (callback_url), and `response_type=code`
4. The backend shall store a temporary state/session to correlate the callback with the originating setup request

### FR-3: OAuth Callback Handling
The backend shall expose a callback endpoint to receive the OAuth authorization code:
- **Route:** `GET /callback` (to match the registered callback URLs)
- This endpoint shall:
  1. Extract the `code` parameter from the query string
  2. Exchange the authorization code for tokens via `POST /v1/oauth/token` with `grant_type=authorization_code`
  3. Store the received Access Token and Refresh Token
  4. Fetch the account numbers via `GET /trader/v1/accounts/accountNumbers`
  5. If multiple accounts exist, redirect to a UI page showing account selection (see FR-4)
  6. If only one account exists, auto-select it
  7. Persist the Refresh Token and Account Hash in the credential store
  8. Redirect the user back to the provider setup UI with a success indication

### FR-4: Account Selection
When the `/accounts/accountNumbers` endpoint returns multiple accounts:
1. The user shall be presented with a selection UI showing all available accounts
2. Each account shall display the account number (or a masked version for security)
3. The user selects which account to use for this provider instance
4. The selected account's hash value becomes the `account_hash` stored in credentials

### FR-5: Token Persistence & Rotation
- The Refresh Token and Account Hash shall be stored in the existing credential store (`provider_credentials.json`), the same config file used by all other providers
- When Schwab rotates the refresh token during a token refresh (the response includes a new `refresh_token`), the provider shall **persist the new refresh token** back to the credential store automatically
- This requires the provider to have a mechanism to update its own credentials in the store (currently it logs a warning but cannot persist)

### FR-6: Re-Authentication Flow
When the refresh token expires (~7 days without use) or becomes invalid:
1. The provider shall detect the `401 Unauthorized` response from the token endpoint
2. The provider shall surface a clear error message indicating re-authentication is needed
3. The UI shall provide a way to re-initiate the OAuth flow (e.g., a "Reconnect" button on the provider instance card) without requiring the user to delete and recreate the provider
4. The re-authentication flow reuses the existing App Key, App Secret, and Callback URL — only the tokens are refreshed

### FR-7: Callback URL Configuration
- The default callback URL shall be `https://127.0.0.1/callback` (matching the registered Schwab callback)
- The user may change this to `https://juicytrade.muxpie.com/callback` or any other registered callback URL
- The callback endpoint on the backend must be accessible at the path `/callback` (not under `/api/`) to match the registered URLs

### FR-8: Base URL Configuration (Advanced)
- The Base URL field shall remain as an advanced/optional field with default `https://api.schwabapi.com`
- For paper accounts, the same base URL is used (Schwab sandbox uses the same API endpoint)

---

## 5. Acceptance Criteria

### Setup Flow
- **AC-1:** When adding a Schwab provider, the credential form shows only: App Key, App Secret, Callback URL, and Base URL (optional). Refresh Token and Account Hash are NOT shown.
- **AC-2:** After entering App Key and App Secret, a "Connect to Schwab" button is available.
- **AC-3:** Clicking "Connect to Schwab" opens the Schwab authorization page in a new browser tab.
- **AC-4:** After the user authorizes in Schwab, the browser redirects to the callback URL and the backend successfully exchanges the code for tokens.
- **AC-5:** If the user's Schwab account has multiple accounts, an account selection UI is displayed.
- **AC-6:** After successful authorization (and account selection if needed), the provider instance is created with all necessary credentials (App Key, App Secret, Callback URL, Refresh Token, Account Hash) stored in `provider_credentials.json`.

### Token Lifecycle
- **AC-7:** When the Schwab API rotates the refresh token, the new token is automatically persisted to the credential store.
- **AC-8:** When the refresh token expires, the provider surfaces a clear error and the UI shows a "Reconnect" option.
- **AC-9:** Re-authentication via "Reconnect" reuses existing App Key/Secret and only refreshes tokens.

### Callback Endpoint
- **AC-10:** A `GET /callback` route exists on the backend that handles the Schwab OAuth redirect.
- **AC-11:** The callback endpoint properly handles error cases (missing code parameter, invalid code, expired code).
- **AC-12:** The callback endpoint is excluded from the authentication middleware (it must be accessible without a JuicyTrade auth token).

### Persistence
- **AC-13:** Refresh Token and Account Hash are stored in the same `provider_credentials.json` file alongside other provider configurations.
- **AC-14:** The credential store is updated atomically — partial writes do not corrupt the file.

### Error Handling
- **AC-15:** If the user cancels the Schwab authorization, the callback handles the error gracefully and shows a meaningful message.
- **AC-16:** If the authorization code exchange fails (network error, invalid credentials), a clear error message is shown.
- **AC-17:** If no accounts are returned from the account numbers endpoint, an appropriate error is shown.

---

## 6. Scope Boundaries

### In Scope
- Backend OAuth callback endpoint (`/callback`)
- Authorization code exchange for tokens
- Automatic account hash retrieval and account selection UI
- Token persistence and rotation in credential store
- Re-authentication flow for expired tokens
- Updated credential field definitions (removing Refresh Token and Account Hash from user-facing form)
- Frontend changes to the provider setup dialog for Schwab (Connect button, account selection)

### Out of Scope
- Changes to other providers (Alpaca, Tradier, TastyTrade, Public)
- Changes to the existing auth system (JuicyTrade's own OAuth for user login)
- Mobile-specific UI for the OAuth flow
- Automated refresh token renewal before expiry (the existing 5-minute buffer refresh for access tokens is sufficient; refresh token renewal happens naturally during access token refresh)

---

## 7. Non-Functional Requirements

- **NFR-1: Security** — Authorization codes and tokens must never be logged in plain text. The callback endpoint must validate state parameters to prevent CSRF attacks.
- **NFR-2: Timeout** — The OAuth flow (from clicking "Connect" to callback completion) should handle cases where the user takes a long time or abandons the flow. State tokens should expire after 10 minutes.
- **NFR-3: Idempotency** — If the callback is hit multiple times with the same code, only the first should succeed; subsequent attempts should fail gracefully.
- **NFR-4: Backward Compatibility** — Existing Schwab provider instances that were configured with manual refresh tokens must continue to work. The provider should support both modes: OAuth-flow-obtained tokens and manually-provided tokens.

---

## 8. Technical Notes

### Files Likely Affected
**Backend (Go):**
- `trade-backend-go/internal/providers/provider_types.go` — Update credential field definitions
- `trade-backend-go/internal/providers/schwab/schwab.go` — Constructor changes, credential update mechanism
- `trade-backend-go/internal/providers/schwab/auth.go` — Add authorization code exchange, token persistence
- `trade-backend-go/internal/providers/manager.go` — Support for credential updates from within provider
- `trade-backend-go/cmd/server/main.go` — Register `/callback` route, OAuth initiation endpoint
- New file: `trade-backend-go/internal/providers/schwab/oauth.go` — OAuth flow handlers

**Frontend (Vue.js):**
- `trade-app/src/components/settings/ProvidersTab.vue` — Custom Schwab setup flow, Connect button, account selection
- `trade-app/src/services/api.js` — New API calls for OAuth initiation and status checking

### Existing Infrastructure
- The backend already has an OAuth system for JuicyTrade user authentication (`internal/auth/`) with `OAuthAuthorize` and `OAuthCallback` handlers. However, this is for **app authentication**, not broker authentication. The Schwab OAuth flow is separate and should NOT reuse the auth middleware's OAuth infrastructure — it needs its own dedicated endpoints.
- The `/auth/oauth/callback` path is already used by the app auth system. The Schwab callback must use `/callback` to match the registered URLs.
