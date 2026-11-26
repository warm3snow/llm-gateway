# API Reference

LLM Gateway exposes two main API groups:

- OpenAI-compatible API: the application-facing request entry point, authenticated with virtual keys
- Admin API: the dashboard and automation management entry point, authenticated with JWT

Default base URL: `http://localhost:8080`.

## Authentication

### OpenAI-compatible API

Use a virtual key for `/v1/*` requests:

```http
x-llm-gateway-api-key: <virtual-key>
```

### Admin API

Use a JWT for `/api/v1/*` management requests:

```http
Authorization: Bearer <token>
```

The login endpoint returns a JWT. Multi-tenant users may need to select a tenant before receiving the final access token.

Accounts are identified by email. Sign-in accepts the email only (a bare username is rejected), and every account email must be globally unique across tenants.

Emails follow `local@domain` and are shown in the console as e.g. `user123456@llmgw.org`. Users created without an explicit address get `name@<tenant-slug>.llmgw.org`; `super_admin` accounts are platform-level, have no tenant binding and use `admin@llmgw.org`. Creating a member with an email that is already taken returns `400` with `email "…" is already taken`.

## OpenAI-compatible API

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/chat/completions` | Chat completions (buffered, or SSE when `"stream": true`) |
| `POST` | `/v1/completions` | Text completions |
| `POST` | `/v1/embeddings` | Embeddings |
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/images/generations` | Image generation |
| `POST` | `/v1/audio/speech` | Text to speech |
| `POST` | `/v1/audio/transcriptions` | Audio transcription |
| `POST` | `/v1/audio/translations` | Audio translation |
| `POST` | `/v1/messages` | Anthropic Messages API (translated to the canonical chat protocol; streaming included) |
| `POST` | `/v1/messages/count_tokens` | Anthropic-compatible token count estimate |
| `ANY` | `/v1/proxy/*path` | Generic upstream proxy |

### Authentication headers

The virtual key can be supplied with any of:

- `x-llm-gateway-api-key: <virtual-key>` (configurable via `security.apiKeyHeader`)
- `Authorization: Bearer <virtual-key>` (OpenAI/Anthropic SDK compatible)
- `x-api-key: <virtual-key>` (Anthropic client compatible)

`/v1/chat/completions` serves both buffered and streamed requests: send `"stream": true` and the gateway replies with OpenAI-format SSE, regardless of the upstream provider's native protocol (Anthropic and Gemini streams are translated). Anthropic clients should call `/v1/messages` instead and receive Anthropic-format SSE.

### Fallback routing

A provider entry may name `fallbacks`: other provider entries tried in order when the primary fails with a retryable status. Each attempt is retried per the entry's `retry` settings, and the usage record's `tries` field reports how many upstream attempts the call used.

### Privacy masking

When `gateway.redact.enabled` is on, sensitive values in `/v1/chat/completions`, `/v1/completions` and `/v1/messages` request bodies are replaced with stable placeholders (`{{API_KEY_k3v9x2mq}}`) before the request leaves the gateway, and put back in the response (buffered JSON and SSE deltas alike). The same value always yields the same placeholder, so conversations, prompt caches and thinking signatures stay valid. Per-kind actions (`mask`/`warn`/`block`), custom words, custom prefix/regex rules and an allow-list are configurable; the number of masked values is recorded on the usage record.

### Example: Chat Completions

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-virtual-key" \
  -d '{
    "model": "llama3",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Admin API

| Module | Endpoint | Description |
|---|---|---|
| Auth | `POST /api/v1/auth/login` | Log in and get a JWT |
| Auth | `POST /api/v1/auth/select-tenant` | Select a tenant after multi-tenant login |
| Config | `GET /api/v1/admin/config` | Get gateway configuration |
| Config | `POST /api/v1/admin/config` | Update gateway configuration |
| Providers | `GET /api/v1/admin/providers` | List providers |
| Providers | `POST /api/v1/admin/providers` | Create a provider |
| Providers | `PUT /api/v1/admin/providers/:name` | Update a provider |
| Providers | `DELETE /api/v1/admin/providers/:name` | Delete a provider |
| Health | `GET /api/v1/admin/health` | Get gateway health status |
| Health | `GET /api/v1/admin/providers/health` | Get provider health status |
| Stats | `GET /api/v1/stats/overview` | Get usage overview |
| Stats | `GET /api/v1/stats/analytics` | Get analytics data |
| Stats | `GET /api/v1/stats/hourly` | Get hourly stats |
| Stats | `GET /api/v1/stats/summary?period=today\|7d\|30d\|all` | Period-window billing summary |
| Pricing | `GET /api/v1/admin/pricing` | List model prices |
| Pricing | `POST /api/v1/admin/pricing` | Upsert a model price |
| Pricing | `PUT /api/v1/admin/pricing/:id` | Update a model price |
| Pricing | `DELETE /api/v1/admin/pricing/:id` | Delete a model price |
| Pricing | `POST /api/v1/admin/pricing/sync` | Sync prices from Portkey |
| Redact | `GET /api/v1/admin/redact` | Get masking configuration |
| Redact | `PUT /api/v1/admin/redact` | Update masking configuration (live + persisted) |
| Redact | `POST /api/v1/admin/redact/preview` | Preview masking on sample text |
| Usage | `GET /api/v1/usage` | List usage records |
| Usage | `GET /api/v1/usage/:id` | Get one usage record |
| Virtual Keys | `POST /api/v1/virtual-keys` | Create a virtual key |
| Virtual Keys | `GET /api/v1/virtual-keys` | List virtual keys |
| Virtual Keys | `GET /api/v1/virtual-keys/:id` | Get virtual key details |
| Virtual Keys | `PUT /api/v1/virtual-keys/:id` | Update a virtual key |
| Virtual Keys | `DELETE /api/v1/virtual-keys/:id` | Delete a virtual key |
| Virtual Keys | `POST /api/v1/virtual-keys/:id/reset` | Reset a virtual key secret |
| Alerts | `GET /api/v1/alerts/rules` | List alert rules |
| Alerts | `POST /api/v1/alerts/rules` | Create an alert rule |
| Alerts | `GET /api/v1/alerts/events` | List alert events |
| Tenants | `GET /api/v1/tenants` | List tenants |
| Tenants | `POST /api/v1/tenants` | Create a tenant |
| Tenants | `PUT /api/v1/tenants/:id/status` | Update tenant status |
| Users | `GET /api/v1/users` | List users |
| Users | `POST /api/v1/users` | Create a user |
| Users | `PUT /api/v1/users/me/password` | Change the current user's password |
| Users | `PUT /api/v1/users/:id/status` | Update user status |
| Users | `PUT /api/v1/users/:id/role` | Change a tenant member's role (super_admin only) |

## Operations API

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Service health check |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/swagger/*` | Swagger documentation |
| `GET` | `/debug/pprof/*` | Go pprof endpoints, available when enabled |
