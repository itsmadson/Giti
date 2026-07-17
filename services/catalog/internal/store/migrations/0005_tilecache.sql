-- E5: GeoWebCache-equivalent configuration (gridsets, blobstores, quota, per-layer).
CREATE TABLE IF NOT EXISTS gridsets (
    name text PRIMARY KEY,
    srs text NOT NULL DEFAULT 'EPSG:3857',
    extent jsonb NOT NULL DEFAULT '[]',
    tile_size int NOT NULL DEFAULT 256,
    levels int NOT NULL DEFAULT 22
);

CREATE TABLE IF NOT EXISTS blobstores (
    name text PRIMARY KEY,
    type text NOT NULL DEFAULT 'file', -- file | s3
    config jsonb NOT NULL DEFAULT '{}',
    is_default boolean NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS layer_cache (
    workspace text NOT NULL,
    layer text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    metatile_x int NOT NULL DEFAULT 4,
    metatile_y int NOT NULL DEFAULT 4,
    gutter int NOT NULL DEFAULT 0,
    formats text[] NOT NULL DEFAULT '{application/vnd.mapbox-vector-tile,image/png}',
    expire_server int NOT NULL DEFAULT 0,
    expire_client int NOT NULL DEFAULT 0,
    gridsets text[] NOT NULL DEFAULT '{EPSG:3857}',
    blobstore text NOT NULL DEFAULT '',
    PRIMARY KEY (workspace, layer)
);

CREATE TABLE IF NOT EXISTS disk_quota (
    id int PRIMARY KEY DEFAULT 1,
    policy text NOT NULL DEFAULT 'LRU',
    max_bytes bigint NOT NULL DEFAULT 0
);

INSERT INTO disk_quota(id, policy, max_bytes) VALUES (1, 'LRU', 0)
    ON CONFLICT (id) DO NOTHING;

INSERT INTO gridsets(name, srs, tile_size, levels) VALUES
    ('EPSG:3857', 'EPSG:3857', 256, 22),
    ('EPSG:4326', 'EPSG:4326', 256, 21)
    ON CONFLICT (name) DO NOTHING;

INSERT INTO blobstores(name, type, is_default) VALUES ('default-file', 'file', true)
    ON CONFLICT (name) DO NOTHING;
