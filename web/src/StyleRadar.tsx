import { useState } from 'react'
import type { StyleRadar as Radar } from './api'
import type { LayerKey } from './radarInsights'

// The style radar (docs/mockups/style-radar.html): what you own, what you
// wore in the last 90 days and what your inspo board likes, as three
// outlines on one spider chart, then the biggest gaps in words. Each layer
// is a share of its source's style tags, so the scales match.
//
// Colours were checked for colour-blind separation (blue worn, aqua owned,
// red inspo); inspo is also dashed, the legend names every layer and each
// style's values are one tap away, so colour is never the only cue.

const LAYERS: { key: LayerKey; name: string; color: string; dash: string }[] = [
  { key: 'owned', name: 'Owned', color: '#1baf7a', dash: '' },
  { key: 'worn', name: 'Worn', color: '#2a78d6', dash: '' },
  { key: 'inspo', name: 'Inspo', color: '#ec3013', dash: '5 4' },
]

// Geometry in viewBox units (-40 0 480 380): the chart sits at CX,CY with
// radius R; the outer ring is MAX percent, wide enough for a typical mix.
const CX = 200
const CY = 190
const R = 128
const RINGS = [10, 20, 30, 40]
const MAX = 40

const label = (axis: string) => axis.toUpperCase()

export default function StyleRadar({ radar }: { radar: Radar }) {
  const [shown, setShown] = useState<Record<LayerKey, boolean>>({ owned: true, worn: true, inspo: true })
  const [hot, setHot] = useState<number | null>(null)

  const n = radar.axes.length
  const angle = (i: number) => -Math.PI / 2 + (i * 2 * Math.PI) / n
  // Shares above the outer ring are drawn on it, so one dominant style
  // can't push the chart off its card.
  const pt = (i: number, v: number): [number, number] => {
    const r = (R * Math.min(v, MAX)) / MAX
    return [CX + Math.cos(angle(i)) * r, CY + Math.sin(angle(i)) * r]
  }
  const layers = LAYERS.filter((l) => radar[l.key].count > 0)
  const visible = layers.filter((l) => shown[l.key])

  if (layers.length === 0) {
    return (
      <p className="px-4 py-4 text-sm">
        No styles yet. Pieces and inspo fits get their styles when they're tagged; it can take a few minutes after
        an upload.
      </p>
    )
  }

  // Ring values sit halfway between the last two axes (upper left), where
  // the outlines usually stay near the centre.
  const ringAngle = (angle(n - 2) + angle(n - 1)) / 2

  return (
    <div>
      <div className="flex flex-wrap gap-1.5 px-4 pt-3" role="group" aria-label="Show or hide a layer">
        {layers.map((l) => (
          <button
            key={l.key}
            type="button"
            className="wy-chip inline-flex items-center gap-2"
            aria-pressed={shown[l.key]}
            style={shown[l.key] ? undefined : { color: 'var(--color-neutral-500)', borderStyle: 'dashed' }}
            onClick={() => setShown((s) => ({ ...s, [l.key]: !s[l.key] }))}
          >
            <svg width="22" height="10" aria-hidden="true" style={{ opacity: shown[l.key] ? 1 : 0.3 }}>
              <line x1="1" y1="5" x2="21" y2="5" stroke={l.color} strokeWidth="3" strokeDasharray={l.dash} />
            </svg>
            {l.name}
            <span className="text-(--color-neutral-700)">{radar[l.key].count}</span>
          </button>
        ))}
      </div>

      <div className="relative">
        <svg viewBox="-40 0 480 380" className="mx-auto block h-auto w-full max-w-[480px]" role="img" aria-label={describe(radar)}>
          {RINGS.map((r) => (
            <polygon
              key={r}
              points={radar.axes.map((_, i) => pt(i, r).join(',')).join(' ')}
              fill="none"
              stroke="rgba(32,30,29,0.14)"
            />
          ))}
          {radar.axes.map((_, i) => {
            const [x, y] = pt(i, MAX)
            return <line key={i} x1={CX} y1={CY} x2={x} y2={y} stroke={hot === i ? '#9b9797' : 'rgba(32,30,29,0.14)'} strokeWidth={hot === i ? 1.5 : 1} />
          })}
          {[20, 40].map((r) => (
            <text
              key={r}
              x={CX + (Math.cos(ringAngle) * R * r) / MAX - 3}
              y={CY + (Math.sin(ringAngle) * R * r) / MAX - 3}
              textAnchor="end"
              className="fill-(--color-neutral-500) text-[8.5px] font-extrabold tracking-[0.08em]"
            >
              {r}%
            </text>
          ))}
          {visible.map((l) => (
            <g key={l.key}>
              <polygon
                points={radar[l.key].shares.map((v, i) => pt(i, v).join(',')).join(' ')}
                fill={l.color}
                fillOpacity={l.key === 'inspo' ? 0 : 0.08}
                stroke={l.color}
                strokeWidth={2}
                strokeLinejoin="miter"
                strokeDasharray={l.dash}
              />
              {radar[l.key].shares.map((v, i) => {
                const [x, y] = pt(i, v)
                const m = hot === i ? 9 : 7
                return <rect key={i} x={x - m / 2} y={y - m / 2} width={m} height={m} fill={l.color} stroke="var(--color-bg)" strokeWidth={2} />
              })}
            </g>
          ))}
          {radar.axes.map((axis, i) => {
            const a = angle(i)
            const anchor = Math.abs(Math.cos(a)) < 0.2 ? 'middle' : Math.cos(a) > 0 ? 'start' : 'end'
            return (
              <text
                key={axis}
                x={CX + Math.cos(a) * (R + 22)}
                y={CY + Math.sin(a) * (R + 22) + 4}
                textAnchor={anchor}
                className={`text-[10px] font-extrabold tracking-[0.08em] ${hot === i ? 'fill-(--color-ink)' : 'fill-(--color-neutral-700)'}`}
              >
                {label(axis)}
              </text>
            )
          })}
          {/* Hit targets: one wedge per axis, far bigger than the marks */}
          {radar.axes.map((axis, i) => {
            const half = Math.PI / n
            const big = R + 40
            const a0 = angle(i) - half
            const a1 = angle(i) + half
            return (
              <path
                key={axis}
                d={`M${CX},${CY} L${CX + Math.cos(a0) * big},${CY + Math.sin(a0) * big} A${big},${big} 0 0 1 ${CX + Math.cos(a1) * big},${CY + Math.sin(a1) * big} Z`}
                fill="transparent"
                tabIndex={0}
                aria-label={`${axis}: ${layers.map((l) => `${l.name} ${radar[l.key].shares[i]}%`).join(', ')}`}
                onPointerEnter={() => setHot(i)}
                onPointerDown={() => setHot(i)}
                onFocus={() => setHot(i)}
                onPointerLeave={() => setHot(null)}
                onBlur={() => setHot(null)}
              />
            )
          })}
        </svg>
        {hot !== null && visible.length > 0 && <Tip radar={radar} i={hot} layers={visible} pt={pt} />}
      </div>
    </div>
  )
}

// The values of one style, over the chart near its outermost point.
function Tip({
  radar,
  i,
  layers,
  pt,
}: {
  radar: Radar
  i: number
  layers: typeof LAYERS
  pt: (i: number, v: number) => [number, number]
}) {
  const [x, y] = pt(i, Math.max(...layers.map((l) => radar[l.key].shares[i]), 4))
  // viewBox units to percent of the chart box (480 wide from -40, 380 tall)
  const left = Math.min(Math.max(((x + 40) / 480) * 100, 22), 78)
  const top = Math.max((y / 380) * 100, 18)
  return (
    <div
      className="pointer-events-none absolute min-w-[150px] -translate-x-1/2 -translate-y-[calc(100%+12px)] bg-(--color-ink) px-3 py-2.5 text-[13px] leading-[18px] text-(--color-bg) shadow-[0_3px_10px_rgb(45_43_43/0.16)]"
      style={{ left: `${left}%`, top: `${top}%` }}
      role="status"
    >
      <div className="mb-1.5 text-[10.5px] font-extrabold tracking-[0.1em]">{label(radar.axes[i])}</div>
      {layers.map((l) => (
        <div key={l.key} className="flex items-center justify-between gap-3">
          <span className="inline-flex items-center gap-1.5">
            <i className="inline-block size-2" style={{ background: l.color }} />
            {l.name}
          </span>
          <span className="font-extrabold tabular-nums">{radar[l.key].shares[i]}%</span>
        </div>
      ))}
    </div>
  )
}

function describe(radar: Radar): string {
  return `Style radar: ${radar.axes.length} styles comparing your wardrobe, what you wore in the last 90 days and your inspo fits. Focus a style for its values.`
}
