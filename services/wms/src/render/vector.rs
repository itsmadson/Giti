//! Vector feature fetch (PostGIS WKB) + drawing (tiny-skia).

use super::MapRequest;
use crate::sld::Symbolizer;
use geo_core::filter::{to_sql, Arg};
use geozero::ToGeo;
use sqlx::Row;
use std::collections::HashMap;
use tiny_skia::{Paint, PathBuilder, Pixmap, Stroke, Transform};

pub type Feature = (Vec<u8>, HashMap<String, String>);

fn valid_ident(name: &str) -> bool {
    let mut c = name.chars();
    matches!(c.next(), Some(ch) if ch == '_' || ch.is_ascii_alphabetic())
        && c.all(|ch| ch == '_' || ch.is_ascii_alphanumeric())
}

/// fetch_features returns (wkb, attrs) rows within bbox, filtered by cql.
pub async fn fetch_features(pool: &sqlx::PgPool, req: &MapRequest) -> Result<Vec<Feature>, String> {
    if !valid_ident(&req.layer.table) || !valid_ident(&req.layer.geom_col) {
        return Err("invalid table or geometry column".into());
    }
    let geom = &req.layer.geom_col;
    let mut cols_sql = String::new();
    for c in &req.layer.columns {
        if !valid_ident(c) {
            continue;
        }
        cols_sql.push_str(&format!(", \"{c}\"::text"));
    }

    // On-the-fly reprojection (like GeoServer): the request bbox is in req.srid,
    // the data is in its native srid. Build the filter envelope in the request
    // SRS and transform it to the native SRS so the spatial index is still used;
    // output geometry is transformed to the request SRS to match the pixel bbox.
    let req_srid = if req.srid > 0 { req.srid } else { 4326 };
    let native = if req.layer.srid > 0 { req.layer.srid } else { 4326 };

    // bbox -> $1..$4 ; cql args continue from $5
    let mut where_sql = format!(
        "\"{geom}\" && ST_Transform(ST_MakeEnvelope($1, $2, $3, $4, {req_srid}), {native})"
    );
    let mut cql_args: Vec<Arg> = Vec::new();
    if let Some(e) = &req.cql {
        let (frag, args) = to_sql(e, 5)?;
        where_sql.push_str(&format!(" AND {frag}"));
        cql_args = args;
    }

    let sql = format!(
        "SELECT ST_AsBinary(ST_Transform(\"{geom}\", {req_srid})) AS wkb{cols_sql}
         FROM \"{}\" WHERE {where_sql}",
        req.layer.table
    );

    let mut q = sqlx::query(&sql)
        .bind(req.bbox[0])
        .bind(req.bbox[1])
        .bind(req.bbox[2])
        .bind(req.bbox[3]);
    for a in &cql_args {
        q = match a {
            Arg::Str(s) => q.bind(s.clone()),
            Arg::Num(n) => q.bind(*n),
            Arg::Bool(b) => q.bind(*b),
        };
    }

    let rows = q.fetch_all(pool).await.map_err(|e| e.to_string())?;
    let mut out = Vec::with_capacity(rows.len());
    for row in &rows {
        let wkb: Vec<u8> = row.get("wkb");
        let mut attrs = HashMap::new();
        let mut idx = 1; // column 0 is wkb
        for c in &req.layer.columns {
            if !valid_ident(c) {
                continue;
            }
            let v: Option<String> = row.try_get(idx).ok().flatten();
            if let Some(v) = v {
                attrs.insert(c.clone(), v);
            }
            idx += 1;
        }
        out.push((wkb, attrs));
    }
    Ok(out)
}

struct Proj {
    minx: f64,
    miny: f64,
    sx: f64,
    sy: f64,
    height: f64,
}

impl Proj {
    fn new(req: &MapRequest) -> Proj {
        let [minx, miny, maxx, maxy] = req.bbox;
        Proj {
            minx,
            miny,
            sx: req.width as f64 / (maxx - minx),
            sy: req.height as f64 / (maxy - miny),
            height: req.height as f64,
        }
    }
    fn px(&self, x: f64, y: f64) -> (f32, f32) {
        let px = (x - self.minx) * self.sx;
        let py = self.height - (y - self.miny) * self.sy;
        (px as f32, py as f32)
    }
}

/// draw_features renders decoded features onto the pixmap using the style.
pub fn draw_features(px: &mut Pixmap, req: &MapRequest, feats: &[Feature]) -> Result<(), String> {
    let proj = Proj::new(req);
    for (wkb, _attrs) in feats {
        let geom = geozero::wkb::Wkb(wkb.clone())
            .to_geo()
            .map_err(|e| e.to_string())?;
        for rule in &req.style.rules {
            for sym in &rule.symbolizers {
                draw_geom(px, &proj, &geom, sym);
            }
        }
    }
    Ok(())
}

fn color(c: [u8; 4]) -> tiny_skia::Color {
    tiny_skia::Color::from_rgba8(c[0], c[1], c[2], c[3])
}

fn draw_geom(px: &mut Pixmap, proj: &Proj, geom: &geo_types::Geometry<f64>, sym: &Symbolizer) {
    use geo_types::Geometry::*;
    match geom {
        Polygon(p) => draw_polygon(px, proj, p, sym),
        MultiPolygon(mp) => {
            for p in &mp.0 {
                draw_polygon(px, proj, p, sym);
            }
        }
        LineString(ls) => draw_line(px, proj, ls, sym),
        MultiLineString(mls) => {
            for ls in &mls.0 {
                draw_line(px, proj, ls, sym);
            }
        }
        Point(pt) => draw_point(px, proj, pt.0.x, pt.0.y, sym),
        MultiPoint(mp) => {
            for pt in &mp.0 {
                draw_point(px, proj, pt.0.x, pt.0.y, sym);
            }
        }
        _ => {}
    }
}

fn ring_path(proj: &Proj, coords: impl Iterator<Item = (f64, f64)>) -> Option<tiny_skia::Path> {
    let mut pb = PathBuilder::new();
    let mut first = true;
    for (x, y) in coords {
        let (px, py) = proj.px(x, y);
        if first {
            pb.move_to(px, py);
            first = false;
        } else {
            pb.line_to(px, py);
        }
    }
    if first {
        return None;
    }
    pb.close();
    pb.finish()
}

fn draw_polygon(px: &mut Pixmap, proj: &Proj, poly: &geo_types::Polygon<f64>, sym: &Symbolizer) {
    let (fill, stroke, sw) = match sym {
        Symbolizer::Polygon {
            fill,
            stroke,
            stroke_width,
        } => (*fill, *stroke, *stroke_width),
        _ => ([170, 170, 170, 255], [0, 0, 0, 255], 1.0),
    };
    if let Some(path) = ring_path(proj, poly.exterior().coords().map(|c| (c.x, c.y))) {
        let mut paint = Paint::default();
        paint.set_color(color(fill));
        paint.anti_alias = true;
        px.fill_path(
            &path,
            &paint,
            tiny_skia::FillRule::EvenOdd,
            Transform::identity(),
            None,
        );
        let mut sp = Paint::default();
        sp.set_color(color(stroke));
        sp.anti_alias = true;
        px.stroke_path(
            &path,
            &sp,
            &Stroke {
                width: sw,
                ..Default::default()
            },
            Transform::identity(),
            None,
        );
    }
}

fn draw_line(px: &mut Pixmap, proj: &Proj, ls: &geo_types::LineString<f64>, sym: &Symbolizer) {
    let (stroke, width) = match sym {
        Symbolizer::Line { stroke, width } => (*stroke, *width),
        _ => ([0, 0, 255, 255], 1.0),
    };
    let mut pb = PathBuilder::new();
    let mut first = true;
    for c in ls.coords() {
        let (x, y) = proj.px(c.x, c.y);
        if first {
            pb.move_to(x, y);
            first = false;
        } else {
            pb.line_to(x, y);
        }
    }
    if let Some(path) = pb.finish() {
        let mut sp = Paint::default();
        sp.set_color(color(stroke));
        sp.anti_alias = true;
        px.stroke_path(
            &path,
            &sp,
            &Stroke {
                width,
                ..Default::default()
            },
            Transform::identity(),
            None,
        );
    }
}

fn draw_point(px: &mut Pixmap, proj: &Proj, x: f64, y: f64, sym: &Symbolizer) {
    let (fill, size) = match sym {
        Symbolizer::Point { fill, size, .. } => (*fill, *size),
        _ => ([255, 0, 0, 255], 6.0),
    };
    let (cx, cy) = proj.px(x, y);
    if let Some(circle) = PathBuilder::from_circle(cx, cy, size / 2.0) {
        let mut paint = Paint::default();
        paint.set_color(color(fill));
        paint.anti_alias = true;
        px.fill_path(
            &circle,
            &paint,
            tiny_skia::FillRule::Winding,
            Transform::identity(),
            None,
        );
    }
}
