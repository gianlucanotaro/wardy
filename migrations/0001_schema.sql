CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL,
    subcategory TEXT NOT NULL DEFAULT '',
    color_primary TEXT NOT NULL DEFAULT '',
    color_secondary TEXT NOT NULL DEFAULT '',
    pattern TEXT NOT NULL DEFAULT '',
    material TEXT NOT NULL DEFAULT 'unknown',
    formality INTEGER NOT NULL DEFAULT 3,
    warmth INTEGER NOT NULL DEFAULT 3,
    fit TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    seasons TEXT NOT NULL DEFAULT '[]',
    occasions TEXT NOT NULL DEFAULT '[]',
    photo_path TEXT NOT NULL,
    thumb_path TEXT NOT NULL,
    ai_tagged_at TEXT,
    user_edited_at TEXT,
    archived INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_items_user_id ON items(user_id);
CREATE INDEX idx_items_user_archived ON items(user_id, archived);

CREATE TABLE looks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    cover_item_id INTEGER REFERENCES items(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_looks_user_id ON looks(user_id);

CREATE TABLE look_items (
    look_id INTEGER NOT NULL REFERENCES looks(id) ON DELETE CASCADE,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (look_id, item_id)
);

CREATE TABLE wears (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    look_id INTEGER REFERENCES looks(id) ON DELETE SET NULL,
    worn_on TEXT NOT NULL,
    occasion TEXT NOT NULL DEFAULT '',
    weather_summary TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_wears_user_id ON wears(user_id);
CREATE INDEX idx_wears_user_worn_on ON wears(user_id, worn_on);

CREATE TABLE wear_items (
    wear_id INTEGER NOT NULL REFERENCES wears(id) ON DELETE CASCADE,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    PRIMARY KEY (wear_id, item_id)
);

CREATE TABLE jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    run_after TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_jobs_status_run_after ON jobs(status, run_after);

CREATE TABLE usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    model TEXT NOT NULL,
    input_tokens INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    est_cost_usd REAL NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_usage_created_at ON usage(created_at);
