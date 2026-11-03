-- Styles on items: up to 3 of the inspo style vocabulary (minimal, classic,
-- streetwear…), so the wardrobe, what's worn and the inspo board can be
-- compared on one scale (the style radar).
--
-- styles_ai_at records that the AI has looked at the item's styles: set by
-- full AI tagging of a new item, and by the one-time backfill that tags the
-- styles of items uploaded before this migration. The backfill queues only
-- items where it is NULL, so it runs once per item, not on every start.
ALTER TABLE items ADD COLUMN styles TEXT NOT NULL DEFAULT '[]';
ALTER TABLE items ADD COLUMN styles_ai_at TEXT;
