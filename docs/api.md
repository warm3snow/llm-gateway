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

## OpenAI-compatible API

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/chat/completions` | Chat completions |
| `POST` | `/v1/chat/completions/stream` | Streaming chat completions |
| `POST` | `/v1/completions` | Text completions |
| `POST` | `/v1/embeddings` | Embeddings |
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/images/generations` | Image generation |
| `POST` | `/v1/audio/speech` | Text to speech |
| `POST` | `/v1/audio/transcriptions` | Audio transcription |
| `POST` | `/v1/audio/translations` | Audio translation |
| `ANY` | `/v1/proxy/*path` | Generic upstream proxy |

### Example: Chat Completions

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-llm-gateway-api-key: your-virtual-key" \
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

## Operations API

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Service health check |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/swagger/*` | Swagger documentation |
| `GET` | `/debug/pprof/*` | Go pprof endpoints, available when enabled |
