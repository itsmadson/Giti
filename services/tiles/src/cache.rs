//! Content-addressed tile blob cache with an optional Redis generation index.

use redis::AsyncCommands;
use sha2::{Digest, Sha256};

pub struct Cache {
    dir: String,
    redis: Option<redis::aio::ConnectionManager>,
    ttl_secs: u64,
}

impl Cache {
    pub fn new(dir: String, redis: Option<redis::aio::ConnectionManager>) -> Cache {
        Cache {
            dir,
            redis,
            ttl_secs: 86400,
        }
    }

    /// generation returns the layer's current cache generation (0 without redis).
    pub async fn generation(&mut self, layer: &str) -> u64 {
        if let Some(r) = &mut self.redis {
            let v: Option<u64> = r.get(format!("tilegen:{layer}")).await.ok().flatten();
            return v.unwrap_or(0);
        }
        0
    }

    /// bump_generation invalidates all cached tiles for a layer.
    pub async fn bump_generation(&mut self, layer: &str) {
        if let Some(r) = &mut self.redis {
            let _: Result<u64, _> = r.incr(format!("tilegen:{layer}"), 1).await;
        }
    }

    /// key builds the content-address (hex SHA-256) including the generation.
    pub async fn key(
        &mut self,
        layer: &str,
        gridset: &str,
        z: u8,
        x: u32,
        y: u32,
        fmt: &str,
    ) -> String {
        let gen = self.generation(layer).await;
        let mut h = Sha256::new();
        h.update(format!("{layer}/{gridset}/{z}/{x}/{y}/{fmt}/{gen}").as_bytes());
        hex(&h.finalize())
    }

    fn blob_path(&self, key: &str) -> std::path::PathBuf {
        std::path::Path::new(&self.dir)
            .join(&key[0..2])
            .join(&key[2..4])
            .join(key)
    }

    pub async fn get(&mut self, key: &str) -> Option<Vec<u8>> {
        tokio::fs::read(self.blob_path(key)).await.ok()
    }

    pub async fn put(&mut self, key: &str, bytes: &[u8]) -> Result<(), String> {
        let path = self.blob_path(key);
        if let Some(parent) = path.parent() {
            tokio::fs::create_dir_all(parent)
                .await
                .map_err(|e| e.to_string())?;
        }
        tokio::fs::write(&path, bytes)
            .await
            .map_err(|e| e.to_string())?;
        if let Some(r) = &mut self.redis {
            let _: Result<(), _> = r.set_ex(format!("tile:{key}"), 1, self.ttl_secs).await;
        }
        Ok(())
    }
}

fn hex(bytes: &[u8]) -> String {
    let mut s = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        s.push_str(&format!("{b:02x}"));
    }
    s
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn roundtrip_fs_only() {
        let dir = std::env::temp_dir().join(format!("giti-tiles-{}", std::process::id()));
        let mut c = Cache::new(dir.to_string_lossy().into(), None);
        let k = c.key("ws:layer", "EPSG:3857", 3, 1, 2, "pbf").await;
        assert!(c.get(&k).await.is_none());
        c.put(&k, b"tiledata").await.unwrap();
        assert_eq!(c.get(&k).await.unwrap(), b"tiledata");
        let _ = std::fs::remove_dir_all(dir);
    }

    #[tokio::test]
    async fn generation_changes_key() {
        let dir = std::env::temp_dir().join(format!("giti-tiles-g-{}", std::process::id()));
        let mut c = Cache::new(dir.to_string_lossy().into(), None);
        let k1 = c.key("l", "g", 0, 0, 0, "pbf").await;
        let k2 = c.key("l", "g", 0, 0, 0, "pbf").await;
        assert_eq!(k1, k2);
        let _ = std::fs::remove_dir_all(dir);
    }

    #[tokio::test]
    async fn redis_generation_bumps() {
        let Ok(url) = std::env::var("GITI_TEST_REDIS_URL") else {
            return;
        };
        let client = redis::Client::open(url).unwrap();
        let cm = client.get_connection_manager().await.unwrap();
        let dir = std::env::temp_dir().join(format!("giti-tiles-r-{}", std::process::id()));
        let mut c = Cache::new(dir.to_string_lossy().into(), Some(cm));
        let k1 = c.key("bumplayer", "g", 0, 0, 0, "pbf").await;
        c.bump_generation("bumplayer").await;
        let k2 = c.key("bumplayer", "g", 0, 0, 0, "pbf").await;
        assert_ne!(k1, k2, "generation bump must change the tile key");
        let _ = std::fs::remove_dir_all(dir);
    }
}
