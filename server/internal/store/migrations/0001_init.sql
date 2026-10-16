-- M1 asset core. Every item stack and every coin lives in exactly one container.

CREATE TABLE accounts (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       text NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 64),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE characters (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id     bigint NOT NULL REFERENCES accounts (id),
    name           text NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 32),
    -- permanent 臂力 in 1/100 units; effective strength adds equipment later
    strength_centi bigint NOT NULL DEFAULT 2000 CHECK (strength_centi >= 1000),
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX characters_account ON characters (account_id);

CREATE TABLE containers (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind         text NOT NULL CHECK (kind IN ('backpack', 'node_storage', 'corpse', 'npc_stock', 'escrow', 'system')),
    account_id   bigint REFERENCES accounts (id),
    character_id bigint REFERENCES characters (id),
    node         text,
    slot_cap     integer CHECK (slot_cap >= 0), -- NULL means no slot limit
    coins        bigint NOT NULL DEFAULT 0 CHECK (coins >= 0),
    CHECK (kind <> 'backpack' OR character_id IS NOT NULL),
    CHECK (kind <> 'node_storage' OR (account_id IS NOT NULL AND node IS NOT NULL))
);
CREATE UNIQUE INDEX containers_one_backpack ON containers (character_id) WHERE kind = 'backpack';
CREATE UNIQUE INDEX containers_one_node_storage ON containers (account_id, node) WHERE kind = 'node_storage';

CREATE TABLE requests (
    request_id     text PRIMARY KEY CHECK (length(request_id) BETWEEN 1 AND 128),
    account_id     bigint REFERENCES accounts (id), -- NULL for system requests
    kind           text NOT NULL,
    payload_sha256 bytea NOT NULL,
    outcome        text NOT NULL CHECK (outcome IN ('ok', 'rejected')),
    result         jsonb NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE items (
    id           text PRIMARY KEY,
    proto        text NOT NULL,
    state        text NOT NULL DEFAULT '',
    qty          bigint NOT NULL CHECK (qty > 0),
    container_id bigint NOT NULL REFERENCES containers (id),
    version      bigint NOT NULL DEFAULT 1,
    created_by   text NOT NULL REFERENCES requests (request_id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX items_container ON items (container_id);

CREATE TABLE events (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id text NOT NULL REFERENCES requests (request_id) DEFERRABLE INITIALLY DEFERRED,
    kind       text NOT NULL,
    data       jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX events_request ON events (request_id);
