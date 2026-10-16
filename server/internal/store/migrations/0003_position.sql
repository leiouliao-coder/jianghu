-- M3: last position on the ground plane, saved when a character leaves the world.
ALTER TABLE characters
    ADD COLUMN pos_x double precision NOT NULL DEFAULT 0,
    ADD COLUMN pos_z double precision NOT NULL DEFAULT 0;
