//! Giti WMS render service (library crate). `main.rs` is the thin binary.

pub mod encode;
pub mod filter_xml;
pub mod meta;
pub mod ows;
pub mod render;
pub mod sld;
pub mod wms;

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
        .route("/wms", get(wms::wms_endpoint))
        .with_state(state)
        .merge(health)
}
