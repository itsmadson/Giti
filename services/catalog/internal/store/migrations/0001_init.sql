CREATE TABLE IF NOT EXISTS giti_migrations (
    version int PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspaces (
    name text PRIMARY KEY,
    isolated boolean NOT NULL DEFAULT false,
    namespace_uri text NOT NULL DEFAULT ''
);

CREATE TABLE stores (
    workspace text NOT NULL REFERENCES workspaces(name) ON DELETE CASCADE,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('datastore','coveragestore')),
    type text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    description text NOT NULL DEFAULT '',
    connection jsonb NOT NULL DEFAULT '{}',
    PRIMARY KEY (workspace, name)
);

CREATE TABLE resources (
    workspace text NOT NULL,
    store text NOT NULL,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('featuretype','coverage')),
    native_name text NOT NULL,
    title text NOT NULL DEFAULT '',
    srs text NOT NULL DEFAULT 'EPSG:4326',
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (workspace, store, name),
    FOREIGN KEY (workspace, store) REFERENCES stores(workspace, name) ON DELETE CASCADE
);

CREATE TABLE layers (
    workspace text NOT NULL REFERENCES workspaces(name) ON DELETE CASCADE,
    name text NOT NULL,
    type text NOT NULL CHECK (type IN ('VECTOR','RASTER')),
    resource_name text NOT NULL,
    default_style text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (workspace, name)
);

CREATE TABLE styles (
    workspace text NOT NULL DEFAULT '',
    name text NOT NULL,
    format text NOT NULL DEFAULT 'sld',
    filename text NOT NULL DEFAULT '',
    body text NOT NULL DEFAULT '',
    PRIMARY KEY (workspace, name)
);

CREATE TABLE layer_groups (
    workspace text NOT NULL DEFAULT '',
    name text NOT NULL,
    mode text NOT NULL DEFAULT 'SINGLE',
    layers jsonb NOT NULL DEFAULT '[]',
    PRIMARY KEY (workspace, name)
);
