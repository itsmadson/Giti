CREATE TABLE IF NOT EXISTS geoson_auth_migrations (
    version int PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth_users (
    name text PRIMARY KEY,
    enabled boolean NOT NULL DEFAULT true,
    password_hash text NOT NULL
);

CREATE TABLE auth_groups (
    name text PRIMARY KEY
);

CREATE TABLE auth_user_groups (
    username text NOT NULL REFERENCES auth_users(name) ON DELETE CASCADE,
    groupname text NOT NULL REFERENCES auth_groups(name) ON DELETE CASCADE,
    PRIMARY KEY (username, groupname)
);

CREATE TABLE auth_roles (
    name text PRIMARY KEY
);

CREATE TABLE auth_role_users (
    rolename text NOT NULL REFERENCES auth_roles(name) ON DELETE CASCADE,
    username text NOT NULL REFERENCES auth_users(name) ON DELETE CASCADE,
    PRIMARY KEY (rolename, username)
);

CREATE TABLE auth_role_groups (
    rolename text NOT NULL REFERENCES auth_roles(name) ON DELETE CASCADE,
    groupname text NOT NULL REFERENCES auth_groups(name) ON DELETE CASCADE,
    PRIMARY KEY (rolename, groupname)
);

CREATE TABLE geofence_rules (
    id bigserial PRIMARY KEY,
    priority bigint NOT NULL,
    username text NOT NULL DEFAULT '*',
    rolename text NOT NULL DEFAULT '*',
    service text NOT NULL DEFAULT '*',
    request text NOT NULL DEFAULT '*',
    workspace text NOT NULL DEFAULT '*',
    layer text NOT NULL DEFAULT '*',
    access text NOT NULL CHECK (access IN ('ALLOW','DENY','LIMIT')),
    cql_read text NOT NULL DEFAULT '',
    cql_write text NOT NULL DEFAULT '',
    attributes jsonb NOT NULL DEFAULT '[]'
);
CREATE INDEX geofence_rules_priority ON geofence_rules(priority);
