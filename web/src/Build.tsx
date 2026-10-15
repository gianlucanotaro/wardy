import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createLook, listItems, type Item } from './api'
import { FORMALITY_LABELS, WARMTH_LABELS, type Category } from './taxonomy'
import { useToast } from './useToast'

// One slot per clothing category plus an accessories slot, matching the
// design's isBuild block (stacked slot rows, each with an inline picker
// strip). Client-only state — no new backend endpoint, "SAVE AS LOOK"
// reuses the same createLook call Today.tsx and Looks.tsx already use.
const SLOTS: { category: Category; label: string }[] = [
  { category: 'outerwear', label: 'OUTER' },
  { category: 'top', label: 'TOP' },
  { category: 'bottom', label: 'BOTTOM' },
  { category: 'shoes', label: 'SHOES' },
]

// Which categories Build can take — dress/bag don't have a slot.
// Accessories hold several pieces at once (a belt and a watch), so they
// live outside SLOTS. Wardrobe's "Add to outfit" button keeps its own copy
// of this list (see Wardrobe.tsx) to avoid exporting a non-component value
// out of this file, which breaks fast refresh.
const BUILDABLE_CATEGORIES: string[] = [...SLOTS.map((s) => s.category), 'accessory']

function itemTitle(it: Item): string {
  return (it.name || it.subcategory || it.category).toUpperCase()
}

// A slot's row: thumbnail, label, what's in it, and the PICK/SWAP/CLOSE
// affordance. Tapping it opens the slot's picker strip below.
function SlotRow({
  label,
  thumb,
  title,
  action,
  open,
  onToggle,
}: {
  label: string
  thumb: Item | undefined
  title: string
  action: string
  open: boolean
  onToggle: () => void
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-expanded={open}
      className={`w-full grid grid-cols-[44px_minmax(0,1fr)_auto] items-center gap-3 min-h-[76px] px-4 py-2.5 text-left hover:bg-(--color-ink)/7 ${
        open ? 'bg-(--color-ink)/7' : ''
      }`}
    >
      <span className="w-11 h-14 border border-(--color-line-strong) bg-(--color-surface) overflow-hidden grid place-items-center font-extrabold text-xl text-(--color-neutral-700)">
        {thumb ? (
          <img src={`/api/items/${thumb.id}/thumb`} alt={thumb.name || thumb.category} className="w-full h-full object-cover" />
        ) : (
          '+'
        )}
      </span>
      <span className="flex flex-col gap-0.5 min-w-0">
        <span className="wy-lbl">{label}</span>
        <span className="font-extrabold text-[15px] tracking-tight truncate">{title}</span>
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
        className={`aspect-3/4 overflow-hidden bg-(--color-bg) ${
          picked ? 'border-2 border-(--color-accent)' : 'border border-(--color-line-strong)'
        }`}
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

const STRIP_CLASS =
  'flex overflow-x-auto snap-x snap-mandatory border-t-2 border-(--color-line-strong) bg-(--color-surface)'
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
  const [slots, setSlots] = useState<Partial<Record<Category, Item>>>({})
  const [accessories, setAccessories] = useState<Item[]>([])
  const [pickSlot, setPickSlot] = useState<Category | null>(null)
  const flash = useToast()

  // Pre-fill the matching slot from Wardrobe's "Add to outfit", overwriting
  // whatever was there — the more recent explicit user intent wins. Consume
  // immediately so this doesn't re-fire on the next re-render.
  useEffect(() => {
    if (!pendingItem) return
    if (pendingItem.category === 'accessory') {
      setAccessories((prev) => (prev.some((it) => it.id === pendingItem.id) ? prev : [...prev, pendingItem]))
    } else if (BUILDABLE_CATEGORIES.includes(pendingItem.category)) {
      setSlots((prev) => ({ ...prev, [pendingItem.category as Category]: pendingItem }))
    }
    onConsumePendingItem?.()
  }, [pendingItem, onConsumePendingItem])

  // Clothes drive the formality/warmth read-out; accessories ride along in
  // the saved look but don't move the averages (most fit any weather).
  const clothes = SLOTS.map((s) => slots[s.category]).filter((it): it is Item => !!it)
  const filled = [...clothes, ...accessories]
  const filledSlots = clothes.length + (accessories.length > 0 ? 1 : 0)

  function toggleAccessory(it: Item) {
    setAccessories((prev) => (prev.some((a) => a.id === it.id) ? prev.filter((a) => a.id !== it.id) : [...prev, it]))
  }

  const saveAsLook = useMutation({
    mutationFn: (input: { itemIds: number[] }) => createLook({ name: 'Custom look', item_ids: input.itemIds }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['looks'] })
      setSlots({})
      setAccessories([])
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

  // Each piece counts at the middle of its formality/warmth range.
  const avgFormality = clothes.length
    ? clothes.reduce((sum, it) => sum + (it.formality_min + it.formality_max) / 2, 0) / clothes.length
    : null
  const avgWarmth = clothes.length
    ? clothes.reduce((sum, it) => sum + (it.warmth_min + it.warmth_max) / 2, 0) / clothes.length
    : null

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <span className="font-extrabold text-3xl tracking-tight">BUILD</span>
        <span className="wy-lbl">{filledSlots}/{SLOTS.length + 1} SLOTS</span>
      </div>

      {/* One row per slot; tapping a row opens a strip of matching pieces
          right under it (only one open at a time), so picking never moves
          the user away from the slot they're filling. */}
      <div className="border-b-2 border-(--color-line-strong)">
        {SLOTS.map((slot) => {
          const item = slots[slot.category]
          const open = pickSlot === slot.category
          const options = open ? (items.data?.filter((it) => it.category === slot.category) ?? []) : []
          return (
            <div key={slot.category} className="border-b border-(--color-line)">
              <SlotRow
                label={slot.label}
                thumb={item}
                title={item ? itemTitle(item) : 'EMPTY'}
                action={item ? 'SWAP ▾' : 'PICK ▾'}
                open={open}
                onToggle={() => setPickSlot(open ? null : slot.category)}
              />
              {open && (
                <div className={STRIP_CLASS}>
                  {options.map((it) => (
                    <PickCard
                      key={it.id}
                      item={it}
                      picked={item?.id === it.id}
                      onPick={() => {
                        setSlots((prev) => ({ ...prev, [slot.category]: it }))
                        setPickSlot(null)
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
                          delete next[slot.category]
                          return next
                        })
                        setPickSlot(null)
                      }}
                      className={STRIP_ACTION_CLASS}
                    >
                      ✕ EMPTY SLOT
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
                    <button type="button" onClick={() => setAccessories([])} className={STRIP_ACTION_CLASS}>
                      ✕ REMOVE ALL
                    </button>
                  )}
                </div>
              )}
            </div>
          )
        })()}
      </div>

      <p className="px-4 py-3 border-b-2 border-(--color-line-strong) text-sm leading-relaxed">
        {clothes.length
          ? `Average formality ${avgFormality?.toFixed(1)} (${FORMALITY_LABELS[Math.round(avgFormality ?? 3)]}) · warmth ${avgWarmth?.toFixed(1)} (${WARMTH_LABELS[Math.round(avgWarmth ?? 3)]})${filled.length < 3 ? ' — add a piece or two before saving.' : '.'}`
          : accessories.length
            ? 'Accessories picked — add some clothes to see how formal and warm the outfit is.'
            : 'Empty rack. Tap a slot to pull something in from the wardrobe.'}
      </p>

      <div className="grid grid-cols-2 border-b-2 border-(--color-line-strong)">
        <button type="button" data-primary="1" onClick={save} disabled={saveAsLook.isPending} className="wy-act">
          {saveAsLook.isPending ? 'SAVING…' : 'SAVE AS LOOK'}
        </button>
        <button
          type="button"
          onClick={() => {
            setSlots({})
            setAccessories([])
            setPickSlot(null)
          }}
          className="wy-act border-l-2 border-(--color-line-strong)"
        >
          CLEAR SLOTS
        </button>
      </div>

    </div>
  )
}
