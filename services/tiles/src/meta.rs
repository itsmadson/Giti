//! Read-only catalog access for tile layer metadata (sqlx).

use sqlx::Row;

#[derive(Debug, Clone)]
pub struct LayerMeta {
    pub workspace: String,
    pub name: String,
    pub table: String,
    pub geom_col: String,
    pub srs: String,
}

pub async fn resolve(pool: &sqlx::PgPool, ws: &str, name: &str) -> Result<LayerMeta, String> {
    let row = sqlx::query(
        r#"SELECT r.native_name, r.srs
           FROM resources r
           WHERE r.workspace=$1 AND r.name=$2 AND r.kind='featuretype'"#,
    )
    .bind(ws)
    .bind(name)
    .fetch_optional(pool)
    .await
    .map_err(|e| e.to_string())?
    .ok_or_else(|| format!("layer not found: {ws}:{name}"))?;
    let native_name: String = row.get(0);
    let srs: String = row.get(1);

    let geo =
        sqlx::query("SELECT f_geometry_column FROM geometry_columns WHERE f_table_name=$1 LIMIT 1")
            .bind(&native_name)
            .fetch_optional(pool)
            .await
            .map_err(|e| e.to_string())?
            .ok_or_else(|| format!("no geometry column for {native_name}"))?;

    Ok(LayerMeta {
        workspace: ws.to_string(),
        name: name.to_string(),
        table: native_name,
        geom_col: geo.get(0),
        srs,
    })
}

pub async fn list_layers(pool: &sqlx::PgPool) -> Result<Vec<LayerMeta>, String> {
    let rows = sqlx::query(
        r#"SELECT l.workspace, l.name, r.native_name, r.srs
           FROM layers l
           JOIN resources r ON r.workspace=l.workspace AND r.name=l.resource_name AND r.kind='featuretype'
           WHERE l.type='VECTOR' AND l.enabled
           ORDER BY l.workspace, l.name"#,
    )
    .fetch_all(pool)
    .await
    .map_err(|e| e.to_string())?;
    Ok(rows
        .iter()
        .map(|r| LayerMeta {
            workspace: r.get(0),
            name: r.get(1),
            table: r.get(2),
            geom_col: String::new(),
            srs: r.get(3),
        })
        .collect())
}
