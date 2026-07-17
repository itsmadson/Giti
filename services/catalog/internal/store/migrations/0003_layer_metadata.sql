-- E2: richer layer/featuretype metadata for the admin editor.
ALTER TABLE resources ADD COLUMN IF NOT EXISTS abstract text NOT NULL DEFAULT '';
ALTER TABLE resources ADD COLUMN IF NOT EXISTS keywords text[] NOT NULL DEFAULT '{}';
ALTER TABLE resources ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}';
ALTER TABLE resources ADD COLUMN IF NOT EXISTS declared_srs text NOT NULL DEFAULT '';
ALTER TABLE resources ADD COLUMN IF NOT EXISTS srs_handling text NOT NULL DEFAULT 'FORCE';
ALTER TABLE resources ADD COLUMN IF NOT EXISTS native_sql text NOT NULL DEFAULT '';

ALTER TABLE layers ADD COLUMN IF NOT EXISTS advertised boolean NOT NULL DEFAULT true;
ALTER TABLE layers ADD COLUMN IF NOT EXISTS queryable boolean NOT NULL DEFAULT true;
ALTER TABLE layers ADD COLUMN IF NOT EXISTS opaque boolean NOT NULL DEFAULT false;
ALTER TABLE layers ADD COLUMN IF NOT EXISTS alternate_styles text[] NOT NULL DEFAULT '{}';
