//! Geoson tiles service (WMTS/XYZ/TMS, MVT + raster cache).

pub mod cache;
pub mod grid;
pub mod meta;
pub mod mvt;
pub mod raster;
pub mod wmts;

use axum::routing::get;
use axum::Router;
use std::collections::HashMap;

#[derive(Default, Clone)]
pub struct AppState {
    pub pool: Option<sqlx::PgPool>,
    pub redis: Option<redis::aio::ConnectionManager>,
    pub cache_dir: String,
    pub wms_url: String,
}

/// app builds the axum router: health + WMTS/XYZ/TMS tile endpoints.
pub fn app(state: AppState) -> Router {
    Router::new()
        .route("/wmts", get(wmts::wmts_kvp))
        .route("/wmts/{layer}/{tms}/{z}/{y}/{xext}", get(wmts::wmts_rest))
        .route("/tiles/{layer}/{z}/{x}/{yext}", get(wmts::xyz))
        .route(
            "/gwc/service/tms/1.0.0/{layer}/{z}/{x}/{yext}",
            get(wmts::tms),
        )
        .with_state(state)
        .merge(geo_core::health::router(HashMap::new()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use tower::ServiceExt;

    #[tokio::test]
    async fn healthz() {
        let app = app(AppState::default());
        let res = app
            .oneshot(Request::get("/healthz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::OK);
    }
}
