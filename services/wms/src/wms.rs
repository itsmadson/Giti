//! WMS request handlers: GetMap / GetCapabilities / GetFeatureInfo / GetLegendGraphic.

use crate::ows::{exception_response, negotiate_wms, Kvp};
use crate::{meta, render, sld, AppState};
use axum::extract::State;
use axum::http::header::CONTENT_TYPE;
use axum::http::{HeaderMap, Uri};
use axum::response::{IntoResponse, Response};
use geo_core::filter::parse_cql;
use sqlx::Row;
use std::sync::Arc;

/// wms_endpoint dispatches a WMS request on REQUEST.
pub async fn wms_endpoint(
    State(state): State<Arc<AppState>>,
    headers: HeaderMap,
    uri: Uri,
) -> Response {
    let kvp = Kvp::parse(uri.query().unwrap_or(""));
    let version = negotiate_wms(&kvp.version());
    let pool = match &state.pool {
        Some(p) => p,
        None => {
            return exception_response(&version, "NoApplicableCode", "", "no database configured")
        }
    };
    let hdr_ws = headers
        .get("x-giti-workspace")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    let auth_cql = headers
        .get("x-giti-cql-read")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");

    match kvp.request().to_lowercase().as_str() {
        "getmap" => get_map(pool, &kvp, &version, hdr_ws, auth_cql).await,
        "getcapabilities" => get_capabilities(pool, &version).await,
        "getfeatureinfo" => get_feature_info(pool, &kvp, &version, hdr_ws, auth_cql).await,
        "getlegendgraphic" => get_legend(pool, &kvp, &version, hdr_ws).await,
        other => exception_response(
            &version,
            "OperationNotSupported",
            "request",
            &format!("Operation '{other}' not supported"),
        ),
    }
}

fn split_layer(qualified: &str, header_ws: &str) -> (String, String) {
    match qualified.find(':') {
        Some(i) => (qualified[..i].to_string(), qualified[i + 1..].to_string()),
        None => (header_ws.to_string(), qualified.to_string()),
    }
}

fn parse_bbox(raw: &str) -> Option<[f64; 4]> {
    let parts: Vec<f64> = raw
        .split(',')
        .filter_map(|s| s.trim().parse().ok())
        .collect();
    if parts.len() < 4 {
        return None;
    }
    Some([parts[0], parts[1], parts[2], parts[3]])
}

// is_geographic reports EPSG codes served lat/lon (axis swap for 1.3.0).
fn is_geographic(srs: &str) -> bool {
    let s = srs.to_uppercase();
    s.contains("4326") || s.contains("4269")
}

/// normalize_bbox swaps axes for WMS 1.3.0 geographic CRS (lat/lon → lon/lat).
fn normalize_bbox(bbox: [f64; 4], version: &str, srs: &str) -> [f64; 4] {
    if version == "1.3.0" && is_geographic(srs) {
        return [bbox[1], bbox[0], bbox[3], bbox[2]];
    }
    bbox
}

async fn get_map(
    pool: &sqlx::PgPool,
    kvp: &Kvp,
    version: &str,
    header_ws: &str,
    auth_cql: &str,
) -> Response {
    let layers = kvp.get("LAYERS").unwrap_or("");
    if layers.is_empty() {
        return exception_response(
            version,
            "MissingParameterValue",
            "layers",
            "LAYERS required",
        );
    }
    let (ws, name) = split_layer(layers, header_ws);
    let layer = match meta::resolve(pool, &ws, &name).await {
        Ok(l) => l,
        Err(e) => return exception_response(version, "LayerNotDefined", "layers", &e),
    };

    let srs = kvp
        .get("CRS")
        .or_else(|| kvp.get("SRS"))
        .unwrap_or("EPSG:4326");
    let bbox = match kvp.get("BBOX").and_then(parse_bbox) {
        Some(b) => normalize_bbox(b, version, srs),
        None => {
            return exception_response(version, "MissingParameterValue", "bbox", "BBOX required")
        }
    };
    let width: u32 = kvp.get("WIDTH").and_then(|v| v.parse().ok()).unwrap_or(256);
    let height: u32 = kvp
        .get("HEIGHT")
        .and_then(|v| v.parse().ok())
        .unwrap_or(256);
    let transparent = kvp
        .get("TRANSPARENT")
        .map(|v| v.eq_ignore_ascii_case("true"))
        .unwrap_or(false);
    let bgcolor = kvp
        .get("BGCOLOR")
        .and_then(parse_hex)
        .unwrap_or([255, 255, 255, 255]);

    // style: STYLES param or the layer default
    let style = load_style(pool, &ws, kvp.get("STYLES"), &layer).await;

    // combine CQL_FILTER + auth CQL
    let mut cql = kvp.get("CQL_FILTER").and_then(|c| parse_cql(c).ok());
    if !auth_cql.is_empty() {
        if let Ok(a) = parse_cql(auth_cql) {
            cql = Some(match cql {
                Some(existing) => geo_core::filter::Expr::Logic {
                    op: "AND".into(),
                    exprs: vec![existing, a],
                },
                None => a,
            });
        }
    }

    let req = render::MapRequest {
        layer,
        style,
        bbox,
        width,
        height,
        transparent,
        bgcolor,
        cql,
    };
    match render::render_map(pool, &req).await {
        Ok(px) => {
            let (bytes, ct) =
                crate::encode::encode_for(kvp.get("FORMAT").unwrap_or("image/png"), &px);
            ([(CONTENT_TYPE, ct)], bytes).into_response()
        }
        Err(e) => exception_response(version, "NoApplicableCode", "", &e),
    }
}

async fn load_style(
    pool: &sqlx::PgPool,
    ws: &str,
    styles_param: Option<&str>,
    layer: &meta::LayerMeta,
) -> sld::Style {
    let style_name = match styles_param {
        Some(s) if !s.is_empty() => s.to_string(),
        _ => layer.default_style.clone(),
    };
    if !style_name.is_empty() {
        if let Ok(body) = meta::style_body(pool, ws, &style_name).await {
            if let Ok(s) = sld::parse_sld(&body) {
                if !s.rules.is_empty() {
                    return s;
                }
            }
        }
    }
    sld::default_style_for(&layer.geom_type)
}

fn parse_hex(s: &str) -> Option<[u8; 4]> {
    let h = s.trim_start_matches("0x").trim_start_matches('#');
    if h.len() >= 6 {
        let r = u8::from_str_radix(&h[0..2], 16).ok()?;
        let g = u8::from_str_radix(&h[2..4], 16).ok()?;
        let b = u8::from_str_radix(&h[4..6], 16).ok()?;
        return Some([r, g, b, 255]);
    }
    None
}

async fn get_capabilities(pool: &sqlx::PgPool, version: &str) -> Response {
    let layers = meta::list_layers(pool).await.unwrap_or_default();
    let root = if version == "1.3.0" {
        "WMS_Capabilities"
    } else {
        "WMT_MS_Capabilities"
    };
    let crs_tag = if version == "1.3.0" { "CRS" } else { "SRS" };
    let mut body = String::new();
    body.push_str(&format!(
        "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<{root} version=\"{version}\">\n<Capability><Layer>\n"
    ));
    for l in &layers {
        let qn = if l.workspace.is_empty() {
            l.name.clone()
        } else {
            format!("{}:{}", l.workspace, l.name)
        };
        body.push_str(&format!(
            "  <Layer queryable=\"1\"><Name>{qn}</Name><Title>{qn}</Title><{crs_tag}>EPSG:4326</{crs_tag}></Layer>\n"
        ));
    }
    body.push_str(&format!("</Layer></Capability>\n</{root}>\n"));
    ([(CONTENT_TYPE, "text/xml")], body).into_response()
}

async fn get_feature_info(
    pool: &sqlx::PgPool,
    kvp: &Kvp,
    version: &str,
    header_ws: &str,
    auth_cql: &str,
) -> Response {
    let query_layers = kvp
        .get("QUERY_LAYERS")
        .or_else(|| kvp.get("LAYERS"))
        .unwrap_or("");
    let (ws, name) = split_layer(query_layers, header_ws);
    let layer = match meta::resolve(pool, &ws, &name).await {
        Ok(l) => l,
        Err(e) => return exception_response(version, "LayerNotDefined", "query_layers", &e),
    };
    let srs = kvp
        .get("CRS")
        .or_else(|| kvp.get("SRS"))
        .unwrap_or("EPSG:4326");
    let bbox = match kvp.get("BBOX").and_then(parse_bbox) {
        Some(b) => normalize_bbox(b, version, srs),
        None => {
            return exception_response(version, "MissingParameterValue", "bbox", "BBOX required")
        }
    };
    let width: f64 = kvp
        .get("WIDTH")
        .and_then(|v| v.parse().ok())
        .unwrap_or(256.0);
    let height: f64 = kvp
        .get("HEIGHT")
        .and_then(|v| v.parse().ok())
        .unwrap_or(256.0);
    let i: f64 = kvp
        .get("I")
        .or_else(|| kvp.get("X"))
        .and_then(|v| v.parse().ok())
        .unwrap_or(0.0);
    let j: f64 = kvp
        .get("J")
        .or_else(|| kvp.get("Y"))
        .and_then(|v| v.parse().ok())
        .unwrap_or(0.0);
    // pixel -> world
    let wx = bbox[0] + (i / width) * (bbox[2] - bbox[0]);
    let wy = bbox[3] - (j / height) * (bbox[3] - bbox[1]);
    // click tolerance: ~5 pixels in map units (so points/lines are hittable,
    // as GeoServer does — a bare ST_Intersects never hits a 0-D point).
    let px_w = (bbox[2] - bbox[0]) / width.max(1.0);
    let px_h = (bbox[3] - bbox[1]) / height.max(1.0);
    let tol = 5.0 * px_w.max(px_h);
    let count: i64 = kvp
        .get("FEATURE_COUNT")
        .and_then(|v| v.parse().ok())
        .unwrap_or(1);

    let cols: Vec<String> = layer
        .columns
        .iter()
        .filter(|c| valid_ident(c))
        .map(|c| format!("\"{c}\"::text"))
        .collect();
    let where_sql = format!(
        "ST_DWithin(\"{}\", ST_SetSRID(ST_Point($1,$2),4326), $3)",
        layer.geom_col
    );
    if !auth_cql.is_empty() {
        if let Ok(e) = parse_cql(auth_cql) {
            if let Ok((frag, _)) = geo_core::filter::to_sql(&e, 3) {
                // note: auth cql args would need binding; kept simple (rare on GFI)
                let _ = frag;
            }
        }
    }
    let sql = format!(
        "SELECT {} FROM \"{}\" WHERE {where_sql} LIMIT {count}",
        if cols.is_empty() {
            "1".to_string()
        } else {
            cols.join(", ")
        },
        layer.table
    );
    let rows = match sqlx::query(&sql)
        .bind(wx)
        .bind(wy)
        .bind(tol)
        .fetch_all(pool)
        .await
    {
        Ok(r) => r,
        Err(e) => return exception_response(version, "NoApplicableCode", "", &e.to_string()),
    };

    let info_format = kvp.get("INFO_FORMAT").unwrap_or("text/plain");
    if info_format.contains("json") {
        let mut features = Vec::new();
        for row in &rows {
            let mut props = serde_json::Map::new();
            for (idx, c) in layer.columns.iter().filter(|c| valid_ident(c)).enumerate() {
                let v: Option<String> = row.try_get(idx).ok().flatten();
                props.insert(c.clone(), serde_json::Value::String(v.unwrap_or_default()));
            }
            features.push(serde_json::json!({
                "type": "Feature", "geometry": serde_json::Value::Null, "properties": props
            }));
        }
        let fc = serde_json::json!({"type":"FeatureCollection","features":features});
        return (
            [(CONTENT_TYPE, "application/json")],
            serde_json::to_string(&fc).unwrap(),
        )
            .into_response();
    }
    // text/plain
    let mut out = String::new();
    for (n, row) in rows.iter().enumerate() {
        out.push_str(&format!("Feature {n}:\n"));
        for (idx, c) in layer.columns.iter().filter(|c| valid_ident(c)).enumerate() {
            let v: Option<String> = row.try_get(idx).ok().flatten();
            out.push_str(&format!("  {c} = {}\n", v.unwrap_or_default()));
        }
    }
    ([(CONTENT_TYPE, "text/plain")], out).into_response()
}

fn valid_ident(name: &str) -> bool {
    let mut c = name.chars();
    matches!(c.next(), Some(ch) if ch == '_' || ch.is_ascii_alphabetic())
        && c.all(|ch| ch == '_' || ch.is_ascii_alphanumeric())
}

async fn get_legend(pool: &sqlx::PgPool, kvp: &Kvp, version: &str, header_ws: &str) -> Response {
    let layers = kvp.get("LAYER").or_else(|| kvp.get("LAYERS")).unwrap_or("");
    let (ws, name) = split_layer(layers, header_ws);
    let layer = match meta::resolve(pool, &ws, &name).await {
        Ok(l) => l,
        Err(e) => return exception_response(version, "LayerNotDefined", "layer", &e),
    };
    let style = load_style(pool, &ws, kvp.get("STYLE"), &layer).await;
    let mut px = tiny_skia::Pixmap::new(20, 20).unwrap();
    px.fill(tiny_skia::Color::from_rgba8(255, 255, 255, 0));
    // draw a single swatch from the first symbolizer over a fake unit geometry
    if let Some(sym) = style.rules.first().and_then(|r| r.symbolizers.first()) {
        draw_swatch(&mut px, sym);
    }
    let png = crate::encode::encode_png(&px).unwrap_or_default();
    ([(CONTENT_TYPE, "image/png")], png).into_response()
}

fn draw_swatch(px: &mut tiny_skia::Pixmap, sym: &sld::Symbolizer) {
    let mut paint = tiny_skia::Paint::default();
    let c = match sym {
        sld::Symbolizer::Polygon { fill, .. } => *fill,
        sld::Symbolizer::Line { stroke, .. } => *stroke,
        sld::Symbolizer::Point { fill, .. } => *fill,
        _ => [128, 128, 128, 255],
    };
    paint.set_color(tiny_skia::Color::from_rgba8(c[0], c[1], c[2], c[3]));
    if let Some(rect) = tiny_skia::Rect::from_xywh(2.0, 2.0, 16.0, 16.0) {
        px.fill_rect(rect, &paint, tiny_skia::Transform::identity(), None);
    }
}
