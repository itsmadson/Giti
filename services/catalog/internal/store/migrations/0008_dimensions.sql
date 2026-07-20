-- WMS TIME/ELEVATION dimensions: which columns carry time/elevation values.
ALTER TABLE resources ADD COLUMN IF NOT EXISTS time_column text NOT NULL DEFAULT '';
ALTER TABLE resources ADD COLUMN IF NOT EXISTS elevation_column text NOT NULL DEFAULT '';
