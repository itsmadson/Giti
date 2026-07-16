//! Raster tiles by proxying WMS GetMap for a tile's bbox.

use crate::grid::{tile_bbox, Gridset};

/// fetch_raster_tile requests an image tile from the WMS GetMap endpoint.
/// Returns (bytes, content_type).
pub async fn fetch_raster_tile(
    wms_url: &str,
    layer: &str,
    grid: &Gridset,
    z: u8,
    x: u32,
    y: u32,
    fmt: &str,
) -> Result<(Vec<u8>, String), String> {
    let b = tile_bbox(grid, z, x, y);
    // WMS 1.3.0 + EPSG:4326 expects lat/lon (miny,minx,maxy,maxx); 3857 is x,y.
    let bbox = if grid.srid == 4326 {
        format!("{},{},{},{}", b[1], b[0], b[3], b[2])
    } else {
        format!("{},{},{},{}", b[0], b[1], b[2], b[3])
    };
    let image_fmt = if fmt.contains("jpeg") || fmt.contains("jpg") {
        "image/jpeg"
    } else {
        "image/png"
    };
    let url = format!(
        "{wms_url}/wms?service=WMS&version=1.3.0&request=GetMap&layers={layer}\
         &crs=EPSG:{srid}&bbox={bbox}&width={ts}&height={ts}&format={image_fmt}&transparent=true",
        srid = grid.srid,
        ts = grid.tile_size,
    );
    let resp = reqwest::get(&url).await.map_err(|e| e.to_string())?;
    if !resp.status().is_success() {
        return Err(format!("wms getmap failed: {}", resp.status()));
    }
    let ct = resp
        .headers()
        .get(reqwest::header::CONTENT_TYPE)
        .and_then(|v| v.to_str().ok())
        .unwrap_or(image_fmt)
        .to_string();
    let bytes = resp.bytes().await.map_err(|e| e.to_string())?.to_vec();
    Ok((bytes, ct))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn proxies_getmap() {
        use axum::routing::get;
        let app = axum::Router::new().route(
            "/wms",
            get(|| async {
                (
                    [(axum::http::header::CONTENT_TYPE, "image/png")],
                    vec![0x89u8, b'P', b'N', b'G'],
                )
            }),
        );
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
        let g = crate::grid::web_mercator();
        let (bytes, ct) =
            fetch_raster_tile(&format!("http://{addr}"), "ws:layer", &g, 0, 0, 0, "png")
                .await
                .unwrap();
        assert!(ct.contains("png"));
        assert_eq!(&bytes[..4], &[0x89, b'P', b'N', b'G']);
    }
}
