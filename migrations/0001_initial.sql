PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    system_name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    treasury INTEGER NOT NULL DEFAULT 0 CHECK (treasury >= 0),
    archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS characters (
    id TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    level INTEGER NOT NULL DEFAULT 0 CHECK (level >= 0),
    experience INTEGER NOT NULL DEFAULT 0 CHECK (experience >= 0),
    health INTEGER NOT NULL DEFAULT 0 CHECK (health >= 0),
    movement INTEGER NOT NULL DEFAULT 0 CHECK (movement >= 0),
    armor INTEGER NOT NULL DEFAULT 0 CHECK (armor >= 0),
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_characters_campaign_id ON characters(campaign_id);

CREATE TABLE IF NOT EXISTS equipment (
    id TEXT PRIMARY KEY,
    character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity >= 0),
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_equipment_character_id ON equipment(character_id);

CREATE TABLE IF NOT EXISTS traits (
    id TEXT PRIMARY KEY,
    character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_traits_character_id ON traits(character_id);

CREATE TABLE IF NOT EXISTS injuries (
    id TEXT PRIMARY KEY,
    character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    recovered INTEGER NOT NULL DEFAULT 0 CHECK (recovered IN (0, 1)),
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_injuries_character_id ON injuries(character_id);

CREATE TABLE IF NOT EXISTS custom_fields (
    character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (character_id, key)
);
