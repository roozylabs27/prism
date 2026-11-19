#!/usr/bin/env bash
set -euo pipefail

# Timed Client #N+1 Workspace, Project, Quota & Pinned Gateway Key Onboarding
# Requires: PRISM_API_URL, PRISM_SESSION_COOKIE, PRISM_ORG_ID, CLIENT_NAME
# Optional: PROJECT_NAME, DAILY_SPEND_LIMIT_USD, MONTHLY_SPEND_LIMIT_USD, DAILY_REQUEST_LIMIT

require_env() {
  local var_name="$1"
  if [[ -z "${!var_name:-}" ]]; then
    echo "ERROR: Required environment variable $var_name is missing." >&2
    exit 1
  fi
}

require_env "PRISM_API_URL"
require_env "PRISM_SESSION_COOKIE"
require_env "PRISM_ORG_ID"
require_env "CLIENT_NAME"

PROJECT_NAME="${PROJECT_NAME:-Default Workflow}"
DAILY_SPEND_LIMIT_USD="${DAILY_SPEND_LIMIT_USD:-10.0}"
MONTHLY_SPEND_LIMIT_USD="${MONTHLY_SPEND_LIMIT_USD:-200.0}"
DAILY_REQUEST_LIMIT="${DAILY_REQUEST_LIMIT:-2000}"
KEY_RATE_LIMIT="${KEY_RATE_LIMIT:-120}"

API_BASE="${PRISM_API_URL%/}/api"
START_TS="$(date +%s)"

json_get() {
  local field="$1"
  local raw="$2"
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$raw" | jq -er --arg f "$field" '.[$f]'
  elif command -v python3 >/dev/null 2>&1; then
    printf '%s' "$raw" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d[sys.argv[1]])' "$field"
  else
    echo "ERROR: Either jq or python3 is required to parse API responses." >&2
    exit 1
  fi
}

json_build_name() {
  local name="$1"
  if command -v jq >/dev/null 2>&1; then
    jq -cn --arg n "$name" '{name: $n}'
  elif command -v python3 >/dev/null 2>&1; then
    python3 -c 'import json,sys; print(json.dumps({"name": sys.argv[1]}))' "$name"
  else
    printf '{"name":"%s"}' "$(printf '%s' "$name" | sed 's/"/\\"/g')"
  fi
}

api_call() {
  local method="$1"
  local path="$2"
  local body="$3"
  local resp http_code payload
  resp="$(curl -sS -w $'\n%{http_code}' -X "$method" "${API_BASE}${path}" \
    -H "Content-Type: application/json" \
    -H "Cookie: ${PRISM_SESSION_COOKIE}" \
    -H "X-Prism-Org-ID: ${PRISM_ORG_ID}" \
    -d "$body")"
  http_code="${resp##*$'\n'}"
  payload="${resp%$'\n'*}"
  if [[ "$http_code" -lt 200 || "$http_code" -ge 300 ]]; then
    echo "ERROR: $method $path failed with HTTP $http_code: $payload" >&2
    exit 1
  fi
  printf '%s' "$payload"
}

echo "[1/4] Creating client workspace '${CLIENT_NAME}' in org '${PRISM_ORG_ID}'..."
WS_RESP="$(api_call "POST" "/workspaces" "$(json_build_name "$CLIENT_NAME")")"
WS_ID="$(json_get "id" "$WS_RESP")"

echo "[2/4] Creating project '${PROJECT_NAME}' under workspace '${WS_ID}'..."
PROJ_RESP="$(api_call "POST" "/workspaces/${WS_ID}/projects" "$(json_build_name "$PROJECT_NAME")")"
PROJ_ID="$(json_get "id" "$PROJ_RESP")"

echo "[3/4] Setting hard workspace quotas (daily=\$${DAILY_SPEND_LIMIT_USD}, monthly=\$${MONTHLY_SPEND_LIMIT_USD}, reqs/day=${DAILY_REQUEST_LIMIT})..."
QUOTA_PAYLOAD="$(printf '{"monthlySpendLimitUsd":%s,"dailySpendLimitUsd":%s,"dailyRequestLimit":%s}' \
  "$MONTHLY_SPEND_LIMIT_USD" "$DAILY_SPEND_LIMIT_USD" "$DAILY_REQUEST_LIMIT")"
api_call "PUT" "/quotas/workspace/${WS_ID}" "$QUOTA_PAYLOAD" >/dev/null

echo "[4/4] Issuing workspace+project-pinned gateway API key..."
KEY_NAME="${CLIENT_NAME} - ${PROJECT_NAME}"
if command -v jq >/dev/null 2>&1; then
  KEY_PAYLOAD="$(jq -cn --arg n "$KEY_NAME" --arg ws "$WS_ID" --arg pr "$PROJ_ID" --argjson rl "$KEY_RATE_LIMIT" \
    '{name: $n, workspaceId: $ws, projectId: $pr, rateLimit: $rl}')"
elif command -v python3 >/dev/null 2>&1; then
  KEY_PAYLOAD="$(python3 -c 'import json,sys; print(json.dumps({"name": sys.argv[1], "workspaceId": sys.argv[2], "projectId": sys.argv[3], "rateLimit": int(sys.argv[4])}))' \
    "$KEY_NAME" "$WS_ID" "$PROJ_ID" "$KEY_RATE_LIMIT")"
else
  KEY_PAYLOAD="$(printf '{"name":"%s","workspaceId":"%s","projectId":"%s","rateLimit":%s}' \
    "$(printf '%s' "$KEY_NAME" | sed 's/"/\\"/g')" "$WS_ID" "$PROJ_ID" "$KEY_RATE_LIMIT")"
fi
KEY_RESP="$(api_call "POST" "/gateway-keys" "$KEY_PAYLOAD")"
RAW_KEY="$(json_get "key" "$KEY_RESP")"
KEY_PREFIX="$(json_get "keyPrefix" "$KEY_RESP")"

END_TS="$(date +%s)"
ELAPSED=$((END_TS - START_TS))

cat <<EOF

====================================================================
Client #N+1 Staging Onboarding Complete (${ELAPSED}s)
--------------------------------------------------------------------
Organization ID : ${PRISM_ORG_ID}
Workspace ID    : ${WS_ID}  (${CLIENT_NAME})
Project ID      : ${PROJ_ID}  (${PROJECT_NAME})
Gateway Key     : ${KEY_PREFIX}  (shown ONCE below; store securely!)
Base URL (AI)   : ${PRISM_API_URL%/}/v1
Endpoint (MCP)  : ${PRISM_API_URL%/}/v1/mcp
====================================================================
ONE-TIME GATEWAY SECRET (do not commit or re-share in plain logs):
${RAW_KEY}
====================================================================
EOF
