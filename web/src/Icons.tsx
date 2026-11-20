// Drawn, not typed: ✕ and ✓ are missing from some fonts and then show as a box.
const ICON = {
  viewBox: '0 0 12 12',
  width: 12,
  height: 12,
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 2,
  'aria-hidden': true,
} as const

export function Cross() {
  return (
    <svg {...ICON}>
      <path d="M2 2l8 8M10 2l-8 8" />
    </svg>
  )
}

export function Tick() {
  return (
    <svg {...ICON}>
      <path d="M1.5 6.5l3 3 6-7" />
    </svg>
  )
}
