import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteItem, listItems, logout, uploadItem, type Me } from './api'
import HistoryScreen from './History'
import LooksScreen from './Looks'
import SettingsScreen from './Settings'
import SuggestScreen from './Suggest'
import TagEditor from './TagEditor'

type View = 'wardrobe' | 'settings' | 'suggest' | 'looks' | 'history'

export default function Wardrobe({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const queryClient = useQueryClient()
  const [view, setView] = useState<View>('wardrobe')
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

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: onLogout,
  })

  if (view === 'settings') {
    return <SettingsScreen onClose={() => setView('wardrobe')} />
  }
  if (view === 'suggest') {
    return <SuggestScreen onClose={() => setView('wardrobe')} />
  }
  if (view === 'looks') {
    return <LooksScreen onClose={() => setView('wardrobe')} />
  }
  if (view === 'history') {
    return <HistoryScreen onClose={() => setView('wardrobe')} />
  }

  return (
    <main className="min-h-svh flex flex-col px-4 py-4 gap-4 max-w-2xl mx-auto w-full">
      <header className="flex items-center justify-between">
        <h1 className="text-lg font-medium">Wardy</h1>
        <div className="flex items-center gap-3 text-sm text-gray-500">
          <span>{me.display_name}</span>
          <button type="button" onClick={() => setView('suggest')} className="underline">
            Suggest
          </button>
          <button type="button" onClick={() => setView('looks')} className="underline">
            Looks
          </button>
          <button type="button" onClick={() => setView('history')} className="underline">
            History
          </button>
          <button type="button" onClick={() => setView('settings')} className="underline">
            Settings
          </button>
          <button type="button" onClick={() => logoutMutation.mutate()} className="underline">
            Log out
          </button>
        </div>
      </header>

      <div>
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
          className="border rounded px-3 py-2 text-sm disabled:opacity-50"
        >
          {upload.isPending ? 'Uploading…' : '+ Add item'}
        </button>
        {upload.isError && <p className="text-sm text-red-600 mt-1">Upload failed.</p>}
      </div>

      {items.isLoading && <p className="text-sm text-gray-500">Loading wardrobe…</p>}
      {items.isSuccess && items.data.length === 0 && (
        <p className="text-sm text-gray-500">No items yet — add your first photo above.</p>
      )}

      {items.isSuccess && items.data.length > 0 && (
        <input
          type="text"
          value={tagQuery}
          onChange={(e) => setTagQuery(e.target.value)}
          placeholder="Search your tags…"
          className="border rounded px-2 py-1.5 text-sm"
        />
      )}
      {query && visibleItems?.length === 0 && (
        <p className="text-sm text-gray-500">No items tagged "{tagQuery.trim()}".</p>
      )}

      <div className="grid grid-cols-3 gap-2">
        {visibleItems?.map((item) => (
          <button
            type="button"
            key={item.id}
            onClick={() => setSelectedId(item.id)}
            className="relative aspect-square bg-gray-100 rounded overflow-hidden"
          >
            <img
              src={`/api/items/${item.id}/thumb`}
              alt={item.name || 'wardrobe item'}
              className="w-full h-full object-cover"
              loading="lazy"
            />
            {item.tagging_pending && (
              <span className="absolute bottom-1 right-1 bg-black/70 text-white text-[10px] px-1.5 py-0.5 rounded">
                Tagging…
              </span>
            )}
          </button>
        ))}
      </div>

      {selected && (
        <div
          className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 safe-area-overlay"
          onClick={() => setSelectedId(null)}
        >
          <div
            className="bg-white rounded-lg p-4 max-w-sm w-full max-h-[90vh] overflow-y-auto flex flex-col gap-3"
            onClick={(e) => e.stopPropagation()}
          >
            <img
              src={`/api/items/${selected.id}/photo`}
              alt={selected.name || 'wardrobe item'}
              className="w-full rounded"
            />

            {selected.tagging_pending && (
              <p className="text-sm text-gray-500">Tagging in progress — you can edit these now, or wait.</p>
            )}
            <TagEditor item={selected} />

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
                {remove.isPending ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </main>
  )
}
