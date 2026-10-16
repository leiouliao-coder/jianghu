-- M4: corpses are containers placed in the world with a fixed expiry that
-- counts from death (viewing or going offline never extends it).
ALTER TABLE containers
    ADD COLUMN pos_x double precision,
    ADD COLUMN pos_z double precision,
    ADD COLUMN owner_name text,
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN expires_at timestamptz,
    ADD COLUMN destroyed_at timestamptz,
    ADD CONSTRAINT corpse_has_place CHECK (kind <> 'corpse' OR (pos_x IS NOT NULL AND pos_z IS NOT NULL AND expires_at IS NOT NULL));
CREATE INDEX containers_live_corpses ON containers (expires_at) WHERE kind = 'corpse' AND destroyed_at IS NULL;

-- A character that left the world while dead comes back dead (no second drop).
ALTER TABLE characters ADD COLUMN dead boolean NOT NULL DEFAULT false;
