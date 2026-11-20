import type { ReactNode } from 'react'
import Shape from './Shape'

// Loading, failed, or nothing here: a shape and a few words on a tinted
// block, in place of a line of small print.
export default function Notice({ kind, children }: { kind: 'loading' | 'error' | 'empty'; children: ReactNode }) {
  const tint = kind === 'error' ? 'var(--color-accent)' : 'var(--color-ink)'
  return (
    <div
      role={kind === 'error' ? 'alert' : 'status'}
      className="mx-4 my-4 flex items-center gap-3 px-4 py-4"
      style={{ background: `color-mix(in srgb, ${tint} ${kind === 'error' ? 14 : 8}%, var(--color-bg))` }}
    >
      <span className={kind === 'loading' ? 'motion-safe:animate-pulse' : ''} style={{ color: kind === 'error' ? 'var(--color-accent)' : 'var(--color-ink)' }}>
        <Shape name={kind === 'error' ? 'triangle' : kind === 'loading' ? 'circle' : 'square'} size={22} />
      </span>
      <span className="wy-display text-2xl">{children}</span>
    </div>
  )
}
