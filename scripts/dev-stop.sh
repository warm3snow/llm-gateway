#!/usr/bin/env bash
# Stop every process belonging to this project's dev stack.
#
# Killing by listening port alone is not enough: `next dev` picks a free port
# (3001, 3002, ...) when 3000 is taken, and it spawns child worker processes;
# `go run` spawns a compiled binary that keeps running after its parent dies.
# This script matches listening ports *and* command lines that point into this
# repository, and never touches processes owned by someone else.
#
# Written for bash 3.2 (the macOS default): no associative arrays.
set -uo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$BASE_DIR/web/frontend"
BACKEND_PORT="${BACKEND_PORT:-18080}"
FRONTEND_PORTS="${FRONTEND_PORTS:-3000 3001 3002 3003 3004 3005}"

SELF="$$"
TARGETS=""

# is_candidate PID -> 0 when the process looks like part of this stack.
is_candidate() {
  local pid="$1"
  local cmd owner
  [[ -z "$pid" || "$pid" == "$SELF" ]] && return 1
  owner="$(ps -o user= -p "$pid" 2>/dev/null | tr -d ' ')"
  [[ "$owner" != "$(whoami)" ]] && return 1
  cmd="$(ps -o command= -p "$pid" 2>/dev/null)"
  [[ -z "$cmd" ]] && return 1
  case "$cmd" in
    *"$FRONTEND_DIR"*|*"$BASE_DIR"*) return 0 ;;
  esac
  # `go run` compiles the server into the Go build cache and execs it from
  # there: .../go-build/aa/<hash>-d/server (or .../exe/server).
  if printf '%s' "$cmd" | grep -Eq '/go-build/.*/(server|llm-gateway)( .*)?$'; then
    return 0
  fi
  # A listener on our port running a dev server is ours too.
  case "$cmd" in
    *next*|*"npm run dev"*|*"cmd/server"*|*llm-gateway*) return 0 ;;
  esac
  return 1
}

add_pid() {
  local pid="$1"
  is_candidate "$pid" || return 0
  case " $TARGETS " in
    *" $pid "*) return 0 ;;
  esac
  TARGETS="$TARGETS $pid"
}

# 1. Listeners on our ports only (not clients that merely connected).
for port in "$BACKEND_PORT" $FRONTEND_PORTS; do
  for pid in $(lsof -ti "tcp:$port" -sTCP:LISTEN 2>/dev/null); do
    add_pid "$pid"
  done
done

# 2. Anything whose command line points into this repository (next workers,
#    npm wrapper, go run, compiled server binary).
for pid in $(ps -eo pid=,command= | grep -F "$BASE_DIR" | grep -v 'dev-stop.sh' | awk '{print $1}'); do
  add_pid "$pid"
done

# 3. The backend binary spawned by `go run` (its path lives in the go build
#    cache, so it is matched by name rather than by repository path).
for pid in $(ps -eo pid=,command= | grep -E '/go-build/.*/(server|llm-gateway)( |$)' | grep -v 'dev-stop.sh' | awk '{print $1}'); do
  add_pid "$pid"
done

# 4. PID files written by `make start`.
for pidfile in /tmp/llm-gateway/backend.pid /tmp/llm-gateway/frontend.pid; do
  [[ -f "$pidfile" ]] && add_pid "$(tr -d '[:space:]' < "$pidfile")"
done

if [[ -z "${TARGETS// /}" ]]; then
  echo "    Nothing running."
  rm -f /tmp/llm-gateway/backend.pid /tmp/llm-gateway/frontend.pid
  exit 0
fi

# shellcheck disable=SC2086
set -- $TARGETS
echo "    Stopping $# process(es):$TARGETS"
kill -TERM "$@" 2>/dev/null || true

# Give them a moment to exit cleanly before escalating.
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do
  ALIVE=""
  for pid in "$@"; do
    kill -0 "$pid" 2>/dev/null && ALIVE="$ALIVE $pid"
  done
  [[ -z "${ALIVE// /}" ]] && break
  sleep 0.1
done

if [[ -n "${ALIVE// /}" ]]; then
  # shellcheck disable=SC2086
  echo "    Force-killing:$ALIVE"
  # shellcheck disable=SC2086
  kill -KILL $ALIVE 2>/dev/null || true
fi

rm -f /tmp/llm-gateway/backend.pid /tmp/llm-gateway/frontend.pid

# Report anything that survived.
remaining=0
for port in "$BACKEND_PORT" $FRONTEND_PORTS; do
  if [[ -n "$(lsof -ti "tcp:$port" -sTCP:LISTEN 2>/dev/null)" ]]; then
    echo "    WARNING: port $port is still in use:" >&2
    lsof -nP -iTCP:"$port" -sTCP:LISTEN >&2 || true
    remaining=1
  fi
done

# Next.js spawns a short-lived telemetry flusher as it shuts down; it is not
# a server, so let it finish instead of reporting it as leftover.
leftover=""
for _ in 1 2 3 4 5 6 7 8 9 10; do
  leftover="$(ps -eo pid=,command= | grep -F "$FRONTEND_DIR" \
    | grep -v 'dev-stop.sh' | grep -v 'detached-flush' | grep -v grep || true)"
  [[ -z "$leftover" ]] && break
  sleep 0.3
done
if [[ -n "$leftover" ]]; then
  echo "    WARNING: frontend processes from this repo are still present:" >&2
  echo "$leftover" >&2
  remaining=1
fi

exit "$remaining"
