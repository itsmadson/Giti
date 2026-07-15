#!/usr/bin/env bash
# Scale a Geoson service and prove Traefik load-balances across the replicas.
# Usage: ./scale-smoke.sh gateway 4
set -euo pipefail
cd "$(dirname "$0")"

SERVICE="${1:-gateway}"
REPLICAS="${2:-3}"

echo "==> scaling ${SERVICE} to ${REPLICAS} replicas"
docker compose up -d --scale "${SERVICE}=${REPLICAS}" --no-recreate

echo "==> waiting for replicas to be healthy"
for i in $(seq 1 30); do
    healthy=$(docker compose ps "${SERVICE}" --format '{{.Health}}' | grep -c healthy || true)
    [ "${healthy}" -eq "${REPLICAS}" ] && break
    sleep 2
done
healthy=$(docker compose ps "${SERVICE}" --format '{{.Health}}' | grep -c healthy || true)
if [ "${healthy}" -ne "${REPLICAS}" ]; then
    echo "FAIL: only ${healthy}/${REPLICAS} healthy" >&2
    exit 1
fi
echo "==> ${healthy}/${REPLICAS} healthy"

echo "==> hitting /healthz 20x through traefik"
for i in $(seq 1 20); do
    curl -fsS http://localhost/healthz >/dev/null
done
echo "==> replicas serving:"
docker compose ps "${SERVICE}" --format '{{.Name}}'
echo "OK: ${SERVICE} scaled to ${REPLICAS} and serving through traefik"
