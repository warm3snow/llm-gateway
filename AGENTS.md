# AGENTS.md — LLM Gateway

> This document helps AI coding agents (Cursor, GitHub Copilot, etc.) quickly understand the project structure, conventions, and how to add new features.

---

## Project Overview

LLM Gateway is a **unified API gateway** for multiple LLM providers (OpenAI, Anthropic, Gemini, etc.). It provides a single OpenAI-compatible API endpoint that routes requests to configured providers with support for:

- Virtual key management (budget tracking, rate limiting)
- Response caching (memory + Redis)
- JWT-based admin authentication
- Next.js admin dashboard

**Module path:** `github.com/warm3snow/llm-gateway`

---

## Directory Structure

```
llm-gateway/
├── cmd/server/main.go          # Application entrypoint
├── internal/
│   ├── config/              # Viper-based config loading
│   ├── database/            # GORM database layer (SQLite/PostgreSQL)
│   ├── handler/             # Gin HTTP handlers (auth, stats, usage, virtual keys)
│   ├── middleware/          # Gin middleware (logger, recovery, CORS, JWT, virtual key auth, cache, usage record)
│   ├── models/              # GORM models (VirtualKey, UsageRecord, ProviderConfig, CacheEntry, ModelPricing)
│   ├── provider/            # LLM provider implementations (OpenAI, Anthropic, Gemini, etc.)
│   ├── routing/             # Request routing (load-balance, fallback, conditional)
│   ├── service/             # Business logic (stats, usage, virtual keys)
│   └── types/               # Shared type definitions
├── pkg/
│   ├── cache/               # Cache interface + memory/Redis implementations
│   ├── encryption/         # AES-256-GCM encryption for API keys
│   ├── guard-rail/          # Request/response guardrail framework
│   ├── proxy/               # Core proxy handler (HTTP request forwarding)
│   └── retry/               # Retry logic with backoff
├── web/frontend/           # Next.js 14+ admin dashboard (App Router)
│   ├── app/                 # Pages (dashboard, providers, virtual-keys, logs, analytics, settings)
│   ├── components/          # React components (ui/, layout/, providers/)
│   └── lib/                # API client (api.ts), utilities
├── configs/config.yaml       # Runtime configuration (YAML)
├── Makefile                 # Build, run, test, deploy commands
└── .gitignore              # Ignores binaries, .next/, node_modules/, etc.
```

---

## Architecture & Key Patterns

### Request Flow

```
Client Request
  → Gin Middleware Stack (logger → recovery → CORS → [JWT Auth for /api/v1/*] → [VirtualKeyAuth for /v1/*])
  → Proxy Handler (pkg/proxy)
  → Routing Engine (internal/routing)
  → Provider Adapter (internal/provider/{name})
  → Upstream LLM API
  → Response (with optional caching via CacheMiddleware)
```

### Provider Interface (`internal/provider/provider.go`)

All providers implement:
```go
type Provider interface {
    ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)
    Completion(ctx context.Context, req *types.CompletionRequest) (*types.ChatCompletionResponse, error)
    Embedding(ctx context.Context, req *types.EmbeddingRequest) ([]float64, error)
    // ...other methods
}
```

To add a new provider: create `internal/provider/{name}/{name}.go` implementing `Provider`.

### Handler Pattern (`internal/handler/`)

Each handler struct takes dependencies via constructor:
```go
type StatsHandler struct {
    service *service.StatsService
    cfg     *config.Config
}
func NewStatsHandler(cfg *config.Config) *StatsHandler { ... }
func (h *StatsHandler) RegisterRoutesWithAuth(router, jwtMiddleware) { ... }
```

**Important:** Admin API routes (`/api/v1/*`) use `RegisterRoutesWithAuth()` to apply JWT middleware. Proxy routes (`/v1/*`) use `middleware.VirtualKeyAuth()` instead.

### Virtual Key Auth vs JWT Auth

| Middleware | Applies To | Purpose |
|---|---|---|
| `middleware.VirtualKeyAuth()` | `/v1/*` (proxy routes) | Validates virtual key from `x-llm-gateway-api-key` header |
| `middleware.JWTAuth(cfg)` | `/api/v1/*` (admin routes) | Validates JWT Bearer token for admin API access |

### Configuration (`internal/config/config.go`)

- Loaded via **Viper** from `configs/config.yaml`
- Supports environment variable overrides: `LLM_GATEWAY_SERVER_PORT=9090`
- Key config blocks: `server`, `gateway`, `database`, `cache`, `logging`, `security`

---

## How to Add a New Feature

### Add a New API Endpoint

1. Create service in `internal/service/{name}_service.go`
2. Create handler in `internal/handler/{name}_handler.go`
3. Register routes in `cmd/server/main.go` (apply `jwtMiddleware` for admin routes)
4. Update frontend `web/frontend/lib/api.ts` with new API function
5. Create/Update frontend page in `web/frontend/app/{page}/page.tsx`

### Add a New LLM Provider

1. Create `internal/provider/{name}/{name}.go`
2. Implement the `Provider` interface
3. Register in `internal/provider/provider.go` factory
4. Add to `supportedProviders` in `configs/config.yaml`

### Add a New Frontend Page

1. Create `web/frontend/app/{page}/page.tsx`
2. Add route to `web/frontend/components/layout/Sidebar.tsx`
3. Add API function to `web/frontend/lib/api.ts`
4. Run `cd web/frontend && npm run build` to verify TypeScript compiles

---

## Frontend Conventions

- **Path alias:** `@/` maps to `web/frontend/` (see `tsconfig.json`)
- **UI components:** Use `shadcn/ui` style components in `components/ui/` (built with `class-variance-authority`)
- **Data fetching:** Use `api.{function}()` from `@/lib/api.ts` (wraps `fetch` with JWT auth + error handling)
- **State:** React `useState` + `useEffect`; React Query (`@tanstack/react-query`) for server state
- **Styling:** Tailwind CSS utility classes
- **TypeScript:** Strict mode enabled; all API responses must have matching interfaces

## Backend Conventions

- **Error responses:** Always use `types.ErrorResponse` struct; return via `c.AbortWithStatusJSON()`
- **JSON field names:** Use `snake_case` in JSON (Gin's default); frontend uses `snake_case` to match
- **Database:** GORM models in `internal/models/`; run `database.Migrate(&model)` on startup
- ** imports:** Run `goimports -w .` before committing

---

## Common Pitfalls for AI Agents

1. **`snake_case` vs `camelCase`:** Backend JSON uses `snake_case` (Gin default). Frontend `api.ts` and page interfaces must use `snake_case` to match.
2. **`AbortWithStatusJSON` vs `JSON`:** Use `AbortWithStatusJSON` for errors (stops the middleware chain); use `c.JSON` for success responses.
3. **JWT middleware ordering:** `JWTAuth` must be applied per-route-group (not globally) because `/v1/*` proxy routes use `VirtualKeyAuth` instead.
4. **Smart quote corruption:** The `Write` tool sometimes corrupts `"`/`"` into `"`/`"`. Always verify with `python3 -c "open(...).read()"` or use `cat` to inspect.
5. **Frontend build errors:** Next.js build runs TypeScript type-checking. Fix all TypeScript errors before declaring a task complete.
6. **`RegisterRoutes` vs `RegisterRoutesWithAuth`:** Handlers have two registration methods — without JWT (public routes like `/auth/login`) and with JWT (protected admin routes).

---

## Makefile Commands

| Command | Description |
|---|---|
| `make dev` | Start backend + frontend in foreground (Ctrl+C to stop) |
| `make start` | Start backend + frontend in background; logs to `/tmp/llm-gateway-*.log` |
| `make stop` | Kill all backend + frontend processes |
| `make restart` | `stop` + `start` |
| `make build` | Build Go binary + Next.js static export |
| `make test` | Run all Go tests |
| `make clean` | Remove `bin/`, `llm-gateway.db`, `.next/` |
| `make deps` | `go mod tidy` + `npm install` |
| `make help` | Show all available commands |

---

## Environment Setup

1. **Backend:** Go 1.21+; copy `configs/config.yaml` and set env vars for API keys
2. **Frontend:** Node.js 18+; `cd web/frontend && npm install`
3. **Database:** SQLite (default, auto-created as `llm-gateway.db`); PostgreSQL supported via `database.driver: "postgres"`
4. **Default admin login:** `username: admin`, `password: admin123` (change via `security.adminPass` in config)

---

## File Reference (Most Commonly Edited)

| File | Purpose |
|---|---|
| `cmd/server/main.go` | App entrypoint, route registration |
| `internal/handler/handler.go` | Admin handlers (config, providers) |
| `internal/handler/auth_handler.go` | JWT login endpoint |
| `internal/middleware/jwt.go` | JWT auth middleware |
| `internal/provider/provider.go` | Provider factory + interface |
| `internal/routing/strategy.go` | Routing engine (load-balance, fallback, conditional) |
| `internal/models/virtual_key.go` | VirtualKey GORM model |
| `web/frontend/lib/api.ts` | Frontend API client (MUST match backend JSON field names) |
| `web/frontend/app/dashboard/page.tsx` | Dashboard page |
| `configs/config.yaml` | Runtime configuration |
| `Makefile` | Build/run/test automation |
