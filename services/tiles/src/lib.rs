//! Geoson tiles service (WMTS/XYZ/TMS, MVT + raster cache).

pub mod grid;

use axum::Router;
use std::collections::HashMap;

#[derive(Default, Clone)]
pub struct AppState {
    pub pool: Option<sqlx::PgPool>,
    pub redis: Option<redis::aio::ConnectionManager>,
    pub cache_dir: String,
    pub wms_url: String,
}

/// app builds the axum router. Health is always present; tile endpoints are
/// mounted in later tasks.
pub fn app(_state: AppState) -> Router {
    Router::new().merge(geo_core::health::router(HashMap::new()))
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
