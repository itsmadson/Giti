//! Geoson WMS renderer binary.

#[tokio::main]
async fn main() {
    let addr = std::env::var("GEOSON_HTTP_ADDR").unwrap_or_else(|_| ":8080".into());
    let addr = if let Some(rest) = addr.strip_prefix(':') {
        format!("0.0.0.0:{rest}")
    } else {
        addr
    };

    let app = match std::env::var("GEOSON_DATABASE_URL") {
        Ok(dsn) if !dsn.is_empty() => {
            let pool = sqlx::postgres::PgPoolOptions::new()
                .max_connections(10)
                .connect(&dsn)
                .await
                .expect("connect postgres");
            wms::app(pool)
        }
        _ => {
            // No DB configured: health-only.
            use std::collections::HashMap;
            geo_core::health::router(HashMap::new())
        }
    };

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
