# Sprint E7 — Importer + Server Status + Monitoring + Settings — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** Import Data wizard (upload/dir/db → preview → batch publish), Server Status/About/Logs/Process-status, Monitor (per-request audit log + reports), Global Settings (contact, service metadata, proxy base, logging, image processing).

**Architecture:** `convert` (Go) already ingests + auto-publishes (S8). E7 adds an importer job engine (import context → items → targets → batch publish) over convert, a request-audit middleware in gateway writing to Postgres with a query/report API, status/logs endpoints, and a global-settings store surfaced in GetCapabilities.

**Tech Stack:** Go (convert/gateway/catalog), Next.js UI, NATS/SSE progress, PostGIS (audit + settings).

## Global Constraints

Same as prior. Audit must be low-overhead (async write, sampled if needed). Settings feed OWS GetCapabilities (contact/service metadata) — verify capabilities reflect changes.

## File Structure

- `services/convert/internal/importer/{context,item,run}.go` — import contexts + batch publish.
- `services/convert/internal/rest/apiv1_import.go` — importer API + SSE progress.
- `services/gateway/internal/audit/middleware.go` — per-request audit writer.
- `services/catalog/internal/rest/apiv1_monitor.go` — audit query + reports.
- `services/catalog/internal/rest/apiv1_settings.go` + `apiv1_status.go` — global settings, status, logs, process-status.
- `frontend/src/api/dashboard/{importer,monitor,settings,status}/*`.
- `frontend/src/components/dashboard/pages/{Importer,Monitor,Settings,Status}.tsx`.

## Task 1: Importer engine (backend)

**Interfaces produced:** `POST /api/v1/imports` (create context; body source `{type:upload|directory|database, ...}`) → `{id, items:[{name, detectedType, srs, target}]}`; `PUT /api/v1/imports/{id}/items/{i}` (edit target ws/store/srs/style); `POST /api/v1/imports/{id}/run` → SSE progress; `GET /api/v1/imports/{id}`.

- [ ] **Test:** create an import from a directory of shapefiles → items detected; run → layers published; progress events emitted.
- [ ] Implement context/item/run over the existing convert ingest + catalog REST auto-publish; detect type + SRS per item; batch publish with per-item status.
- [ ] Build; live verify: import a shapefile zip + a directory → published layers.
- [ ] Commit `feat(convert): importer engine + api (E7)`.

## Task 2: Request audit middleware + reports (backend)

**Interfaces produced:** audit row `{ts, service, operation, layer, status, duration_ms, user, remote_ip, bytes}`; `GET /api/v1/monitor/activity?since=&service=&layer=` (recent requests), `GET /api/v1/monitor/reports` (aggregate: count/avg-duration/errors by service+layer, slowest).

- [ ] **Test:** a WMS GetMap through the gateway writes one audit row; report aggregates it.
- [ ] Implement gateway middleware (async channel → batched insert into `request_audit`); catalog query + report endpoints.
- [ ] Build; live verify requests appear + report aggregates.
- [ ] Commit `feat(gateway,catalog): request audit + reports (E7)`.

## Task 3: Status / logs / process-status / settings (backend)

**Interfaces produced:** `GET /api/v1/status` (service health matrix, versions, uptime, memory), `GET /api/v1/logs?tail=` (recent log lines), `GET /api/v1/processes` (running WPS/seed/import jobs), `GET/PUT /api/v1/settings/global` (contact, service metadata, proxy base URL, logging level, image-processing/raster-access tuning).

- [ ] **Test:** settings PUT persists; GetCapabilities (WMS) shows updated contact/service title.
- [ ] Implement: status aggregates each service `/readyz` + build info; logs read from a ring buffer/log file; processes union WPS+seed+import job stores; settings feed OWS capabilities generators.
- [ ] Build; live verify capabilities reflect a settings change.
- [ ] Commit `feat(catalog): status/logs/processes/global-settings (E7)`.

## Task 4: Frontend — Importer wizard

- [ ] `Importer.tsx` 3-step wizard: **Source** (upload file/zip, server directory path, or database picker), **Preview** (items table with detected type/SRS + editable target ws/store/style), **Run** (batch publish with per-item progress via SSE). Route `dashboard/import`; nav (data group).
- [ ] i18n both dicts; live verify import flow end-to-end.
- [ ] Commit `feat(frontend): import data wizard (E7)`.

## Task 5: Frontend — Monitor / Status / Settings

- [ ] `Status.tsx` (About & Status: service health matrix, versions, uptime, logs viewer, process status). `Monitor.tsx` (Activity live table + Reports charts: requests/sec, avg duration, errors by service/layer, slowest). `Settings.tsx` (Global: contact info, service metadata per OWS, proxy base URL, logging, image processing/raster access). Routes under nav groups (System).
- [ ] i18n both dicts; live verify each page 200 + data renders.
- [ ] Commit `feat(frontend): status + monitor + settings pages (E7)`.

## Task 6: E7 acceptance

- [ ] Import a shapefile zip and a directory via wizard → published layers.
- [ ] View live server status + logs + running processes.
- [ ] Requests appear in audit Activity; Reports aggregate.
- [ ] Edit global contact/settings → reflected in GetCapabilities.
- [ ] Commit `chore: E7 importer + monitor + settings complete`.

## Self-Review

Spec E7 coverage: importer wizard ✅(T1/T4), status/about/logs/process ✅(T3/T5), monitor activity+reports ✅(T2/T5), global settings/contact/service-metadata/image-processing ✅(T3/T5). Audit async (low overhead). Endpoint shapes consistent backend↔client.
