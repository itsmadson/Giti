//! WCS 2.0 (Web Coverage Service): GetCapabilities / DescribeCoverage /
//! GetCoverage over the same GeoTIFF coverages served by WMS.

use crate::coverage::Coverage;
use crate::meta;
use crate::AppState;
use axum::extract::State;
use axum::http::header::CONTENT_TYPE;
use axum::http::Uri;
use axum::response::{IntoResponse, Response};
use std::collections::HashMap;
use std::sync::Arc;

fn kvp(query: &str) -> HashMap<String, String> {
    let mut m = HashMap::new();
    for (k, v) in form_urlencoded::parse(query.as_bytes()) {
        m.insert(k.to_uppercase(), v.into_owned());
    }
    m
}

// coverage id "ws__name" or "ws:name" or bare "name"
fn split_cov(id: &str) -> (String, String) {
    for sep in ["__", ":"] {
        if let Some((ws, n)) = id.split_once(sep) {
            return (ws.to_string(), n.to_string());
        }
    }
    (String::new(), id.to_string())
}

pub async fn wcs_endpoint(State(state): State<Arc<AppState>>, uri: Uri) -> Response {
    let q = kvp(uri.query().unwrap_or(""));
    let pool = match &state.pool {
        Some(p) => p,
        None => return xml_err("NoApplicableCode", "no database"),
    };
    match q.get("REQUEST").map(|s| s.to_lowercase()).as_deref() {
        Some("getcapabilities") => get_capabilities(pool).await,
        Some("describecoverage") => describe_coverage(pool, &q).await,
        Some("getcoverage") => get_coverage(pool, &q).await,
        _ => xml_err("OperationNotSupported", "unknown WCS request"),
    }
}

fn xml(body: String) -> Response {
    ([(CONTENT_TYPE, "application/xml")], format!(r#"<?xml version="1.0" encoding="UTF-8"?>{body}"#)).into_response()
}

fn xml_err(code: &str, msg: &str) -> Response {
    ([(CONTENT_TYPE, "application/xml")], format!(
        r#"<?xml version="1.0"?><ows:ExceptionReport xmlns:ows="http://www.opengis.net/ows/2.0" version="2.0.0"><ows:Exception exceptionCode="{code}"><ows:ExceptionText>{msg}</ows:ExceptionText></ows:Exception></ows:ExceptionReport>"#
    )).into_response()
}

async fn get_capabilities(pool: &sqlx::PgPool) -> Response {
    let covs = meta::list_coverages(pool).await;
    let mut summaries = String::new();
    for (ws, n) in &covs {
        let id = format!("{ws}__{n}");
        summaries.push_str(&format!(
            r#"<wcs:CoverageSummary><wcs:CoverageId>{id}</wcs:CoverageId><wcs:CoverageSubtype>RectifiedGridCoverage</wcs:CoverageSubtype></wcs:CoverageSummary>"#
        ));
    }
    xml(format!(
        r#"<wcs:Capabilities xmlns:wcs="http://www.opengis.net/wcs/2.0" xmlns:ows="http://www.opengis.net/ows/2.0" version="2.0.1"><ows:ServiceIdentification><ows:Title>Giti WCS</ows:Title><ows:ServiceType>OGC WCS</ows:ServiceType><ows:ServiceTypeVersion>2.0.1</ows:ServiceTypeVersion></ows:ServiceIdentification><wcs:ServiceMetadata><wcs:formatSupported>image/tiff</wcs:formatSupported><wcs:formatSupported>image/png</wcs:formatSupported></wcs:ServiceMetadata><wcs:Contents>{summaries}</wcs:Contents></wcs:Capabilities>"#
    ))
}

async fn describe_coverage(pool: &sqlx::PgPool, q: &HashMap<String, String>) -> Response {
    let id = q.get("COVERAGEID").cloned().unwrap_or_default();
    let (ws, name) = split_cov(&id);
    let (path, srid) = match meta::resolve_coverage(pool, &ws, &name).await {
        Some(v) => v,
        None => return xml_err("NoSuchCoverage", "coverage not found"),
    };
    let cov = match Coverage::open(&path, srid) {
        Ok(c) => c,
        Err(e) => return xml_err("NoApplicableCode", &e),
    };
    let b = cov.bounds();
    xml(format!(
        r#"<wcs:CoverageDescriptions xmlns:wcs="http://www.opengis.net/wcs/2.0" xmlns:gml="http://www.opengis.net/gml/3.2"><wcs:CoverageDescription gml:id="{id}"><wcs:CoverageId>{id}</wcs:CoverageId><gml:boundedBy><gml:Envelope srsName="EPSG:{srid}"><gml:lowerCorner>{lx} {ly}</gml:lowerCorner><gml:upperCorner>{ux} {uy}</gml:upperCorner></gml:Envelope></gml:boundedBy><gml:domainSet><gml:RectifiedGrid dimension="2"><gml:limits><gml:GridEnvelope><gml:low>0 0</gml:low><gml:high>{w} {h}</gml:high></gml:GridEnvelope></gml:limits></gml:RectifiedGrid></gml:domainSet><wcs:CoverageSubtype>RectifiedGridCoverage</wcs:CoverageSubtype></wcs:CoverageDescription></wcs:CoverageDescriptions>"#,
        lx = b[0], ly = b[1], ux = b[2], uy = b[3], w = cov.width - 1, h = cov.height - 1
    ))
}

// parse WCS 2.0 SUBSET (e.g. "Long(44,63)") into (axis, min, max)
fn parse_subset(s: &str) -> Option<(String, f64, f64)> {
    let (axis, rest) = s.split_once('(')?;
    let inner = rest.trim_end_matches(')');
    let parts: Vec<f64> = inner.split(',').filter_map(|x| x.trim().parse().ok()).collect();
    if parts.len() == 2 {
        Some((axis.trim().to_lowercase(), parts[0], parts[1]))
    } else {
        None
    }
}

async fn get_coverage(pool: &sqlx::PgPool, q: &HashMap<String, String>) -> Response {
    let id = q.get("COVERAGEID").cloned().unwrap_or_default();
    let (ws, name) = split_cov(&id);
    let (path, srid) = match meta::resolve_coverage(pool, &ws, &name).await {
        Some(v) => v,
        None => return xml_err("NoSuchCoverage", "coverage not found"),
    };
    let cov = match Coverage::open(&path, srid) {
        Ok(c) => c,
        Err(e) => return xml_err("NoApplicableCode", &e),
    };
    // subset domain: full bounds unless SUBSET axes narrow it
    let mut bbox = cov.bounds();
    for (k, v) in q {
        if k.starts_with("SUBSET") {
            if let Some((axis, lo, hi)) = parse_subset(v) {
                match axis.as_str() {
                    "long" | "x" | "e" => { bbox[0] = lo; bbox[2] = hi; }
                    "lat" | "y" | "n" => { bbox[1] = lo; bbox[3] = hi; }
                    _ => {}
                }
            }
        }
    }
    let out_w: u32 = q.get("SCALESIZE").and_then(|s| scalesize_w(s)).unwrap_or(cov.width.min(1024));
    let out_h: u32 = ((bbox[3] - bbox[1]) / (bbox[2] - bbox[0]).max(1e-9) * out_w as f64) as u32;
    let px = cov.render(bbox, srid, out_w, out_h.max(1));

    let fmt = q.get("FORMAT").map(|s| s.to_lowercase()).unwrap_or_else(|| "image/png".into());
    if fmt.contains("tiff") {
        // GeoTIFF subset not re-encoded yet → return the source coverage file.
        match std::fs::read(&path) {
            Ok(bytes) => ([(CONTENT_TYPE, "image/tiff")], bytes).into_response(),
            Err(e) => xml_err("NoApplicableCode", &e.to_string()),
        }
    } else {
        let (bytes, ct) = crate::encode::encode_for("image/png", &px);
        ([(CONTENT_TYPE, ct)], bytes).into_response()
    }
}

fn scalesize_w(s: &str) -> Option<u32> {
    // "i(512)" or "x(512)"
    s.split_once('(').and_then(|(_, r)| r.trim_end_matches(')').parse().ok())
}
