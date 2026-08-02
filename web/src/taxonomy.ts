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
  top: [
    't-shirt', 'shirt', 'button-down', 'blouse', 'knit', 'sweater', 'hoodie', 'tank', 'polo',
    'cardigan', 'sweatshirt', 'long-sleeve-tee', 'turtleneck', 'crop-top',
  ],
  bottom: ['jeans', 'chinos', 'trousers', 'shorts', 'skirt', 'leggings', 'sweatpants', 'cargo-pants', 'overalls'],
  dress: [
    'casual-dress', 'cocktail-dress', 'maxi-dress', 'jumpsuit',
    'midi-dress', 'mini-dress', 'shirt-dress', 'sweater-dress', 'romper',
  ],
  outerwear: [
    'jacket', 'blazer', 'coat', 'parka', 'puffer-jacket', 'vest',
    'trench-coat', 'raincoat', 'bomber-jacket', 'fleece-jacket',
  ],
  shoes: [
    'sneakers', 'boots', 'sandals', 'heels', 'flats', 'loafers',
    'dress-shoes', 'slippers', 'flip-flops', 'rain-boots',
  ],
  bag: ['backpack', 'tote', 'crossbody', 'clutch', 'handbag', 'belt-bag', 'duffel', 'briefcase'],
  // "hat" means a brimmed hat specifically, distinct from "cap"
  // (structured, flat-brim/baseball style) and "beanie" (knit).
  accessory: ['belt', 'hat', 'scarf', 'jewelry', 'sunglasses', 'watch', 'gloves', 'tie', 'socks', 'beanie', 'cap', 'glasses'],
}

export const PATTERNS = [
  'solid',
  'striped',
  'checked',
  'floral',
  'graphic',
  'textured',
  'polka-dot',
  'animal-print',
  'camouflage',
  'colorblock',
  'tie-dye',
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
  // Surface textures, not fibers — unlike wool/cashmere/silk, these are
  // things a photo can actually show.
  'corduroy',
  'velvet',
  'suede',
  'fleece',
  'tweed',
  'lace',
  'unknown',
] as const
export type Material = (typeof MATERIALS)[number]

export const FITS = ['skinny', 'slim', 'regular', 'relaxed', 'oversized', 'not-applicable'] as const
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
  'rust',
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
