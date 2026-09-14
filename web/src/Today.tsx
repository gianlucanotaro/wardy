import { useEffect, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { createLook, getSuggestions, getWeather, listItems, logWear, todayISODate } from './api'
import { OCCASIONS } from './taxonomy'

// Suggestion context only — never a filter on the candidate set (see
// internal/http/suggest.go's "?mood= is optional prompt context for the AI
// rationale, not a filter"). Not part of taxonomy.ts since mood isn't a
// closed vocabulary attached to items, only to a suggestion request.
const MOODS = ['sharp', 'easy', 'bold', 'quiet']

// Matches Wardy Prototype.dc.html's d > 90 / d < -90 swipe-out threshold.
const SWIPE_THRESHOLD_PX = 90

const CONDITION_LABELS: Record<string, string> = {
  clear: 'CLEAR',
  cloudy: 'CLOUD',
  fog: 'FOG',
  drizzle: 'DRIZZLE',
  rain: 'RAIN',
  snow: 'SNOW',
  showers: 'SHOWERS',
  thunderstorm: 'STORM',
  unknown: 'UNKNOWN',
}

const WEEKDAY = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT']
const MONTH = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']

// The TODAY tab: weather + occasion/mood inputs above a single outfit card,
// matching Wardy Prototype.dc.html's isToday block — rebuilt from the old
// Suggest.tsx screen (same three states: normal/rules-fallback/no-match)
// with a button-driven deck instead of the mockup's pointer-drag swipe (see
// the plan's "implementation-level calls").
export default function Today() {
  const [occasion, setOccasion] = useState<string>('casual')
  const [mood, setMood] = useState<string>('')
  const [deckIndex, setDeckIndex] = useState(0)
  // Keyed by occasion+mood+index so switching inputs never shows a stale
  // "Saved"/"Logged" state for a different outfit that happens to share an
  // index.
  const [savedKeys, setSavedKeys] = useState<Set<string>>(new Set())
  const [loggedKeys, setLoggedKeys] = useState<Set<string>>(new Set())

  const now = new Date()
  const dateLabel = `${WEEKDAY[now.getDay()]} ${now.getDate()} ${MONTH[now.getMonth()]}`

  const items = useQuery({ queryKey: ['items'], queryFn: () => listItems() })
  const weather = useQuery({ queryKey: ['weather'], queryFn: getWeather })
  const suggestions = useQuery({
    queryKey: ['suggest', occasion, mood],
    queryFn: () => getSuggestions(occasion, mood || undefined),
  })

  const outfits = suggestions.data?.outfits ?? []
  const outfit = outfits.length > 0 ? outfits[deckIndex % outfits.length] : null
  const key = `${occasion}-${mood}-${deckIndex % Math.max(outfits.length, 1)}`
  const saved = savedKeys.has(key)
  const logged = loggedKeys.has(key)

  const saveAsLook = useMutation({
    mutationFn: (input: { key: string; itemIds: number[] }) =>
      createLook({
        name: `${occasion.charAt(0).toUpperCase()}${occasion.slice(1)} look`,
        item_ids: input.itemIds,
      }),
    onSuccess: (_look, input) => setSavedKeys((prev) => new Set(prev).add(input.key)),
  })

  const logAsWorn = useMutation({
    mutationFn: (input: { key: string; itemIds: number[] }) =>
      logWear({ item_ids: input.itemIds, worn_on: todayISODate(), occasion }),
    onSuccess: (_wear, input) => setLoggedKeys((prev) => new Set(prev).add(input.key)),
  })

  function selectInput(next: () => void) {
    next()
    setDeckIndex(0)
  }

  // Card drag — Wardy Prototype.dc.html's isToday pointer-drag: live
  // translateX/rotate while dragging, snap-out past SWIPE_THRESHOLD_PX,
  // spring back otherwise. dragStartX doubles as "is a drag in progress".
  const [dragStartX, setDragStartX] = useState<number | null>(null)
  const [dragX, setDragX] = useState(0)
  const [exiting, setExiting] = useState<'left' | 'right' | null>(null)
  const prefersReducedMotion = useState(
    () => window.matchMedia('(prefers-reduced-motion: reduce)').matches,
  )[0]

  // A new outfit (from the deck advancing, or occasion/mood changing) always
  // starts centered, never mid-drag or mid-exit from the card it replaced.
  useEffect(() => {
    setDragStartX(null)
    setDragX(0)
    setExiting(null)
  }, [key])

  // Fires the same action the buttons trigger, once the swipe-out animation
  // (skipped entirely under prefers-reduced-motion) has had time to play.
  useEffect(() => {
    if (!exiting || !outfit) return
    const itemIds = outfit.items.map((it) => it.id)
    const t = setTimeout(
      () => {
        if (exiting === 'right') {
          if (!logged && !logAsWorn.isPending) logAsWorn.mutate({ key, itemIds })
        } else {
          setDeckIndex((i) => i + 1)
        }
      },
      prefersReducedMotion ? 0 : 220,
    )
    return () => clearTimeout(t)
    // Deliberately keyed only on `exiting` — this fires exactly once per
    // left/right transition, using outfit/key/logged from that render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [exiting])

  function onCardPointerDown(e: React.PointerEvent<HTMLDivElement>) {
    if (exiting) return
    setDragStartX(e.clientX)
    e.currentTarget.setPointerCapture(e.pointerId)
  }

  function onCardPointerMove(e: React.PointerEvent<HTMLDivElement>) {
    if (dragStartX === null) return
    setDragX(e.clientX - dragStartX)
  }

  function onCardPointerUp() {
    if (dragStartX === null) return
    setDragStartX(null)
    if (dragX > SWIPE_THRESHOLD_PX) setExiting('right')
    else if (dragX < -SWIPE_THRESHOLD_PX) setExiting('left')
    else setDragX(0)
  }

  const cardOffset = exiting ? (exiting === 'right' ? 600 : -600) : dragX

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <span className="font-extrabold text-3xl tracking-tight">TODAY</span>
        <span className="wy-lbl">{dateLabel}</span>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 border-b-2 border-(--color-line-strong)">
        <div className="wy-cell">
          <div className="wy-lbl">TEMP</div>
          <div className="font-extrabold text-2xl tracking-tight">
            {weather.data?.available ? `${Math.round(weather.data.temp_high_c)}°/${Math.round(weather.data.temp_low_c)}°` : '—'}
          </div>
        </div>
        <div className="wy-cell">
          <div className="wy-lbl">CONDITION</div>
          <div className="font-extrabold text-2xl tracking-tight">
            {weather.data?.available
              ? `${CONDITION_LABELS[weather.data.condition] ?? weather.data.condition.toUpperCase()} · ${weather.data.precip_probability}%`
              : 'UNKNOWN'}
          </div>
        </div>
        <div className="wy-cell">
          <div className="wy-lbl">WARDROBE</div>
          <div className="font-extrabold text-2xl tracking-tight">{items.data?.length ?? 0} ITEMS</div>
        </div>
        <div className="wy-cell border-r-0" style={{ background: 'var(--color-accent)', color: 'var(--color-bg)' }}>
          <div className="wy-lbl" style={{ color: 'var(--color-bg)' }}>
            OCCASION
          </div>
          <div className="font-extrabold text-2xl tracking-tight uppercase">{occasion}</div>
        </div>
      </div>

      <div className="wy-sec flex justify-between px-4 py-2.5 border-b-2 border-(--color-line-strong)">
        <span className="wy-lbl">INPUTS</span>
        <span className="wy-lbl">TAP TO CHANGE</span>
      </div>
      <div className="px-4 py-3 border-b-2 border-(--color-line-strong) flex flex-col gap-3">
        <div>
          <div className="wy-lbl mb-1.5">OCCASION</div>
          <div className="flex flex-wrap gap-1.5">
            {OCCASIONS.map((o) => (
              <button
                key={o}
                type="button"
                className="wy-chip"
                data-on={o === occasion || undefined}
                onClick={() => selectInput(() => setOccasion(o))}
              >
                {o.toUpperCase()}
              </button>
            ))}
          </div>
        </div>
        <div>
          <div className="wy-lbl mb-1.5">MOOD</div>
          <div className="flex flex-wrap gap-1.5">
            {MOODS.map((m) => (
              <button
                key={m}
                type="button"
                className="wy-chip"
                data-on={m === mood || undefined}
                onClick={() => selectInput(() => setMood(m === mood ? '' : m))}
              >
                {m.toUpperCase()}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="flex justify-between px-4 py-2.5 border-b-2 border-(--color-line-strong) wy-lbl">
        <span>
          OUTFIT {outfits.length > 0 ? String((deckIndex % outfits.length) + 1).padStart(2, '0') : '00'} — RECOMMENDED
        </span>
        <span>{outfits.length > 0 ? `${(deckIndex % outfits.length) + 1}/${outfits.length}` : '0/0'}</span>
      </div>

      {suggestions.isLoading && <p className="px-4 py-4 wy-lbl">FINDING OUTFITS…</p>}
      {suggestions.isError && <p className="px-4 py-4 text-sm text-(--color-accent-700)">Couldn't load suggestions.</p>}

      {suggestions.data && !suggestions.data.weather_available && (
        <p className="px-4 py-2 text-xs text-(--color-neutral-700) border-b border-(--color-line)">
          Weather unavailable — suggestions don't account for today's temperature.
        </p>
      )}
      {suggestions.data?.source === 'rules_fallback' && (
        <p className="px-4 py-2 text-xs text-(--color-neutral-700) border-b border-(--color-line)">
          Showing simple matches — AI suggestions are unavailable right now.
        </p>
      )}

      {suggestions.isSuccess && !outfit && (
        <div className="px-4 py-8 border-b-2 border-(--color-line-strong)">
          <div className="font-extrabold text-xl mb-1.5">NOTHING MATCHES</div>
          <p className="text-sm max-w-[44ch]">
            {suggestions.data.no_match_reason || 'Nothing matches yet — tag a few more items.'}
          </p>
        </div>
      )}

      {outfit && (
        <div>
          <div
            onPointerDown={onCardPointerDown}
            onPointerMove={onCardPointerMove}
            onPointerUp={onCardPointerUp}
            onPointerCancel={onCardPointerUp}
            className="cursor-grab active:cursor-grabbing select-none"
            style={{
              transform: `translateX(${cardOffset}px) rotate(${cardOffset / 14}deg)`,
              transition: prefersReducedMotion || dragStartX !== null ? 'none' : 'transform 220ms ease-out',
              touchAction: 'pan-y',
            }}
          >
            <div className="grid grid-cols-2 sm:grid-cols-4 border-b-2 border-(--color-line-strong)">
              {outfit.items.map((item) => (
                <div key={item.id} className="wy-tile">
                  <div className="h-[130px] border border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
                    <img
                      src={`/api/items/${item.id}/thumb`}
                      alt={item.name || item.category}
                      className="w-full h-full object-cover"
                    />
                  </div>
                  <div className="font-extrabold text-xs mt-1.5 tracking-tight truncate">
                    {(item.name || item.subcategory || item.category).toUpperCase()}
                  </div>
                  <div className="text-[9.5px] text-(--color-neutral-700) tracking-wide truncate">
                    {item.color_primary?.toUpperCase()}
                  </div>
                </div>
              ))}
            </div>
            <p className="px-4 py-3 border-b-2 border-(--color-line-strong) text-sm leading-relaxed">{outfit.note}</p>
          </div>

          <div className="grid grid-cols-3 border-b-2 border-(--color-line-strong)">
            <button
              type="button"
              data-primary="1"
              disabled={logged || logAsWorn.isPending}
              onClick={() => logAsWorn.mutate({ key, itemIds: outfit.items.map((it) => it.id) })}
              className="wy-act"
            >
              {logged ? 'LOGGED ✓' : 'LOG AS WORN'}
            </button>
            <button
              type="button"
              disabled={saved || saveAsLook.isPending}
              onClick={() => saveAsLook.mutate({ key, itemIds: outfit.items.map((it) => it.id) })}
              className="wy-act border-l-2 border-(--color-line-strong)"
            >
              {saved ? 'SAVED ✓' : 'SAVE AS LOOK'}
            </button>
            <button
              type="button"
              onClick={() => setDeckIndex((i) => i + 1)}
              className="wy-act border-l-2 border-(--color-line-strong)"
            >
              NOT TODAY →
            </button>
          </div>
          <p className="px-4 py-2 text-xs text-(--color-neutral-700) border-b border-(--color-line)">
            Swipe the outfit left to skip, right to accept.
          </p>
        </div>
      )}
    </div>
  )
}
