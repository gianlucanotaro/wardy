import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getPlan, listLooks, setPlanDay } from './api'

const WEEKDAY_LABELS = ['MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT', 'SUN']

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

// The PLAN tab: the current calendar week (Mon-Sun), tap a day to cycle
// through saved looks then back to unplanned — matching
// Wardy Prototype.dc.html's isPlan block, but backed by real plan_days
// rows instead of local state (see the plan's "PLAN is calendar-date-based,
// not day-of-week-template-based").
export default function Plan() {
  const queryClient = useQueryClient()
  const [message, setMessage] = useState<string | null>(null)

  const monday = mondayOf(new Date())
  const weekDates = Array.from({ length: 7 }, (_, i) => {
    const d = new Date(monday)
    d.setDate(d.getDate() + i)
    return isoDate(d)
  })
  const start = weekDates[0]
  const end = weekDates[6]

  const plan = useQuery({ queryKey: ['plan', start, end], queryFn: () => getPlan(start, end) })
  const looks = useQuery({ queryKey: ['looks'], queryFn: listLooks })

  const setDay = useMutation({
    mutationFn: (input: { date: string; lookId: number | null }) => setPlanDay(input.date, input.lookId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['plan', start, end] }),
  })

  function cycle(date: string, currentLookId: number | null) {
    const list = looks.data ?? []
    if (list.length === 0) {
      setMessage('SAVE A LOOK FIRST')
      return
    }
    setMessage(null)
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
        <span className="wy-lbl">TAP A DAY TO ASSIGN</span>
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
            <span className="font-extrabold text-[15px] tracking-wide">{WEEKDAY_LABELS[i]}</span>
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

      <p className="px-4 py-3 text-xs text-(--color-neutral-700) border-b-2 border-(--color-line-strong)">
        {looks.data && looks.data.length === 0
          ? 'Save a look from Today or Build, then plan the week around it.'
          : 'Tap a day to cycle through saved looks; tap past the last one to clear it.'}
      </p>
      {message && <p className="px-4 py-2 text-sm text-(--color-neutral-700)">{message}</p>}
    </div>
  )
}
