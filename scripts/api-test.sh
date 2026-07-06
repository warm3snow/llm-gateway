#!/usr/bin/env bash
# LLM Gateway API Test & Data Generation Script
#
# Purpose:
#   - Exercise representative endpoints (auth, admin, virtual-keys, stats, logs, proxy).
#   - Generate a small, bounded amount of sample data for manual inspection.
#   - Exercise DELETE endpoints while keeping only a few generated rows behind.
#
# Usage:
#   export BASE_URL=http://localhost:8080
#   export ADMIN_USER=admin ADMIN_PASS=admin123
#   export NUM_KEYS=4 NUM_PROVIDERS=2 PROXY_CALLS_PER_KEY=2 DRIVING_KEYS=2
#   ./scripts/api-test.sh

set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-admin123}"

# ─── Data-generation volume knobs ──────────────────────────────────────────
NUM_KEYS="${NUM_KEYS:-4}"                       # virtual keys to create
NUM_PROVIDERS="${NUM_PROVIDERS:-2}"              # provider configs to add
PROXY_CALLS_PER_KEY="${PROXY_CALLS_PER_KEY:-2}"   # proxy requests per key (log rows)
DRIVING_KEYS="${DRIVING_KEYS:-2}"                 # how many keys actually drive proxy traffic

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TOTAL=0
FAILED=0
TOKEN=""

# Arrays holding created resources
declare -a CREATED_VK_IDS=()
declare -a CREATED_VK_KEYS=()
declare -a CREATED_PROVIDERS=()

# ─── Helper functions ───────────────────────────────────────────────────────

pass()  { TOTAL=$((TOTAL+1)); echo -e "  ${GREEN}PASS${NC}: $1"; }
fail()  { TOTAL=$((TOTAL+1)); FAILED=$((FAILED+1)); echo -e "  ${RED}FAIL${NC}: $1 — $2"; }
info()  { echo -e "  ${YELLOW}INFO${NC}: $1"; }
note()  { echo -e "  ${BLUE}····${NC} $1"; }

jval() {
    # jval '<json>' 'key1' ['key2' ...]  → extract nested value with python3.
    # JSON is passed via stdin (NOT string-interpolated) so values containing
    # quotes/apostrophes (e.g. "won't be shown again!") don't break parsing.
    local json="$1"; shift
    printf '%s' "$json" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    for k in sys.argv[1:]:
        d = d[int(k)] if isinstance(d, list) else d[k]
    print(d)
except Exception:
    print('')
" "$@" 2>/dev/null
}

assert_status() {
    local expected="$1" actual="$2" name="$3"
    if [ "$actual" = "$expected" ]; then
        pass "$name (status=$actual)"
    else
        fail "$name" "expected=$expected, got=$actual"
    fi
}

assert_optional_200() {
    local actual="$1" name="$2" detail="${3:-}"
    if [ "$actual" = "200" ]; then
        pass "$name (status=$actual)"
    elif [ "${PROXY_STRICT_OPTIONAL:-false}" = "true" ]; then
        fail "$name" "expected=200, got=$actual${detail:+; $detail}"
    else
        info "$name returned status=$actual; skipped as optional${detail:+ ($detail)}"
    fi
}

# call_status method url [body] [token] [api_key]
call_status() {
    local method="$1" url="$2" body="${3:-}" token="${4:-}" api_key="${5:-}"
    local full_url="${BASE_URL}${url}"
    local curl_cmd="curl -s -o /dev/null -w %{http_code} -X ${method}"
    curl_cmd+=" -H 'Content-Type: application/json'"
    [ -n "$token" ]   && curl_cmd+=" -H 'Authorization: Bearer ${token}'"
    if [ -n "$api_key" ]; then
        curl_cmd+=" -H 'x-llm-gateway-api-key: ${api_key}'"
        curl_cmd+=" -H 'x-llm-provider: ${PROXY_PROVIDER:-ollama}'"
    fi
    [ -n "$body" ]    && curl_cmd+=" -d '${body}'"
    curl_cmd+=" '${full_url}'"
    eval "$curl_cmd" 2>/dev/null
}

# call_body method url [body] [token] [api_key]  → full response body
call_body() {
    local method="$1" url="$2" body="${3:-}" token="${4:-}" api_key="${5:-}"
    local full_url="${BASE_URL}${url}"
    local curl_cmd="curl -s -X ${method}"
    curl_cmd+=" -H 'Content-Type: application/json'"
    [ -n "$token" ]   && curl_cmd+=" -H 'Authorization: Bearer ${token}'"
    if [ -n "$api_key" ]; then
        curl_cmd+=" -H 'x-llm-gateway-api-key: ${api_key}'"
        curl_cmd+=" -H 'x-llm-provider: ${PROXY_PROVIDER:-ollama}'"
    fi
    [ -n "$body" ]    && curl_cmd+=" -d '${body}'"
    curl_cmd+=" '${full_url}'"
    eval "$curl_cmd" 2>/dev/null
}

chat_model_candidates_from_models_response() {
    printf '%s' "$1" | python3 -c '
import json, re, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
items = d.get("data") or d.get("models") or []
if isinstance(items, dict):
    items = list(items.values())
names = []
for item in items:
    if isinstance(item, str):
        names.append(item)
    elif isinstance(item, dict):
        model = item.get("id") or item.get("name") or item.get("model")
        if model:
            names.append(model)
seen = set()
chat_names = []
for name in names:
    if name in seen or re.search(r"(embed|bge|nomic|mxbai|e5|minilm)", name, re.I):
        continue
    seen.add(name)
    chat_names.append(name)
# Prefer local models. Ollama can list :cloud models that return 500 when cloud
# auth/quota is unavailable, so probe them only after local candidates.
chat_names.sort(key=lambda name: (name.endswith(":cloud"), name))
for name in chat_names:
    print(name)
' 2>/dev/null
}

embedding_model_from_models_response() {
    printf '%s' "$1" | python3 -c '
import json, re, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
items = d.get("data") or d.get("models") or []
if isinstance(items, dict):
    items = list(items.values())
names = []
for item in items:
    if isinstance(item, str):
        names.append(item)
    elif isinstance(item, dict):
        model = item.get("id") or item.get("name") or item.get("model")
        if model:
            names.append(model)
for name in names:
    if re.search(r"(embed|bge|nomic|mxbai|e5|minilm)", name, re.I):
        print(name); break
' 2>/dev/null
}

RUN_ID="$(date +%s)"
# Provider used for proxy traffic — must be a real, reachable backend so calls
# return 200. Defaults to the ollama provider configured in configs/config.yaml.
PROXY_PROVIDER="${PROXY_PROVIDER:-ollama}"
# Real models served by the PROXY_PROVIDER backend. If unset, the script
# detects models from GET /v1/models after creating a virtual key.
CHAT_MODEL="${CHAT_MODEL:-}"
EMBED_MODEL="${EMBED_MODEL:-}"
PROMPTS=("Hello" "Explain quantum computing in one line" "Write a haiku" "Summarize: the sky is blue" "What is 2+2?" "Translate hi to French")
PROVIDER_TYPES=("openai" "anthropic" "gemini" "ollama" "openai")

# ─── 1. Login ────────────────────────────────────────────────────────────────
echo "=== 1. Login ==="

LOGIN_RESP=$(call_body POST /api/v1/auth/login "{\"username\":\"${ADMIN_USER}\",\"password\":\"${ADMIN_PASS}\"}")
TOKEN=$(jval "$LOGIN_RESP" token)
if [ -z "$TOKEN" ]; then
    echo -e "  ${RED}Cannot get JWT token, aborting${NC}  (response: $LOGIN_RESP)"
    exit 1
fi
pass "POST /api/v1/auth/login (got token)"

echo ""
echo "=== 2. Wrong password (expect 401) ==="
CODE=$(call_status POST /api/v1/auth/login '{"username":"admin","password":"wrong"}')
assert_status "401" "$CODE" "POST /api/v1/auth/login (wrong password)"

echo ""
echo "=== 3. Health check ==="
assert_status "200" "$(call_status GET /health)" "GET /health"

# ─── 4. Read-only admin / stats / logs endpoints ────────────────────────────
echo ""
echo "=== 4. Admin / Stats / Logs (JWT) ==="
assert_status "200" "$(call_status GET /api/v1/admin/config    "" "$TOKEN")" "GET /api/v1/admin/config"
assert_status "200" "$(call_status GET /api/v1/admin/providers "" "$TOKEN")" "GET /api/v1/admin/providers"
assert_status "200" "$(call_status GET /api/v1/admin/stats     "" "$TOKEN")" "GET /api/v1/admin/stats"
assert_status "200" "$(call_status GET /api/v1/admin/health    "" "$TOKEN")" "GET /api/v1/admin/health"
assert_status "200" "$(call_status GET /api/v1/stats/overview  "" "$TOKEN")" "GET /api/v1/stats/overview"
assert_status "200" "$(call_status GET /api/v1/stats/analytics "" "$TOKEN")" "GET /api/v1/stats/analytics"
assert_status "200" "$(call_status GET '/api/v1/stats/hourly?hours=24'  "" "$TOKEN")" "GET /api/v1/stats/hourly"
assert_status "200" "$(call_status GET '/api/v1/usage?limit=5' "" "$TOKEN")" "GET /api/v1/usage"
assert_status "200" "$(call_status GET /api/v1/virtual-keys   "" "$TOKEN")" "GET /api/v1/virtual-keys"

# ─── 5. Generate provider configs ────────────────────────────────────────────
echo ""
echo "=== 5. Generate ${NUM_PROVIDERS} providers ==="
for i in $(seq 1 "$NUM_PROVIDERS"); do
    pname="test-provider-${RUN_ID}-${i}"
    ptype="${PROVIDER_TYPES[$(( (i-1) % ${#PROVIDER_TYPES[@]} ))]}"
    body="{\"name\":\"${pname}\",\"provider\":\"${ptype}\",\"apiKey\":\"sk-test-${RUN_ID}-${i}\",\"weight\":$((i%5+1)),\"requestTimeout\":30}"
    CODE=$(call_status POST /api/v1/admin/providers "$body" "$TOKEN")
    if [ "$CODE" = "200" ] || [ "$CODE" = "201" ]; then
        pass "POST /api/v1/admin/providers ($pname / $ptype)"
        CREATED_PROVIDERS+=("$pname")
    else
        fail "POST /api/v1/admin/providers ($pname)" "status=$CODE"
    fi
done
note "Created ${#CREATED_PROVIDERS[@]} providers"

# ─── 6. Generate virtual keys ────────────────────────────────────────────────
echo ""
echo "=== 6. Generate ${NUM_KEYS} virtual keys ==="
for i in $(seq 1 "$NUM_KEYS"); do
    vk_name="load-key-${RUN_ID}-${i}"
    budget=$(( (RANDOM % 900) + 100 ))
    rate=$(( (RANDOM % 500) + 50 ))
    # The first DRIVING_KEYS keys drive real proxy traffic, so they MUST permit
    # the proxy provider. Give the rest a varied provider allow-list for variety.
    if [ "$i" -le "${DRIVING_KEYS:-10}" ]; then
        provs="[\"${PROXY_PROVIDER}\"]"
    else
        case $((i % 3)) in
            0) provs='["openai","anthropic"]' ;;
            1) provs='["ollama"]' ;;
            *) provs='[]' ;;
        esac
    fi
    body="{\"name\":\"${vk_name}\",\"budget_total\":${budget},\"rate_limit\":${rate},\"rate_limit_window\":60,\"providers\":${provs},\"metadata\":\"{\\\"batch\\\":\\\"${RUN_ID}\\\"}\"}"
    RESP=$(call_body POST /api/v1/virtual-keys "$body" "$TOKEN")
    STATUS=$(jval "$RESP" status)
    if [ "$STATUS" = "success" ]; then
        vk_id=$(jval "$RESP" key id)
        vk_key=$(jval "$RESP" key key)
        CREATED_VK_IDS+=("$vk_id")
        CREATED_VK_KEYS+=("$vk_key")
        [ $((i % 5)) -eq 0 ] && note "created ${i}/${NUM_KEYS} keys..."
    else
        fail "POST /api/v1/virtual-keys ($vk_name)" "resp: $RESP"
    fi
done
pass "Created ${#CREATED_VK_IDS[@]}/${NUM_KEYS} virtual keys"

if [ "${#CREATED_VK_IDS[@]}" -eq 0 ]; then
    echo -e "  ${RED}No virtual keys created; skipping key-dependent tests${NC}"
fi

# ─── 7. Per-key operations (detail / update / reset) ─────────────────────────
if [ "${#CREATED_VK_IDS[@]}" -gt 0 ]; then
    echo ""
    echo "=== 7. Virtual key operations (sample of first 3) ==="
    sample=$(( ${#CREATED_VK_IDS[@]} < 3 ? ${#CREATED_VK_IDS[@]} : 3 ))
    for idx in $(seq 0 $((sample-1))); do
        vid="${CREATED_VK_IDS[$idx]}"
        assert_status "200" "$(call_status GET "/api/v1/virtual-keys/${vid}" "" "$TOKEN")" "GET virtual-keys/${vid}"
        assert_status "200" "$(call_status PUT "/api/v1/virtual-keys/${vid}" '{"name":"load-key-updated","rate_limit":999}' "$TOKEN")" "PUT virtual-keys/${vid}"
        assert_status "200" "$(call_status POST "/api/v1/virtual-keys/${vid}/reset" "" "$TOKEN")" "POST virtual-keys/${vid}/reset"
    done
fi

# ─── 8. Proxy APIs (VirtualKey Auth) — must return 200 via real backend ──────
echo ""
echo "=== 8. Proxy APIs (VirtualKey Auth via '${PROXY_PROVIDER}') ==="
if [ "${#CREATED_VK_KEYS[@]}" -gt 0 ]; then
    PK="${CREATED_VK_KEYS[0]}"   # first key permits the proxy provider

    # NOTE: capture status into a var FIRST, then assert. Do NOT inline
    # $(call_status ...) inside assert_status "..." — nesting a command
    # substitution inside a double-quoted arg mangles the JSON body's
    # backslash-escapes and the server receives a double-encoded string.
    echo "  8.1 GET /v1/models"
    MODELS_RESP=$(call_body GET /v1/models "" "" "$PK")
    C=$(call_status GET /v1/models "" "" "$PK")
    assert_status "200" "$C" "GET /v1/models"

    if [ -z "$CHAT_MODEL" ]; then
        while IFS= read -r candidate; do
            [ -z "$candidate" ] && continue
            probe_status=$(call_status POST /v1/chat/completions \
                "{\"model\":\"${candidate}\",\"messages\":[{\"role\":\"user\",\"content\":\"ping\"}],\"max_tokens\":1}" \
                "" "$PK")
            if [ "$probe_status" = "200" ]; then
                CHAT_MODEL="$candidate"
                break
            fi
            note "chat model candidate ${candidate} returned status=${probe_status}; trying next"
        done < <(chat_model_candidates_from_models_response "$MODELS_RESP")
    fi
    if [ -z "$EMBED_MODEL" ]; then
        EMBED_MODEL=$(embedding_model_from_models_response "$MODELS_RESP")
    fi
    if [ -z "$CHAT_MODEL" ]; then
        info "No working chat model returned 200; skipping chat proxy checks (set CHAT_MODEL to force a model)"
    else
        note "Using CHAT_MODEL=${CHAT_MODEL}"
    fi
    if [ -n "$EMBED_MODEL" ]; then
        note "Using EMBED_MODEL=${EMBED_MODEL}"
    else
        info "No embedding-looking model found; set EMBED_MODEL to test /v1/embeddings"
    fi

    if [ -n "$CHAT_MODEL" ]; then
        echo "  8.2 POST /v1/chat/completions"
        C=$(call_status POST /v1/chat/completions \
            "{\"model\":\"${CHAT_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hi in one word\"}],\"max_tokens\":10}" \
            "" "$PK")
        assert_status "200" "$C" "POST /v1/chat/completions"

        echo "  8.3 POST /v1/chat/completions/stream"
        C=$(call_status POST /v1/chat/completions/stream \
            "{\"model\":\"${CHAT_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"Hi\"}],\"max_tokens\":5,\"stream\":true}" \
            "" "$PK")
        assert_status "200" "$C" "POST /v1/chat/completions/stream"

        echo "  8.4 POST /v1/completions"
        C=$(call_status POST /v1/completions \
            "{\"model\":\"${CHAT_MODEL}\",\"prompt\":\"Hello\",\"max_tokens\":5}" \
            "" "$PK")
        assert_optional_200 "$C" "POST /v1/completions" "some providers/models expose chat but not legacy completions"
    fi

    echo "  8.5 POST /v1/embeddings"
    if [ -n "$EMBED_MODEL" ]; then
        C=$(call_status POST /v1/embeddings \
            "{\"model\":\"${EMBED_MODEL}\",\"input\":\"hello world\"}" \
            "" "$PK")
        assert_optional_200 "$C" "POST /v1/embeddings" "embedding support depends on installed model"
    else
        info "POST /v1/embeddings skipped (no EMBED_MODEL)"
    fi
else
    info "No virtual key API keys available; skipping proxy API tests"
fi

# ─── 8b. Generate proxy traffic → request logs ───────────────────────────────
echo ""
echo "=== 8b. Generate proxy traffic (${PROXY_CALLS_PER_KEY} calls/key → request logs) ==="
if [ "${#CREATED_VK_KEYS[@]}" -gt 0 ] && [ -n "$CHAT_MODEL" ]; then
    total_calls=0
    ok_calls=0
    chat_calls=0
    chat_ok_calls=0
    driving=$(( ${#CREATED_VK_KEYS[@]} < DRIVING_KEYS ? ${#CREATED_VK_KEYS[@]} : DRIVING_KEYS ))
    for idx in $(seq 0 $((driving-1))); do
        vkey="${CREATED_VK_KEYS[$idx]}"
        for c in $(seq 1 "$PROXY_CALLS_PER_KEY"); do
            prompt="${PROMPTS[$((RANDOM % ${#PROMPTS[@]}))]}"
            if [ $((c % 2)) -eq 1 ]; then
                code=$(call_status POST /v1/chat/completions \
                     "{\"model\":\"${CHAT_MODEL}\",\"messages\":[{\"role\":\"user\",\"content\":\"${prompt}\"}],\"max_tokens\":20}" \
                     "" "$vkey")
                chat_calls=$((chat_calls+1))
                [ "$code" = "200" ] && chat_ok_calls=$((chat_ok_calls+1))
            else
                code=$(call_status GET /v1/models "" "" "$vkey")
            fi
            total_calls=$((total_calls+1))
            [ "$code" = "200" ] && ok_calls=$((ok_calls+1))
        done
        note "driven traffic through key #$((idx+1))/${driving} (running 200s: ${ok_calls}/${total_calls}, chat 200s: ${chat_ok_calls}/${chat_calls})"
    done
    if [ "$ok_calls" -eq "$total_calls" ]; then
        pass "Generated ${total_calls} proxy requests, all 200"
    else
        fail "Proxy traffic 200 rate" "${ok_calls}/${total_calls} returned 200 (CHAT_MODEL=${CHAT_MODEL})"
    fi

    # Verify usage records grew. The writer is async, so give it a few short chances.
    USAGE_TOTAL=""
    for attempt in 1 2 3; do
        USAGE_RESP=$(call_body GET '/api/v1/usage?limit=1' "" "$TOKEN")
        USAGE_TOTAL=$(jval "$USAGE_RESP" total)
        [ -n "$USAGE_TOTAL" ] && [ "$USAGE_TOTAL" != "0" ] && break
        [ "$attempt" != "3" ] && sleep 1
    done
    [ -n "$USAGE_TOTAL" ] && note "usage_records total now: ${USAGE_TOTAL}"
else
    info "No virtual key API keys or chat model available; skipping proxy traffic"
fi

# ─── 9. DELETE endpoint tests (subset only — bulk data kept) ─────────────────
echo ""
echo "=== 9. DELETE endpoints (subset — remaining data intentionally kept) ==="

# 9.1 Delete the LAST virtual key only (keep the rest as generated data)
if [ "${#CREATED_VK_IDS[@]}" -gt 0 ]; then
    last_idx=$(( ${#CREATED_VK_IDS[@]} - 1 ))
    del_vid="${CREATED_VK_IDS[$last_idx]}"
    assert_status "200" "$(call_status DELETE "/api/v1/virtual-keys/${del_vid}" "" "$TOKEN")" "DELETE virtual-keys/${del_vid}"
    # Confirm it's gone (soft-deleted → 404 on GET)
    AFTER=$(call_status GET "/api/v1/virtual-keys/${del_vid}" "" "$TOKEN")
    if [ "$AFTER" = "404" ]; then
        pass "GET deleted virtual-keys/${del_vid} → 404 (confirmed removed)"
    else
        info "GET deleted virtual-keys/${del_vid} → status=$AFTER (soft-delete may return differently)"
    fi
    note "Kept $((${#CREATED_VK_IDS[@]} - 1)) virtual keys as generated data"
fi

# 9.2 Delete the LAST provider only
if [ "${#CREATED_PROVIDERS[@]}" -gt 0 ]; then
    last_p_idx=$(( ${#CREATED_PROVIDERS[@]} - 1 ))
    del_p="${CREATED_PROVIDERS[$last_p_idx]}"
    assert_status "200" "$(call_status DELETE "/api/v1/admin/providers/${del_p}" "" "$TOKEN")" "DELETE admin/providers/${del_p}"
    note "Kept $((${#CREATED_PROVIDERS[@]} - 1)) providers as generated data"
fi

# ─── 10. Auth failure tests ──────────────────────────────────────────────────
echo ""
echo "=== 10. Auth failure tests ==="
assert_status "401" "$(call_status GET /api/v1/admin/config)" "GET admin/config (no token)"
assert_status "401" "$(call_status GET /api/v1/admin/config "" "invalid.token.here")" "GET admin/config (invalid token)"
if [ "${#CREATED_VK_KEYS[@]}" -gt 0 ]; then
    assert_status "401" "$(call_status GET /v1/models "" "" "invalid-key")" "GET /v1/models (invalid api key)"
fi

# ─── Summary ─────────────────────────────────────────────────────────────────
echo ""
echo "========================================="
echo -e "  Data generated (kept, NOT cleaned up):"
echo -e "    virtual keys : ${GREEN}$(( ${#CREATED_VK_IDS[@]} > 0 ? ${#CREATED_VK_IDS[@]} - 1 : 0 ))${NC} (1 deleted for DELETE test)"
echo -e "    providers    : ${GREEN}$(( ${#CREATED_PROVIDERS[@]} > 0 ? ${#CREATED_PROVIDERS[@]} - 1 : 0 ))${NC} (1 deleted for DELETE test)"
echo "-----------------------------------------"
if [ "$FAILED" -eq 0 ]; then
    echo -e "  All checks passed: ${GREEN}${TOTAL}/${TOTAL}${NC}"
else
    echo -e "  Result: ${GREEN}$((TOTAL-FAILED))${NC} passed, ${RED}${FAILED}${NC} failed, total ${TOTAL}"
fi
echo "========================================="
exit $FAILED
