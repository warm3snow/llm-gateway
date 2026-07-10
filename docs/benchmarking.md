# Benchmarking llm-gateway

This runbook measures current gateway bottlenecks under production-like `/v1/chat/completions` load. The default benchmark config uses the in-memory deterministic provider so single-node gateway capacity is not dominated by model latency.

## Prepare

Start local dependencies used by your scenario. Default capacity runs need Postgres; Redis is needed only when response cache is enabled for cache scenarios.

```bash
docker compose up -d postgres redis
```

Seed benchmark data and start the gateway:

```bash
LLM_GATEWAY_CONFIG_PATH=configs/benchmark.yaml go run ./cmd/seed --profile demo --base-url http://localhost:8080 --virtual-keys-per-tenant 1000
LLM_GATEWAY_CONFIG_PATH=configs/benchmark.yaml go run ./cmd/server
```

`configs/benchmark.yaml` defaults to provider `deterministic`, model `deterministic-chat`, and response cache disabled. If you benchmark against real upstream providers, their latency and rate limits can dominate results; use a separate config or override `PROVIDER`/`MODEL` for those runs.

## Single-node capacity targets

Run these against one gateway process with local Postgres:

```bash
make load-test-capacity-500
make load-test-capacity-1000
```

Targets:

- 500 RPS / 500 concurrency / p95 < 100ms
- 1000 RPS / 1000 concurrency / p95 < 200ms

To find the next bottleneck, push gradually toward 5000 RPS:

```bash
make load-test-capacity-push RPS=2000 CONCURRENCY=2000
make load-test-capacity-push RPS=3000 CONCURRENCY=3000
make load-test-capacity-push RPS=5000 CONCURRENCY=5000
```

The push target intentionally has no p95 threshold; use it to collect metrics and pprof data.

## Scenarios

Baseline non-streaming path:

```bash
make load-test SCENARIO=baseline-nonstream PROVIDER=deterministic MODEL=deterministic-chat DURATION=5m WARMUP=30s CONCURRENCY=500 RPS=500 P95_THRESHOLD=100ms
```

Streaming TTFT path:

```bash
make load-test-stream DURATION=5m WARMUP=30s CONCURRENCY=50 RPS=100
```

Large prompt/body-copy pressure:

```bash
make load-test SCENARIO=large-prompt DURATION=5m WARMUP=30s CONCURRENCY=50 RPS=50
```

Cache-repeat path (start the gateway with `LLM_GATEWAY_CACHE_ENABLED=true` or enable cache in config first):

```bash
make load-test SCENARIO=cache-repeat DURATION=5m WARMUP=30s CONCURRENCY=50 RPS=100
```

Virtual-key cardinality pressure:

```bash
make load-test SCENARIO=many-keys DURATION=5m WARMUP=30s CONCURRENCY=50 RPS=100
```

Soak test:

```bash
make load-test-soak DURATION=30m CONCURRENCY=100 RPS=200
```

JSON artifact:

```bash
make load-test-json SCENARIO=baseline-nonstream OUTPUT_FILE=benchmark-result.json
```

## Metrics to inspect

The load generator reports client-side:

- throughput RPS
- success/failure counts and error rate
- status-code distribution
- successful request latency p50/p90/p95/p99/max
- streaming TTFT p50/p90/p95/p99/max
- request/response byte totals

The gateway exposes server-side metrics at `/metrics`, including:

- `llmgw_request_duration_seconds`
- `llmgw_time_to_first_token_seconds`
- `llmgw_virtual_key_auth_duration_seconds`
- `llmgw_virtual_key_active_keys_loaded`
- `llmgw_usage_record_middleware_duration_seconds`
- `llmgw_usage_record_cost_lookup_duration_seconds`
- `llmgw_usage_record_track_usage_duration_seconds`
- `llmgw_usage_record_request_body_bytes`
- `llmgw_usage_record_response_body_bytes`
- `llmgw_logstore_enqueue_duration_seconds`
- `llmgw_usage_records_dropped_total`
- `llmgw_database_pool_max_open_connections`
- `llmgw_database_pool_open_connections`
- `llmgw_database_pool_in_use_connections`
- `llmgw_database_pool_idle_connections`
- `llmgw_database_pool_wait_count_total`
- `llmgw_database_pool_wait_duration_seconds_total`
- `llmgw_database_pool_max_idle_closed_total`
- `llmgw_database_pool_max_lifetime_closed_total`
- `llmgw_budget_tracker_enqueue_total`
- `llmgw_budget_tracker_queue_depth`
- `llmgw_budget_tracker_pending_cost_usd`
- `llmgw_budget_tracker_flush_duration_seconds`
- `llmgw_budget_tracker_flush_updates`

## Profiling

`configs/benchmark.yaml` enables pprof for local/staging benchmark runs. Capture CPU during load:

```bash
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

Capture heap:

```bash
go tool pprof http://localhost:8080/debug/pprof/heap
```

Do not enable pprof on a public production endpoint.

## Interpreting results

- High `virtual_key_auth_duration_seconds` with many loaded keys points to per-request active-key DB scan/hash overhead.
- High `usage_record_cost_lookup_duration_seconds` points to model pricing DB lookup overhead.
- High `usage_record_track_usage_duration_seconds` with result `queued` means async budget enqueue overhead; with `fallback_*` it means the budget queue is full and the request path fell back to synchronous DB updates.
- High `database_pool_in_use_connections` near `database_pool_max_open_connections` with rising `database_pool_wait_count_total` means DB pool saturation.
- Rising `budget_tracker_queue_depth` or `budget_tracker_pending_cost_usd` means async budget flushes cannot keep up with accepted usage.
- Rising `usage_records_dropped_total` during soak means the async usage writer cannot keep up.
- Good TTFT with poor full latency usually means upstream generation dominates after first token.
- Poor TTFT and poor gateway internal timings suggest gateway hot-path overhead before upstream streaming begins.
