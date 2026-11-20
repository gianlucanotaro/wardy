import { COLOR_HEX } from './colors'
import type { Color } from './taxonomy'

function fill(color: string): string {
  if (color === 'multicolor') {
    return 'conic-gradient(#d62a1f, #f4c20d, #2e8b57, #2f6fd0, #6f42a8, #d62a1f)'
  }
  return COLOR_HEX[color as Exclude<Color, 'multicolor'>] ?? 'transparent'
}

// Colour tags as round swatches: easier to scan than 22 word-pills, and a
// third of the height. "" is "none" (a dashed ring). Each swatch is a button
// named for its colour; the picked names are written under the row.
export default function ColorSwatches({
  value,
  options,
  onPick,
}: {
  value: string[]
  options: string[]
  onPick: (color: string) => void
}) {
  const picked = value.filter(Boolean)
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap gap-2.5 py-1">
        {options.map((opt) => {
          const on = value.includes(opt)
          return (
            <button
              type="button"
              key={opt || 'none'}
              aria-label={opt || 'none'}
              aria-pressed={on}
              onClick={() => onPick(opt)}
              className="size-11 rounded-full border-2 border-(--color-line)"
              style={{
                background: opt ? fill(opt) : 'transparent',
                borderStyle: opt ? 'solid' : 'dashed',
                outline: on ? '3px solid var(--color-ink)' : 'none',
                outlineOffset: 2,
              }}
            />
          )
        })}
      </div>
      <span className="wy-lbl">{picked.length ? picked.join(' · ') : value.includes('') ? 'none' : '—'}</span>
    </div>
  )
}
