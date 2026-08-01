import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteItem, listItems, logout, uploadItem, type Me } from './api'
import TagEditor from './TagEditor'

export default function Wardrobe({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const queryClient = useQueryClient()
  const items = useQuery({ queryKey: ['items'], queryFn: listItems })
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const fileInput = useRef<HTMLInputElement>(null)

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

  return (
    <main className="min-h-svh flex flex-col px-4 py-4 gap-4 max-w-2xl mx-auto w-full">
      <header className="flex items-center justify-between">
        <h1 className="text-lg font-medium">Wardy</h1>
        <div className="flex items-center gap-3 text-sm text-gray-500">
          <span>{me.display_name}</span>
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

      <div className="grid grid-cols-3 gap-2">
        {items.data?.map((item) => (
          <button
            type="button"
            key={item.id}
            onClick={() => setSelectedId(item.id)}
            className="aspect-square bg-gray-100 rounded overflow-hidden"
          >
            <img
              src={`/api/items/${item.id}/thumb`}
              alt={item.name || 'wardrobe item'}
              className="w-full h-full object-cover"
              loading="lazy"
            />
          </button>
        ))}
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
            <img
              src={`/api/items/${selected.id}/photo`}
              alt={selected.name || 'wardrobe item'}
              className="w-full rounded"
            />

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
