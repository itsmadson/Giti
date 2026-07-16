//! Geoson WMS render service (library crate). `main.rs` is the thin binary.

pub mod encode;
pub mod meta;
pub mod ows;
pub mod render;
pub mod sld;

use axum::extract::State;
use axum::http::Uri;
use axum::response::Response;
use axum::routing::get;
use axum::Router;
use std::collections::HashMap;
use std::sync::Arc;

pub struct AppState {
    pub pool: Option<sqlx::PgPool>,
}

/// app builds the axum router: health endpoints + /wms.
pub fn app(pool: sqlx::PgPool) -> Router {
    let state = Arc::new(AppState { pool: Some(pool) });
    let health = geo_core::health::router(HashMap::new());
    Router::new()
        .route("/wms", get(wms_stub))
        .with_state(state)
        .merge(health)
}

async fn wms_stub(State(_state): State<Arc<AppState>>, uri: Uri) -> Response {
    let kvp = ows::Kvp::parse(uri.query().unwrap_or(""));
    let version = ows::negotiate_wms(&kvp.version());
    // Handlers land in Task 7; until then report not-implemented as an exception.
    ows::exception_response(
        &version,
        "OperationNotSupported",
        "request",
        "WMS handlers pending",
    )
}
