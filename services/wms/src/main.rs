//! Geoson WMS renderer. Sprint 1: health endpoints only.

use std::collections::HashMap;

#[tokio::main]
async fn main() {
    let addr = std::env::var("GEOSON_HTTP_ADDR").unwrap_or_else(|_| ":8080".into());
    // Accept ":8080" (Go-style) and "0.0.0.0:8080" forms.
    let addr = if addr.starts_with(':') {
        format!("0.0.0.0{addr}")
    } else {
        addr
    };
    let app = geo_core::health::router(HashMap::new());
    let listener = tokio::net::TcpListener::bind(&addr).await.expect("bind");
    println!("wms listening on {addr}");
    axum::serve(listener, app)
        .with_graceful_shutdown(async {
            let mut term =
                tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
                    .expect("sigterm handler");
            tokio::select! {
                _ = term.recv() => {},
                _ = tokio::signal::ctrl_c() => {},
            }
        })
        .await
        .expect("server");
}
