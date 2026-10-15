-- Formality and warmth become per-item ranges (#33): a shirt can be
-- formality 2-4 and warmth 1-5 (worn layered in winter).
--
-- Existing formality keeps exactly its current value. Existing warmth is
-- spread to the span the outfit rules before this migration already
-- allowed for its category, so nobody's suggestions change:
--   top:                    worn layered on any colder day, up to one step warmer
--   outerwear:              one step either side
--   bottom, dress, shoes:   one step warmer to two steps cooler
--   anything else:          any weather (bags, accessories, untagged)
-- SQLite evaluates every SET expression against the row's old values, so
-- warmth_min below still reads the pre-update warmth.
ALTER TABLE items RENAME COLUMN formality TO formality_min;
ALTER TABLE items ADD COLUMN formality_max INTEGER NOT NULL DEFAULT 3;
ALTER TABLE items RENAME COLUMN warmth TO warmth_min;
ALTER TABLE items ADD COLUMN warmth_max INTEGER NOT NULL DEFAULT 3;

UPDATE items SET
    formality_max = formality_min,
    warmth_min = CASE
        WHEN category IN ('top', 'outerwear', 'bottom', 'dress', 'shoes') THEN MAX(1, warmth_min - 1)
        ELSE 1
    END,
    warmth_max = CASE
        WHEN category = 'top' THEN 5
        WHEN category = 'outerwear' THEN MIN(5, warmth_min + 1)
        WHEN category IN ('bottom', 'dress', 'shoes') THEN MIN(5, warmth_min + 2)
        ELSE 5
    END;
