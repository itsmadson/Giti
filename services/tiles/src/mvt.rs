//! Mapbox Vector Tile generation via PostGIS ST_AsMVT.

use crate::grid::{tile_bbox, Gridset};
use crate::meta::LayerMeta;
use sqlx::Row;

fn valid_ident(name: &str) -> bool {
    let mut c = name.chars();
    matches!(c.next(), Some(ch) if ch == '_' || ch.is_ascii_alphabetic())
        && c.all(|ch| ch == '_' || ch.is_ascii_alphanumeric())
}

/// render_mvt returns the MVT bytes for a layer at z/x/y. An empty tile (no
/// features) returns an empty Vec.
pub async fn render_mvt(
    pool: &sqlx::PgPool,
    layer: &LayerMeta,
    grid: &Gridset,
    z: u8,
    x: u32,
    y: u32,
) -> Result<Vec<u8>, String> {
    if !valid_ident(&layer.table) || !valid_ident(&layer.geom_col) {
        return Err("invalid table or geometry column".into());
    }
    let b = tile_bbox(grid, z, x, y);
    let geom = &layer.geom_col;
    let table = &layer.table;
    let srid = grid.srid;

    // Build the tile envelope in the grid SRID, transform table geom into it,
    // clip + encode with ST_AsMVTGeom / ST_AsMVT. Filter by bbox on the source
    // geometry (transformed envelope) so the spatial index is used.
    let sql = format!(
        r#"WITH bounds AS (
             SELECT ST_MakeEnvelope($1,$2,$3,$4,{srid}) AS geom
           )
           SELECT ST_AsMVT(t, '{layer_name}') FROM (
             SELECT ST_AsMVTGeom(
                      ST_Transform("{geom}", {srid}),
                      (SELECT geom FROM bounds), 4096, 64, true) AS geom
             FROM "{table}"
             WHERE ST_Transform("{geom}", {srid}) && (SELECT geom FROM bounds)
           ) t WHERE t.geom IS NOT NULL"#,
        layer_name = layer.name,
    );

    let row = sqlx::query(&sql)
        .bind(b[0])
        .bind(b[1])
        .bind(b[2])
        .bind(b[3])
        .fetch_optional(pool)
        .await
        .map_err(|e| e.to_string())?;
    let bytes: Vec<u8> = match row {
        Some(r) => r
            .try_get::<Option<Vec<u8>>, _>(0)
            .ok()
            .flatten()
            .unwrap_or_default(),
        None => Vec::new(),
    };
    Ok(bytes)
}
