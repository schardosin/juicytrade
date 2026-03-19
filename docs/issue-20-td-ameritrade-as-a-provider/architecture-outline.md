# Architecture Document Outline — Schwab Provider (Issue #20)

## Sections to Write

1. **Overview & Design Goals** — High-level summary, design principles, key trade-offs.

2. **System Context & Integration Points** — How the Schwab provider fits into JuicyTrade's existing architecture. Component diagram showing provider manager, streaming manager, frontend, and external Schwab APIs.

3. **Package & File Structure** — Multi-file organization within `providers/schwab/`, rationale for splitting, and file responsibility map.

4. **Struct Design & Type Definitions** — `SchwabProvider` struct with all fields, embedded `BaseProviderImpl`, and supporting types (token state, streaming state, rate limiter).

5. **Provider Registration & Factory Integration** — Changes to `provider_types.go` and `manager.go`. Credential field definitions, capability declarations, factory case.

6. **OAuth 2.0 Token Management** — Token lifecycle, thread-safe refresh with mutex, proactive refresh strategy, error handling for expired refresh tokens.

7. **REST Client Design** — Dual base-path architecture (marketdata vs trader), authenticated request helper, response parsing patterns, error handling.

8. **Rate Limiting Strategy** — Token bucket or sliding window approach, backoff on 429, integration with the HTTP request flow.

9. **Data Transformation Layer** — Mapping Schwab API responses to JuicyTrade models (quotes, options chains, positions, orders). Option symbol format conversion (OCC ↔ Schwab).

10. **WebSocket Streaming Architecture** — Connection lifecycle, login handshake, subscription management, numerical field decoding, message routing to StreamingCache, reconnection strategy.

11. **Account Event Streaming** — ACCT_ACTIVITY subscription, order event parsing, callback dispatch.

12. **Error Handling & Resilience Patterns** — Error classification, retry strategy, circuit breaker integration, logging conventions.

13. **Paper Account (Sandbox) Support** — URL switching, limitation warnings, configuration approach.

14. **Implementation Guidance & Phasing** — Recommended implementation order, dependencies between components, estimated complexity per file.

15. **Appendix: Schwab Streaming Field Maps** — Complete numerical field → name mappings for LEVELONE_EQUITIES and LEVELONE_OPTIONS.
