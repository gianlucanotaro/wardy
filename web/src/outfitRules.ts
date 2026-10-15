// Mirrors internal/suggest (Go) — the outfit rules that decide when a
// piece can be suggested — so the item view can show each piece's range.
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

// Occasions a piece of this formality can be suggested for, from least
// to most formal (OCCASION_FORMALITY's key order).
export function occasionsFor(formality: number): Occasion[] {
  return (Object.keys(OCCASION_FORMALITY) as Occasion[]).filter((o) => {
    const [min, max] = OCCASION_FORMALITY[o]
    return formality >= min && formality <= max
  })
}

// suggest.FitsWeather: can a piece of this category and warmth be part of
// an outfit for a day whose target warmth is `target`?
function fitsWeather(category: string, warmth: number, target: number): boolean {
  switch (category) {
    case 'top':
      return warmth <= target + 1
    case 'outerwear':
      return warmth >= target - 1 && warmth <= target + 1
    case 'bottom':
    case 'dress':
    case 'shoes':
      return warmth >= target - 2 && warmth <= target + 1
    default:
      return true
  }
}

// suggest.TargetWarmth's bands, as the lowest day-average °C for each
// target (target 5 is everything below 7°).
const BAND_FLOOR_C: Record<number, number> = { 1: 27, 2: 21, 3: 14, 4: 7 }

// When a piece can be worn, in day-average temperatures: "ANY WEATHER",
// "21° AND UP", "UP TO 21°" or "7–27°". `layerBelow` is set for a top that
// only works on colder days with a knit or coat over it (the outfit's
// warmest upper layer must reach target - 1): the °C below which that
// starts.
export function weatherRange(category: string, warmth: number): { label: string; layerBelow: number | null } {
  const targets = [1, 2, 3, 4, 5].filter((t) => fitsWeather(category, warmth, t))
  if (targets.length === 0) return { label: 'NEVER', layerBelow: null }

  const warmest = targets[0] // lowest target = hottest day it suits
  const coldest = targets[targets.length - 1]
  const upTo = warmest === 1 ? null : BAND_FLOOR_C[warmest - 1]
  const from = coldest === 5 ? null : BAND_FLOOR_C[coldest]

  let label: string
  if (upTo === null && from === null) label = 'ANY WEATHER'
  else if (upTo === null) label = `${from}° AND UP`
  else if (from === null) label = `UP TO ${upTo}°`
  else label = `${from}–${upTo}°`

  const firstLayered = warmth + 2
  const layerBelow = category === 'top' && firstLayered <= 5 ? BAND_FLOOR_C[firstLayered - 1] : null
  return { label, layerBelow }
}
