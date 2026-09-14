import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteItem, listItems, logWear, todayISODate, uploadItem, type Item } from './api'
import TagEditor from './TagEditor'
import { FORMALITY_LABELS, WARMTH_LABELS } from './taxonomy'

// Mirrors Build.tsx's SLOTS categories — those are the only slots Build can
// pre-fill into, so "Add to outfit" only shows for items in this list.
const BUILDABLE_CATEGORIES = ['outerwear', 'top', 'bottom', 'shoes']

// The WARDROBE tab: grid + tag search + item detail, matching
// Wardy Prototype.dc.html's isWardrobe/isDetail blocks. Deliberately does
// NOT apply the design system's .grayscale photo treatment — color is
// functional here (matching outfits by color is a core use case), not
// decorative, so item photos stay in color unlike a marketing page's hero
// images.
export default function Wardrobe({ onAddToOutfit }: { onAddToOutfit: (item: Item) => void }) {
  const queryClient = useQueryClient()
  const items = useQuery({
    queryKey: ['items'],
    queryFn: listItems,
    // Poll while any item is still being tagged; stop once none are —
    // tagging is a background job, so the UI can't just wait on the
    // upload response for the result.
    refetchInterval: (query) => (query.state.data?.some((it) => it.tagging_pending) ? 2000 : false),
  })
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [tagQuery, setTagQuery] = useState('')
  const fileInput = useRef<HTMLInputElement>(null)

  // Client-side — the grid already has the full item list loaded, so a
  // search endpoint would just be a second round trip for the same data.
  const query = tagQuery.trim().toLowerCase()
  const visibleItems = query
    ? items.data?.filter((it) => (it.custom_tags ?? []).some((tag) => tag.toLowerCase().includes(query)))
    : items.data

  // Look the selected item up from the live query cache (rather than
  // holding a snapshot) so optimistic tag edits show up immediately.
  const selected = items.data?.find((it) => it.id === selectedId) ?? null

  const upload = useMutation({
    mutationFn: uploadItem,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['items'] }),
  })

  const remove = useMutation({
    mutationFn: deleteItem,
    onSuccess: () => {
      setSelectedId(null)
      queryClient.invalidateQueries({ queryKey: ['items'] })
    },
  })

  const logAsWorn = useMutation({
    mutationFn: (itemId: number) => logWear({ item_ids: [itemId], worn_on: todayISODate() }),
  })

  function meta(item: { subcategory: string; color_primary: string; material: string }) {
    return `${item.subcategory} · ${item.color_primary} · ${item.material}`.toUpperCase()
  }

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <h1 className="font-extrabold text-3xl tracking-tight">WARDROBE</h1>
        <input
          ref={fileInput}
          type="file"
          accept="image/*"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) upload.mutate(file)
            e.target.value = ''
          }}
        />
        <button
          type="button"
          onClick={() => fileInput.current?.click()}
          disabled={upload.isPending}
          className="wy-chip disabled:opacity-45"
          style={{ background: 'var(--color-accent)', color: 'var(--color-bg)', borderColor: 'var(--color-accent)' }}
        >
          {upload.isPending ? 'UPLOADING…' : '+ ADD ITEM'}
        </button>
      </div>
      {upload.isError && (
        <p className="px-4 py-2 text-sm text-(--color-accent-700) border-b-2 border-(--color-line-strong)">
          Upload failed.
        </p>
      )}

      {items.isLoading && <p className="px-4 py-4 wy-lbl">LOADING WARDROBE…</p>}
      {items.isSuccess && items.data.length === 0 && (
        <p className="px-4 py-4 text-sm">No items yet — add your first photo above.</p>
      )}

      {items.isSuccess && items.data.length > 0 && (
        <div className="px-4 py-3 border-b-2 border-(--color-line-strong)">
          <input
            type="text"
            value={tagQuery}
            onChange={(e) => setTagQuery(e.target.value)}
            placeholder="SEARCH YOUR TAGS…"
            className="w-full border border-(--color-line) bg-(--color-surface) px-3 py-2 text-sm placeholder:text-(--color-neutral-500) focus:outline-2 focus:outline-(--color-accent)"
          />
        </div>
      )}

      {items.isSuccess && items.data.length > 0 && (
        <div className="flex justify-between px-4 py-2.5 border-b-2 border-(--color-line-strong) wy-lbl">
          <span>{query ? `"${tagQuery.trim()}"` : 'EVERYTHING'}</span>
          <span>{visibleItems?.length ?? 0} ITEMS</span>
        </div>
      )}

      <div className="grid grid-cols-3">
        {visibleItems?.map((item) => (
          <button
            type="button"
            key={item.id}
            onClick={() => {
              setSelectedId(item.id)
              logAsWorn.reset()
            }}
            className="wy-tile"
          >
            <div className="relative aspect-3/4 border border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
              <img
                src={`/api/items/${item.id}/thumb`}
                alt={item.name || 'wardrobe item'}
                className="w-full h-full object-cover"
                loading="lazy"
              />
              {item.tagging_pending && (
                <span className="absolute bottom-1 right-1 bg-(--color-ink) text-(--color-bg) text-[9px] font-extrabold uppercase tracking-wide px-1.5 py-0.5">
                  Tagging…
                </span>
              )}
            </div>
            <div className="font-extrabold text-xs mt-1.5 tracking-tight truncate">
              {(item.name || item.subcategory || item.category).toUpperCase()}
            </div>
            <div className="text-[9.5px] text-(--color-neutral-700) tracking-wide truncate">{meta(item)}</div>
          </button>
        ))}
      </div>
      {query && visibleItems?.length === 0 && (
        <p className="px-4 py-4 text-sm">No items tagged "{tagQuery.trim()}".</p>
      )}

      {selected && (
        <div
          className="fixed inset-0 z-20 bg-(--color-ink)/60 overflow-y-auto safe-area-overlay"
          onClick={() => setSelectedId(null)}
        >
          <div
            className="bg-(--color-bg) min-h-full sm:min-h-0 sm:max-w-lg sm:mx-auto sm:my-8 border-(--color-line-strong) sm:border-2"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-3 px-4 py-3 border-b-2 border-(--color-line-strong)">
              <button type="button" onClick={() => setSelectedId(null)} className="wy-chip">
                ← BACK
              </button>
              <span className="wy-lbl">ITEM #{selected.id}</span>
            </div>

            <div className="border-b-2 border-(--color-line-strong) p-4">
              <div className="aspect-3/4 border-2 border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
                <img
                  src={`/api/items/${selected.id}/photo`}
                  alt={selected.name || 'wardrobe item'}
                  className="w-full h-full object-cover"
                />
              </div>
            </div>

            <div className="px-4 pt-4">
              <div className="wy-lbl">{(selected.category || 'UNTAGGED').toUpperCase()}</div>
              <div className="font-extrabold text-[28px] leading-none tracking-tight mt-1 mb-3">
                {(selected.name || selected.subcategory || 'Item').toUpperCase()}
              </div>
            </div>

            {selected.tagging_pending && (
              <p className="px-4 pb-2 text-sm text-(--color-neutral-700)">
                Tagging in progress — you can edit these now, or wait.
              </p>
            )}

            <div className="grid grid-cols-2 border-t border-(--color-line)">
              <div className="wy-cell">
                <div className="wy-lbl">FORMALITY</div>
                <div className="font-extrabold text-[15px] tracking-tight">
                  {FORMALITY_LABELS[selected.formality]}
                </div>
              </div>
              <div className="wy-cell">
                <div className="wy-lbl">WARMTH</div>
                <div className="font-extrabold text-[15px] tracking-tight">{WARMTH_LABELS[selected.warmth]}</div>
              </div>
            </div>

            <div className="p-4">
              <TagEditor item={selected} />
            </div>

            <div
              className={`grid border-t-2 border-(--color-line-strong) ${
                BUILDABLE_CATEGORIES.includes(selected.category) ? 'grid-cols-2' : 'grid-cols-1'
              }`}
            >
              <button
                type="button"
                onClick={() => logAsWorn.mutate(selected.id)}
                disabled={logAsWorn.isPending}
                className="wy-act"
              >
                {logAsWorn.isPending ? 'LOGGING…' : 'LOG AS WORN TODAY'}
              </button>
              {BUILDABLE_CATEGORIES.includes(selected.category) && (
                <button
                  type="button"
                  onClick={() => {
                    onAddToOutfit(selected)
                    setSelectedId(null)
                  }}
                  className="wy-act border-l-2 border-(--color-line-strong)"
                >
                  ADD TO OUTFIT
                </button>
              )}
            </div>
            {logAsWorn.isSuccess && (
              <p className="px-4 py-2 text-sm text-(--color-neutral-700) border-t border-(--color-line)">
                Logged as worn today.
              </p>
            )}

            <div className="grid grid-cols-2 border-t-2 border-(--color-line-strong)">
              <button type="button" onClick={() => setSelectedId(null)} className="wy-act">
                DONE
              </button>
              <button
                type="button"
                onClick={() => remove.mutate(selected.id)}
                disabled={remove.isPending}
                className="wy-act border-l-2 border-(--color-line-strong) text-(--color-accent-700)"
              >
                {remove.isPending ? 'DELETING…' : 'DELETE'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
