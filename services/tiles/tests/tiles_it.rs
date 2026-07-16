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
