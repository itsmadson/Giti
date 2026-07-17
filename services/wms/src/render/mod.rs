//! Render orchestration: fetch features → draw with tiny-skia.

pub mod vector;

use crate::meta::LayerMeta;
use crate::sld::Style;
use geo_core::filter::Expr;
use tiny_skia::Pixmap;

pub struct MapRequest {
    pub layer: LayerMeta,
    pub style: Style,
    pub bbox: [f64; 4], // minx,miny,maxx,maxy (axis-normalized to x,y) in the request SRS
    pub srid: i32,      // request SRS numeric code (bbox + output projection)
    pub width: u32,
    pub height: u32,
    pub transparent: bool,
    pub bgcolor: [u8; 4],
    pub cql: Option<Expr>,
}

/// render_map renders the request into a Pixmap.
pub async fn render_map(pool: &sqlx::PgPool, req: &MapRequest) -> Result<Pixmap, String> {
    let mut px = Pixmap::new(req.width, req.height).ok_or("invalid image size")?;
    if !req.transparent {
        let [r, g, b, a] = req.bgcolor;
        px.fill(tiny_skia::Color::from_rgba8(r, g, b, a));
    }
    let feats = vector::fetch_features(pool, req).await?;
    vector::draw_features(&mut px, req, &feats)?;
    Ok(px)
}
