import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { getPlan, listLooks, setPlanDay } from './api'
import { useToast } from './useToast'

const WEEKDAY_LABELS = ['MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT', 'SUN']
const MONTH_LABELS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']

function mondayOf(date: Date): Date {
  const d = new Date(date)
  const day = d.getDay() // 0 = Sun .. 6 = Sat
  const diff = day === 0 ? -6 : 1 - day
  d.setDate(d.getDate() + diff)
  d.setHours(0, 0, 0, 0)
  return d
}

function isoDate(d: Date): string {
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${mm}-${dd}`
}

// "OCT 19 – OCT 25", with the year added when the week isn't in this one.
function weekLabel(monday: Date, sunday: Date): string {
  const thisYear = new Date().getFullYear()
  const fmt = (d: Date) =>
    `${MONTH_LABELS[d.getMonth()]} ${d.getDate()}${d.getFullYear() !== thisYear ? ` ${d.getFullYear()}` : ''}`
  return `${fmt(monday)} – ${fmt(sunday)}`
}

// The PLAN tab: one calendar week (Mon-Sun), this week by default with
// previous/next arrows to page through others (#21), tap a day to cycle
// through saved looks then back to unplanned — matching
// Wardy Prototype.dc.html's isPlan block, but backed by real plan_days
// rows instead of local state (see the plan's "PLAN is calendar-date-based,
// not day-of-week-template-based").
export default function Plan() {
  const queryClient = useQueryClient()
  const flash = useToast()

  // Weeks away from this one, unbounded both ways: the API caps a single
  // request at 31 days, not how far you can page.
  const [offset, setOffset] = useState(0)
  const monday = mondayOf(new Date())
  monday.setDate(monday.getDate() + offset * 7)
  const days = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(monday)
    d.setDate(d.getDate() + i)
    return d
  })
  const start = isoDate(days[0])
  const end = isoDate(days[6])

  // Keep the old week on screen while the next one loads, so paging
  // doesn't flash LOADING… between weeks.
  const plan = useQuery({
    queryKey: ['plan', start, end],
    queryFn: () => getPlan(start, end),
    placeholderData: keepPreviousData,
  })
  const looks = useQuery({ queryKey: ['looks'], queryFn: listLooks })

  const setDay = useMutation({
    mutationFn: (input: { date: string; lookId: number | null }) => setPlanDay(input.date, input.lookId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['plan', start, end] }),
  })

  function cycle(date: string, currentLookId: number | null) {
    const list = looks.data ?? []
    if (list.length === 0) {
      flash('SAVE A LOOK FIRST')
      return
    }
    if (currentLookId == null) {
      setDay.mutate({ date, lookId: list[0].id })
      return
    }
    const next = list.findIndex((l) => l.id === currentLookId) + 1
    setDay.mutate({ date, lookId: next >= list.length ? null : list[next].id })
  }

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <span className="font-extrabold text-3xl tracking-tight">THE WEEK</span>
        {offset !== 0 && (
          <button type="button" onClick={() => setOffset(0)} className="wy-chip">
            THIS WEEK
          </button>
        )}
      </div>

      <div className="flex items-center justify-between gap-2 px-2 py-1.5 border-b-2 border-(--color-line-strong)">
        <WeekArrow direction="previous" onClick={() => setOffset(offset - 1)} />
        <span className="flex flex-col items-center gap-0.5 text-center" aria-live="polite">
          <span className="font-extrabold text-[13px] tracking-[0.06em]">{weekLabel(days[0], days[6])}</span>
          <span className={`wy-lbl ${offset === 0 ? 'text-(--color-accent-700)' : ''}`}>
            {offset === 0
              ? 'THIS WEEK'
              : `${Math.abs(offset)} WEEK${Math.abs(offset) === 1 ? '' : 'S'} ${offset > 0 ? 'AHEAD' : 'AGO'}`}
          </span>
        </span>
        <WeekArrow direction="next" onClick={() => setOffset(offset + 1)} />
      </div>

      {plan.isLoading && <p className="px-4 py-4 wy-lbl">LOADING…</p>}

      {plan.data?.map((day, i) => {
        const on = !!day.look
        return (
          <button
            key={day.date}
            type="button"
            onClick={() => cycle(day.date, day.look?.id ?? null)}
            disabled={setDay.isPending}
            className="w-full grid items-center gap-3 min-h-16 px-4 py-3 text-left border-b"
            style={{
              gridTemplateColumns: '64px 1fr auto',
              borderColor: 'var(--color-line)',
              background: on ? 'var(--color-accent)' : 'var(--color-bg)',
              color: on ? 'var(--color-bg)' : 'var(--color-ink)',
            }}
          >
            <span className="flex flex-col">
              <span className="font-extrabold text-[15px] tracking-wide">{WEEKDAY_LABELS[i]}</span>
              <span className="text-[11px] font-semibold opacity-70">{days[i].getDate()}</span>
            </span>
            <span className="font-extrabold text-[19px] tracking-tight truncate">
              {day.look?.name?.toUpperCase() || 'UNPLANNED'}
            </span>
            <span className="flex gap-1">
              {(day.look?.items ?? []).slice(0, 4).map((it) => (
                <img
                  key={it.id}
                  src={`/api/items/${it.id}/thumb`}
                  alt=""
                  className="w-[18px] h-6 object-cover border"
                  style={{ borderColor: 'var(--color-ink)' }}
                />
              ))}
            </span>
          </button>
        )
      })}
    </div>
  )
}

function WeekArrow({ direction, onClick }: { direction: 'previous' | 'next'; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={`${direction === 'previous' ? 'Previous' : 'Next'} week`}
      className="wy-chip w-11 min-h-11 flex items-center justify-center"
    >
      <svg
        aria-hidden="true"
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="3"
        strokeLinecap="square"
      >
        <path d={direction === 'previous' ? 'M15 4 7 12l8 8' : 'M9 4l8 8-8 8'} />
      </svg>
    </button>
  )
}
