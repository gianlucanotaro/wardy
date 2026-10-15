import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createLook, listItems, type Item } from './api'
import { FORMALITY_LABELS, WARMTH_LABELS, type Category } from './taxonomy'
import { useToast } from './useToast'

// Fixed 4 slots, matching the design's isBuild block (stacked slot rows,
// each with an inline picker strip). Client-
// only state — no new backend endpoint, "SAVE AS LOOK" reuses the same
// createLook call Today.tsx and Looks.tsx already use.
const SLOTS: { category: Category; label: string }[] = [
  { category: 'outerwear', label: 'OUTER' },
  { category: 'top', label: 'TOP' },
  { category: 'bottom', label: 'BOTTOM' },
  { category: 'shoes', label: 'SHOES' },
]

// Which categories have a slot above — dress/bag/accessory don't. Wardrobe's
// "Add to outfit" button keeps its own copy of this list (see Wardrobe.tsx)
// to avoid exporting a non-component value out of this file, which breaks
// fast refresh.
const BUILDABLE_CATEGORIES: string[] = SLOTS.map((s) => s.category)

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
  const [pickSlot, setPickSlot] = useState<Category | null>(null)
  const flash = useToast()

  // Pre-fill the matching slot from Wardrobe's "Add to outfit", overwriting
  // whatever was there — the more recent explicit user intent wins. Consume
  // immediately so this doesn't re-fire on the next re-render.
  useEffect(() => {
    if (!pendingItem) return
    if (BUILDABLE_CATEGORIES.includes(pendingItem.category)) {
      setSlots((prev) => ({ ...prev, [pendingItem.category as Category]: pendingItem }))
    }
    onConsumePendingItem?.()
  }, [pendingItem, onConsumePendingItem])

  const filled = SLOTS.map((s) => slots[s.category]).filter((it): it is Item => !!it)

  const saveAsLook = useMutation({
    mutationFn: (input: { itemIds: number[] }) => createLook({ name: 'Custom look', item_ids: input.itemIds }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['looks'] })
      setSlots({})
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

  const avgFormality = filled.length
    ? filled.reduce((sum, it) => sum + it.formality, 0) / filled.length
    : null
  const avgWarmth = filled.length ? filled.reduce((sum, it) => sum + it.warmth, 0) / filled.length : null

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <span className="font-extrabold text-3xl tracking-tight">BUILD</span>
        <span className="wy-lbl">{filled.length}/4 SLOTS</span>
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
            <div key={slot.category} className="border-b border-(--color-line) last:border-b-0">
              <button
                type="button"
                onClick={() => setPickSlot(open ? null : slot.category)}
                aria-expanded={open}
                className={`w-full grid grid-cols-[44px_minmax(0,1fr)_auto] items-center gap-3 min-h-[76px] px-4 py-2.5 text-left hover:bg-(--color-ink)/7 ${
                  open ? 'bg-(--color-ink)/7' : ''
                }`}
              >
                <span className="w-11 h-14 border border-(--color-line-strong) bg-(--color-surface) overflow-hidden grid place-items-center font-extrabold text-xl text-(--color-neutral-700)">
                  {item ? (
                    <img
                      src={`/api/items/${item.id}/thumb`}
                      alt={item.name || item.category}
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    '+'
                  )}
                </span>
                <span className="flex flex-col gap-0.5 min-w-0">
                  <span className="wy-lbl">{slot.label}</span>
                  <span className="font-extrabold text-[15px] tracking-tight truncate">
                    {item ? (item.name || item.subcategory || item.category).toUpperCase() : 'EMPTY'}
                  </span>
                </span>
                <span
                  className={`font-extrabold text-[10px] tracking-[0.09em] ${
                    open ? 'text-(--color-ink)' : 'text-(--color-accent-700)'
                  }`}
                >
                  {open ? 'CLOSE ▴' : item ? 'SWAP ▾' : 'PICK ▾'}
                </span>
              </button>

              {open && (
                <div className="flex overflow-x-auto snap-x snap-mandatory border-t-2 border-(--color-line-strong) bg-(--color-surface)">
                  {options.map((it) => (
                    <button
                      type="button"
                      key={it.id}
                      onClick={() => {
                        setSlots((prev) => ({ ...prev, [slot.category]: it }))
                        setPickSlot(null)
                      }}
                      className="flex-none w-26 snap-start p-2.5 border-r border-(--color-line) text-left hover:bg-(--color-ink)/7"
                    >
                      <div
                        className={`aspect-3/4 overflow-hidden bg-(--color-bg) ${
                          item?.id === it.id
                            ? 'border-2 border-(--color-accent)'
                            : 'border border-(--color-line-strong)'
                        }`}
                      >
                        <img
                          src={`/api/items/${it.id}/thumb`}
                          alt={it.name || it.category}
                          className="w-full h-full object-cover"
                          loading="lazy"
                        />
                      </div>
                      <div className="font-extrabold text-[10.5px] leading-tight mt-1.5 truncate">
                        {(it.name || it.subcategory || it.category).toUpperCase()}
                      </div>
                      <div className="text-[9px] tracking-wide text-(--color-neutral-700) truncate">
                        {it.color_primary.toUpperCase()}
                      </div>
                    </button>
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
                      className="flex-none w-26 p-2.5 text-left font-extrabold text-[10.5px] tracking-[0.06em] text-(--color-accent-700) hover:bg-(--color-ink)/7"
                    >
                      ✕ EMPTY SLOT
                    </button>
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>

      <p className="px-4 py-3 border-b-2 border-(--color-line-strong) text-sm leading-relaxed">
        {filled.length
          ? `Average formality ${avgFormality?.toFixed(1)} (${FORMALITY_LABELS[Math.round(avgFormality ?? 3)]}) · warmth ${avgWarmth?.toFixed(1)} (${WARMTH_LABELS[Math.round(avgWarmth ?? 3)]})${filled.length < 3 ? ' — add a piece or two before saving.' : '.'}`
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
