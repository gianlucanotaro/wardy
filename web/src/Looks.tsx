import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteLook, listLooks, updateLook } from './api'

// Same grid + selected-id modal shape as Wardrobe.tsx's item detail — not
// a new pattern, just applied to looks instead of items.
export default function Looks({ onClose }: { onClose: () => void }) {
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

  return (
    <main className="min-h-svh flex flex-col px-4 py-4 gap-4 max-w-2xl mx-auto w-full">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">Looks</h2>
        <button type="button" onClick={onClose} className="text-sm underline">
          Close
        </button>
      </div>

      {looks.isLoading && <p className="text-sm text-gray-500">Loading looks…</p>}
      {looks.isSuccess && looks.data.length === 0 && (
        <p className="text-sm text-gray-500">No saved looks yet — save one from a suggestion.</p>
      )}

      <div className="grid grid-cols-3 gap-2">
        {looks.data?.map((look) => {
          const cover = look.items.find((it) => it.id === look.cover_item_id) ?? look.items[0]
          return (
            <button
              type="button"
              key={look.id}
              onClick={() => {
                setSelectedId(look.id)
                setNameDraft(look.name)
              }}
              className="relative aspect-square bg-gray-100 rounded overflow-hidden"
            >
              {cover && (
                <img
                  src={`/api/items/${cover.id}/thumb`}
                  alt={look.name || 'saved look'}
                  className="w-full h-full object-cover"
                  loading="lazy"
                />
              )}
              <span className="absolute bottom-1 left-1 right-1 bg-black/70 text-white text-[10px] px-1.5 py-0.5 rounded truncate">
                {look.name || 'Untitled look'}
              </span>
            </button>
          )
        })}
      </div>

      {selected && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center p-4"
          onClick={() => setSelectedId(null)}
        >
          <div
            className="bg-white rounded-lg p-4 max-w-sm w-full max-h-[90vh] overflow-y-auto flex flex-col gap-3"
            onClick={(e) => e.stopPropagation()}
          >
            <label className="flex flex-col gap-1">
              <span className="text-xs text-gray-500">Name</span>
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
                placeholder="Look name"
                className="border rounded px-2 py-1.5 text-sm"
              />
            </label>

            <div className="flex flex-col gap-2">
              {selected.items.map((item) => (
                <div key={item.id} className="flex items-center gap-2 border rounded p-2">
                  <img
                    src={`/api/items/${item.id}/thumb`}
                    alt={item.name || item.category}
                    className="w-14 h-14 object-cover rounded flex-shrink-0"
                  />
                  <span className="text-sm text-gray-600 flex-1 capitalize">{item.name || item.category}</span>
                  {selected.items.length > 1 && (
                    <button
                      type="button"
                      onClick={() =>
                        removeItem.mutate({
                          id: selected.id,
                          itemIds: selected.items.filter((it) => it.id !== item.id).map((it) => it.id),
                        })
                      }
                      className="text-xs text-red-600 underline"
                    >
                      Remove
                    </button>
                  )}
                </div>
              ))}
            </div>

            <div className="flex justify-between pt-2 border-t">
              <button type="button" onClick={() => setSelectedId(null)} className="text-sm underline">
                Close
              </button>
              <button
                type="button"
                onClick={() => remove.mutate(selected.id)}
                disabled={remove.isPending}
                className="text-sm text-red-600 underline disabled:opacity-50"
              >
                {remove.isPending ? 'Deleting…' : 'Delete look'}
              </button>
            </div>
          </div>
        </div>
      )}
    </main>
  )
}
