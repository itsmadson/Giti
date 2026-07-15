# Scaling & HA

Every request-path service is stateless — scale any of them independently:

    cd deploy/compose
    docker compose up -d --scale wms=4 --scale gateway=2 --no-recreate

Traefik discovers replicas via the Docker provider and round-robins with
per-replica health checks (`/healthz`).

Prove it works:

    ./scale-smoke.sh gateway 4

## Notes

- Postgres is the single stateful primary; scale reads later via replicas.
- Tile seeding uses Redis locks so replicas never render the same metatile twice (Sprint 7).
- Docker Swarm stack for multi-node HA lands in Sprint 10 (`deploy/swarm/`).
