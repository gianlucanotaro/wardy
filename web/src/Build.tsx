import { useEffect, useRef, useState } from 'react'
import { Cross } from './Icons'
import Shape from './Shape'
import { blockOf, shapeOf, tintOf } from './shapes'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createLook,
  getSuggestions,
  getWeather,
  listItems,
  MOODS,
  type Item,
  type SuggestOutfit,
  type SuggestSource,
} from './api'
import { useLocationSource, useWeatherCoords } from './location'
import PlanForSheet from './PlanForSheet'
import { OCCASIONS, type Category } from './taxonomy'
import { useToast } from './useToast'

// One slot per clothing category plus an accessories slot, matching the
// design's isBuild block (stacked slot rows, each with an inline picker
// strip). Client-only state — "SAVE AS LOOK" reuses the same createLook
// call Today.tsx and Looks.tsx already use, and SUGGEST OUTFIT the same
// /api/suggest.
//
// LAYER and DRESS only show once something is in them: a suggestion for a
// cold day can put a knit over the shirt, and one for a dress has no top
// or bottom. Picking by hand never needs them.
type SlotKey = 'outerwear' | 'layer' | 'top' | 'bottom' | 'dress' | 'shoes'
const SLOTS: { key: SlotKey; category: Category; label: string; onlyWhenFilled?: boolean }[] = [
  { key: 'outerwear', category: 'outerwear', label: 'OUTER' },
  { key: 'layer', category: 'top', label: 'LAYER', onlyWhenFilled: true },
  { key: 'top', category: 'top', label: 'TOP' },
  { key: 'bottom', category: 'bottom', label: 'BOTTOM' },
  { key: 'dress', category: 'dress', label: 'DRESS', onlyWhenFilled: true },
  { key: 'shoes', category: 'shoes', label: 'SHOES' },
]
type Slots = Partial<Record<SlotKey, Item>>

// Which categories Build can take — bags don't have a slot. Accessories
// hold several pieces at once (a belt and a watch), so they live outside
// SLOTS. Wardrobe's "Add to outfit" button keeps its own copy of this list
// (see Wardrobe.tsx) to avoid exporting a non-component value out of this
// file, which breaks fast refresh.
const BUILDABLE_CATEGORIES: string[] = ['outerwear', 'top', 'bottom', 'dress', 'shoes', 'accessory']

// Spreads a suggested outfit over the slots. Of two tops, the lighter one
// goes underneath and the warmer one is the layer over it; anything without
// a slot of its own (accessories, a bag) rides along as an accessory.
function placeOutfit(outfit: Item[]): { slots: Slots; accessories: Item[] } {
  const slots: Slots = {}
  const accessories: Item[] = []
  const tops = outfit.filter((it) => it.category === 'top').sort((a, b) => a.warmth_min - b.warmth_min)
  if (tops[0]) slots.top = tops[0]
  if (tops[1]) slots.layer = tops[1]
  for (const it of outfit) {
    if (it.category === 'outerwear' || it.category === 'bottom' || it.category === 'dress' || it.category === 'shoes') {
      slots[it.category] ??= it
    } else if (it.category !== 'top') {
      accessories.push(it)
    }
  }
  return { slots, accessories }
}

// suggest.MaxPromptLength (Go): the server rejects longer requests.
const PROMPT_MAX = 280

// The last few requests typed into the suggest sheet, newest first, as
// quick-pick chips. Per device, like the weather location: a convenience,
// so a blocked or empty localStorage just means no chips.
const RECENT_PROMPTS_KEY = 'wy-recent-prompts'
const RECENT_PROMPTS_MAX = 3

function readRecentPrompts(): string[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(RECENT_PROMPTS_KEY) ?? '[]')
    return Array.isArray(parsed) ? parsed.filter((p): p is string => typeof p === 'string').slice(0, RECENT_PROMPTS_MAX) : []
  } catch {
    return []
  }
}

function rememberPrompt(prompt: string): string[] {
  const next = [prompt, ...readRecentPrompts().filter((p) => p !== prompt)].slice(0, RECENT_PROMPTS_MAX)
  try {
    localStorage.setItem(RECENT_PROMPTS_KEY, JSON.stringify(next))
  } catch {
    // Private mode or blocked storage: no chips next time, nothing lost.
  }
  return next
}

// The suggestion currently shown in the slots: the full response, so NEXT
// can step through it without another API call, and whether the user has
// since changed a slot by hand.
type Suggestion = {
  occasion: string
  mood: string
  outfits: SuggestOutfit[]
  index: number
  source?: SuggestSource
  // The user's inspo style profile went into it (#49).
  styleUsed: boolean
  edited: boolean
}

function itemTitle(it: Item): string {
  return (it.name || it.subcategory || it.category).toUpperCase()
}

// A slot's row: thumbnail, label, what's in it, and the PICK/SWAP/CLOSE
// affordance. Tapping it opens the slot's picker strip below.
function SlotRow({
  label,
  category,
  thumb,
  title,
  badge,
  action,
  open,
  onToggle,
}: {
  label: string
  category: string
  thumb: Item | undefined
  title: string
  badge?: string
  action: string
  open: boolean
  onToggle: () => void
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-expanded={open}
      className="w-full grid grid-cols-[64px_minmax(0,1fr)_auto] items-center gap-3 min-h-24 px-4 py-3 text-left"
      style={{ background: tintOf(category) }}
    >
      <span
        className="wy-block w-16 h-[76px] grid place-items-center font-extrabold text-3xl text-(--color-ink)"
        style={thumb ? { background: blockOf(thumb.category) } : { border: `2px dashed ${blockOf(category)}` }}
      >
        {thumb ? (
          <img src={`/api/items/${thumb.id}/thumb`} alt={thumb.name || thumb.category} className="w-full h-full object-cover" />
        ) : (
          '+'
        )}
      </span>
      <span className="flex flex-col gap-0.5 min-w-0">
        <span className="wy-lbl flex items-center gap-1.5">
          <Shape name={shapeOf(thumb?.category ?? category)} size={10} />
          {label}
        </span>
        <span className="font-extrabold text-xl tracking-tight truncate lowercase">{title}</span>
        {badge && (
          <span className="self-start mt-0.5 bg-(--color-accent-100) text-(--color-accent-700) font-extrabold text-[8.5px] tracking-[0.1em] px-1.5 py-0.5">
            {badge}
          </span>
        )}
      </span>
      <span
        className={`font-extrabold text-[10px] tracking-[0.09em] ${open ? 'text-(--color-ink)' : 'text-(--color-accent-700)'}`}
      >
        {open ? 'CLOSE ▴' : action}
      </span>
    </button>
  )
}

// One piece in a slot's picker strip.
function PickCard({ item, picked, onPick }: { item: Item; picked: boolean; onPick: () => void }) {
  return (
    <button
      type="button"
      onClick={onPick}
      aria-pressed={picked}
      className="flex-none w-26 snap-start p-2.5 border-r border-(--color-line) text-left hover:bg-(--color-ink)/7"
    >
      <div
        className={`wy-block aspect-3/4 ${picked ? 'outline-3 -outline-offset-3 outline-(--color-accent)' : ''}`}
        style={{ background: blockOf(item.category) }}
      >
        <img
          src={`/api/items/${item.id}/thumb`}
          alt={item.name || item.category}
          className="w-full h-full object-cover"
          loading="lazy"
        />
      </div>
      <div className="font-extrabold text-[10.5px] leading-tight mt-1.5 truncate">{itemTitle(item)}</div>
      <div className="text-[9px] tracking-wide text-(--color-neutral-700) truncate">
        {item.color_primary.toUpperCase()}
      </div>
    </button>
  )
}

// Drag a slot sideways to swap its piece for the previous or next one of the
// same category (it wraps round). The PICK strip is the same choice by tap, so
// nothing depends on the gesture. A drag never counts as a tap on the row.
const SWIPE_PX = 56

function SwipeRow({ onSwipe, children }: { onSwipe: (dir: 1 | -1) => void; children: React.ReactNode }) {
  const [dx, setDx] = useState(0)
  const [dragging, setDragging] = useState(false)
  const startX = useRef<number | null>(null)
  const moved = useRef(false)
  const reduced = useRef(window.matchMedia('(prefers-reduced-motion: reduce)').matches)

  function end() {
    if (startX.current === null) return
    startX.current = null
    setDragging(false)
    if (Math.abs(dx) >= SWIPE_PX) onSwipe(dx < 0 ? 1 : -1)
    setDx(0)
  }

  return (
    <div
      className="overflow-hidden"
      style={{ touchAction: 'pan-y' }}
      onPointerDown={(e) => {
        startX.current = e.clientX
        moved.current = false
      }}
      onPointerMove={(e) => {
        if (startX.current === null) return
        const d = e.clientX - startX.current
        if (!moved.current && Math.abs(d) < 8) return
        if (!moved.current) e.currentTarget.setPointerCapture(e.pointerId)
        moved.current = true
        setDragging(true)
        setDx(d)
      }}
      onPointerUp={end}
      onPointerCancel={end}
      onClickCapture={(e) => {
        if (moved.current) {
          e.stopPropagation()
          moved.current = false
        }
      }}
    >
      <div
        style={{
          transform: `translateX(${dx}px)`,
          transition: reduced.current || dragging ? 'none' : 'transform 180ms ease-out',
        }}
      >
        {children}
      </div>
    </div>
  )
}

const STRIP_CLASS =
  'flex overflow-x-auto snap-x snap-mandatory border-t border-(--color-line) bg-(--color-surface)'
const STRIP_ACTION_CLASS =
  'flex-none w-26 p-2.5 text-left font-extrabold text-[10.5px] tracking-[0.06em] text-(--color-accent-700) hover:bg-(--color-ink)/7'

export default function Build({
  pendingItem,
  onConsumePendingItem,
}: {
  pendingItem?: Item | null
  onConsumePendingItem?: () => void
}) {
  const queryClient = useQueryClient()
  const items = useQuery({ queryKey: ['items'], queryFn: () => listItems() })
  const [slots, setSlots] = useState<Slots>({})
  const [accessories, setAccessories] = useState<Item[]>([])
  const [pickSlot, setPickSlot] = useState<SlotKey | 'accessory' | null>(null)
  const flash = useToast()

  const [sheetOpen, setSheetOpen] = useState(false)
  const [planForOpen, setPlanForOpen] = useState(false)
  const [occasion, setOccasion] = useState<string>('casual')
  const [mood, setMood] = useState<string>('')
  const [prompt, setPrompt] = useState('')
  const [recentPrompts, setRecentPrompts] = useState<string[]>(readRecentPrompts)
  const trimmedPrompt = prompt.trim().replace(/\s+/g, ' ')
  const [suggestion, setSuggestion] = useState<Suggestion | null>(null)
  const [noMatch, setNoMatch] = useState<string | null>(null)

  // Same location and weather as Today, so the sheet can say what weather
  // the suggestion will be dressed for.
  const [locationSource] = useLocationSource()
  const { coords, ready } = useWeatherCoords(locationSource)
  const weather = useQuery({
    queryKey: ['weather', coords],
    queryFn: () => getWeather(coords),
    enabled: sheetOpen && ready,
  })

  function markEdited() {
    setSuggestion((prev) => (prev && !prev.edited ? { ...prev, edited: true } : prev))
  }

  function showOutfit(outfit: SuggestOutfit) {
    const placed = placeOutfit(outfit.items)
    setSlots(placed.slots)
    setAccessories(placed.accessories)
    setPickSlot(null)
  }

  const suggest = useMutation({
    mutationFn: () => getSuggestions(occasion, mood || undefined, coords, trimmedPrompt || undefined),
    onSuccess: (res) => {
      setSheetOpen(false)
      if (trimmedPrompt) setRecentPrompts(rememberPrompt(trimmedPrompt))
      if (res.outfits.length === 0) {
        setNoMatch(res.no_match_reason ?? 'Nothing in your wardrobe matches right now.')
        return
      }
      setNoMatch(null)
      showOutfit(res.outfits[0])
      setSuggestion({
        occasion,
        mood,
        outfits: res.outfits,
        index: 0,
        source: res.source,
        styleUsed: Boolean(res.style_used),
        edited: false,
      })
    },
    onError: () => flash("COULDN'T SUGGEST — TRY AGAIN"),
  })

  function nextSuggestion() {
    if (!suggestion) return
    const index = (suggestion.index + 1) % suggestion.outfits.length
    showOutfit(suggestion.outfits[index])
    setSuggestion({ ...suggestion, index, edited: false })
  }

  // Pre-fill the matching slot from Wardrobe's "Add to outfit", overwriting
  // whatever was there — the more recent explicit user intent wins. Consume
  // immediately so this doesn't re-fire on the next re-render.
  useEffect(() => {
    if (!pendingItem) return
    if (pendingItem.category === 'accessory') {
      setAccessories((prev) => (prev.some((it) => it.id === pendingItem.id) ? prev : [...prev, pendingItem]))
    } else if (BUILDABLE_CATEGORIES.includes(pendingItem.category)) {
      setSlots((prev) => ({ ...prev, [pendingItem.category as SlotKey]: pendingItem }))
    }
    markEdited()
    onConsumePendingItem?.()
  }, [pendingItem, onConsumePendingItem])

  // Clothes drive the formality/warmth read-out; accessories ride along in
  // the saved look but don't move the averages (most fit any weather).
  const visibleSlots = SLOTS.filter((s) => !s.onlyWhenFilled || slots[s.key])
  const clothes = visibleSlots.map((s) => slots[s.key]).filter((it): it is Item => !!it)
  const filled = [...clothes, ...accessories]
  const filledSlots = clothes.length + (accessories.length > 0 ? 1 : 0)

  function toggleAccessory(it: Item) {
    setAccessories((prev) => (prev.some((a) => a.id === it.id) ? prev.filter((a) => a.id !== it.id) : [...prev, it]))
    markEdited()
  }

  // Swipe on a slot: the previous or next piece of its category.
  function swapSlot(slot: (typeof SLOTS)[number], dir: 1 | -1) {
    const options = items.data?.filter((it) => it.category === slot.category) ?? []
    const current = slots[slot.key]
    if (options.length === 0 || (options.length === 1 && current)) return
    const at = options.findIndex((it) => it.id === current?.id)
    const next = at < 0 ? (dir > 0 ? 0 : options.length - 1) : (at + dir + options.length) % options.length
    setSlots((prev) => ({ ...prev, [slot.key]: options[next] }))
    setPickSlot(null)
    markEdited()
  }

  function clearAll() {
    setSlots({})
    setAccessories([])
    setPickSlot(null)
    setSuggestion(null)
    setNoMatch(null)
  }

  // A suggested look is named after what it was asked for ("Bold date
  // look"); one built by hand stays a "Custom look".
  const lookName = suggestion
    ? `${[suggestion.mood, suggestion.occasion].filter(Boolean).join(' ')} look`.replace(/^./, (c) => c.toUpperCase())
    : 'Custom look'

  const saveAsLook = useMutation({
    mutationFn: (input: { itemIds: number[] }) => createLook({ name: lookName, item_ids: input.itemIds }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['looks'] })
      clearAll()
      flash('SAVED TO LOOKS')
    },
  })

  function save() {
    if (filled.length < 2) {
      flash('PICK AT LEAST TWO PIECES')
      return
    }
    saveAsLook.mutate({ itemIds: filled.map((it) => it.id) })
  }

  function openPlanFor() {
    if (filled.length < 2) {
      flash('PICK AT LEAST TWO PIECES')
      return
    }
    setPlanForOpen(true)
  }

  // "LIKE 6 OF YOUR INSPO FITS" on a suggested piece that several of the
  // user's inspo fits show (same kind of piece, same colour). One match
  // could be chance, so it takes two.
  function inspoBadge(it: Item): string | undefined {
    const n = suggestion?.outfits[suggestion.index].inspo_matches?.[it.id] ?? 0
    return n >= 2 ? `LIKE ${n} OF YOUR INSPO FITS` : undefined
  }

  // Each piece counts at the middle of its formality/warmth range.
  const avgFormality = clothes.length
    ? clothes.reduce((sum, it) => sum + (it.formality_min + it.formality_max) / 2, 0) / clothes.length
    : null
  const avgWarmth = clothes.length
    ? clothes.reduce((sum, it) => sum + (it.warmth_min + it.warmth_max) / 2, 0) / clothes.length
    : null

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 pt-4 pb-3">
        <span className="wy-display text-5xl">BUILD</span>
        <span className="wy-lbl text-right">
          {filledSlots}/{visibleSlots.length + 1} SLOTS
          <br />← SWIPE A SLOT TO SWAP →
        </span>
      </div>

      {suggestion ? (
        <div className="grid grid-cols-[1fr_auto] border-b-2 border-(--color-line-strong)">
          <button
            type="button"
            data-primary="1"
            onClick={() => setSheetOpen(true)}
            className="wy-act flex items-center justify-between"
          >
            SUGGEST AGAIN <span aria-hidden="true">→</span>
          </button>
          <button
            type="button"
            onClick={nextSuggestion}
            disabled={suggestion.outfits.length < 2}
            className="wy-act"
          >
            NEXT {suggestion.index + 1}/{suggestion.outfits.length}
          </button>
        </div>
      ) : (
        <div className="border-b border-(--color-line)">
          <button
            type="button"
            data-primary="1"
            onClick={() => setSheetOpen(true)}
            className="wy-act w-full flex items-center justify-between"
          >
            SUGGEST OUTFIT <span aria-hidden="true">→</span>
          </button>
        </div>
      )}

      {/* One row per slot; tapping a row opens a strip of matching pieces
          right under it (only one open at a time), so picking never moves
          the user away from the slot they're filling. */}
      <div className="flex flex-col gap-1.5 px-4 py-3 border-b border-(--color-line)">
        {visibleSlots.map((slot) => {
          const item = slots[slot.key]
          const open = pickSlot === slot.key
          const options = open ? (items.data?.filter((it) => it.category === slot.category) ?? []) : []
          return (
            <div key={slot.key}>
              <SwipeRow onSwipe={(dir) => swapSlot(slot, dir)}>
                <SlotRow
                  label={slot.label}
                  category={slot.category}
                  thumb={item}
                  title={item ? itemTitle(item) : 'EMPTY'}
                  badge={item ? inspoBadge(item) : undefined}
                  action={item ? 'SWAP ▾' : 'PICK ▾'}
                  open={open}
                  onToggle={() => setPickSlot(open ? null : slot.key)}
                />
              </SwipeRow>
              {open && (
                <div className={STRIP_CLASS}>
                  {options.map((it) => (
                    <PickCard
                      key={it.id}
                      item={it}
                      picked={item?.id === it.id}
                      onPick={() => {
                        setSlots((prev) => ({ ...prev, [slot.key]: it }))
                        setPickSlot(null)
                        markEdited()
                      }}
                    />
                  ))}
                  {options.length === 0 && (
                    <p className="px-4 py-4 text-sm">No {slot.category} items in your wardrobe yet.</p>
                  )}
                  {item && (
                    <button
                      type="button"
                      onClick={() => {
                        setSlots((prev) => {
                          const next = { ...prev }
                          delete next[slot.key]
                          return next
                        })
                        setPickSlot(null)
                        markEdited()
                      }}
                      className={STRIP_ACTION_CLASS}
                    >
                      <Cross /> EMPTY SLOT
                    </button>
                  )}
                </div>
              )}
            </div>
          )
        })}

        {/* Accessories take several pieces: tapping one in the strip adds
            or removes it, and the strip stays open until CLOSE. */}
        {(() => {
          const open = pickSlot === 'accessory'
          const options = open ? (items.data?.filter((it) => it.category === 'accessory') ?? []) : []
          return (
            <div>
              <SlotRow
                label={accessories.length > 1 ? `ACCESSORIES · ${accessories.length}` : 'ACCESSORIES'}
                category="accessory"
                thumb={accessories[0]}
                title={accessories.length ? accessories.map(itemTitle).join(' + ') : 'NONE'}
                action={accessories.length ? 'EDIT ▾' : 'ADD ▾'}
                open={open}
                onToggle={() => setPickSlot(open ? null : 'accessory')}
              />
              {open && (
                <div className={STRIP_CLASS}>
                  {options.map((it) => (
                    <PickCard
                      key={it.id}
                      item={it}
                      picked={accessories.some((a) => a.id === it.id)}
                      onPick={() => toggleAccessory(it)}
                    />
                  ))}
                  {options.length === 0 && (
                    <p className="px-4 py-4 text-sm">No accessories in your wardrobe yet.</p>
                  )}
                  {accessories.length > 0 && (
                    <button
                      type="button"
                      onClick={() => {
                        setAccessories([])
                        markEdited()
                      }}
                      className={STRIP_ACTION_CLASS}
                    >
                      <Cross /> REMOVE ALL
                    </button>
                  )}
                </div>
              )}
            </div>
          )
        })()}
      </div>

      {suggestion && (
        <div className="px-4 py-3 border-b border-(--color-line)">
          <div className="wy-lbl" style={{ color: 'var(--color-accent-700)' }}>
            {[
              suggestion.source === 'ai' ? 'AI' : 'SIMPLE MATCH',
              suggestion.occasion,
              suggestion.mood,
              suggestion.styleUsed ? 'YOUR STYLE' : '',
              suggestion.edited ? 'EDITED' : '',
            ]
              .filter(Boolean)
              .join(' · ')}
          </div>
        </div>
      )}
      {noMatch && (
        <div className="px-4 py-3 border-b border-(--color-line)">
          <div className="wy-lbl mb-1">NO OUTFIT FOUND</div>
          <p className="text-sm leading-relaxed">{noMatch}</p>
        </div>
      )}

      {clothes.length > 0 && (
        <p className="px-4 py-3 border-b border-(--color-line) wy-lbl">
          FORMALITY {avgFormality?.toFixed(1)} · WARMTH {avgWarmth?.toFixed(1)}
        </p>
      )}

      <div className="wy-actions grid grid-cols-3 border-b-2 border-(--color-line-strong)">
        <button type="button" data-primary="1" onClick={save} disabled={saveAsLook.isPending} className="wy-act">
          {saveAsLook.isPending ? 'SAVING…' : 'SAVE AS LOOK'}
        </button>
        <button type="button" onClick={openPlanFor} className="wy-act">
          PLAN FOR…
        </button>
        <button
          type="button"
          onClick={clearAll}
          className="wy-act"
        >
          CLEAR SLOTS
        </button>
      </div>

      {planForOpen && (
        <PlanForSheet
          itemIds={filled.map((it) => it.id)}
          namePrefix="BUILD"
          onClose={() => setPlanForOpen(false)}
          onPlanned={() => {
            setPlanForOpen(false)
            clearAll()
          }}
        />
      )}

      {/* The suggest sheet: occasion, mood and an optional request in the
          user's own words (context for the AI, never a filter), and the
          weather it will dress for. Above the tab bar. */}
      {sheetOpen && (
        <div className="fixed inset-0 z-20 flex flex-col justify-end" role="dialog" aria-label="Suggest outfit">
          <button
            type="button"
            aria-label="Close"
            className="absolute inset-0 bg-(--color-ink)/45"
            onClick={() => setSheetOpen(false)}
          />
          <div className="relative bg-(--color-bg) border-t border-(--color-line) safe-area-overlay max-h-full overflow-y-auto">
            <div className="flex items-center justify-between px-4 py-3 border-b border-(--color-line)">
              <span className="wy-display text-3xl">suggest outfit</span>
              <button type="button" onClick={() => setSheetOpen(false)} className="wy-chip">
                <Cross /> CLOSE
              </button>
            </div>
            <div className="px-4 py-3 border-b border-(--color-line)">
              <div className="wy-lbl mb-1.5">OCCASION</div>
              <div className="flex flex-wrap gap-1.5">
                {OCCASIONS.map((o) => (
                  <button
                    key={o}
                    type="button"
                    className="wy-chip"
                    data-on={o === occasion || undefined}
                    aria-pressed={o === occasion}
                    onClick={() => setOccasion(o)}
                  >
                    {o.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
            <div className="px-4 py-3 border-b border-(--color-line)">
              <div className="wy-lbl mb-1.5">MOOD</div>
              <div className="flex flex-wrap gap-1.5">
                {MOODS.map((m) => (
                  <button
                    key={m}
                    type="button"
                    className="wy-chip"
                    data-on={m === mood || undefined}
                    aria-pressed={m === mood}
                    onClick={() => setMood(m === mood ? '' : m)}
                  >
                    {m.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
            <div className="px-4 py-3 border-b border-(--color-line)">
              <label htmlFor="suggest-prompt" className="wy-lbl block mb-1.5">
                ANYTHING ELSE? <span className="font-normal normal-case tracking-normal">— optional</span>
              </label>
              <textarea
                id="suggest-prompt"
                rows={3}
                maxLength={PROMPT_MAX}
                placeholder="e.g. dinner with my parents, smart but not stiff"
                className="wy-input w-full resize-none px-3 py-2 text-base"
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
              />
              <div className="flex items-center justify-between mt-1.5">
                <span className="wy-lbl">{recentPrompts.length > 0 ? 'RECENT' : ''}</span>
                <span className="wy-lbl" aria-live="polite">
                  {prompt.length}/{PROMPT_MAX}
                </span>
              </div>
              {recentPrompts.length > 0 && (
                <div className="flex flex-wrap gap-1.5 mt-1.5">
                  {recentPrompts.map((p) => (
                    <button
                      key={p}
                      type="button"
                      className="wy-chip max-w-full truncate"
                      aria-pressed={p === trimmedPrompt}
                      data-on={p === trimmedPrompt || undefined}
                      onClick={() => setPrompt(p === trimmedPrompt ? '' : p)}
                      title={p}
                    >
                      {p.toUpperCase()}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <div className="px-4 py-2.5 wy-lbl">
              {!ready || weather.isLoading
                ? 'CHECKING THE WEATHER…'
                : weather.data?.available
                  ? `${Math.round(weather.data.temp_high_c)}°/${Math.round(weather.data.temp_low_c)}° · ${coords ? 'HERE' : 'HOME'}`
                  : 'NO WEATHER'}
            </div>
            <div className="border-t border-(--color-line)">
              <button
                type="button"
                data-primary="1"
                onClick={() => suggest.mutate()}
                disabled={suggest.isPending || !ready}
                className="wy-act w-full flex items-center justify-between"
              >
                {suggest.isPending ? 'FINDING AN OUTFIT…' : 'SUGGEST'} <span aria-hidden="true">→</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
