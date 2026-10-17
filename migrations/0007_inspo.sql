-- Inspiration board (#49): photos of other people's outfits, tagged so
-- they add up to a style profile the outfit suggestions use.
--
-- Unlike items, every tag may be empty — an untagged fit just doesn't
-- count toward that part of the profile. formality 0-0 means unset.
CREATE TABLE inspo_fits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    photo_path TEXT NOT NULL,
    thumb_path TEXT NOT NULL,
    styles TEXT NOT NULL DEFAULT '[]',
    colors TEXT NOT NULL DEFAULT '[]',
    pieces TEXT NOT NULL DEFAULT '[]',
    fit TEXT NOT NULL DEFAULT '',
    formality_min INTEGER NOT NULL DEFAULT 0,
    formality_max INTEGER NOT NULL DEFAULT 0,
    seasons TEXT NOT NULL DEFAULT '[]',
    occasions TEXT NOT NULL DEFAULT '[]',
    custom_tags TEXT NOT NULL DEFAULT '[]',
    ai_tagged_at TEXT,
    user_edited_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_inspo_fits_user_id ON inspo_fits(user_id);

-- use_inspo: whether suggestions use the style profile (on by default; it
-- only kicks in once there are enough tagged fits). style_summary is the
-- AI's one-line description of the profile, cached until the profile
-- changes — style_summary_key identifies the profile it was written for.
ALTER TABLE users ADD COLUMN use_inspo INTEGER NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN style_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN style_summary_key TEXT NOT NULL DEFAULT '';
