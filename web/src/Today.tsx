import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { Item } from './api'
import { createLook, getPlan, getSuggestions, getWeather, listItems, listWears, logWear, MOODS, todayISODate } from './api'
import { useLocationSource, useWeatherCoords } from './location'
import OutfitStyle from './OutfitStyle'
import { OCCASIONS } from './taxonomy'
import { useToast } from './useToast'

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
  const flash = useToast()
  const queryClient = useQueryClient()
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

  // A look planned for today (#79) comes first; the suggestion deck is the
  // alternative, fetched only once asked for so a planned day doesn't spend
  // an AI suggestion nobody looks at. A failed plan load degrades to the
  // usual suggestions.
  const today = todayISODate()
  const plan = useQuery({ queryKey: ['plan', today, today], queryFn: () => getPlan(today, today) })
  const plannedLook = plan.data?.[0]?.look ?? null
  const [showSuggestions, setShowSuggestions] = useState(false)
  const planSettled = plan.isSuccess || plan.isError
  const showPlanned = plannedLook !== null && !showSuggestions
  const wears = useQuery({ queryKey: ['wears'], queryFn: listWears, enabled: plannedLook !== null })
  const wearPlanned = useMutation({
    mutationFn: (lookId: number) => logWear({ look_id: lookId, worn_on: today }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wears'] })
      queryClient.invalidateQueries({ queryKey: ['history'] })
      queryClient.invalidateQueries({ queryKey: ['plan'] })
      flash('LOGGED AS WORN TODAY')
    },
  })
  // The server also logs a look at most once a day, so a double tap that
  // slips past the disabled button still can't log it twice.
  const plannedWorn =
    plannedLook !== null &&
    (wearPlanned.isSuccess || (wears.data ?? []).some((w) => w.look_id === plannedLook.id && w.worn_on === today))

  // Both wait for the browser's answer on location, so they fetch once for
  // here rather than once for home and again for here.
  const [locationSource] = useLocationSource()
  const { coords, ready } = useWeatherCoords(locationSource)
  const weather = useQuery({ queryKey: ['weather', coords], queryFn: () => getWeather(coords), enabled: ready })
  const suggestions = useQuery({
    queryKey: ['suggest', occasion, mood, coords],
    queryFn: () => getSuggestions(occasion, mood || undefined, coords),
    enabled: ready && planSettled && !showPlanned,
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
    onSuccess: (_look, input) => {
      setSavedKeys((prev) => new Set(prev).add(input.key))
      flash('SAVED TO LOOKS')
    },
  })

  const logAsWorn = useMutation({
    mutationFn: (input: { key: string; itemIds: number[] }) =>
      logWear({ item_ids: input.itemIds, worn_on: todayISODate(), occasion }),
    onSuccess: (_wear, input) => {
      setLoggedKeys((prev) => new Set(prev).add(input.key))
      flash('LOGGED AS WORN TODAY')
    },
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
          <div className="wy-lbl">{weather.data?.available ? (coords ? 'TEMP · HERE' : 'TEMP · HOME') : 'TEMP'}</div>
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

      {!planSettled && <p className="px-4 py-4 wy-lbl">LOADING…</p>}

      {showPlanned && plannedLook && (
        <div>
          <div
            className="flex justify-between gap-3 px-4 py-2.5 border-b-2 border-(--color-line-strong) wy-lbl"
            style={{ background: 'var(--color-accent-100)', color: 'var(--color-accent-700)' }}
          >
            <span>PLANNED FOR TODAY</span>
            <span className="truncate">{plannedLook.name.toUpperCase()}</span>
          </div>
          <OutfitTiles items={plannedLook.items} />
          <OutfitStyle key={`plan-${plannedLook.id}`} items={plannedLook.items} />
          <div className="grid grid-cols-2 border-b-2 border-(--color-line-strong)">
            <button
              type="button"
              data-primary="1"
              disabled={plannedWorn || wearPlanned.isPending}
              onClick={() => wearPlanned.mutate(plannedLook.id)}
              className="wy-act"
            >
              {plannedWorn ? 'WORN ✓' : 'WEAR THIS'}
            </button>
            <button
              type="button"
              onClick={() => setShowSuggestions(true)}
              className="wy-act border-l-2 border-(--color-line-strong)"
            >
              SUGGEST INSTEAD
            </button>
          </div>
          {wearPlanned.isError && (
            <p className="px-4 py-2 text-sm text-(--color-accent-700)">Couldn't log the wear. Try again.</p>
          )}
        </div>
      )}

      {planSettled && !showPlanned && (
        <>
          {plannedLook && (
            <button
              type="button"
              onClick={() => setShowSuggestions(false)}
              className="wy-act text-left border-b-2 border-(--color-line-strong)"
              style={{ background: 'var(--color-accent-100)', color: 'var(--color-accent-700)' }}
            >
              ← BACK TO TODAY'S PLAN · {plannedLook.name.toUpperCase()}
            </button>
          )}
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
              OUTFIT {outfits.length > 0 ? String((deckIndex % outfits.length) + 1).padStart(2, '0') : '00'}
            </span>
            <span>{outfits.length > 0 ? `${(deckIndex % outfits.length) + 1}/${outfits.length}` : '0/0'}</span>
          </div>

          {(!ready || suggestions.isLoading) && <p className="px-4 py-4 wy-lbl">FINDING OUTFITS…</p>}
          {suggestions.isError && <p className="px-4 py-4 text-sm text-(--color-accent-700)">Couldn't load suggestions.</p>}

          {suggestions.data && !suggestions.data.weather_available && (
            <p className="px-4 py-2 wy-lbl border-b border-(--color-line)">
              {weather.data?.available === false && weather.data.reason === 'no_location_set' ? 'NO LOCATION' : 'NO WEATHER'}
            </p>
          )}
          {suggestions.data?.source === 'rules_fallback' && (
            <p className="px-4 py-2 wy-lbl border-b border-(--color-line)">SIMPLE MATCH</p>
          )}

          {suggestions.isSuccess && !outfit && (
            <div className="px-4 py-8 border-b-2 border-(--color-line-strong)">
              <div className="font-extrabold text-xl mb-1.5">NOTHING MATCHES</div>
              {suggestions.data.no_match_reason && <p className="text-sm max-w-[44ch]">{suggestions.data.no_match_reason}</p>}
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
                <OutfitTiles items={outfit.items} />
              </div>

              <OutfitStyle key={key} items={outfit.items} />

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
            </div>
          )}
        </>
      )}
    </div>
  )
}

// The outfit's pieces as a grid of photo tiles, shared by the planned look
// and the suggestion deck.
function OutfitTiles({ items }: { items: Item[] }) {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 border-b-2 border-(--color-line-strong)">
      {items.map((item) => (
        <div key={item.id} className="wy-tile">
          <div className="h-[130px] border border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
            <img src={`/api/items/${item.id}/thumb`} alt={item.name || item.category} className="w-full h-full object-cover" />
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
  )
}
