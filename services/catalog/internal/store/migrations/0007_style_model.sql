-- Store the visual style-builder model (rules) alongside the generated SLD so
-- styles built visually can be reopened and edited.
ALTER TABLE styles ADD COLUMN IF NOT EXISTS model jsonb;
