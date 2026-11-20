import type { ShapeName } from './shapes'

// Decorative: the category is always also written out next to it.
export default function Shape({ name, size = 14 }: { name: ShapeName; size?: number }) {
  return (
    <svg viewBox="0 0 20 20" width={size} height={size} fill="currentColor" aria-hidden="true">
      {name === 'circle' && <circle cx="10" cy="10" r="8" />}
      {name === 'square' && <rect x="2" y="2" width="16" height="16" />}
      {name === 'triangle' && <polygon points="10,2 18,18 2,18" />}
      {name === 'half' && <path d="M2 14A8 8 0 0 1 18 14Z" />}
    </svg>
  )
}
