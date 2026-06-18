#!/usr/bin/env bash
set -e

resolve() {
  nslookup "$1" 2>/dev/null | awk '/^Name:/{f=1;next} f&&/^Address:/{print $2}'
}

if printf '%s' "$VPS_HOST" | grep -Eq '^[0-9]+(\.[0-9]+){3}$'; then
  EXPECTED="$VPS_HOST"
else
  EXPECTED=$(resolve "$VPS_HOST" | head -n1)
fi
if [ -z "$EXPECTED" ]; then
  echo "::error::could not determine VPS IP from VPS_HOST"
  exit 1
fi
echo "Expected VPS IP: $EXPECTED"

fail=0
for d in $DOMAINS $MONITOR_DOMAIN; do
  ips=$(resolve "$d")
  if [ -z "$ips" ]; then
    echo "::error::DNS resolution returned no A record: domain=$d"
    fail=1
    continue
  fi
  for ip in $ips; do
    if [ "$ip" != "$EXPECTED" ]; then
      echo "::error::domain=$d resolves to $ip, expected $EXPECTED"
      fail=1
    fi
  done
  echo "[$d] -> $(echo $ips | tr '\n' ' ')"
done
[ "$fail" -eq 0 ] || exit 1
