import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteLook, listLooks, logWear, todayISODate, updateLook } from './api'

// The LOOKS tab: saved outfits, tap one open to rename, drop an item, log
// as worn, or delete — matching Wardy Prototype.dc.html's isLooks block.
export default function Looks() {
  const queryClient = useQueryClient()
  const looks = useQuery({ queryKey: ['looks'], queryFn: listLooks })
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [nameDraft, setNameDraft] = useState('')

  // Look the selected look up from the live query cache (rather than
  // holding a snapshot) so a rename or item removal shows up immediately.
  const selected = looks.data?.find((l) => l.id === selectedId) ?? null

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['looks'] })

  const rename = useMutation({
    mutationFn: (input: { id: number; name: string }) => updateLook(input.id, { name: input.name }),
    onSuccess: invalidate,
  })

  const removeItem = useMutation({
    mutationFn: (input: { id: number; itemIds: number[] }) => updateLook(input.id, { item_ids: input.itemIds }),
    onSuccess: invalidate,
  })

  const remove = useMutation({
    mutationFn: deleteLook,
    onSuccess: () => {
      setSelectedId(null)
      invalidate()
    },
  })

  const logAsWorn = useMutation({
    mutationFn: (lookId: number) => logWear({ look_id: lookId, worn_on: todayISODate() }),
  })

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <span className="font-extrabold text-3xl tracking-tight">LOOKS</span>
        <span className="wy-lbl">{looks.data?.length ?? 0} SAVED</span>
      </div>

      {looks.isLoading && <p className="px-4 py-4 wy-lbl">LOADING LOOKS…</p>}
      {looks.isSuccess && looks.data.length === 0 && (
        <div className="px-4 py-8 border-b-2 border-(--color-line-strong)">
          <div className="font-extrabold text-xl mb-1.5">NO LOOKS SAVED</div>
          <p className="text-sm max-w-[44ch]">Save one from Today or build it yourself in Build.</p>
        </div>
      )}

      {looks.data?.map((look) => (
        <div key={look.id} className="border-b-2 border-(--color-line-strong)">
          <button
            type="button"
            onClick={() => {
              setSelectedId(selectedId === look.id ? null : look.id)
              setNameDraft(look.name)
              logAsWorn.reset()
            }}
            className="w-full flex items-baseline justify-between px-4 py-3 text-left"
          >
            <span className="font-extrabold text-xl tracking-tight">{look.name?.toUpperCase() || 'UNTITLED'}</span>
            <span className="wy-lbl">{look.items.length} PIECES</span>
          </button>

          <div className="flex border-t border-(--color-line)">
            {look.items.map((item) => (
              <div key={item.id} className="flex-1 p-2.5 border-r border-(--color-line) last:border-r-0">
                <div className="aspect-3/4 border border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
                  <img
                    src={`/api/items/${item.id}/thumb`}
                    alt={item.name || item.category}
                    className="w-full h-full object-cover"
                  />
                </div>
                <div className="font-extrabold text-[10px] mt-1.5 tracking-tight truncate">
                  {(item.name || item.category).toUpperCase()}
                </div>
              </div>
            ))}
          </div>

          <div className="grid grid-cols-2 border-t border-(--color-line)">
            <button
              type="button"
              onClick={() => logAsWorn.mutate(look.id)}
              disabled={logAsWorn.isPending}
              className="wy-act"
              style={{ minHeight: 46, padding: '12px 16px' }}
            >
              LOG AS WORN
            </button>
            <button
              type="button"
              onClick={() => remove.mutate(look.id)}
              disabled={remove.isPending}
              className="wy-act border-l border-(--color-line)"
              style={{ minHeight: 46, padding: '12px 16px' }}
            >
              {remove.isPending ? 'DELETING…' : 'DELETE'}
            </button>
          </div>

          {selected?.id === look.id && (
            <div className="px-4 py-3 border-t border-(--color-line) flex flex-col gap-3">
              <label className="flex flex-col gap-1">
                <span className="wy-lbl">NAME</span>
                <input
                  type="text"
                  value={nameDraft}
                  onChange={(e) => setNameDraft(e.target.value)}
                  onBlur={() => {
                    const next = nameDraft.trim()
                    if (next && next !== selected.name) {
                      rename.mutate({ id: selected.id, name: next })
                    }
                  }}
                  className="border border-(--color-line) bg-(--color-surface) px-3 py-2 text-sm focus:outline-2 focus:outline-(--color-accent)"
                />
              </label>

              {selected.items.length > 1 && (
                <div className="flex flex-col gap-1">
                  <span className="wy-lbl">REMOVE A PIECE</span>
                  <div className="flex flex-wrap gap-1.5">
                    {selected.items.map((item) => (
                      <button
                        key={item.id}
                        type="button"
                        className="wy-chip"
                        onClick={() =>
                          removeItem.mutate({
                            id: selected.id,
                            itemIds: selected.items.filter((it) => it.id !== item.id).map((it) => it.id),
                          })
                        }
                      >
                        ✕ {(item.name || item.category).toUpperCase()}
                      </button>
                    ))}
                  </div>
                </div>
              )}
              {logAsWorn.isSuccess && (
                <p className="text-sm text-(--color-neutral-700)">Logged as worn today.</p>
              )}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
