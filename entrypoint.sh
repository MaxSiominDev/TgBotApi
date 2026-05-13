#!/bin/sh
set -e
exec /usr/local/bin/telegram-bot-api \
    --api-id="${TELEGRAM_API_ID}" \
    --api-hash="${TELEGRAM_API_HASH}" \
    --local \
    --http-port=8081 \
    --dir=/data \
    "$@"
