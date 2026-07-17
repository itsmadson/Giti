-- E4: richer layer groups for the admin editor (ordered members + bounds).
ALTER TABLE layer_groups ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '';
ALTER TABLE layer_groups ADD COLUMN IF NOT EXISTS abstract text NOT NULL DEFAULT '';
ALTER TABLE layer_groups ADD COLUMN IF NOT EXISTS srs text NOT NULL DEFAULT 'EPSG:4326';
ALTER TABLE layer_groups ADD COLUMN IF NOT EXISTS bounds jsonb NOT NULL DEFAULT '[]';
ALTER TABLE layer_groups ADD COLUMN IF NOT EXISTS members jsonb NOT NULL DEFAULT '[]';
