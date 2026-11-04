import { useState } from 'react'
import type { Item } from './api'

// An outfit's style mix (docs/mockups/outfit-style-lines.html): one thin line
// per style, biggest first, as a share of the outfit's style tags. Tap a line
// for the pieces behind it.

// Close neighbours share a name, like the style radar's axes
// (radarAxes in internal/inspo/radar.go).
const STYLE_GROUP: Record<string, string> = {
  preppy: 'classic',
  'avant-garde': 'edgy',
  bohemian: 'romantic',
}

// Always this many rows, empty ones held open, so NOT TODAY stays under the
// thumb from one outfit to the next.
const ROWS = 4
// A look's card is one of many in a list, so it stays short: just the lines.
const COMPACT_ROWS = 3

type Share = { style: string; pct: number; pieces: string[] }

function outfitShares(items: Item[]): Share[] {
  const pieces = new Map<string, string[]>()
  const counts = new Map<string, number>()
  let total = 0
  for (const item of items) {
    const name = item.name || item.subcategory || item.category
    for (const raw of item.styles ?? []) {
      const style = STYLE_GROUP[raw] ?? raw
      counts.set(style, (counts.get(style) ?? 0) + 1)
      total++
      const list = pieces.get(style) ?? []
      if (!list.includes(name)) list.push(name)
      pieces.set(style, list)
    }
  }
  return [...counts]
    .map(([style, count]) => ({ style, pct: (count / total) * 100, pieces: pieces.get(style) ?? [] }))
    .sort((a, b) => b.pct - a.pct || a.style.localeCompare(b.style))
}

export default function OutfitStyle({ items, compact = false }: { items: Item[]; compact?: boolean }) {
  const [picked, setPicked] = useState<string | null>(null)
  const shares = outfitShares(items)
  // Items uploaded moments ago may not have styles yet.
  if (shares.length === 0) return null

  const peak = shares[0].pct
  const lineColor = (s: Share) => (s.pct === peak ? 'var(--color-accent)' : 'var(--color-ink)')

  if (compact) {
    return (
      <div className="px-4 pb-2.5" aria-label="Style mix">
        {shares.slice(0, COMPACT_ROWS).map((s) => (
          <div key={s.style} className="grid grid-cols-[96px_1fr_34px] items-center gap-2.5 h-6">
            <span className="font-extrabold text-[10px] tracking-[0.06em] uppercase truncate">{s.style}</span>
            <span className="relative h-px bg-(--color-line)">
              <span className="absolute left-0 -top-px h-[3px]" style={{ width: `${s.pct}%`, background: lineColor(s) }} />
            </span>
            <span className="font-extrabold text-[11px] text-right">{Math.round(s.pct)}%</span>
          </div>
        ))}
      </div>
    )
  }

  const tops = shares.filter((s) => s.pct === peak)
  const selected = shares.find((s) => s.style === picked)

  return (
    <div className="border-b-2 border-(--color-line-strong)">
      <div className="wy-lbl px-4 pt-2.5">STYLE</div>
      <div className="px-4 pt-0.5">
        {Array.from({ length: ROWS }, (_, row) => {
          const s = shares[row]
          if (!s) return <div key={row} className="h-11" aria-hidden="true" />
          return (
            <button
              key={s.style}
              type="button"
              aria-pressed={picked === s.style}
              onClick={() => setPicked(picked === s.style ? null : s.style)}
              className="grid grid-cols-[96px_1fr_34px] items-center gap-2.5 w-full h-11 text-left"
              style={{ background: picked === s.style ? 'color-mix(in srgb, var(--color-ink) 7%, transparent)' : undefined }}
            >
              <span className="font-extrabold text-[10px] tracking-[0.06em] uppercase truncate">{s.style}</span>
              <span className="relative h-px bg-(--color-line)">
                <span
                  className="absolute left-0 -top-px h-[3px] motion-safe:transition-[width] duration-200"
                  style={{ width: `${s.pct}%`, background: lineColor(s) }}
                />
              </span>
              <span className="font-extrabold text-[11px] text-right">{Math.round(s.pct)}%</span>
            </button>
          )
        })}
      </div>
      {/* Two lines held open for the same reason as the empty rows. */}
      <p className="px-4 pt-1 pb-3 min-h-[50px] text-[12.5px] leading-[17px]" aria-live="polite">
        {selected ? (
          <>
            <b className="font-extrabold uppercase tracking-wide">
              {selected.style} {Math.round(selected.pct)}%
            </b>
            <span className="text-(--color-neutral-700)"> · {selected.pieces.join(', ')}</span>
          </>
        ) : tops.length > 1 ? (
          <>
            <b className="font-extrabold uppercase tracking-wide">A mix: {tops.map((s) => s.style).join(', ')}</b>
            <span className="text-(--color-neutral-700)"> · {Math.round(peak)}% each</span>
          </>
        ) : (
          <>
            <b className="font-extrabold uppercase tracking-wide">
              Mostly {tops[0].style} {Math.round(peak)}%
            </b>
            <span className="text-(--color-neutral-700)"> · {tops[0].pieces.join(', ')}</span>
          </>
        )}
      </p>
    </div>
  )
}
