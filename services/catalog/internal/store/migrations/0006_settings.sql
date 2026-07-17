-- E7: global settings (contact + service metadata + proxy base) for OWS capabilities.
CREATE TABLE IF NOT EXISTS settings (
    id int PRIMARY KEY DEFAULT 1,
    config jsonb NOT NULL DEFAULT '{}'
);
INSERT INTO settings(id, config) VALUES (1, '{}') ON CONFLICT (id) DO NOTHING;
