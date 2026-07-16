# wps

Web Processing Service. Go. Geometry ops computed in PostGIS.

## Operations
- GetCapabilities — lists processes
- DescribeProcess?identifier=... — inputs/outputs
- Execute?identifier=...&DataInputs=name=value;name=value — run a process
  - sync (default): returns `wps:ExecuteResponse` with the literal result
  - async (`storeExecuteResponse=true` or `mode=async`): enqueues a NATS job,
    returns `statusLocation=/wps/status/{id}`; poll for status
- `GET /wps/status/{id}` — job status JSON (accepted/running/succeeded/failed)

## Processes (v1)
buffer, centroid, area, length, reproject, intersection, union, simplify.
Each runs as a parameterized PostGIS query (`ST_Buffer`/`ST_Centroid`/… ) — the
DB is the compute engine (spec §10). Geometry inputs/outputs are WKT.

## Async
Execute (async) writes status to `GEOSON_WPS_RESULTS_DIR` and publishes a job on
NATS `wps.jobs`; a worker pool (scale `wps` replicas) executes and updates status.

## Env
GEOSON_HTTP_ADDR, GEOSON_DATABASE_URL, GEOSON_NATS_URL, GEOSON_WPS_RESULTS_DIR

## Known limitation
Polling `/wps/status/{id}` via the gateway `/geoserver/` prefix is parsed as an
OWS request (needs REQUEST param). Poll the tiles/wps service directly, or use
the absolute statusLocation. Full gateway pass-through for status is a follow-up.
