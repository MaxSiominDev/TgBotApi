#!/usr/bin/env bash
mkdir -p /opt/telegram-bot-api
cd /opt/telegram-bot-api

echo "$COMPOSE_B64" | base64 -d > docker-compose.yml
echo "$COMPOSE_PROXY_B64" | base64 -d > docker-compose.proxy.yml
echo "$CADDYFILE_B64" | base64 -d > Caddyfile
echo "$NETDATA_CONF_B64" | base64 -d > netdata.conf

if [ -z "$MONITOR_DOMAIN" ]; then
  echo "ERROR: MONITOR_DOMAIN is empty. Set the MONITOR_DOMAIN repo variable to the monitor's domain." >&2
  exit 1
fi

if [ -n "$PROXY_NETWORK" ]; then
  docker network create "$PROXY_NETWORK" 2>/dev/null || true
  SITE_DOMAINS=$(for d in $DOMAINS; do printf 'http://%s ' "$d"; done)
  MONITOR_ADDRESSES="http://$MONITOR_DOMAIN"
else
  SITE_DOMAINS="$DOMAINS"
  MONITOR_ADDRESSES="$MONITOR_DOMAIN"
fi

{
  printf 'IMAGE=%s\n' "$IMAGE"
  printf 'HEALTHCHECK_IMAGE=%s\n' "$HEALTHCHECK_IMAGE"
  printf 'MONITOR_AUTH_IMAGE=%s\n' "$MONITOR_AUTH_IMAGE"
  printf 'DOMAINS=%s\n' "$SITE_DOMAINS"
  printf 'API_TOKEN=%s\n' "$API_TOKEN"
  printf 'TEST_BOT_TOKEN=%s\n' "$TEST_BOT_TOKEN"
  printf 'TEST_CHAT_ID=%s\n' "$TEST_CHAT_ID"
  printf 'MONITOR_ADDRESSES=%s\n' "$MONITOR_ADDRESSES"
} > .env
if [ -n "$PROXY_NETWORK" ]; then
  printf 'COMPOSE_FILE=docker-compose.yml:docker-compose.proxy.yml\n' >> .env
fi
printf 'TELEGRAM_API_ID=%s\nTELEGRAM_API_HASH=%s\n' \
  "$TELEGRAM_API_ID" "$TELEGRAM_API_HASH" > .env.api

{
  printf 'MONITOR_USER=%s\n' "$MONITOR_USER"
  printf 'MONITOR_PASSWORD=%s\n' "$MONITOR_PASSWORD"
} > .env.monitor

echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USER" --password-stdin

docker compose pull
docker compose up -d --remove-orphans

docker compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
