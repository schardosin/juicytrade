# QA Test Results: Schwab OAuth Authorization Flow

**Issue:** #20 (TD Ameritrade as a Provider) — OAuth Enhancement
**Test Plan:** [test-plan-oauth.md](./test-plan-oauth.md)
**Date:** 2025-07-15
**QA Verdict:** ✅ PASS — All acceptance criteria verified, all tests pass

---

## Executive Summary

All 13 test steps executed successfully. 30 new QA tests written across 3 test files. 292 total tests pass across schwab and providers packages with zero failures and zero regressions. All 17 acceptance criteria and 4 non-functional requirements verified.

One low-severity finding documented (potential XSS in renderCallbackPage — error messages injected via fmt.Sprintf without html.EscapeString). Risk is low since error strings are backend-generated, not user-input.

---

## Test Execution Summary

| Step | Area | AC Coverage | Result | Tests |
|------|------|-------------|--------|-------|
| 1 | CredentialStore.UpdateCredentialFields | AC-13, AC-14 | ✅ PASS | 8/8 (5 existing + 3 new) |
| 2 | Provider struct (instanceID, credentialUpdater, authExpired) | AC-7, AC-8 | ✅ PASS | 18/18 (existing + 3 new) |
| 3 | Provider types and credential fields | AC-1 | ✅ PASS | 9/9 (5 existing + 4 new) |
| 4 | OAuth state store | NFR-1, NFR-2, NFR-3 | ✅ PASS | 14/14 (11 existing + 3 new) with -race |
| 5 | Token exchange and account fetch helpers | AC-4, AC-17 | ✅ PASS | 18/18 (14 existing + 4 new) |
| 6 | HandleAuthorize endpoint | AC-2, AC-3 | ✅ PASS | 8/8 (6 existing + 2 new) |
| 7 | HandleCallback endpoint | AC-4, AC-10, AC-11, AC-15, AC-16 | ✅ PASS | 13/13 (10 existing + 3 new) |
| 8 | HandleOAuthStatus and HandleSelectAccount | AC-5, AC-6, AC-9 | ✅ PASS | 18/18 (14 existing + 4 new) |
| 9 | Route registration in main.go | AC-10, AC-12 | ✅ PASS | Build + code review (27/27 checks) |
| 10 | Frontend OAuth flow | AC-2, AC-3, AC-5, AC-8 | ✅ PASS | Build + code review |
| 11 | Manager.go factory and ReinitializeInstance | AC-7, AC-9 | ✅ PASS | Build + code review |
| 12 | Code quality, regression, cross-cutting | NFR-1 through NFR-4 | ✅ PASS | Full backend suite passes |
| 13 | Edge cases and security | NFR-1, NFR-2, NFR-3 | ✅ PASS | 4 new security tests with -race |

---

## QA Test Files Created

| File | Tests | Purpose |
|------|-------|---------|
| trade-backend-go/internal/providers/schwab/qa_oauth_edge_test.go | 23 | OAuth state store, helpers, handlers, CSRF, concurrency, backward compatibility |
| trade-backend-go/internal/providers/qa_credential_store_test.go | 3 | UpdateCredentialFields edge cases |
| trade-backend-go/internal/providers/qa_provider_types_test.go | 4 | AuthMethod JSON omitempty, defaults, credential field validation |
| **Total** | **30** | |

---

## Acceptance Criteria Traceability

| AC | Description | Status | Verified By |
|----|-------------|--------|-------------|
| AC-1 | Credential form: only App Key, App Secret, Callback URL, Base URL | ✅ | Step 3: TestSchwabCredentialFieldNames, TestSchwabNoRefreshTokenOrAccountHash |
| AC-2 | "Connect to Schwab" button available | ✅ | Step 6: TestHandleAuthorize_Success; Step 10: code review |
| AC-3 | Opens Schwab auth page in new tab | ✅ | Step 6: TestHandleAuthorize_AuthURLFormat; Step 10: code review |
| AC-4 | Backend exchanges code for tokens | ✅ | Steps 5, 7: TestExchangeCodeForTokens_Success, TestHandleCallback_Success_SingleAccount |
| AC-5 | Account selection UI for multiple accounts | ✅ | Step 8: TestHandleOAuthStatus_Completed; Step 10: code review |
| AC-6 | Credentials persisted after authorization | ✅ | Step 8: TestHandleSelectAccount_CredentialsMapComplete |
| AC-7 | Token rotation persisted | ✅ | Step 2: TestRefreshAccessToken_RotatesAndPersists; Step 11: code review |
| AC-8 | Auth expired surfaced, "Reconnect" shown | ✅ | Step 2: TestTestCredentials_AuthExpired; Step 10: code review |
| AC-9 | Re-auth reuses existing credentials | ✅ | Step 8: TestHandleSelectAccount_ReAuth; Step 10, 11: code review |
| AC-10 | GET /callback route exists | ✅ | Step 9: code review (main.go route registration) |
| AC-11 | Callback handles error cases | ✅ | Step 7: 10 error scenario tests |
| AC-12 | Callback excluded from auth middleware | ✅ | Step 9: code review (routes before auth middleware) |
| AC-13 | Credentials in provider_credentials.json | ✅ | Step 1: TestUpdateCredentialFields_Persistence |
| AC-14 | Atomic credential writes | ✅ | Step 1: TestUpdateCredentialFields_DoesNotClobberTopLevel |
| AC-15 | Cancellation handled gracefully | ✅ | Step 7: TestHandleCallback_UserCancelled |
| AC-16 | Token exchange failure shows error | ✅ | Step 7: TestHandleCallback_TokenExchangeFails |
| AC-17 | No accounts shows error | ✅ | Step 7: TestHandleCallback_NoAccounts |

---

## Non-Functional Requirements

| NFR | Description | Status | Verified By |
|-----|-------------|--------|-------------|
| NFR-1 | Security (no plaintext tokens, CSRF) | ✅ | Steps 4, 12, 13: json:"-" tags verified, CSRF token tested, no raw tokens in logs |
| NFR-2 | Timeout (10-min state TTL) | ✅ | Steps 4, 7: TestOAuthStore_GetState_Expired, TestHandleCallback_ExpiredState |
| NFR-3 | Idempotency (duplicate callback) | ✅ | Steps 7, 13: TestHandleCallback_DuplicateCallback, TestConcurrentCallbacks_OnlyOneSucceeds |
| NFR-4 | Backward compatibility | ✅ | Steps 2, 12, 13: TestBackwardCompatibility_ManualTokens |

---

## Findings

### Low Severity

1. **Potential XSS in renderCallbackPage** (oauth.go:342-420): Error messages are injected into HTML via `fmt.Sprintf` without `html.EscapeString()`. Risk is LOW because error messages are backend-generated strings from Schwab API responses, not direct user input. Recommendation: Apply `html.EscapeString()` as defense-in-depth.

### Pre-existing Issues (Not Related to OAuth Changes)

1. **3 streaming race conditions** in `streaming.go` (lines 200, 308, 724) — detected with `-race` flag, pre-existing before OAuth enhancement
2. **1 go vet warning** — unreachable code in `tradier.go:2510` — pre-existing

---

## Test Metrics

| Metric | Value |
|--------|-------|
| QA tests written | 30 |
| QA test files | 3 |
| Total tests (schwab + providers) | 292 |
| Test failures | 0 |
| Regressions | 0 |
| Acceptance criteria verified | 17/17 |
| Non-functional requirements verified | 4/4 |
| Code review checks passed | 75+ |
| Go backend build | ✅ |
| Vue frontend build | ✅ |
