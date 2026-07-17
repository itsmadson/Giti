# Sprint E10 — Compat + Benchmarks + Ops + Release — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** Prove drop-in compatibility and speed, harden ops, ship the open-source release. Absorbs the old S10 (golden-diff harness + k6 benchmarks) plus control-flow throttling, UI RBAC pass, i18n completeness, and cluster stack.

**Architecture:** A dockerized GeoServer runs beside Giti; a harness fires identical OWS/REST requests at both and diffs (canonical XML + SSIM image). k6 drives load benchmarks. Gateway gains control-flow throttling. Frontend enforces RBAC from JWT roles + completes fa/en i18n.

**Tech Stack:** Docker Compose (GeoServer + Giti), Go/Python harness, k6, Traefik, Swarm/K8s manifests.

## Global Constraints

Same as prior. Benchmarks + diff run against the same seeded corpus. Release under Apache-2.0. No secrets in repo; dev creds only.

## File Structure

- `compat/harness/*` — request corpus + XML canonicalizer + SSIM diff runner.
- `compat/docker-compose.geoserver.yml` — reference GeoServer.
- `bench/k6/*.js` — load scripts (GetMap/GetFeature/GetTile).
- `services/gateway/internal/controlflow/*` — request throttling.
- `frontend/src/lib/rbac.ts` — role gate helpers; apply to nav + actions.
- `deploy/swarm/*`, `deploy/k8s/*` — cluster stacks.
- `docs/benchmarks.md`, `README.md`, `LICENSE`.

## Task 1: GeoServer golden-diff harness

- [ ] `compat/docker-compose.geoserver.yml`: reference GeoServer seeded with the same layers/styles as Giti.
- [ ] Harness: request corpus (WMS GetCapabilities/GetMap/GetFeatureInfo, WFS GetCapabilities/DescribeFeatureType/GetFeature, WMTS GetCapabilities/GetTile). Canonical-XML diff (sort attrs, normalize namespaces/whitespace) for XML; SSIM image diff (threshold) for GetMap/GetTile.
- [ ] **Test:** harness runs both servers, reports per-request pass/fail; commit the corpus + a passing baseline (allow documented known-diffs list).
- [ ] Commit `test(compat): GeoServer golden-diff harness (E10)`.

## Task 2: k6 load benchmarks

- [ ] `bench/k6/`: scripts for GetMap, GetFeature, GetTile at increasing VUs against Giti and GeoServer. Capture RPS, p50/p95/p99, error rate.
- [ ] Run both; write `docs/benchmarks.md` with results + methodology (hardware, dataset, config).
- [ ] **Acceptance:** Giti meets-or-beats GeoServer on the measured mix (document any losses + why).
- [ ] Commit `docs(bench): k6 benchmarks vs GeoServer (E10)`.

## Task 3: Control-flow throttling (gateway)

- [ ] `controlflow/`: per-service + per-user concurrent-request limits + queue (config `controlflow.rules`); 429/503 with Retry-After when exceeded.
- [ ] **Test:** exceeding a limit queues/rejects per rule; normal load passes.
- [ ] Commit `feat(gateway): control-flow throttling (E10)`.

## Task 4: UI RBAC pass + i18n completeness

- [ ] `rbac.ts`: read roles from JWT; gate nav items + destructive actions (hide/disable) per role; wire into Sidebar/CommandPalette/action buttons across all E1–E9 pages.
- [ ] i18n audit: every key present in both `en.ts` and `fa.ts`; RTL verified on all new pages; add a CI check script `scripts/i18n-check.mjs` failing on missing keys.
- [ ] **Test:** i18n-check passes; a non-admin role sees a reduced nav.
- [ ] Commit `feat(frontend): RBAC gating + i18n completeness (E10)`.

## Task 5: Cluster stack + release

- [ ] `deploy/swarm/` + `deploy/k8s/`: per-service horizontal scaling, shared Postgres/Redis/NATS/MinIO, Traefik ingress; health/readiness wired.
- [ ] `README.md` (quickstart, architecture, parity table link, benchmarks link), `LICENSE` (Apache-2.0), CONTRIBUTING, release notes; tag `v1.0.0`.
- [ ] **Acceptance:** `docker stack deploy` (or k8s apply) brings up a scaled, healthy Giti; quickstart works from a clean checkout.
- [ ] Commit `chore(release): cluster stacks + v1.0.0 docs (E10)`.

## Task 6: E10 acceptance

- [ ] Golden-diff harness green on the corpus (known-diffs documented).
- [ ] Benchmark report published showing Giti faster.
- [ ] Control-flow throttling enforces limits.
- [ ] RBAC gates nav; i18n-check passes.
- [ ] Clean release artifacts + docs; scaled stack healthy.
- [ ] Commit `chore: E10 compat + bench + release complete`.

## Self-Review

Spec E10 coverage: golden-diff(T1), k6 bench(T2), control-flow(T3), RBAC + i18n(T4), cluster + release(T5). Absorbs old S10. Compat corpus shared with E9 protocols. Apache-2.0, no secrets.
