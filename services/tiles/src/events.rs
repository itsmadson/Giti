//! NATS-driven tile cache invalidation: catalog layer changes bump the tile
//! cache generation for the affected layer.

use crate::cache::Cache;
use futures::StreamExt;

/// layer_key extracts "workspace:name" from a catalog event JSON payload.
pub fn layer_key(payload: &[u8]) -> Option<String> {
    let v: serde_json::Value = serde_json::from_slice(payload).ok()?;
    let name = v.get("name")?.as_str()?;
    let ws = v.get("workspace").and_then(|w| w.as_str()).unwrap_or("");
    if ws.is_empty() {
        Some(name.to_string())
    } else {
        Some(format!("{ws}:{name}"))
    }
}

/// subscribe_invalidations subscribes to catalog layer events and bumps the
/// tile cache generation for each changed layer. Runs until the process exits.
pub async fn subscribe_invalidations(
    nats_url: &str,
    cache_dir: String,
    redis: redis::aio::ConnectionManager,
) -> Result<(), String> {
    let client = async_nats::connect(nats_url)
        .await
        .map_err(|e| e.to_string())?;
    let mut sub = client
        .subscribe("catalog.layer.*")
        .await
        .map_err(|e| e.to_string())?;
    let mut sub_ft = client
        .subscribe("catalog.featuretype.*")
        .await
        .map_err(|e| e.to_string())?;
    let mut cache = Cache::new(cache_dir, Some(redis));
    loop {
        let msg = tokio::select! {
            Some(m) = sub.next() => m,
            Some(m) = sub_ft.next() => m,
            else => break,
        };
        if let Some(layer) = layer_key(&msg.payload) {
            cache.bump_generation(&layer).await;
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn layer_key_from_event() {
        let payload = br#"{"name":"roads","workspace":"topp"}"#;
        assert_eq!(layer_key(payload), Some("topp:roads".to_string()));
        assert_eq!(layer_key(b"not json"), None);
    }
}
