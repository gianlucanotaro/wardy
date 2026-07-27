CREATE TABLE IF NOT EXISTS users (
    id           INTEGER PRIMARY KEY,
    email        TEXT    NOT NULL UNIQUE,
    password_hash TEXT   NOT NULL,
    display_name TEXT    NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS items (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT    NOT NULL DEFAULT '',
    category        TEXT    NOT NULL,
    subcategory     TEXT    NOT NULL DEFAULT '',
    color_primary   TEXT    NOT NULL DEFAULT '',
    color_secondary TEXT    NOT NULL DEFAULT '',
    pattern         TEXT    NOT NULL DEFAULT 'solid',
    material        TEXT    NOT NULL DEFAULT 'unknown',
    formality       INTEGER NOT NULL DEFAULT 3,
    warmth          INTEGER NOT NULL DEFAULT 3,
    fit             TEXT    NOT NULL DEFAULT 'regular',
    brand           TEXT    NOT NULL DEFAULT '',
    notes           TEXT    NOT NULL DEFAULT '',
    seasons         TEXT    NOT NULL DEFAULT '[]',
    occasions       TEXT    NOT NULL DEFAULT '[]',
    photo_path      TEXT    NOT NULL DEFAULT '',
    thumb_path      TEXT    NOT NULL DEFAULT '',
    ai_tagged_at    DATETIME,
    user_edited_at  DATETIME,
    archived        INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS looks (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT    NOT NULL DEFAULT '',
    notes        TEXT    NOT NULL DEFAULT '',
    cover_item_id INTEGER REFERENCES items(id) ON DELETE SET NULL,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS look_items (
    look_id  INTEGER NOT NULL REFERENCES looks(id) ON DELETE CASCADE,
    item_id  INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (look_id, item_id)
);

CREATE TABLE IF NOT EXISTS wears (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    look_id         INTEGER REFERENCES looks(id) ON DELETE SET NULL,
    worn_on         DATE    NOT NULL,
    occasion        TEXT    NOT NULL DEFAULT '',
    weather_summary TEXT    NOT NULL DEFAULT '',
    notes           TEXT    NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS wear_items (
    wear_id INTEGER NOT NULL REFERENCES wears(id) ON DELETE CASCADE,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    PRIMARY KEY (wear_id, item_id)
);

CREATE TABLE IF NOT EXISTS jobs (
    id         INTEGER PRIMARY KEY,
    type       TEXT    NOT NULL,
    payload    TEXT    NOT NULL DEFAULT '{}',
    status     TEXT    NOT NULL DEFAULT 'pending',
    attempts   INTEGER NOT NULL DEFAULT 0,
    last_error TEXT    NOT NULL DEFAULT '',
    run_after  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS usage (
    id                INTEGER PRIMARY KEY,
    model             TEXT    NOT NULL,
    input_tokens      INTEGER NOT NULL DEFAULT 0,
    output_tokens     INTEGER NOT NULL DEFAULT 0,
    estimated_cost_usd REAL   NOT NULL DEFAULT 0,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_items_user_id       ON items(user_id);
CREATE INDEX IF NOT EXISTS idx_items_user_archived  ON items(user_id, archived);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status         ON jobs(status, run_after);
CREATE INDEX IF NOT EXISTS idx_wears_user_worn_on  ON wears(user_id, worn_on);
