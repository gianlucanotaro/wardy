-- Cut-outs: an item may carry a mask (a PNG whose alpha channel is opaque where
-- the garment is) next to its photo. The phone makes it; the photo itself stays a
-- plain JPEG, so a mask can be added, replaced or removed without touching
-- the original. '' means no cut-out.
ALTER TABLE items ADD COLUMN mask_path TEXT NOT NULL DEFAULT '';
