// Mirrors internal/taxonomy (Go) — the closed vocabularies every item is
// tagged against. Keep these two in sync by hand; there is no shared
// codegen for this yet.

export const CATEGORIES = [
  'top',
  'bottom',
  'dress',
  'outerwear',
  'shoes',
  'bag',
  'accessory',
] as const
export type Category = (typeof CATEGORIES)[number]

export const SUBCATEGORIES: Record<Category, string[]> = {
  top: ['t-shirt', 'shirt', 'button-down', 'blouse', 'knit', 'sweater', 'hoodie', 'tank', 'polo'],
  bottom: ['jeans', 'chinos', 'trousers', 'shorts', 'skirt', 'leggings'],
  dress: ['casual-dress', 'cocktail-dress', 'maxi-dress', 'jumpsuit'],
  outerwear: ['jacket', 'blazer', 'coat', 'parka', 'puffer-jacket', 'vest'],
  shoes: ['sneakers', 'boots', 'sandals', 'heels', 'flats', 'loafers'],
  bag: ['backpack', 'tote', 'crossbody', 'clutch', 'handbag'],
  accessory: ['belt', 'hat', 'scarf', 'jewelry', 'sunglasses', 'watch'],
}

export const PATTERNS = [
  'solid',
  'striped',
  'checked',
  'floral',
  'graphic',
  'textured',
  'other',
] as const
export type Pattern = (typeof PATTERNS)[number]

export const MATERIALS = [
  'cotton',
  'linen',
  'wool',
  'cashmere',
  'denim',
  'leather',
  'silk',
  'synthetic',
  'blend',
  'unknown',
] as const
export type Material = (typeof MATERIALS)[number]

export const FITS = ['slim', 'regular', 'relaxed', 'oversized'] as const
export type Fit = (typeof FITS)[number]

export const COLORS = [
  'black',
  'white',
  'gray',
  'navy',
  'blue',
  'red',
  'burgundy',
  'pink',
  'orange',
  'yellow',
  'green',
  'olive',
  'brown',
  'tan',
  'beige',
  'cream',
  'purple',
  'teal',
  'gold',
  'silver',
  'multicolor',
] as const
export type Color = (typeof COLORS)[number]

export const SEASONS = ['spring', 'summer', 'autumn', 'winter'] as const
export type Season = (typeof SEASONS)[number]

export const OCCASIONS = [
  'work',
  'casual',
  'formal',
  'date',
  'sport',
  'outdoor',
  'home',
] as const
export type Occasion = (typeof OCCASIONS)[number]

export const SCALE_MIN = 1
export const SCALE_MAX = 5

export const FORMALITY_LABELS: Record<number, string> = {
  1: 'Loungewear',
  2: 'Casual',
  3: 'Smart casual',
  4: 'Business',
  5: 'Black tie',
}

export const WARMTH_LABELS: Record<number, string> = {
  1: 'Hot weather only',
  2: 'Warm',
  3: 'Mild',
  4: 'Cool',
  5: 'Deep winter',
}
