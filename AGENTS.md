# AGENTS.md

## Project Overview

JuicyTrade is a multi-service options trading platform. It consists of 4 microservices:

| Service | Language | Port | Purpose |
|---------|----------|------|---------|
| `trade-backend-go` | Go 1.24 | 8008 | Core API gateway, provider integrations, streaming, automation |
| `trade-app` | JavaScript (Vue 3) | 3001 | Frontend SPA |
| `strategy-service` | Python 3.11 | 8009 | Strategy management, backtesting, data import |
| `strategy-simulation-go` | Go | - | Strategy simulation (on-demand) |

## Project Structure

```
juicytrade/
  trade-backend-go/          # Go backend (Gin framework)
    cmd/server/main.go       # Entry point (~1770 lines, routes registered inline)
    internal/
      api/handlers/           # HTTP request handlers
      auth/                   # JWT, OAuth, middleware
      automation/             # Trade automation engine + indicators
      clients/                # HTTP clients to other services
      config/                 # Viper-based settings
      models/                 # Shared domain types
      providers/              # Broker integrations (alpaca/, tradier/, tastytrade/, schwab/)
        base/provider.go      # Provider interface (~30 methods)
        manager.go            # Routes operations to configured providers
      services/               # Business logic (IVx, watchlist)
      streaming/              # WebSocket aggregation + health monitoring
      utils/                  # Cache, HTTP helpers, path resolution

  trade-app/                 # Vue.js frontend (Vite 5)
    src/
      components/             # ~50 Vue components (flat + feature-grouped subdirs)
        auth/, automation/, notifications/, settings/, setup/, strategies/
      composables/            # 15 composables (use* pattern)
      services/               # Singleton stores + API clients
      utils/                  # Pure utility modules
      views/                  # 7 route views
      router/index.js         # Vue Router with auth/setup guards
      assets/styles/theme.css # CSS custom properties design system

  strategy-service/          # Python backend (FastAPI)
    src/
      main.py                # FastAPI app entry point
      api/                   # REST endpoints
      core/                  # Strategy framework (rules, actions, flow engine)
      models/                # Domain + state models
      execution/             # Order execution + position management
      persistence/           # SQLAlchemy ORM, strategy store
      backtest/              # Backtesting engine
      config.py              # Pydantic Settings (env prefix: STRATEGY_)

  charts/juicytrade/         # Helm chart (backend + frontend only)
  docker/                    # nginx.conf, supervisord.conf
  docs/                      # Issue-specific design docs
```

## Build & Run Commands

### All Services

```sh
make dev              # Run backend + strategy + frontend concurrently
make dev-all          # Run all 4 services
make build            # Build all Docker images
make push             # Push all images to registry
make test             # Run all tests (backend + simulation + strategy)
```

### Go Backend (`trade-backend-go`)

```sh
make run-backend      # go run cmd/server/main.go (port 8008)
make test-backend     # go test ./...
make logs-backend     # Hot reload with air
```

### Vue Frontend (`trade-app`)

```sh
# From trade-app/ directory:
npm install           # Install dependencies
npm run dev           # Vite dev server (port 3001)
npm run build         # Production build
npm run test          # Vitest (all tests)
npx vitest run        # Single test run (no watch)
```

### Python Strategy Service (`strategy-service`)

```sh
make run-strategy     # uvicorn src.main:app --port 8009
make test-strategy    # python -m pytest
```

### Docker

```sh
docker compose up                          # Backend + frontend
docker compose --profile simulation up     # Include simulation service
```

## Testing

### Go Backend
- Tests co-located with source (`*_test.go`), standard Go convention
- QA tests use `qa_` prefix (edge cases, concurrency)
- Uses `httptest.NewServer` for mock broker APIs, no mocking framework
- Standard library `testing.T` (no testify)
- Run: `cd trade-backend-go && go test ./...`

### Vue Frontend
- Vitest + @vue/test-utils + happy-dom
- Tests in `trade-app/tests/` (not co-located), named `*.test.js`
- QA tests use `qa-` prefix
- Global setup mocks `authService` and `fetch` for auth endpoints
- Heavy use of `vi.mock()` for dependency isolation
- Run: `cd trade-app && npx vitest run`

### Python Strategy Service
- Minimal: 1 integration smoke test in `tests/test_backtest_simple.py`
- Run: `cd strategy-service && python -m pytest`

## Code Conventions

### Commit Messages
- **Conventional Commits**: `feat:`, `fix:`, `docs:`, `test:` prefixes
- Optional scope in parentheses: `feat(schwab):`, `fix(schwab):`
- Lowercase, imperative mood, no trailing period

### Branch Naming
- `fleet/issue-{N}-{kebab-case-description}` (linked to issue numbers)
- `{developer}/{feature}` for personal branches

### Go Backend Conventions
- **Packages**: lowercase single-word names
- **JSON tags**: `snake_case` (matching Python API contract)
- **Error handling**: `fmt.Errorf("...: %w", err)` wrapping, early returns on nil/error
- **API responses**: `{"success": bool, "data": ..., "message": "..."}`
- **Concurrency**: `sync.RWMutex` for shared state, `sync.Once` for singletons, `context.Context` with timeouts
- **Logging**: `log/slog` (structured), emoji prefixes in messages
- **Config**: Viper + `.env` files, `config.Settings` struct
- **Provider pattern**: All brokers implement `base.Provider` interface; `ProviderManager` routes operations via configurable service-to-provider map

### Vue Frontend Conventions
- **JavaScript only** (no TypeScript in source, tsconfig.json is for IDE only)
- **Options API with `setup()` function** (not `<script setup>`, not classic Options API)
- **State management**: Custom singleton classes with Vue `reactive()`/`ref()` (no Vuex/Pinia)
- **Component naming**: PascalCase files, `name` property declared in `export default`
- **Styling**: Scoped CSS + CSS custom properties (no preprocessor, no Tailwind)
- **UI library**: PrimeVue 3.46 (globally registered, dark theme `aura-dark-noir` with custom overrides)
- **Imports**: Relative paths with `.js` extensions (path alias `@/` available but rarely used)
- **Cross-component events**: `window.dispatchEvent(new CustomEvent(...))` pattern
- **Charts**: Chart.js 4 + TradingView Lightweight Charts 5
- **HTTP**: Axios with circuit breaker + retry logic
- **WebSocket**: Offloaded to Web Worker (`streaming-worker.js`)

### Python Strategy Service Conventions
- **No `__init__.py` files**: Uses `sys.path` manipulation instead of proper packaging
- **Absolute imports from `src`**: e.g., `from src.config import settings`
- **Type hints**: Used extensively on parameters and returns
- **Docstrings**: Google-style with `Args:` and `Returns:` sections
- **Singletons**: Module-level instances (`settings = Settings()`, `strategy_store = StrategyStore()`)
- **Pydantic v2**: `BaseModel` for API models, `BaseSettings` for config
- **Env prefix**: `STRATEGY_` for all settings
- **Logging**: `logging.getLogger(__name__)`, emoji prefixes in messages

## Key Architecture Patterns

### Provider Abstraction (Go Backend)
The central pattern is a **strategy/provider pattern** for multi-broker support:
- `base.Provider` interface defines ~30 methods (market data, orders, streaming, account)
- Concrete implementations: `alpaca/`, `tradier/`, `tastytrade/`, `schwab/`
- `ProviderManager` routes each service operation (e.g., `options_chain`, `streaming_quotes`) to a configured provider instance
- `ConfigManager` persists routing in `provider_config.json`
- `CredentialStore` persists provider credentials in `provider_credentials.json`

### Streaming Architecture
- Go backend manages WebSocket connections to multiple broker APIs concurrently
- `StreamingManager` (singleton) aggregates data from all active providers
- `StreamingHealthManager` monitors connection health with auto-recovery
- Frontend connects to backend via WebSocket, offloaded to a Web Worker for sleep resilience

### Frontend Data Flow
- `SmartMarketDataStore` is the central data hub (WebSocket + REST, health monitoring)
- Composables wrap stores with reactive computed properties for component consumption
- `SelectedLegsStore` tracks selected option legs across all components
- Stores are provided via `app.provide()` and direct ES module imports

### Strategy Framework (Python)
- `BaseStrategy` abstract class with declarative flow engine
- Composable rules: `Rules.AllOf()`, `Rules.AnyOf()`, `Rules.Not()`
- Graph-based `FlowEngine` with `DecisionNode` and `ActionNode`
- Dynamic strategy loading via `exec()` in `strategy_registry.py`
- SQLite persistence via SQLAlchemy

## Environment & Configuration

### Required Environment Variables
- Go backend: loaded via Viper from `.env` (searches `.`, `../`, `../../`)
- Python strategy service: `STRATEGY_` prefix via Pydantic Settings
- Frontend: `VITE_` and `JUICYTRADE_` prefixed env vars

### Key Config Files (not committed, in .gitignore)
- `.env` - API keys, auth config, secrets
- `provider_credentials.json` - Broker credentials
- `provider_config.json` - Service-to-provider routing
- `watchlist.json` - User watchlists

### No CI/CD
- No GitHub Actions, GitLab CI, or other pipeline configs
- Builds and pushes are manual via `Makefile`
- Docker images pushed to `registry.gitlab.com/schardosin/juicytrade`

### No Linters/Formatters Configured
- No ESLint, Prettier, ruff, black, or golangci-lint config files
- Code style is maintained by convention
