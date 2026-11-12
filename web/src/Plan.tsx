import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { getPlan, listLooks, setPlanDay, todayISODate, type PlanDay } from './api'
import PlanWorn from './PlanWorn'
import { lastWear, useWornLabel } from './planWear'
import { forecastLabel, useDayForecast } from './dayForecast'
import PlanForecast from './PlanForecast'
import PlanPicker from './PlanPicker'
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

// fromISODate turns "2026-11-09" into a local-midnight Date (new Date() on
// the string would parse it as UTC and can land on the previous day).
function fromISODate(iso: string): Date {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

// The PLAN tab: one calendar week (Mon-Sun), this week by default with
// previous/next arrows to page through others (#21), tap a day to pick its
// look from a sheet (#76) — backed by real plan_days rows (see the plan's
// "PLAN is calendar-date-based, not day-of-week-template-based").
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
  const today = todayISODate()
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
    onError: () => flash("COULDN'T SAVE THE PLAN"),
  })

  // The day whose look picker sheet is open.
  const [picking, setPicking] = useState<PlanDay | null>(null)

  function openPicker(day: PlanDay) {
    if ((looks.data ?? []).length === 0) {
      flash('SAVE A LOOK FIRST')
      return
    }
    setPicking(day)
  }

  function choose(date: string, lookId: number | null) {
    setDay.mutate({ date, lookId })
    setPicking(null)
  }

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <span className="font-extrabold text-3xl tracking-tight">THE WEEK</span>
        {/* Always rendered, hidden on this week, so the header keeps its
            height and the cards don't jump while paging (#77). */}
        <button
          type="button"
          onClick={() => setOffset(0)}
          className={`wy-chip ${offset === 0 ? 'invisible' : ''}`}
        >
          THIS WEEK
        </button>
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

      {plan.isError && <p className="px-4 py-4 text-sm text-(--color-accent-700)">Couldn't load the plan.</p>}

      {plan.isLoading && days.map((d, i) => <DaySkeleton key={i} date={d} today={today} last={i === 6} />)}

      {plan.data?.map((day, i) => (
        <DayCard
          key={day.date}
          day={day}
          today={today}
          last={i === plan.data.length - 1}
          busy={setDay.isPending && setDay.variables?.date === day.date}
          disabled={setDay.isPending}
          onClick={() => openPicker(day)}
        />
      ))}

      {picking && (
        <PlanPicker
          date={fromISODate(picking.date)}
          looks={looks.data ?? []}
          currentLookId={picking.look?.id ?? null}
          onPick={(lookId) => choose(picking.date, lookId)}
          onClear={() => choose(picking.date, null)}
          onClose={() => setPicking(null)}
        />
      )}
    </div>
  )
}

// Every card is the same 112px tall whatever its state, so paging weeks
// and the first load don't shift anything below it (#77).
const CARD = 'w-full h-28 grid items-start gap-3 px-4 py-3 text-left'
const CARD_COLUMNS = { gridTemplateColumns: '64px 1fr' }

// The 3:4 thumbnail slot, shared by thumbnails, the "+N" tile and the
// skeleton blocks.
const THUMB_SIZE = 'w-[45px] h-[60px] shrink-0'
const THUMB = `${THUMB_SIZE} border border-(--color-line-strong)`

// Hairline between cards, strong rule under the last one.
function cardBorder(last: boolean): string {
  return last ? 'border-b-2 border-(--color-line-strong)' : 'border-b border-(--color-line)'
}

// Today: tinted ground plus a 4px accent bar, drawn as an inset shadow so
// it doesn't change the box.
function todayStyle(isToday: boolean) {
  return isToday
    ? { background: 'var(--color-accent-100)', boxShadow: 'inset 4px 0 0 var(--color-accent)' }
    : { background: 'var(--color-bg)' }
}

function DateColumn({ date, isToday, isPast }: { date: Date; isToday: boolean; isPast: boolean }) {
  const color = isToday ? 'text-(--color-accent-700)' : isPast ? 'text-(--color-neutral-700)' : ''
  return (
    <span className={`flex flex-col ${color}`}>
      <span className="font-extrabold text-[13px] tracking-[0.08em]">{WEEKDAY_LABELS[(date.getDay() + 6) % 7]}</span>
      <span className="font-extrabold text-[30px] leading-[30px] tracking-[-0.02em] tabular-nums">{date.getDate()}</span>
      {isToday && <span className="wy-lbl mt-1 text-(--color-accent-700)!">TODAY</span>}
    </span>
  )
}

function DayCard({
  day,
  today,
  last,
  busy,
  disabled,
  onClick,
}: {
  day: PlanDay
  today: string
  last: boolean
  busy: boolean
  disabled: boolean
  onClick: () => void
}) {
  const date = fromISODate(day.date)
  const isToday = day.date === today
  const isPast = day.date < today
  // Past days show what was worn, in colour, when a wear was logged (#78).
  const wear = isPast ? lastWear(day) : undefined
  const items = wear?.items ?? day.look?.items ?? []
  const wornLabel = useWornLabel(day)
  const forecast = useDayForecast(day.date, !isPast)
  // Five slots: up to five thumbs, or four plus a "+N" tile for the rest.
  const shown = items.length > 5 ? items.slice(0, 4) : items
  const more = items.length - shown.length

  const spoken = date.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })
  let label = `${isToday ? 'Today, ' : ''}${spoken}, ${forecast ? `${forecastLabel(forecast)}, ` : ''}`
  if (isPast) label += `past, ${wornLabel}`
  else if (day.look) label += `${day.look.name}. Tap to change`
  else label += 'unplanned. Tap to plan a look'

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      aria-busy={busy}
      className={`${CARD} ${cardBorder(last)} focus-visible:outline-2 focus-visible:outline-(--color-line-strong) focus-visible:-outline-offset-2`}
      style={{ ...CARD_COLUMNS, ...todayStyle(isToday) }}
    >
      <DateColumn date={date} isToday={isToday} isPast={isPast} />
      <span className="min-w-0 flex flex-col gap-1.5">
        {/* The forecast sits at the right end of this row (#80). */}
        <span className="h-5 flex items-center justify-between gap-2">
          {isPast ? (
            <PlanWorn day={day} />
          ) : day.look ? (
            <span className="font-extrabold text-[15px] leading-5 tracking-[-0.01em] truncate">
              {day.look.name.toUpperCase()}
            </span>
          ) : (
            <span className="wy-lbl text-[11px]!">UNPLANNED</span>
          )}
          {forecast && <PlanForecast forecast={forecast} />}
        </span>
        {/* Five thumbs need ~241px; on screens under ~360px the row clips
            rather than widening the card. */}
        <span className="h-[60px] flex gap-1 overflow-hidden">
          {shown.map((it) => (
            <img
              key={it.id}
              src={`/api/items/${it.id}/thumb`}
              alt=""
              loading="lazy"
              className={`${THUMB} object-cover bg-(--color-surface) ${isPast && !wear ? 'grayscale opacity-55' : ''}`}
            />
          ))}
          {more > 0 && (
            <span className={`${THUMB} flex items-center justify-center bg-(--color-bg) font-extrabold text-[12px]`}>
              +{more}
            </span>
          )}
          {!day.look && !isPast && (
            <span className="flex-1 flex items-center justify-center border border-dashed border-(--color-line-strong) font-extrabold text-[10.5px] tracking-[0.08em] text-(--color-neutral-700)">
              + PLAN A LOOK
            </span>
          )}
        </span>
      </span>
    </button>
  )
}

// First load only (paging keeps the previous week on screen): the real
// dates, with grey blocks where the look will go, at the card's height.
function DaySkeleton({ date, today, last }: { date: Date; today: string; last: boolean }) {
  const iso = isoDate(date)
  return (
    <div
      aria-hidden="true"
      className={`${CARD} ${cardBorder(last)}`}
      style={{ ...CARD_COLUMNS, ...todayStyle(iso === today) }}
    >
      <DateColumn date={date} isToday={iso === today} isPast={iso < today} />
      <span className="min-w-0 flex flex-col gap-1.5 animate-pulse motion-reduce:animate-none">
        <span className="h-5 flex items-center">
          <span className="w-[120px] h-3 bg-(--color-surface)" />
        </span>
        <span className="h-[60px] flex gap-1">
          {[0, 1, 2, 3].map((n) => (
            <span key={n} className={`${THUMB_SIZE} bg-(--color-surface)`} />
          ))}
        </span>
      </span>
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
