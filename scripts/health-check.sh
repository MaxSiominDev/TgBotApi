#!/usr/bin/env bash
set -e
MAX_ATTEMPTS=120

probe() {
  local url=$1 attempts=$2 auth=$3
  local i=1 code ok
  local hdr=()
  if [ "$auth" = "auth" ]; then hdr=(-H "X-Api-Token: $API_TOKEN"); fi
  while :; do
    code=$(curl -sS "${hdr[@]}" -o /tmp/body -w '%{http_code}' "$url" || echo "000")
    ok=$(jq -r '.ok // false' /tmp/body 2>/dev/null || echo "false")
    if [ "$code" = "200" ] && [ "$ok" = "true" ]; then
      echo "  OK (attempt $i): $(cat /tmp/body)"
      return 0
    fi
    if [ "$i" -ge "$attempts" ]; then
      echo "  FAILED after $i attempts: code=$code"
      cat /tmp/body || true
      return 1
    fi
    i=$((i+1))
    sleep 5
  done
}

for d in $DOMAINS; do
  echo "[$d] containers"
  probe "https://$d/health/containers" "$MAX_ATTEMPTS" noauth \
    || { echo "::error::healthcheck failed: domain=$d check=containers"; exit 1; }

  echo "[$d] telegram"
  probe "https://$d/health/telegram" 1 auth \
    || { echo "::error::healthcheck failed: domain=$d check=telegram"; exit 1; }

  echo "[$d] send-message"
  probe "https://$d/health/send-message" 1 auth \
    || { echo "::error::healthcheck failed: domain=$d check=send-message"; exit 1; }
done
