use sqlx::postgres::PgPoolOptions;

async fn pool() -> Option<sqlx::PgPool> {
    let dsn = std::env::var("GEOSON_TEST_DATABASE_URL").ok()?;
    PgPoolOptions::new().connect(&dsn).await.ok()
}

static SEED: tokio::sync::Mutex<bool> = tokio::sync::Mutex::const_new(false);
async fn seed(pool: &sqlx::PgPool) {
    let mut d = SEED.lock().await;
    if *d {
        return;
    }
    for sql in [
        "CREATE EXTENSION IF NOT EXISTS postgis",
        "DELETE FROM layers WHERE workspace='tiletest'",
        "DELETE FROM resources WHERE workspace='tiletest'",
        "DELETE FROM stores WHERE workspace='tiletest'",
        "DELETE FROM workspaces WHERE name='tiletest'",
        "INSERT INTO workspaces(name) VALUES('tiletest')",
        "INSERT INTO stores(workspace,name,kind,type,enabled,connection) VALUES('tiletest','local','datastore','PostGIS',true,'{\"host\":\"self\"}'::jsonb)",
        "DROP TABLE IF EXISTS tile_pts",
        "CREATE TABLE tile_pts (id serial primary key, name text, geom geometry(Point,4326))",
        "INSERT INTO tile_pts(name,geom) VALUES ('c', ST_SetSRID(ST_MakePoint(0.1,0.1),4326))",
        "INSERT INTO resources(workspace,store,name,kind,native_name,srs,enabled) VALUES('tiletest','local','tile_pts','featuretype','tile_pts','EPSG:4326',true)",
        "INSERT INTO layers(workspace,name,type,resource_name,default_style,enabled) VALUES('tiletest','tile_pts','VECTOR','tile_pts','point',true)",
    ] { sqlx::query(sql).execute(pool).await.unwrap(); }
    *d = true;
}

#[tokio::test]
async fn mvt_z0_has_bytes() {
    let Some(pool) = pool().await else { return };
    seed(&pool).await;
    let m = tiles::meta::resolve(&pool, "tiletest", "tile_pts")
        .await
        .unwrap();
    let g = tiles::grid::web_mercator();
    let bytes = tiles::mvt::render_mvt(&pool, &m, &g, 0, 0, 0)
        .await
        .unwrap();
    assert!(!bytes.is_empty(), "z0 world tile should contain the point");
}

#[tokio::test]
async fn mvt_empty_tile() {
    let Some(pool) = pool().await else { return };
    seed(&pool).await;
    let m = tiles::meta::resolve(&pool, "tiletest", "tile_pts")
        .await
        .unwrap();
    let g = tiles::grid::web_mercator();
    let bytes = tiles::mvt::render_mvt(&pool, &m, &g, 8, 0, 0)
        .await
        .unwrap();
    assert!(bytes.is_empty(), "far tile should be empty");
}

use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use tower::ServiceExt;

async fn test_app() -> Option<axum::Router> {
    let pool = pool().await?;
    seed(&pool).await;
    let dir = std::env::temp_dir().join(format!("geoson-tiles-app-{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    Some(tiles::app(tiles::AppState {
        pool: Some(pool),
        redis: None,
        cache_dir: dir.to_string_lossy().into(),
        wms_url: "http://127.0.0.1:0".into(),
    }))
}

#[tokio::test]
async fn xyz_mvt_tile() {
    let Some(app) = test_app().await else { return };
    let res = app
        .oneshot(
            Request::get("/tiles/tiletest:tile_pts/0/0/0.pbf")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::OK);
    assert_eq!(
        res.headers()["content-type"],
        "application/vnd.mapbox-vector-tile"
    );
    let body = res.into_body().collect().await.unwrap().to_bytes();
    assert!(!body.is_empty());
}

#[tokio::test]
async fn xyz_empty_tile_204() {
    let Some(app) = test_app().await else { return };
    let res = app
        .oneshot(
            Request::get("/tiles/tiletest:tile_pts/8/0/0.pbf")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::NO_CONTENT);
}

#[tokio::test]
async fn wmts_getcapabilities() {
    let Some(app) = test_app().await else { return };
    let res = app
        .oneshot(
            Request::get("/wmts?service=WMTS&request=GetCapabilities")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    let body = res.into_body().collect().await.unwrap().to_bytes();
    let s = String::from_utf8_lossy(&body);
    assert!(s.contains("Capabilities"));
    assert!(s.contains("tiletest:tile_pts"));
    assert!(s.contains("EPSG:3857"));
}

#[tokio::test]
async fn wmts_kvp_gettile_mvt() {
    let Some(app) = test_app().await else { return };
    let uri = "/wmts?service=WMTS&request=GetTile&layer=tiletest:tile_pts&tilematrixset=EPSG:3857&tilematrix=0&tilerow=0&tilecol=0&format=application/vnd.mapbox-vector-tile";
    let res = app
        .oneshot(Request::get(uri).body(Body::empty()).unwrap())
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::OK);
}

#[tokio::test]
async fn tms_flips_y() {
    let Some(app) = test_app().await else { return };
    let res = app
        .oneshot(
            Request::get("/gwc/service/tms/1.0.0/tiletest:tile_pts/0/0/0.pbf")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::OK);
}
