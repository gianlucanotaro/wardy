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

// taxonomy.Styles (Go): the overall look of an inspiration fit (#49).
export const STYLES = [
  'minimal',
  'classic',
  'preppy',
  'workwear',
  'streetwear',
  'sporty',
  'smart-casual',
  'bohemian',
  'edgy',
  'romantic',
  'avant-garde',
  'vintage',
] as const

// taxonomy.MaxInspo* (Go): caps on an inspo fit's lists.
export const MAX_INSPO_STYLES = 3
export const MAX_INSPO_COLORS = 5
export const MAX_INSPO_PIECES = 8

// Approximate swatches for the colour palette, for showing colours as
// colours (the inspo profile) rather than words.
export const COLOR_SWATCH: Record<Color, string> = {
  black: '#1d1c1b',
  white: '#f7f6f2',
  gray: '#8a8d90',
  navy: '#26314f',
  blue: '#3b6fb6',
  red: '#c23a2b',
  burgundy: '#6e1f2b',
  pink: '#e7a2b4',
  orange: '#e57a2e',
  yellow: '#e9c440',
  green: '#4f7a47',
  olive: '#6b6b3a',
  brown: '#6b4428',
  tan: '#c2a07a',
  beige: '#d8c8a8',
  cream: '#efe6d4',
  purple: '#6d4a93',
  teal: '#2f7f7a',
  gold: '#c9a23a',
  silver: '#bfc3c7',
  rust: '#a4492a',
  multicolor: 'conic-gradient(#c23a2b, #e9c440, #4f7a47, #3b6fb6, #6d4a93, #c23a2b)',
}
