import type { ReactNode } from 'react'
import type { PlanDay } from './api'
import { asPlanned, lastWear, useWearName } from './planWear'

// A past day's title row (#78, mock-up frame 2): "PLANNED X · wore Y",
// "Wore Y", "✓ X · as planned" or "PLANNED X · nothing logged".
export default function PlanWorn({ day }: { day: PlanDay }) {
  const wear = lastWear(day)
  const name = useWearName(wear)

  if (!wear && !day.look) return <span className="wy-lbl text-[11px]!">NOTHING PLANNED</span>

  return (
    <span className="min-w-0 truncate text-[12px] leading-5 text-(--color-neutral-700)">
      {asPlanned(day, wear) ? (
        <>
          <svg aria-hidden="true" className="inline-block w-3 h-3 mr-[3px] align-[-1px]" viewBox="0 0 24 24">
            <path fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="square" d="M20 6 9 17l-5-5" />
          </svg>
          <Strong>{day.look!.name}</Strong> · as planned
        </>
      ) : (
        <>
          {day.look && <span className="uppercase font-semibold tracking-[0.02em]">Planned {day.look.name} · </span>}
          {wear ? (
            <>
              {day.look ? 'wore' : 'Wore'} <Strong>{name}</Strong>
            </>
          ) : (
            <Strong>nothing logged</Strong>
          )}
        </>
      )}
    </span>
  )
}

function Strong({ children }: { children: ReactNode }) {
  return <b className="font-extrabold uppercase tracking-[0.02em] text-(--color-ink)">{children}</b>
}
