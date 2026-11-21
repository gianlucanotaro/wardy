// Gives every cut-out garment the same size in its tile (docs/frontend.md,
// "Cut-outs"): a shoe and a coat both fill their tile, whatever size they were
// in their photos. The server measures the garment (its box in the photo);
// this works out how to move and scale the photo so that box fits the tile.

// Where the garment sits in its photo (items.mask_box): fractions of the
// photo's width and height, and the photo's width/height.
export type GarmentBox = { x: number; y: number; w: number; h: number; ratio: number }

// The space left round the garment, as a share of the tile on each side.
export const MARGIN = 0.08
// How far a small garment may be enlarged. More would show the photo's pixels.
export const MAX_SCALE = 2.5

export type Fit = { scale: number; dx: number; dy: number }

// The photo is shown the way object-fit: cover shows it: it fills the tile,
// and the part that does not fit is cut off, centred. `tile` is the tile's
// width/height. The result is a CSS transform, `scale(s) translate(dx, dy)`
// with dx and dy as shares of the tile and the origin in its middle, which
// takes the box's centre to the tile's centre and enlarges it to fit.
export function fitGarment(box: GarmentBox, tile: number, margin = MARGIN, maxScale = MAX_SCALE): Fit {
  const still: Fit = { scale: 1, dx: 0, dy: 0 }
  if (!(box.w > 0 && box.h > 0 && box.ratio > 0 && tile > 0)) return still

  // the photo's size in the tile, as shares of the tile (1 = the tile)
  const photoW = Math.max(1, box.ratio / tile)
  const photoH = Math.max(1, tile / box.ratio)

  const boxW = box.w * photoW
  const boxH = box.h * photoH
  const centreX = (1 - photoW) / 2 + (box.x + box.w / 2) * photoW
  const centreY = (1 - photoH) / 2 + (box.y + box.h / 2) * photoH

  const room = 1 - 2 * margin
  const scale = Math.min(room / boxW, room / boxH, maxScale)
  return { scale, dx: 0.5 - centreX, dy: 0.5 - centreY }
}
