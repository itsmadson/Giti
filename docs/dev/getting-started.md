# Getting Started (dev)

## Prerequisites

Go 1.26+, Rust stable, Docker + compose plugin.

## Run everything

    cd deploy/compose
    cp .env.example .env
    docker compose up -d --build
    curl http://localhost/healthz    # -> ok (gateway via Traefik)

## Run tests

    go test github.com/geoson/geoson/...   # Go (all workspace modules)
    cargo test --workspace                 # Rust

## Repo layout

See `docs/architecture.md` and spec §6. Task tracker: `task.md`.
