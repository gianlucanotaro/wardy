import { useState } from 'react'
import ItemImg from './ItemImg'
import Notice from './Notice'
import { Cross } from './Icons'
import { blockOf } from './shapes'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteLook, listLooks, logWear, todayISODate, updateLook } from './api'
import OutfitStyle from './OutfitStyle'
import { useToast } from './useToast'

// "TODAY" or e.g. "12 OCT", in the viewer's local time (created_at is UTC).
function savedLabel(createdAt: string): string {
  const d = new Date(createdAt)
  if (Number.isNaN(d.getTime())) return ''
  const local = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  if (local === todayISODate()) return 'TODAY'
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' }).toUpperCase()
}

// The LOOKS tab: saved outfits, tap one open to rename, drop an item, log
// as worn, or delete — matching Wardy Prototype.dc.html's isLooks block.
export default function Looks() {
  const queryClient = useQueryClient()
  const flash = useToast()
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
      flash('LOOK DELETED')
    },
  })

  const logAsWorn = useMutation({
    mutationFn: (lookId: number) => logWear({ look_id: lookId, worn_on: todayISODate() }),
    onSuccess: () => flash('LOGGED AS WORN TODAY'),
  })

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 pt-4 pb-3">
        <span className="wy-display text-5xl">LOOKS</span>
        <span className="wy-lbl">{looks.data?.length ?? 0} SAVED</span>
      </div>

      {looks.isLoading && <Notice kind="loading">loading looks…</Notice>}
      {looks.isSuccess && looks.data.length === 0 && (
        <div className="px-4 py-8 border-b border-(--color-line)">
          <div className="wy-display text-4xl mb-1.5">no looks saved</div>
        </div>
      )}

      {looks.data?.map((look) => (
        <div key={look.id} className="border-b border-(--color-line)">
          <button
            type="button"
            onClick={() => {
              setSelectedId(selectedId === look.id ? null : look.id)
              setNameDraft(look.name)
              logAsWorn.reset()
            }}
            className="w-full flex items-baseline justify-between gap-3 px-4 py-3 text-left"
          >
            <span className="font-extrabold text-xl tracking-tight truncate min-w-0 lowercase">
              {look.name?.toUpperCase() || 'UNTITLED'}
            </span>
            <span className="wy-lbl whitespace-nowrap">
              {look.items.length} PIECES
              {savedLabel(look.created_at) && ` · SAVED ${savedLabel(look.created_at)}`}
            </span>
          </button>

          <div className="grid grid-cols-6 gap-1.5 px-4 py-2.5 border-t border-(--color-line) sm:max-w-[440px]">
            {look.items.map((item) => (
              <div
                key={item.id}
                className="wy-block aspect-3/4"
                style={{ background: blockOf(item.category) }}
              >
                <ItemImg item={item} loading="lazy" />
              </div>
            ))}
          </div>

          <OutfitStyle items={look.items} compact />

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
                  className="wy-input px-3 py-2 text-sm"
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
                        <Cross /> {(item.name || item.category).toUpperCase()}
                      </button>
                    ))}
                  </div>
                </div>
              )}
              {logAsWorn.isSuccess && (
                <p className="wy-lbl">WORN TODAY</p>
              )}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
