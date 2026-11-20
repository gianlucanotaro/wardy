import type { Color } from './taxonomy'

// What each colour tag looks like in a swatch. multicolor and "none" are drawn
// by ColorSwatches itself.
export const COLOR_HEX: Record<Exclude<Color, 'multicolor'>, string> = {
  black: '#151413',
  white: '#ffffff',
  gray: '#8c8c8c',
  navy: '#1b2a4a',
  blue: '#2f6fd0',
  red: '#d62a1f',
  burgundy: '#6e1423',
  pink: '#f2a7c3',
  orange: '#f28a1e',
  yellow: '#f4c20d',
  green: '#2e8b57',
  olive: '#6b6b2a',
  brown: '#6b4226',
  tan: '#c8a27a',
  beige: '#d9c7a3',
  cream: '#f3ead2',
  purple: '#6f42a8',
  teal: '#1f8a8a',
  gold: '#c9a227',
  silver: '#c0c4c8',
  rust: '#b7410e',
}
