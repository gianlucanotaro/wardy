// Mirrors internal/suggest (Go) — the outfit rules that decide when a
// piece can be suggested — so the item view can say what each piece's
// formality and warmth ranges mean in occasions and temperatures.
// Keep the two in sync by hand, like taxonomy.ts; the Go side is the one
// that actually picks outfits.

import { type Occasion } from './taxonomy'

// suggest.occasionFormality: the item formality each occasion accepts.
const OCCASION_FORMALITY: Record<Occasion, [number, number]> = {
  home: [1, 2],
  sport: [1, 2],
  outdoor: [1, 3],
  casual: [1, 4],
  date: [2, 4],
  work: [3, 5],
  formal: [4, 5],
}

// Occasions a piece can be suggested for: those whose formality range
// overlaps the piece's own, from least to most formal (OCCASION_FORMALITY's
// key order).
export function occasionsFor(formalityMin: number, formalityMax: number): Occasion[] {
  return (Object.keys(OCCASION_FORMALITY) as Occasion[]).filter((o) => {
    const [min, max] = OCCASION_FORMALITY[o]
    return formalityMin <= max && formalityMax >= min
  })
}

// suggest.TargetWarmth's bands, as the lowest day-average °C for each
// target (target 5 is everything below 7°).
const BAND_FLOOR_C: Record<number, number> = { 1: 27, 2: 21, 3: 14, 4: 7 }

// When a piece can be worn, in day-average temperatures, from its own
// warmth range (suggest.FitsWeather): "ANY WEATHER", "21° AND UP",
// "UP TO 21°" or "7–27°". `layerBelow` is set for a top that only works on
// the colder end of its range with a knit or coat over it: a piece is one
// step warmer than the hottest weather it suits (suggest.pieceWarmth), and
// an outfit's warmest upper layer must reach target - 1.
export function weatherRange(
  category: string,
  warmthMin: number,
  warmthMax: number,
): { label: string; layerBelow: number | null } {
  const upTo = warmthMin === 1 ? null : BAND_FLOOR_C[warmthMin - 1]
  const from = warmthMax === 5 ? null : BAND_FLOOR_C[warmthMax]

  let label: string
  if (upTo === null && from === null) label = 'ANY WEATHER'
  else if (upTo === null) label = `${from}° AND UP`
  else if (from === null) label = `UP TO ${upTo}°`
  else label = `${from}–${upTo}°`

  const firstLayered = warmthMin + 3
  const layerBelow = category === 'top' && firstLayered <= warmthMax ? BAND_FLOOR_C[firstLayered - 1] : null
  return { label, layerBelow }
}
