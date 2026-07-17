# Sprint E9 — CSW + WCS + OGC API + REST Parity — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans. `- [ ]` steps.

**Goal:** Add the remaining OGC protocols (CSW 2.0.2, WCS 2.0, OGC API-Features), complete `/rest` resource parity for third-party apps, virtual services, CRS handling (custom CRS/coordinate-ops/EPSG editing), and publish an OpenAPI doc.

**Architecture:** CSW + OGC API-Features served from catalog (records = catalog resources + metadata); WCS served from wms/tiles over raster coverage stores. REST audit fills gaps against the GeoServer `/rest` surface. Virtual services = per-workspace OWS endpoints via gateway routing.

**Tech Stack:** Go (catalog CSW/OGC-API/REST), Rust (wms WCS), OpenAPI (spec + docs page).

## Global Constraints

Same as prior. Protocol XML must be drop-in (namespaces, exception reports) — validated by the E10 golden-diff harness. OGC API-Features returns GeoJSON + HTML landing.

## File Structure

- `services/catalog/internal/csw/*` + gateway route `/giti/csw`.
- `services/catalog/internal/ogcapi/*` + routes `/ogc/features/...`.
- `services/wms/src/wcs.rs` + gateway route `/giti/wcs`.
- `services/catalog/internal/rest/*` — fill missing resources (fonts, layergroups, gwc, settings, security via auth).
- `services/gateway/internal/virtual/*` — per-workspace OWS routing.
- `docs/openapi/giti.yaml` + `frontend` docs page.

## Task 1: CSW 2.0.2 (backend)

- [ ] GetCapabilities, DescribeRecord, GetRecords (Dublin Core + ISO profile), GetRecordById. Records map from catalog resources + `metadata` jsonb (E2). Filter via existing OGC-Filter engine.
- [ ] **Test:** GetRecords returns records for published layers; GetRecordById returns one; ISO output well-formed.
- [ ] Gateway route `/giti/csw` (priority per convention).
- [ ] Live verify GetRecords through Traefik.
- [ ] Commit `feat(catalog): CSW 2.0.2 (E9)`.

## Task 2: WCS 2.0 (backend)

- [ ] GetCapabilities, DescribeCoverage, GetCoverage over raster coverage stores (E8). GetCoverage returns GeoTIFF subset (bbox/CRS/format).
- [ ] **Test:** GetCoverage returns a valid GeoTIFF for a seeded coverage.
- [ ] Gateway route `/giti/wcs`.
- [ ] Live verify DescribeCoverage + GetCoverage.
- [ ] Commit `feat(wms): WCS 2.0 (E9)`.

## Task 3: OGC API - Features (backend)

- [ ] Landing `/ogc/features/`, `/collections`, `/collections/{id}`, `/collections/{id}/items` (GeoJSON + HTML), conformance, OpenAPI link. Items support bbox/limit/CQL.
- [ ] **Test:** `/collections/{id}/items` returns GeoJSON FeatureCollection; HTML landing renders.
- [ ] Live verify through Traefik.
- [ ] Commit `feat(catalog): OGC API-Features (E9)`.

## Task 4: REST parity audit + virtual services + CRS handling (backend)

- [ ] Enumerate GeoServer `/rest` resources; fill gaps so each is present JSON+XML: workspaces/namespaces/stores(all kinds)/featuretypes/coverages/layers/layergroups/styles/fonts/security(→auth)/gwc(→tiles)/settings/reload. 
- [ ] Virtual services: gateway routes `/giti/{ws}/wms|wfs|...` scoping OWS to a workspace.
- [ ] CRS handling: custom-CRS registration (insert into `spatial_ref_sys`), coordinate-operation overrides, EPSG lookup editing; API `GET/POST /api/v1/srs`.
- [ ] **Test:** a third-party-style REST script round-trips create-workspace→store→featuretype→style→layer; virtual-service URL serves only that workspace's layers; a custom CRS resolves.
- [ ] Commit `feat: REST parity + virtual services + CRS handling (E9)`.

## Task 5: OpenAPI doc + docs page (frontend + docs)

- [ ] Author `docs/openapi/giti.yaml` covering API v1 + `/rest` + OGC endpoints. Serve at `/api/v1/openapi.yaml`; add a frontend docs page (Swagger-UI or Redoc embed) under System nav.
- [ ] Live verify docs page renders + "try it" hits a live endpoint.
- [ ] Commit `docs(api): OpenAPI spec + docs page (E9)`.

## Task 6: E9 acceptance

- [ ] CSW GetRecords returns catalog records; WCS GetCoverage returns a GeoTIFF subset; OGC API `/collections/{id}/items` returns GeoJSON.
- [ ] A REST client round-trips a full publish via the documented OpenAPI.
- [ ] Virtual-service URL scopes to a workspace; custom CRS resolves.
- [ ] Commit `chore: E9 CSW + WCS + OGC-API + REST parity complete`.

## Self-Review

Spec E9 coverage: CSW(T1), WCS(T2), OGC API-Features(T3), REST parity + virtual services + CRS handling(T4), OpenAPI(T5). Protocol conformance deferred to E10 golden-diff. Metadata source = E2 `metadata` jsonb; coverages = E8 raster stores (dependency noted).
