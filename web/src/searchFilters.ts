import type { Item } from './api'
import {
  CATEGORIES,
  COLORS,
  FITS,
  MATERIALS,
  OCCASIONS,
  PATTERNS,
  SEASONS,
  SUBCATEGORIES,
  type Category,
  type Color,
} from './taxonomy'

// Structured search on the wardrobe grid (#14): `field:` filters over the
// closed vocabularies, combined AND across fields and OR within one field,
// plus the free-text custom-tag search. All client-side — the grid already
// has every item loaded.

export const FILTER_FIELDS = [
  'category',
  'subcategory',
  'color',
  'pattern',
  'material',
  'fit',
  'season',
  'occasion',
] as const
export type FilterField = (typeof FILTER_FIELDS)[number]
export type Filters = Partial<Record<FilterField, string[]>>

// One-tap buttons under the search box; the rest sit behind + MORE.
export const SHORTCUT_FIELDS: FilterField[] = ['category', 'color', 'season', 'occasion']

const ALIASES: Record<string, FilterField> = { colour: 'color' }

// Small swatches in the color picker. Approximate on purpose — they only
// help find the word, the item's own photo is the real color.
export const COLOR_SWATCHES: Record<Color, string> = {
  black: '#1b1b1b',
  white: '#ffffff',
  gray: '#8b8b8b',
  navy: '#1f2a44',
  blue: '#2f62b0',
  red: '#c62a1f',
  burgundy: '#6b1f2a',
  pink: '#e79ab0',
  orange: '#e2742b',
  yellow: '#e8c33a',
  green: '#3d8a4c',
  olive: '#5e6340',
  brown: '#5b3a24',
  tan: '#b88a5a',
  beige: '#d9c7a6',
  cream: '#ece3cf',
  purple: '#6c4a9a',
  teal: '#2a7d7b',
  gold: '#c9a227',
  silver: '#b9bcc0',
  rust: '#a2492a',
  multicolor: 'linear-gradient(135deg, #c62a1f 0 33%, #e8c33a 33% 66%, #2f62b0 66%)',
}

// The field a search box holding exactly "color:" (or "colour:") is asking
// for, or null when the text is ordinary tag search.
export function triggerField(text: string): FilterField | null {
  const m = /^\s*([a-z]+):\s*$/i.exec(text)
  if (!m) return null
  const name = m[1].toLowerCase()
  return (FILTER_FIELDS as readonly string[]).includes(name) ? (name as FilterField) : (ALIASES[name] ?? null)
}

// Subcategory narrows to the picked categories, so it can't offer a value
// that never co-occurs with the category filter (docs/taxonomy.md).
export function narrowedCategories(filters: Filters): Category[] {
  return (filters.category ?? []).filter((c): c is Category => (CATEGORIES as readonly string[]).includes(c))
}

export function fieldValues(field: FilterField, filters: Filters): readonly string[] {
  switch (field) {
    case 'category':
      return CATEGORIES
    case 'subcategory': {
      const cats = narrowedCategories(filters)
      return [...new Set((cats.length ? cats : CATEGORIES).flatMap((c) => SUBCATEGORIES[c]))]
    }
    case 'color':
      return COLORS
    case 'pattern':
      return PATTERNS
    case 'material':
      return MATERIALS
    case 'fit':
      return FITS
    case 'season':
      return SEASONS
    case 'occasion':
      return OCCASIONS
  }
}

function itemValues(item: Item, field: FilterField): string[] {
  switch (field) {
    case 'category':
      return [item.category]
    case 'subcategory':
      return [item.subcategory]
    // Either color counts: a navy top with white stripes is both.
    case 'color':
      return [item.color_primary, item.color_secondary]
    case 'pattern':
      return [item.pattern]
    case 'material':
      return [item.material]
    case 'fit':
      return [item.fit]
    case 'season':
      return item.seasons ?? []
    case 'occasion':
      return item.occasions ?? []
  }
}

export function activeFields(filters: Filters): FilterField[] {
  return FILTER_FIELDS.filter((f) => filters[f]?.length)
}

export function withoutField(filters: Filters, field: FilterField): Filters {
  const next = { ...filters }
  delete next[field]
  return next
}

// query is the already trimmed, lower-cased tag search ('' for none).
export function matchesSearch(item: Item, filters: Filters, query: string): boolean {
  for (const f of activeFields(filters)) {
    const wanted = filters[f]!
    if (!itemValues(item, f).some((v) => wanted.includes(v))) return false
  }
  return !query || (item.custom_tags ?? []).some((tag) => tag.toLowerCase().includes(query))
}
