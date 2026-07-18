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
    let scale = request_scale(req);
    for (wkb, attrs) in feats {
        let geom = geozero::wkb::Wkb(wkb.clone())
            .to_geo()
            .map_err(|e| e.to_string())?;
        for rule in &req.style.rules {
            // zoom range: MinScaleDenominator = show when scale >= min; Max = scale <= max
            if let Some(min) = rule.min_scale {
                if scale < min {
                    continue;
                }
            }
            if let Some(max) = rule.max_scale {
                if scale > max {
                    continue;
                }
            }
            // thematic condition
            if let Some(f) = &rule.filter {
                if !f.matches(attrs) {
                    continue;
                }
            }
            for sym in &rule.symbolizers {
                if let crate::sld::Symbolizer::Text { property, fill, size, halo_radius, halo_color } = sym {
                    if let Some(txt) = attrs.get(property) {
                        if !txt.is_empty() {
                            if let Some((wx, wy)) = bbox_center(&geom) {
                                let (cx, cy) = proj.px(wx, wy);
                                draw_label(px, cx, cy, txt, *fill, *size, *halo_radius, *halo_color);
                            }
                        }
                    }
                } else {
                    draw_geom(px, &proj, &geom, sym);
                }
            }
        }
    }
    Ok(())
}

// request_scale approximates the OGC scale denominator (96 dpi ≈ 0.00028 m/px).
// Geographic bbox widths (degrees) are converted to metres at the equator.
fn request_scale(req: &MapRequest) -> f64 {
    let width_units = (req.bbox[2] - req.bbox[0]).abs();
    let geographic = req.srid == 4326 || req.srid == 4269;
    let width_m = if geographic { width_units * 111_320.0 } else { width_units };
    let px = req.width.max(1) as f64;
    width_m / (px * 0.000_28)
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
    // only a PolygonSymbolizer paints an area; other symbolizers (Text label,
    // Point, Line) on a polygon feature are skipped so they don't over-paint.
    let (fill, stroke, sw) = match sym {
        Symbolizer::Polygon {
            fill,
            stroke,
            stroke_width,
        } => (*fill, *stroke, *stroke_width),
        _ => return,
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
        _ => return, // skip non-line symbolizers (e.g. Text label) on line features
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
        _ => return, // skip non-point symbolizers (e.g. Text label) on point features
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

// ---- Text label rendering (ab_glyph + embedded DejaVu Sans) ----

use ab_glyph::{Font, FontRef};
use std::sync::OnceLock;

// Vazirmatn covers Persian + Latin and carries the GSUB/GPOS tables needed for
// Arabic-script shaping (contextual joining), so labels read correctly.
static FONT_BYTES: &[u8] = include_bytes!("../../assets/Vazirmatn-Regular.ttf");

fn font() -> &'static FontRef<'static> {
    static F: OnceLock<FontRef<'static>> = OnceLock::new();
    F.get_or_init(|| FontRef::try_from_slice(FONT_BYTES).expect("embedded font"))
}

fn shaper() -> &'static rustybuzz::Face<'static> {
    static F: OnceLock<rustybuzz::Face<'static>> = OnceLock::new();
    F.get_or_init(|| rustybuzz::Face::from_slice(FONT_BYTES, 0).expect("shaper face"))
}

// bbox_center returns the centre of a geometry's bounding box in world coords.
fn bbox_center(g: &geo_types::Geometry<f64>) -> Option<(f64, f64)> {
    let mut b = [f64::MAX, f64::MAX, f64::MIN, f64::MIN];
    let mut any = false;
    accumulate(g, &mut b, &mut any);
    if !any {
        return None;
    }
    Some(((b[0] + b[2]) / 2.0, (b[1] + b[3]) / 2.0))
}

fn upd(b: &mut [f64; 4], any: &mut bool, x: f64, y: f64) {
    if x < b[0] {
        b[0] = x;
    }
    if y < b[1] {
        b[1] = y;
    }
    if x > b[2] {
        b[2] = x;
    }
    if y > b[3] {
        b[3] = y;
    }
    *any = true;
}

fn accumulate(g: &geo_types::Geometry<f64>, b: &mut [f64; 4], any: &mut bool) {
    use geo_types::Geometry::*;
    match g {
        Point(p) => upd(b, any, p.0.x, p.0.y),
        MultiPoint(mp) => mp.0.iter().for_each(|p| upd(b, any, p.0.x, p.0.y)),
        LineString(ls) => ls.coords().for_each(|c| upd(b, any, c.x, c.y)),
        MultiLineString(mls) => mls.0.iter().for_each(|ls| ls.coords().for_each(|c| upd(b, any, c.x, c.y))),
        Polygon(p) => p.exterior().coords().for_each(|c| upd(b, any, c.x, c.y)),
        MultiPolygon(mp) => mp.0.iter().for_each(|p| p.exterior().coords().for_each(|c| upd(b, any, c.x, c.y))),
        GeometryCollection(gc) => gc.0.iter().for_each(|x| accumulate(x, b, any)),
        _ => {}
    }
}

// blend_px does straight-alpha src-over onto tiny-skia's premultiplied buffer.
fn blend_px(px: &mut Pixmap, x: i32, y: i32, c: [u8; 4], cov: f32) {
    let (w, h) = (px.width() as i32, px.height() as i32);
    if x < 0 || y < 0 || x >= w || y >= h || cov <= 0.0 {
        return;
    }
    let sa = cov * (c[3] as f32 / 255.0);
    if sa <= 0.0 {
        return;
    }
    let idx = (y as u32 * px.width() + x as u32) as usize;
    let pixels = px.pixels_mut();
    let dst = pixels[idx];
    // dst is premultiplied 0..255
    let (dr, dg, db, da) = (
        dst.red() as f32 / 255.0,
        dst.green() as f32 / 255.0,
        dst.blue() as f32 / 255.0,
        dst.alpha() as f32 / 255.0,
    );
    let inv = 1.0 - sa;
    let (sr, sg, sb) = (
        c[0] as f32 / 255.0 * sa,
        c[1] as f32 / 255.0 * sa,
        c[2] as f32 / 255.0 * sa,
    );
    let or = (sr + dr * inv).clamp(0.0, 1.0);
    let og = (sg + dg * inv).clamp(0.0, 1.0);
    let ob = (sb + db * inv).clamp(0.0, 1.0);
    let oa = (sa + da * inv).clamp(0.0, 1.0);
    if let Some(p) = tiny_skia::PremultipliedColorU8::from_rgba(
        (or * 255.0) as u8,
        (og * 255.0) as u8,
        (ob * 255.0) as u8,
        (oa * 255.0) as u8,
    ) {
        pixels[idx] = p;
    }
}

// A shaped glyph: font glyph id + pen position (screen px).
struct PlacedGlyph {
    gid: u16,
    x: f32,
    y: f32,
}

// shape lays out text with rustybuzz (Arabic joining + RTL handled), returning
// placed glyphs centred around (cx, cy) and the total advance width.
fn shape(text: &str, cx: f32, cy: f32, size: f32) -> Vec<PlacedGlyph> {
    let face = shaper();
    let upem = face.units_per_em() as f32;
    let s = size.max(6.0) / upem;

    let mut buf = rustybuzz::UnicodeBuffer::new();
    buf.push_str(text);
    buf.guess_segment_properties(); // detects Arabic script → RTL visual order
    let out = rustybuzz::shape(face, &[], buf);
    let infos = out.glyph_infos();
    let pos = out.glyph_positions();

    let total: f32 = pos.iter().map(|p| p.x_advance as f32 * s).sum();
    let ascent = face.ascender() as f32 * s;
    let mut pen_x = cx - total / 2.0;
    let base_y = cy + ascent / 2.0;

    let mut glyphs = Vec::with_capacity(infos.len());
    for (info, p) in infos.iter().zip(pos.iter()) {
        glyphs.push(PlacedGlyph {
            gid: info.glyph_id as u16,
            x: pen_x + p.x_offset as f32 * s,
            y: base_y - p.y_offset as f32 * s,
        });
        pen_x += p.x_advance as f32 * s;
    }
    glyphs
}

fn raster_glyph(px: &mut Pixmap, f: &FontRef, gid: u16, x: f32, y: f32, size: f32, col: [u8; 4]) {
    let g = ab_glyph::GlyphId(gid)
        .with_scale_and_position(ab_glyph::PxScale::from(size.max(6.0)), ab_glyph::point(x, y));
    if let Some(outline) = f.outline_glyph(g) {
        let b = outline.px_bounds();
        outline.draw(|gx, gy, cov| {
            blend_px(px, b.min.x as i32 + gx as i32, b.min.y as i32 + gy as i32, col, cov);
        });
    }
}

// draw_label shapes + rasterizes a label centred at (cx,cy), with a halo ring.
fn draw_label(
    px: &mut Pixmap,
    cx: f32,
    cy: f32,
    text: &str,
    fill: [u8; 4],
    size: f32,
    halo_radius: f32,
    halo_color: [u8; 4],
) {
    let f = font();
    let glyphs = shape(text, cx, cy, size);

    // halo: draw the glyph set offset in a ring around the origin
    if halo_radius > 0.0 {
        let r = halo_radius.clamp(0.5, 4.0);
        for (ox, oy) in [(-r, 0.0), (r, 0.0), (0.0, -r), (0.0, r), (-r, -r), (r, r), (-r, r), (r, -r)] {
            for g in &glyphs {
                raster_glyph(px, f, g.gid, g.x + ox, g.y + oy, size, halo_color);
            }
        }
    }
    for g in &glyphs {
        raster_glyph(px, f, g.gid, g.x, g.y, size, fill);
    }
}
