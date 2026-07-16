//! Giti tiles service binary.

use tiles::AppState;

#[tokio::main]
async fn main() {
    let addr = std::env::var("GITI_HTTP_ADDR").unwrap_or_else(|_| ":8080".into());
    let addr = if let Some(rest) = addr.strip_prefix(':') {
        format!("0.0.0.0:{rest}")
    } else {
        addr
    };

    let mut state = AppState {
        cache_dir: std::env::var("GITI_TILE_CACHE_DIR")
            .unwrap_or_else(|_| "/var/cache/giti/tiles".into()),
        wms_url: std::env::var("GITI_WMS_URL").unwrap_or_else(|_| "http://wms:8080".into()),
        ..Default::default()
    };
    if let Ok(dsn) = std::env::var("GITI_DATABASE_URL") {
        if !dsn.is_empty() {
            let pool = sqlx::postgres::PgPoolOptions::new()
                .max_connections(10)
                .connect(&dsn)
                .await
                .expect("connect postgres");
            state.pool = Some(pool);
        }
    }
    if let Ok(url) = std::env::var("GITI_REDIS_URL") {
        if !url.is_empty() {
            let url = if url.starts_with("redis://") {
                url
            } else {
                format!("redis://{url}")
            };
            if let Ok(client) = redis::Client::open(url) {
                if let Ok(cm) = client.get_connection_manager().await {
                    state.redis = Some(cm);
                }
            }
        }
    }

    // Spawn NATS-driven cache invalidation when both NATS and Redis are present.
    if let (Ok(nats_url), Some(redis)) = (std::env::var("GITI_NATS_URL"), state.redis.clone()) {
        if !nats_url.is_empty() {
            let cache_dir = state.cache_dir.clone();
            tokio::spawn(async move {
                if let Err(e) =
                    tiles::events::subscribe_invalidations(&nats_url, cache_dir, redis).await
                {
                    eprintln!("cache invalidation subscriber stopped: {e}");
                }
            });
        }
    }

    let app = tiles::app(state);
    let listener = tokio::net::TcpListener::bind(&addr).await.expect("bind");
    println!("tiles listening on {addr}");
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
