import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { archiveItem, deleteItem, listItems, logWear, todayISODate, uploadItem, type Item } from './api'
import TagEditor from './TagEditor'
import { useToast } from './useToast'
import { CATEGORIES, FORMALITY_LABELS, WARMTH_LABELS, type Category } from './taxonomy'
import { occasionsFor, weatherRange } from './outfitRules'

// Mirrors Build.tsx's SLOTS categories — those are the only slots Build can
// pre-fill into, so "Add to outfit" only shows for items in this list.
const BUILDABLE_CATEGORIES = ['outerwear', 'top', 'bottom', 'shoes']

// The WARDROBE tab: grid + category chips + tag search + item detail, matching
// Wardy Prototype.dc.html's isWardrobe/isDetail blocks. Deliberately does
// NOT apply the design system's .grayscale photo treatment — color is
// functional here (matching outfits by color is a core use case), not
// decorative, so item photos stay in color unlike a marketing page's hero
// images.
// Top bar of a full-screen overlay: stays pinned while the overlay's
// content scrolls, so its close button is always one tap away. Offset by
// the top safe-area inset because the overlay's own padding scrolls away.
const OVERLAY_BAR = 'sticky top-[env(safe-area-inset-top)] z-10 bg-(--color-bg)'

export default function Wardrobe({ onAddToOutfit }: { onAddToOutfit: (item: Item) => void }) {
  const queryClient = useQueryClient()
  const flash = useToast()
  const [showArchived, setShowArchived] = useState(false)
  const items = useQuery({
    // Plain ['items'] for the default (active) view — Build.tsx and
    // Today.tsx query the same key with no archived filter, so this has
    // to match exactly for them to share one cache entry rather than each
    // re-fetching. The archived view gets its own separate key.
    queryKey: showArchived ? ['items', 'archived'] : ['items'],
    queryFn: () => listItems(showArchived ? { archived: true } : undefined),
    // Poll while any item is still being tagged; stop once none are —
    // tagging is a background job, so the UI can't just wait on the
    // upload response for the result.
    refetchInterval: (query) => (query.state.data?.some((it) => it.tagging_pending) ? 2000 : false),
  })
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [tagQuery, setTagQuery] = useState('')
  const [category, setCategory] = useState<Category | 'all'>('all')
  const fileInput = useRef<HTMLInputElement>(null)

  // The add-item flow (Wardy Prototype.dc.html's isAdd block): idle (choose
  // a photo) -> review (photo + tag editor, live once uploaded). There's no
  // separate blocking "tagging" stage — docs/frontend.md already requires
  // showing the photo immediately and editing mid-poll, so review just
  // reflects whatever the shared items cache currently has.
  const [addFlow, setAddFlow] = useState<{ stage: 'idle' } | { stage: 'review'; itemId: number } | null>(null)

  // Client-side — the grid already has the full item list loaded, so a
  // search endpoint would just be a second round trip for the same data.
  // The category chip and the tag search combine (AND).
  const query = tagQuery.trim().toLowerCase()
  const visibleItems = items.data?.filter(
    (it) =>
      (category === 'all' || it.category === category) &&
      (!query || (it.custom_tags ?? []).some((tag) => tag.toLowerCase().includes(query))),
  )

  // Look the selected item up from the live query cache (rather than
  // holding a snapshot) so optimistic tag edits show up immediately.
  const selected = items.data?.find((it) => it.id === selectedId) ?? null
  const weather = selected ? weatherRange(selected.category, selected.warmth_min, selected.warmth_max) : null

  const addingItem = addFlow?.stage === 'review' ? (items.data?.find((it) => it.id === addFlow.itemId) ?? null) : null
  // All-or-nothing, not diffed per field — see TagEditor's markAiSuggested
  // comment. False once tagging fails too (ai_tagged_at stays null), which
  // is exactly the "fall through to an empty form, no special failure
  // state" behavior the issue asks for.
  const aiJustSuggested = Boolean(addingItem?.ai_tagged_at && !addingItem.user_edited_at)

  const upload = useMutation({
    mutationFn: uploadItem,
    onSuccess: (newItem) => {
      // Seed the cache synchronously so the review stage's very first
      // render already finds the item — invalidate still follows, to
      // reconcile with the server (and pick up anything a plain POST
      // response wouldn't reflect, like a since-changed wear count).
      queryClient.setQueryData<Item[]>(['items'], (old) => [newItem, ...(old ?? [])])
      queryClient.invalidateQueries({ queryKey: ['items'] })
      setAddFlow({ stage: 'review', itemId: newItem.id })
    },
  })

  const remove = useMutation({
    mutationFn: deleteItem,
    onSuccess: () => {
      setSelectedId(null)
      queryClient.invalidateQueries({ queryKey: ['items'] })
      flash('ITEM DELETED')
    },
  })

  const archive = useMutation({
    mutationFn: (input: { id: number; archived: boolean }) => archiveItem(input.id, input.archived),
    onSuccess: (_item, input) => {
      // Archiving/unarchiving always moves the item out of whichever
      // filtered view is currently showing, same as delete closing the
      // modal.
      setSelectedId(null)
      queryClient.invalidateQueries({ queryKey: ['items'] })
      flash(input.archived ? 'ITEM ARCHIVED' : 'ITEM RESTORED')
    },
  })

  const logAsWorn = useMutation({
    mutationFn: (itemId: number) => logWear({ item_ids: [itemId], worn_on: todayISODate() }),
    // Refetch so the footer's wear count/last-worn updates immediately —
    // the modal reads `selected` from this same cache without leaving it.
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['items'] })
      flash('LOGGED AS WORN TODAY')
    },
  })

  // The item is saved the moment its photo uploads, so leaving review is
  // only ever confirming what's already there.
  function finishAdd(next: { stage: 'idle' } | null) {
    setAddFlow(next)
    flash('ADDED TO WARDROBE')
  }

  function meta(item: { subcategory: string; color_primary: string; material: string }) {
    return `${item.subcategory} · ${item.color_primary} · ${item.material}`.toUpperCase()
  }

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <h1 className="font-extrabold text-3xl tracking-tight">WARDROBE</h1>
        <button
          type="button"
          onClick={() => {
            setShowArchived(false)
            setAddFlow({ stage: 'idle' })
          }}
          className="wy-chip"
          style={{ background: 'var(--color-accent)', color: 'var(--color-bg)', borderColor: 'var(--color-accent)' }}
        >
          + ADD ITEM
        </button>
      </div>

      <div className="flex justify-end px-4 py-2 border-b border-(--color-line)">
        <button
          type="button"
          onClick={() => {
            setShowArchived((v) => !v)
            setSelectedId(null)
          }}
          className="wy-chip"
        >
          {showArchived ? 'SHOW ACTIVE' : 'SHOW ARCHIVED'}
        </button>
      </div>

      {items.isLoading && <p className="px-4 py-4 wy-lbl">LOADING WARDROBE…</p>}
      {items.isSuccess && items.data.length === 0 && (
        <p className="px-4 py-4 text-sm">
          {showArchived ? 'No archived items.' : 'No items yet — add your first photo above.'}
        </p>
      )}

      {items.isSuccess && items.data.length > 0 && (
        <div className="px-4 py-3 border-b-2 border-(--color-line-strong) flex flex-col gap-3">
          <div>
            <div className="wy-lbl mb-1.5">CATEGORY</div>
            <div className="flex flex-wrap gap-1.5">
              {(['all', ...CATEGORIES] as const).map((c) => (
                <button
                  type="button"
                  key={c}
                  onClick={() => setCategory(c)}
                  data-on={category === c || undefined}
                  className="wy-chip"
                >
                  {c.toUpperCase()}
                </button>
              ))}
            </div>
          </div>
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
          <span>
            {category === 'all' ? 'EVERYTHING' : category.toUpperCase()}
            {query && ` · "${tagQuery.trim()}"`}
          </span>
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
      {items.isSuccess && items.data.length > 0 && visibleItems?.length === 0 && (
        <p className="px-4 py-4 text-sm">
          {query ? `No ${category === 'all' ? '' : category + ' '}items tagged "${tagQuery.trim()}".` : `No ${category} items yet.`}
        </p>
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
            <div className={`${OVERLAY_BAR} flex items-center gap-3 px-4 py-3 border-b-2 border-(--color-line-strong)`}>
              <button type="button" onClick={() => setSelectedId(null)} className="wy-chip">
                ✕ CLOSE
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
              <ScaleCell
                label="FORMALITY"
                min={selected.formality_min}
                max={selected.formality_max}
                labels={FORMALITY_LABELS}
                rangeLabel="WORKS FOR"
                range={occasionsFor(selected.formality_min, selected.formality_max).join(' · ').toUpperCase() || '—'}
              />
              <ScaleCell
                label="WARMTH"
                min={selected.warmth_min}
                max={selected.warmth_max}
                labels={WARMTH_LABELS}
                rangeLabel="WEAR IT"
                range={weather?.label ?? '—'}
                note={weather?.layerBelow != null ? `LAYER UP BELOW ${weather.layerBelow}°` : undefined}
              />
            </div>

            <div className="p-4">
              <TagEditor item={selected} />
            </div>

            <div className="px-4 py-2.5 border-t border-(--color-line) wy-lbl">
              WORN {selected.wear_count}× ·{' '}
              {selected.last_worn_on ? `LAST ${selected.last_worn_on}` : 'NEVER WORN'}
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
                    flash('ADDED TO BUILD')
                  }}
                  className="wy-act border-l-2 border-(--color-line-strong)"
                >
                  ADD TO OUTFIT
                </button>
              )}
            </div>

            <div className="grid grid-cols-3 border-t-2 border-(--color-line-strong)">
              <button type="button" onClick={() => setSelectedId(null)} className="wy-act">
                DONE
              </button>
              <button
                type="button"
                onClick={() => archive.mutate({ id: selected.id, archived: !selected.archived })}
                disabled={archive.isPending}
                className="wy-act border-l-2 border-(--color-line-strong)"
              >
                {archive.isPending ? '...' : selected.archived ? 'UNARCHIVE' : 'ARCHIVE'}
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

      {addFlow?.stage === 'idle' && (
        <div className="fixed inset-0 z-20 bg-(--color-bg) overflow-y-auto safe-area-overlay flex flex-col">
          <div className={`${OVERLAY_BAR} flex items-center gap-3 px-4 py-3 border-b-2 border-(--color-line-strong)`}>
            <button type="button" onClick={() => setAddFlow(null)} disabled={upload.isPending} className="wy-chip">
              ✕ CANCEL
            </button>
          </div>

          <div className="flex-1 flex flex-col items-center justify-center gap-6 px-8 text-center">
            <h2 className="font-extrabold text-3xl tracking-tight leading-tight">
              PHOTOGRAPH IT ONCE.
              <br />
              WE TAG THE REST.
            </h2>

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
              className="wy-act disabled:opacity-45"
              style={{
                background: 'var(--color-accent)',
                color: 'var(--color-bg)',
                borderColor: 'var(--color-accent)',
                padding: '14px 28px',
              }}
            >
              {upload.isPending ? 'UPLOADING…' : 'CHOOSE A PHOTO'}
            </button>

            {upload.isError && <p className="text-sm text-(--color-accent-700)">Upload failed. Try again.</p>}
          </div>
        </div>
      )}

      {addFlow?.stage === 'review' && addingItem && (
        <div className="fixed inset-0 z-20 bg-(--color-bg) overflow-y-auto safe-area-overlay">
          <div className={`${OVERLAY_BAR} flex items-center justify-between gap-3 px-4 py-3 border-b-2 border-(--color-line-strong)`}>
            <span className="wy-lbl">REVIEW NEW ITEM</span>
            <button type="button" onClick={() => finishAdd(null)} className="wy-chip">
              ✓ DONE
            </button>
          </div>

          <div className="border-b-2 border-(--color-line-strong) p-4">
            <div className="aspect-3/4 border-2 border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
              <img
                src={`/api/items/${addingItem.id}/photo`}
                alt="newly added wardrobe item"
                className="w-full h-full object-cover"
              />
            </div>
          </div>

          {addingItem.tagging_pending ? (
            <p className="px-4 py-3 text-sm text-(--color-neutral-700) border-b border-(--color-line)">
              TAGGING… you can start editing now, or leave this screen — it's safe, tagging finishes in the
              background.
            </p>
          ) : (
            !aiJustSuggested && (
              <p className="px-4 py-3 text-sm text-(--color-neutral-700) border-b border-(--color-line)">
                Automatic tagging didn't run for this item — fill in the fields below.
              </p>
            )
          )}

          <div className="p-4">
            <TagEditor item={addingItem} markAiSuggested={aiJustSuggested} />
          </div>

          <div className="grid grid-cols-2 border-t-2 border-(--color-line-strong)">
            <button type="button" onClick={() => finishAdd(null)} data-primary="1" className="wy-act">
              SAVE TO WARDROBE
            </button>
            <button
              type="button"
              onClick={() => finishAdd({ stage: 'idle' })}
              className="wy-act border-l-2 border-(--color-line-strong)"
            >
              ADD ANOTHER PHOTO
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// One scale (formality or warmth) in the item view: the piece's own range
// as the design's 5-step bar, then what that range means for suggestions
// (see outfitRules.ts).
function ScaleCell({
  label,
  min,
  max,
  labels,
  rangeLabel,
  range,
  note,
}: {
  label: string
  min: number
  max: number
  labels: Record<number, string>
  rangeLabel: string
  range: string
  note?: string
}) {
  return (
    <div className="wy-cell flex flex-col gap-1.5">
      <div className="flex justify-between gap-2">
        <span className="wy-lbl">{label}</span>
        <span className="wy-lbl text-right">{min === max ? min : `${min}–${max}`}</span>
      </div>
      <div className="grid grid-cols-5 gap-1" aria-hidden="true">
        {[1, 2, 3, 4, 5].map((n) => (
          <div key={n} className={`h-3 ${n >= min && n <= max ? 'bg-(--color-accent)' : 'bg-(--color-ink)/20'}`} />
        ))}
      </div>
      <div className="wy-lbl">{(min === max ? labels[min] : `${labels[min]} – ${labels[max]}`).toUpperCase()}</div>
      <div className="wy-lbl mt-1">{rangeLabel}</div>
      <div className="font-extrabold text-[13px] leading-tight tracking-[0.02em]">{range}</div>
      {note && <div className="wy-lbl text-(--color-accent-700)">{note}</div>}
    </div>
  )
}
