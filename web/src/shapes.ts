import type { Category } from './taxonomy'

// Circle = tops, square = bottoms, triangle = layers, half-circle = shoes.
// The same shapes mark garments in the grid, the Build slots and the tab bar.
export type ShapeName = 'circle' | 'square' | 'triangle' | 'half'

const SHAPE_OF: Record<Category, ShapeName> = {
  top: 'circle',
  dress: 'circle',
  bottom: 'square',
  outerwear: 'triangle',
  shoes: 'half',
  bag: 'square',
  accessory: 'circle',
}

// The block colour behind a category's photos.
const BLOCK_OF: Record<Category, string> = {
  top: 'var(--color-yellow)',
  dress: 'var(--color-yellow)',
  bottom: 'var(--color-blue)',
  outerwear: 'var(--color-accent)',
  shoes: 'var(--color-block-neutral)',
  bag: 'var(--color-blue)',
  accessory: 'var(--color-block-neutral)',
}

export function shapeOf(category: string): ShapeName {
  return SHAPE_OF[category as Category] ?? 'circle'
}

export function blockOf(category: string): string {
  return BLOCK_OF[category as Category] ?? 'var(--color-block-neutral)'
}
