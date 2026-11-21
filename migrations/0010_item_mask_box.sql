-- Where the garment sits in its photo, measured from its mask: the bounding
-- box as fractions of the photo's width and height, and the photo's
-- width/height, as JSON ({"x":..,"y":..,"w":..,"h":..,"ratio":..}). The app
-- uses it to give every cut-out garment the same size in its tile.
-- '' means not measured (no cut-out, or one made before this column: the
-- server measures those when it starts).
ALTER TABLE items ADD COLUMN mask_box TEXT NOT NULL DEFAULT '';
