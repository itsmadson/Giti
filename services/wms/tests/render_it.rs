use sqlx::postgres::PgPoolOptions;

async fn pool() -> Option<sqlx::PgPool> {
    let dsn = std::env::var("GEOSON_TEST_DATABASE_URL").ok()?;
    PgPoolOptions::new().connect(&dsn).await.ok()
}

// Serialize seeding so parallel tests don't race on the shared fixture.
static SEED_LOCK: tokio::sync::Mutex<bool> = tokio::sync::Mutex::const_new(false);

async fn seed(pool: &sqlx::PgPool) {
    let mut done = SEED_LOCK.lock().await;
    if *done {
        return;
    }
    seed_inner(pool).await;
    *done = true;
}

async fn seed_inner(pool: &sqlx::PgPool) {
    for sql in [
        "CREATE EXTENSION IF NOT EXISTS postgis",
        "DELETE FROM layers WHERE workspace='wmstest'",
        "DELETE FROM resources WHERE workspace='wmstest'",
        "DELETE FROM stores WHERE workspace='wmstest'",
        "DELETE FROM workspaces WHERE name='wmstest'",
        "INSERT INTO workspaces(name) VALUES('wmstest')",
        "INSERT INTO stores(workspace,name,kind,type,enabled,connection) VALUES('wmstest','local','datastore','PostGIS',true,'{\"host\":\"self\"}'::jsonb)",
        "DROP TABLE IF EXISTS wms_poly",
        "CREATE TABLE wms_poly (id serial primary key, name text, geom geometry(Polygon,4326))",
        "INSERT INTO wms_poly(name,geom) VALUES ('a', ST_GeomFromText('POLYGON((0 0,0 10,10 10,10 0,0 0))',4326)),('b', ST_GeomFromText('POLYGON((20 20,20 30,30 30,30 20,20 20))',4326))",
        "INSERT INTO resources(workspace,store,name,kind,native_name,srs,enabled) VALUES('wmstest','local','wms_poly','featuretype','wms_poly','EPSG:4326',true)",
        "INSERT INTO layers(workspace,name,type,resource_name,default_style,enabled) VALUES('wmstest','wms_poly','VECTOR','wms_poly','polygon',true)",
    ] {
        sqlx::query(sql).execute(pool).await.unwrap();
    }
}

#[tokio::test]
async fn resolve_layer() {
    let Some(pool) = pool().await else { return };
    seed(&pool).await;
    let m = wms::meta::resolve(&pool, "wmstest", "wms_poly")
        .await
        .unwrap();
    assert_eq!(m.table, "wms_poly");
    assert_eq!(m.geom_col, "geom");
    assert_eq!(m.default_style, "polygon");
    assert!(m.columns.contains(&"name".to_string()));
}

#[tokio::test]
async fn render_polygon_png() {
    let Some(pool) = pool().await else { return };
    seed(&pool).await;
    let m = wms::meta::resolve(&pool, "wmstest", "wms_poly")
        .await
        .unwrap();
    let style = wms::sld::default_style_for("Polygon");
    let req = wms::render::MapRequest {
        layer: m,
        style,
        bbox: [0.0, 0.0, 40.0, 40.0],
        width: 256,
        height: 256,
        transparent: false,
        bgcolor: [255, 255, 255, 255],
        cql: None,
    };
    let px = wms::render::render_map(&pool, &req).await.unwrap();
    assert_eq!(px.width(), 256);
    let png = wms::encode::encode_png(&px).unwrap();
    assert!(png.starts_with(&[0x89, b'P', b'N', b'G']), "not a png");
    let bg = tiny_skia::PremultipliedColorU8::from_rgba(255, 255, 255, 255).unwrap();
    assert!(px.pixels().iter().any(|p| *p != bg), "nothing rendered");
}

#[tokio::test]
async fn render_respects_cql() {
    let Some(pool) = pool().await else { return };
    seed(&pool).await;
    let m = wms::meta::resolve(&pool, "wmstest", "wms_poly")
        .await
        .unwrap();
    let feats = wms::render::vector::fetch_features(
        &pool,
        &wms::render::MapRequest {
            layer: m,
            style: Default::default(),
            bbox: [-1.0, -1.0, 15.0, 15.0],
            width: 256,
            height: 256,
            transparent: true,
            bgcolor: [0, 0, 0, 0],
            cql: Some(geo_core::filter::parse_cql("name = 'a'").unwrap()),
        },
    )
    .await
    .unwrap();
    assert_eq!(feats.len(), 1);
}

use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use tower::ServiceExt;

async fn test_app() -> Option<axum::Router> {
    let pool = pool().await?;
    seed(&pool).await;
    Some(wms::app(pool))
}

#[tokio::test]
async fn getmap_png() {
    let Some(app) = test_app().await else { return };
    let uri = "/wms?service=WMS&version=1.3.0&request=GetMap&layers=wmstest:wms_poly&styles=&crs=EPSG:4326&bbox=0,0,40,40&width=200&height=200&format=image/png";
    let res = app
        .oneshot(Request::get(uri).body(Body::empty()).unwrap())
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::OK);
    assert_eq!(res.headers()["content-type"], "image/png");
    let body = res.into_body().collect().await.unwrap().to_bytes();
    assert!(body.starts_with(&[0x89, b'P', b'N', b'G']));
}

#[tokio::test]
async fn getcapabilities() {
    let Some(app) = test_app().await else { return };
    let res = app
        .oneshot(
            Request::get("/wms?service=WMS&version=1.3.0&request=GetCapabilities")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    let body = res.into_body().collect().await.unwrap().to_bytes();
    let s = String::from_utf8_lossy(&body);
    assert!(s.contains("WMS_Capabilities"), "{}", &s[..s.len().min(400)]);
    assert!(s.contains("wmstest:wms_poly"));
}

#[tokio::test]
async fn getfeatureinfo_json() {
    let Some(app) = test_app().await else { return };
    let uri = "/wms?service=WMS&version=1.3.0&request=GetFeatureInfo&layers=wmstest:wms_poly&query_layers=wmstest:wms_poly&crs=EPSG:4326&bbox=0,0,10,10&width=100&height=100&i=50&j=50&info_format=application/json";
    let res = app
        .oneshot(Request::get(uri).body(Body::empty()).unwrap())
        .await
        .unwrap();
    let body = res.into_body().collect().await.unwrap().to_bytes();
    assert!(String::from_utf8_lossy(&body).contains("FeatureCollection"));
}

#[tokio::test]
async fn getmap_missing_layer_exception() {
    let Some(app) = test_app().await else { return };
    let uri = "/wms?service=WMS&version=1.3.0&request=GetMap&layers=wmstest:ghost&crs=EPSG:4326&bbox=0,0,1,1&width=10&height=10&format=image/png";
    let res = app
        .oneshot(Request::get(uri).body(Body::empty()).unwrap())
        .await
        .unwrap();
    let body = res.into_body().collect().await.unwrap().to_bytes();
    assert!(String::from_utf8_lossy(&body).contains("ServiceExceptionReport"));
}
