use sqlx::postgres::PgPoolOptions;

async fn pool() -> Option<sqlx::PgPool> {
    let dsn = std::env::var("GEOSON_TEST_DATABASE_URL").ok()?;
    PgPoolOptions::new().connect(&dsn).await.ok()
}

async fn seed(pool: &sqlx::PgPool) {
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
