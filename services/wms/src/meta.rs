//! Read-only catalog access for layer metadata (sqlx).

use sqlx::Row;

#[derive(Debug, Clone)]
pub struct LayerMeta {
    pub workspace: String,
    pub name: String,
    pub table: String,
    pub geom_col: String,
    pub geom_type: String,
    pub srs: String,
    pub srid: i32, // native SRID of the geometry column
    pub default_style: String,
    pub columns: Vec<String>,
}

/// resolve looks up ws:name from catalog tables and introspects the data table.
/// v1 supports store host="self" (same DB).
pub async fn resolve(pool: &sqlx::PgPool, ws: &str, name: &str) -> Result<LayerMeta, String> {
    let row = sqlx::query(
        r#"SELECT r.native_name, r.srs, COALESCE(l.default_style, '')
           FROM resources r
           LEFT JOIN layers l ON l.workspace=r.workspace AND l.name=r.name
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
    let default_style: String = row.get(2);

    let geo_row = sqlx::query(
        "SELECT f_geometry_column, type, srid FROM geometry_columns WHERE f_table_name=$1 LIMIT 1",
    )
    .bind(&native_name)
    .fetch_optional(pool)
    .await
    .map_err(|e| e.to_string())?
    .ok_or_else(|| format!("no geometry column for {native_name}"))?;
    let geom_col: String = geo_row.get(0);
    let geom_type: String = geo_row.get(1);
    let mut srid: i32 = geo_row.get(2);
    if srid == 0 {
        srid = 4326; // geometry_columns reports 0 for unconstrained typmod
    }

    let col_rows = sqlx::query(
        "SELECT column_name FROM information_schema.columns
         WHERE table_name=$1 AND column_name <> $2 ORDER BY ordinal_position",
    )
    .bind(&native_name)
    .bind(&geom_col)
    .fetch_all(pool)
    .await
    .map_err(|e| e.to_string())?;
    let columns = col_rows.iter().map(|r| r.get::<String, _>(0)).collect();

    Ok(LayerMeta {
        workspace: ws.to_string(),
        name: name.to_string(),
        table: native_name,
        geom_col,
        geom_type,
        srs,
        srid,
        default_style,
        columns,
    })
}

/// list_layers returns all vector layers for GetCapabilities.
pub async fn list_layers(pool: &sqlx::PgPool) -> Result<Vec<LayerMeta>, String> {
    let rows = sqlx::query(
        r#"SELECT l.workspace, l.name, r.native_name, r.srs, l.default_style
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
            geom_type: String::new(),
            srs: r.get(3),
            srid: 4326,
            default_style: r.get(4),
            columns: Vec::new(),
        })
        .collect())
}

/// style_body returns the SLD body for a style (workspace style shadows global).
pub async fn style_body(pool: &sqlx::PgPool, ws: &str, style: &str) -> Result<String, String> {
    let row = sqlx::query(
        "SELECT body FROM styles WHERE (workspace=$1 OR workspace='') AND name=$2
         ORDER BY workspace DESC LIMIT 1",
    )
    .bind(ws)
    .bind(style)
    .fetch_optional(pool)
    .await
    .map_err(|e| e.to_string())?
    .ok_or_else(|| format!("style not found: {style}"))?;
    Ok(row.get(0))
}
