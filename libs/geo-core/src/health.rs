//! Geoson project-wide health convention:
//! GET /healthz -> 200 "ok" (liveness, never checks dependencies)
//! GET /readyz  -> 200/503 JSON {"status": ..., "checks": {...}} (readiness)

use axum::http::StatusCode;
use axum::response::IntoResponse;
use axum::routing::get;
use axum::{Json, Router};
use futures::future::BoxFuture;
use std::collections::HashMap;
use std::sync::Arc;

pub type Check = Arc<dyn Fn() -> BoxFuture<'static, Result<(), String>> + Send + Sync>;

pub fn router(checks: HashMap<String, Check>) -> Router {
    let checks = Arc::new(checks);
    Router::new()
        .route("/healthz", get(|| async { "ok" }))
        .route(
            "/readyz",
            get(move || {
                let checks = checks.clone();
                async move {
                    let mut results = serde_json::Map::new();
                    let mut ready = true;
                    for (name, check) in checks.iter() {
                        match check().await {
                            Ok(()) => {
                                results.insert(name.clone(), "ok".into());
                            }
                            Err(msg) => {
                                results.insert(name.clone(), msg.into());
                                ready = false;
                            }
                        }
                    }
                    let status = if ready { "ready" } else { "unready" };
                    let code = if ready {
                        StatusCode::OK
                    } else {
                        StatusCode::SERVICE_UNAVAILABLE
                    };
                    (
                        code,
                        Json(serde_json::json!({"status": status, "checks": results})),
                    )
                        .into_response()
                }
            }),
        )
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::{Request, StatusCode};
    use http_body_util::BodyExt;
    use std::collections::HashMap;
    use std::sync::Arc;
    use tower::ServiceExt;

    fn ok_check() -> Check {
        Arc::new(|| Box::pin(async { Ok(()) }))
    }

    fn failing_check(msg: &'static str) -> Check {
        Arc::new(move || Box::pin(async move { Err(msg.to_string()) }))
    }

    #[tokio::test]
    async fn healthz_always_ok() {
        let app = router(HashMap::from([(
            "broken".to_string(),
            failing_check("down"),
        )]));
        let res = app
            .oneshot(Request::get("/healthz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::OK);
        let body = res.into_body().collect().await.unwrap().to_bytes();
        assert_eq!(&body[..], b"ok");
    }

    #[tokio::test]
    async fn readyz_ready_when_checks_pass() {
        let app = router(HashMap::from([("redis".to_string(), ok_check())]));
        let res = app
            .oneshot(Request::get("/readyz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::OK);
        let body = res.into_body().collect().await.unwrap().to_bytes();
        let v: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(v["status"], "ready");
        assert_eq!(v["checks"]["redis"], "ok");
    }

    #[tokio::test]
    async fn readyz_unready_when_check_fails() {
        let app = router(HashMap::from([(
            "postgres".to_string(),
            failing_check("conn refused"),
        )]));
        let res = app
            .oneshot(Request::get("/readyz").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(res.status(), StatusCode::SERVICE_UNAVAILABLE);
        let body = res.into_body().collect().await.unwrap().to_bytes();
        let v: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(v["status"], "unready");
        assert_eq!(v["checks"]["postgres"], "conn refused");
    }
}
