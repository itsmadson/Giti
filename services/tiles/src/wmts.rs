//! WMTS (KVP + REST), XYZ, and TMS tile handlers + GetCapabilities.

use crate::cache::Cache;
use crate::{grid, meta, mvt, raster, AppState};
use axum::extract::{Path, State};
use axum::http::header::CONTENT_TYPE;
use axum::http::{StatusCode, Uri};
use axum::response::{IntoResponse, Response};
use std::collections::HashMap;

const MVT_CT: &str = "application/vnd.mapbox-vector-tile";

fn is_vector_format(fmt: &str) -> bool {
    let f = fmt.to_lowercase();
    f.contains("pbf") || f.contains("mvt") || f.contains("vector") || f.contains("protobuf")
}

fn split_layer(qualified: &str) -> (String, String) {
    match qualified.find(':') {
        Some(i) => (qualified[..i].to_string(), qualified[i + 1..].to_string()),
        None => (String::new(), qualified.to_string()),
    }
}

fn cache_for(state: &AppState) -> Cache {
    Cache::new(state.cache_dir.clone(), state.redis.clone())
}

/// serve_tile is the shared cache-through path for all tile endpoints.
async fn serve_tile(
    state: &AppState,
    layer_q: &str,
    gridset_name: &str,
    z: u8,
    x: u32,
    y: u32,
    fmt: &str,
) -> Response {
    let pool = match &state.pool {
        Some(p) => p,
        None => return (StatusCode::SERVICE_UNAVAILABLE, "no database").into_response(),
    };
    let g = match grid::by_name(gridset_name) {
        Some(g) => g,
        None => return (StatusCode::BAD_REQUEST, "unknown gridset").into_response(),
    };
    let vector = is_vector_format(fmt);
    let ext = if vector { "pbf" } else { "png" };

    let mut cache = cache_for(state);
    let key = cache.key(layer_q, gridset_name, z, x, y, ext).await;
    if let Some(bytes) = cache.get(&key).await {
        if bytes.is_empty() {
            return StatusCode::NO_CONTENT.into_response();
        }
        let ct = if vector { MVT_CT } else { "image/png" };
        return ([(CONTENT_TYPE, ct)], bytes).into_response();
    }

    if vector {
        let (ws, name) = split_layer(layer_q);
        let m = match meta::resolve(pool, &ws, &name).await {
            Ok(m) => m,
            Err(e) => return (StatusCode::NOT_FOUND, e).into_response(),
        };
        let bytes = match mvt::render_mvt(pool, &m, &g, z, x, y).await {
            Ok(b) => b,
            Err(e) => return (StatusCode::INTERNAL_SERVER_ERROR, e).into_response(),
        };
        let _ = cache.put(&key, &bytes).await;
        if bytes.is_empty() {
            return StatusCode::NO_CONTENT.into_response();
        }
        return ([(CONTENT_TYPE, MVT_CT)], bytes).into_response();
    }

    // raster via WMS proxy
    match raster::fetch_raster_tile(&state.wms_url, layer_q, &g, z, x, y, fmt).await {
        Ok((bytes, ct)) => {
            let _ = cache.put(&key, &bytes).await;
            ([(CONTENT_TYPE, ct)], bytes).into_response()
        }
        Err(e) => (StatusCode::BAD_GATEWAY, e).into_response(),
    }
}

/// XYZ: /tiles/{layer}/{z}/{x}/{y}.{ext} (top-left origin, EPSG:3857).
pub async fn xyz(
    State(state): State<AppState>,
    Path((layer, z, x, yext)): Path<(String, u8, u32, String)>,
) -> Response {
    let (y, ext) = split_ext(&yext);
    serve_tile(&state, &layer, "EPSG:3857", z, x, y, &ext).await
}

/// TMS: /gwc/service/tms/1.0.0/{layer}/{z}/{x}/{y}.{ext} (bottom-left origin).
pub async fn tms(
    State(state): State<AppState>,
    Path((layer, z, x, yext)): Path<(String, u8, u32, String)>,
) -> Response {
    let (tms_y, ext) = split_ext(&yext);
    // flip: TMS y origin is bottom
    let g = grid::web_mercator();
    let max = grid::tiles_per_axis(&g, z).saturating_sub(1);
    let y = max.saturating_sub(tms_y);
    serve_tile(&state, &layer, "EPSG:3857", z, x, y, &ext).await
}

/// WMTS RESTful: /wmts/{layer}/{tms}/{z}/{y}/{x}.{ext}
pub async fn wmts_rest(
    State(state): State<AppState>,
    Path((layer, tms, z, y, xext)): Path<(String, String, u8, u32, String)>,
) -> Response {
    let (x, ext) = split_ext(&xext);
    serve_tile(&state, &layer, &tms, z, x, y, &ext).await
}

/// WMTS KVP: /wmts?REQUEST=GetCapabilities|GetTile
pub async fn wmts_kvp(State(state): State<AppState>, uri: Uri) -> Response {
    let kvp = parse_kvp(uri.query().unwrap_or(""));
    match kvp.get("REQUEST").map(|s| s.to_lowercase()).as_deref() {
        Some("getcapabilities") => get_capabilities(&state).await,
        Some("gettile") => {
            let layer = kvp.get("LAYER").cloned().unwrap_or_default();
            let tms = kvp
                .get("TILEMATRIXSET")
                .cloned()
                .unwrap_or_else(|| "EPSG:3857".into());
            let z: u8 = kvp.get("TILEMATRIX").and_then(|v| parse_z(v)).unwrap_or(0);
            let y: u32 = kvp.get("TILEROW").and_then(|v| v.parse().ok()).unwrap_or(0);
            let x: u32 = kvp.get("TILECOL").and_then(|v| v.parse().ok()).unwrap_or(0);
            let fmt = kvp
                .get("FORMAT")
                .cloned()
                .unwrap_or_else(|| MVT_CT.to_string());
            serve_tile(&state, &layer, &tms, z, x, y, &fmt).await
        }
        _ => (StatusCode::BAD_REQUEST, "unknown WMTS request").into_response(),
    }
}

// TILEMATRIX may be "EPSG:3857:5" or just "5".
fn parse_z(v: &str) -> Option<u8> {
    v.rsplit(':').next().and_then(|s| s.parse().ok())
}

fn split_ext(seg: &str) -> (u32, String) {
    match seg.rsplit_once('.') {
        Some((n, ext)) => (n.parse().unwrap_or(0), ext.to_string()),
        None => (seg.parse().unwrap_or(0), "pbf".to_string()),
    }
}

fn parse_kvp(query: &str) -> HashMap<String, String> {
    let mut m = HashMap::new();
    for (k, v) in form_urlencoded_parse(query) {
        m.insert(k.to_uppercase(), v);
    }
    m
}

// minimal urlencoded parse (avoid extra dep; queries are simple)
fn form_urlencoded_parse(q: &str) -> Vec<(String, String)> {
    q.split('&')
        .filter(|s| !s.is_empty())
        .map(|pair| {
            let (k, v) = pair.split_once('=').unwrap_or((pair, ""));
            (decode(k), decode(v))
        })
        .collect()
}

fn decode(s: &str) -> String {
    s.replace('+', " ")
        .replace("%3A", ":")
        .replace("%2F", "/")
        .replace("%2f", "/")
}

/// seed renders and caches all tiles for a layer over a zoom range.
pub async fn seed(State(state): State<AppState>, body: String) -> Response {
    let req: serde_json::Value = match serde_json::from_str(&body) {
        Ok(v) => v,
        Err(e) => return (StatusCode::BAD_REQUEST, e.to_string()).into_response(),
    };
    let layer_q = req["layer"].as_str().unwrap_or("");
    let gridset_name = req["gridset"].as_str().unwrap_or("EPSG:3857");
    let z_start = req["zoomStart"].as_u64().unwrap_or(0) as u8;
    let z_stop = req["zoomStop"].as_u64().unwrap_or(0) as u8;
    if z_stop > 5 {
        return (StatusCode::BAD_REQUEST, "zoomStop must be <= 5 in v1").into_response();
    }
    let pool = match &state.pool {
        Some(p) => p,
        None => return (StatusCode::SERVICE_UNAVAILABLE, "no database").into_response(),
    };
    let g = match grid::by_name(gridset_name) {
        Some(g) => g,
        None => return (StatusCode::BAD_REQUEST, "unknown gridset").into_response(),
    };
    let (ws, name) = split_layer(layer_q);
    let m = match meta::resolve(pool, &ws, &name).await {
        Ok(m) => m,
        Err(e) => return (StatusCode::NOT_FOUND, e).into_response(),
    };
    let mut cache = cache_for(&state);
    let mut seeded = 0u64;
    for z in z_start..=z_stop {
        let per = grid::tiles_per_axis(&g, z);
        let rows = if g.srid == 4326 { per / 2 } else { per };
        for x in 0..per {
            for y in 0..rows {
                if let Ok(bytes) = mvt::render_mvt(pool, &m, &g, z, x, y).await {
                    let key = cache.key(layer_q, gridset_name, z, x, y, "pbf").await;
                    let _ = cache.put(&key, &bytes).await;
                    seeded += 1;
                }
            }
        }
    }
    (
        [(CONTENT_TYPE, "application/json")],
        format!("{{\"seeded\":{seeded}}}"),
    )
        .into_response()
}

/// truncate invalidates all cached tiles for a layer (generation bump).
pub async fn truncate(State(state): State<AppState>, body: String) -> Response {
    let req: serde_json::Value = match serde_json::from_str(&body) {
        Ok(v) => v,
        Err(e) => return (StatusCode::BAD_REQUEST, e.to_string()).into_response(),
    };
    let layer_q = req["layer"].as_str().unwrap_or("");
    let mut cache = cache_for(&state);
    cache.bump_generation(layer_q).await;
    (StatusCode::OK, "truncated").into_response()
}

async fn get_capabilities(state: &AppState) -> Response {
    let pool = match &state.pool {
        Some(p) => p,
        None => return (StatusCode::SERVICE_UNAVAILABLE, "no database").into_response(),
    };
    let layers = meta::list_layers(pool).await.unwrap_or_default();
    let mut body = String::new();
    body.push_str("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n");
    body.push_str(
        "<Capabilities xmlns=\"http://www.opengis.net/wmts/1.0\" version=\"1.0.0\">\n<Contents>\n",
    );
    for l in &layers {
        let qn = if l.workspace.is_empty() {
            l.name.clone()
        } else {
            format!("{}:{}", l.workspace, l.name)
        };
        body.push_str(&format!(
            "  <Layer><ows:Identifier xmlns:ows=\"http://www.opengis.net/ows/1.1\">{qn}</ows:Identifier>\
             <Format>{MVT_CT}</Format><Format>image/png</Format>\
             <TileMatrixSetLink><TileMatrixSet>EPSG:3857</TileMatrixSet></TileMatrixSetLink>\
             <TileMatrixSetLink><TileMatrixSet>EPSG:4326</TileMatrixSet></TileMatrixSetLink></Layer>\n"
        ));
    }
    body.push_str("  <TileMatrixSet><ows:Identifier xmlns:ows=\"http://www.opengis.net/ows/1.1\">EPSG:3857</ows:Identifier></TileMatrixSet>\n");
    body.push_str("  <TileMatrixSet><ows:Identifier xmlns:ows=\"http://www.opengis.net/ows/1.1\">EPSG:4326</ows:Identifier></TileMatrixSet>\n");
    body.push_str("</Contents>\n</Capabilities>\n");
    ([(CONTENT_TYPE, "text/xml")], body).into_response()
}
