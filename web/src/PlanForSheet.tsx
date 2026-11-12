import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createLook, getPlan, setPlanDay, type Look } from './api'
import { useToast } from './useToast'

// "Plan for…" (#81): the plan only holds saved looks, so this saves the
// outfit as a look first, under a name the user can edit, then plans it for
// a date in the next four weeks. One look per day: picking a day that
// already has one replaces it.

const PLAN_AHEAD_DAYS = 28
const NAME_MAX = 80
const WEEKDAY = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT']
const MONTH = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']

// Local calendar date, like todayISODate in api.ts.
function isoDate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function dayLabel(d: Date): string {
  return `${WEEKDAY[d.getDay()]} ${d.getDate()} ${MONTH[d.getMonth()]}`
}

export default function PlanForSheet({
  itemIds,
  namePrefix,
  onClose,
  onPlanned,
}: {
  itemIds: number[]
  // The default name is "<namePrefix> · FRI 13 NOV", e.g. "BUILD · FRI 13 NOV".
  namePrefix: string
  onClose: () => void
  onPlanned: () => void
}) {
  const flash = useToast()
  const queryClient = useQueryClient()

  const [days] = useState(() => {
    const now = new Date()
    return Array.from({ length: PLAN_AHEAD_DAYS }, (_, i) => new Date(now.getFullYear(), now.getMonth(), now.getDate() + i))
  })
  const start = isoDate(days[0])
  const end = isoDate(days[days.length - 1])
  const plan = useQuery({ queryKey: ['plan', start, end], queryFn: () => getPlan(start, end) })
  const planned = new Map((plan.data ?? []).filter((d) => d.look).map((d) => [d.date, d.look!.name]))

  // Tomorrow by default: today is usually already dressed for.
  const [picked, setPicked] = useState(days[1])
  const [name, setName] = useState<string | null>(null) // null until the user edits it
  const lookName = name ?? `${namePrefix} · ${dayLabel(picked)}`

  // If saving the look worked but planning it didn't, a retry plans the
  // look already saved instead of saving a second copy.
  const savedLook = useRef<Look | null>(null)
  const planFor = useMutation({
    mutationFn: async (input: { date: string; name: string }) => {
      savedLook.current ??= await createLook({ name: input.name, item_ids: itemIds })
      return setPlanDay(input.date, savedLook.current.id)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['looks'] })
      queryClient.invalidateQueries({ queryKey: ['plan'] })
      flash(`PLANNED FOR ${dayLabel(picked)}`)
      onPlanned()
    },
    onError: () => flash("COULDN'T PLAN — TRY AGAIN"),
  })

  const trimmed = lookName.trim()
  const replaces = planned.get(isoDate(picked))

  return (
    <div className="fixed inset-0 z-20 flex flex-col justify-end" role="dialog" aria-label="Plan for a day">
      <button type="button" aria-label="Close" className="absolute inset-0 bg-(--color-ink)/45" onClick={onClose} />
      <div className="relative bg-(--color-bg) border-t-2 border-(--color-line-strong) safe-area-overlay max-h-full overflow-y-auto">
        <div className="flex items-center justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
          <span className="font-extrabold text-2xl tracking-tight">PLAN FOR…</span>
          <button type="button" onClick={onClose} className="wy-chip">
            ✕ CLOSE
          </button>
        </div>

        <div className="px-4 py-3 border-b border-(--color-line)">
          <label htmlFor="plan-for-name" className="wy-lbl block mb-1.5">
            SAVE AS LOOK
          </label>
          <input
            id="plan-for-name"
            type="text"
            maxLength={NAME_MAX}
            className="w-full border border-(--color-line) bg-(--color-surface) px-3 py-2 text-base focus:outline-2 focus:outline-(--color-accent)"
            value={lookName}
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        <div className="px-4 py-3 border-b border-(--color-line)">
          <div className="wy-lbl mb-1.5">DAY · NEXT 4 WEEKS</div>
          <div className="grid grid-cols-7 gap-1" role="group" aria-label="Day">
            {days.map((d) => {
              const iso = isoDate(d)
              const on = iso === isoDate(picked)
              const taken = planned.get(iso)
              return (
                <button
                  key={iso}
                  type="button"
                  className="wy-chip flex flex-col items-center justify-center"
                  style={{ minHeight: 48, padding: '4px 0' }}
                  data-on={on || undefined}
                  aria-pressed={on}
                  aria-label={`${dayLabel(d)}${iso === start ? ', today' : ''}${taken ? `, planned: ${taken}` : ''}`}
                  onClick={() => setPicked(d)}
                >
                  <span className="text-[9px]">{WEEKDAY[d.getDay()]}</span>
                  <span className="text-base leading-none">{d.getDate()}</span>
                  <span aria-hidden="true" className="text-[9px] leading-none h-[9px]">
                    {taken ? '●' : ''}
                  </span>
                </button>
              )
            })}
          </div>
          <p className="wy-lbl mt-2">
            {replaces ? `REPLACES ${replaces.toUpperCase()}` : '● ALREADY HAS A LOOK'}
          </p>
        </div>

        <div className="border-t-2 border-(--color-line-strong)">
          <button
            type="button"
            data-primary="1"
            onClick={() => planFor.mutate({ date: isoDate(picked), name: trimmed })}
            disabled={planFor.isPending || trimmed === ''}
            className="wy-act w-full flex items-center justify-between"
          >
            {planFor.isPending ? 'PLANNING…' : `PLAN FOR ${dayLabel(picked)}`} <span aria-hidden="true">→</span>
          </button>
        </div>
      </div>
    </div>
  )
}
