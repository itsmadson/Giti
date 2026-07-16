//! Gridset definitions and tile → bbox math.

pub struct Gridset {
    pub name: String,
    pub tile_size: u32,
    pub max_zoom: u8,
    pub top_left: (f64, f64),
    pub world: f64, // half-extent from origin for square grids
    pub srid: i32,
}

const MERC_HALF: f64 = 20037508.342789244;

pub fn web_mercator() -> Gridset {
    Gridset {
        name: "EPSG:3857".into(),
        tile_size: 256,
        max_zoom: 22,
        top_left: (-MERC_HALF, MERC_HALF),
        world: MERC_HALF,
        srid: 3857,
    }
}

pub fn plate_carree() -> Gridset {
    Gridset {
        name: "EPSG:4326".into(),
        tile_size: 256,
        max_zoom: 21,
        top_left: (-180.0, 90.0),
        world: 180.0,
        srid: 4326,
    }
}

pub fn by_name(name: &str) -> Option<Gridset> {
    match name {
        "EPSG:3857" | "GoogleMapsCompatible" | "WebMercatorQuad" => Some(web_mercator()),
        "EPSG:4326" | "WorldCRS84Quad" => Some(plate_carree()),
        _ => None,
    }
}

/// tiles_per_axis returns tile count along the X axis at zoom z. Web Mercator is
/// 2^z square; plate carrée is 2^(z+1) columns × 2^z rows.
pub fn tiles_per_axis(g: &Gridset, z: u8) -> u32 {
    if g.srid == 4326 {
        1u32 << (z + 1)
    } else {
        1u32 << z
    }
}

/// tile_bbox returns [minx,miny,maxx,maxy] in the gridset SRS.
pub fn tile_bbox(g: &Gridset, z: u8, x: u32, y: u32) -> [f64; 4] {
    if g.srid == 4326 {
        // tile is square in degrees: span = 180 / 2^z
        let span = 180.0 / (1u32 << z) as f64;
        let minx = -180.0 + x as f64 * span;
        let maxy = 90.0 - y as f64 * span;
        return [minx, maxy - span, minx + span, maxy];
    }
    let span = g.world * 2.0 / (1u32 << z) as f64;
    let minx = -g.world + x as f64 * span;
    let maxy = g.world - y as f64 * span;
    [minx, maxy - span, minx + span, maxy]
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn web_mercator_z0_is_world() {
        let g = web_mercator();
        let b = tile_bbox(&g, 0, 0, 0);
        assert!((b[0] + MERC_HALF).abs() < 1e-3);
        assert!((b[3] - MERC_HALF).abs() < 1e-3);
        assert_eq!(tiles_per_axis(&g, 0), 1);
        assert_eq!(tiles_per_axis(&g, 3), 8);
    }

    #[test]
    fn web_mercator_z1_quadrants() {
        let g = web_mercator();
        let tl = tile_bbox(&g, 1, 0, 0);
        assert!(tl[0] < 0.0 && tl[3] > 0.0);
        let br = tile_bbox(&g, 1, 1, 1);
        assert!(br[0] >= 0.0 && br[1] < 0.0);
    }

    #[test]
    fn plate_carree_z0() {
        let g = plate_carree();
        let b = tile_bbox(&g, 0, 0, 0);
        assert!((b[0] + 180.0).abs() < 1e-6);
        assert!((b[3] - 90.0).abs() < 1e-6);
        let b1 = tile_bbox(&g, 0, 1, 0);
        assert!((b1[0] - 0.0).abs() < 1e-6);
    }

    #[test]
    fn lookup() {
        assert!(by_name("EPSG:3857").is_some());
        assert!(by_name("EPSG:4326").is_some());
        assert!(by_name("nope").is_none());
    }
}
