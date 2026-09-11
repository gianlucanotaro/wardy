import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createLook, listItems, type Item } from './api'
import { FORMALITY_LABELS, WARMTH_LABELS, type Category } from './taxonomy'

// Fixed 4 slots, matching Wardy Prototype.dc.html's isBuild block. Client-
// only state — no new backend endpoint, "SAVE AS LOOK" reuses the same
// createLook call Today.tsx and Looks.tsx already use.
const SLOTS: { category: Category; label: string }[] = [
  { category: 'outerwear', label: 'OUTER' },
  { category: 'top', label: 'TOP' },
  { category: 'bottom', label: 'BOTTOM' },
  { category: 'shoes', label: 'SHOES' },
]

export default function Build() {
  const queryClient = useQueryClient()
  const items = useQuery({ queryKey: ['items'], queryFn: listItems })
  const [slots, setSlots] = useState<Partial<Record<Category, Item>>>({})
  const [pickSlot, setPickSlot] = useState<Category | null>(null)
  const [message, setMessage] = useState<string | null>(null)

  const filled = SLOTS.map((s) => slots[s.category]).filter((it): it is Item => !!it)

  const saveAsLook = useMutation({
    mutationFn: (input: { itemIds: number[] }) => createLook({ name: 'Custom look', item_ids: input.itemIds }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['looks'] })
      setSlots({})
      setMessage('SAVED TO LOOKS')
    },
  })

  function save() {
    if (filled.length < 2) {
      setMessage('PICK AT LEAST TWO PIECES')
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

      <div className="grid grid-cols-2 sm:grid-cols-4 border-b-2 border-(--color-line-strong)">
        {SLOTS.map((slot) => {
          const item = slots[slot.category]
          return (
            <button
              type="button"
              key={slot.category}
              onClick={() => setPickSlot(slot.category)}
              className="wy-tile border-b-0"
            >
              <div className="wy-lbl">{slot.label}</div>
              <div className="aspect-3/4 mt-1.5 border border-(--color-line-strong) bg-(--color-surface) overflow-hidden flex items-center justify-center">
                {item ? (
                  <img
                    src={`/api/items/${item.id}/thumb`}
                    alt={item.name || item.category}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <span className="text-3xl font-extrabold text-(--color-neutral-500)">+</span>
                )}
              </div>
              <div className="font-extrabold text-xs mt-1.5 tracking-tight truncate">
                {item ? (item.name || item.subcategory || item.category).toUpperCase() : 'EMPTY — TAP'}
              </div>
            </button>
          )
        })}
      </div>

      {pickSlot && (
        <div>
          <div className="flex justify-between items-center px-4 py-2.5 border-b-2 border-(--color-line-strong)">
            <span className="wy-lbl">PICK A {pickSlot.toUpperCase()}</span>
            <button type="button" onClick={() => setPickSlot(null)} className="wy-chip">
              CLOSE
            </button>
          </div>
          <div className="grid grid-cols-3 border-b-2 border-(--color-line-strong)">
            {items.data
              ?.filter((it) => it.category === pickSlot)
              .map((it) => (
                <button
                  type="button"
                  key={it.id}
                  className="wy-tile"
                  onClick={() => {
                    setSlots((prev) => ({ ...prev, [pickSlot]: it }))
                    setPickSlot(null)
                  }}
                >
                  <div className="aspect-3/4 border border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
                    <img
                      src={`/api/items/${it.id}/thumb`}
                      alt={it.name || it.category}
                      className="w-full h-full object-cover"
                    />
                  </div>
                  <div className="font-extrabold text-xs mt-1.5 tracking-tight truncate">
                    {(it.name || it.subcategory || it.category).toUpperCase()}
                  </div>
                </button>
              ))}
            {items.data?.filter((it) => it.category === pickSlot).length === 0 && (
              <p className="col-span-3 px-4 py-4 text-sm">No {pickSlot} items in your wardrobe yet.</p>
            )}
          </div>
        </div>
      )}

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
            setMessage(null)
          }}
          className="wy-act border-l-2 border-(--color-line-strong)"
        >
          CLEAR SLOTS
        </button>
      </div>

      {message && <p className="px-4 py-2 text-sm text-(--color-neutral-700)">{message}</p>}
    </div>
  )
}
