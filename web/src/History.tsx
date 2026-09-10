import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteWear, getHistory } from './api'

// One screen, two views over the same data: a chronological wear log, and
// every item sorted by staleness (never-worn first) — /api/history
// returns both in one call so the screen has a single loading state.
export default function History({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const history = useQuery({ queryKey: ['history'], queryFn: getHistory })

  const remove = useMutation({
    mutationFn: deleteWear,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['history'] }),
  })

  return (
    <main className="min-h-svh flex flex-col px-4 py-4 gap-6 max-w-2xl mx-auto w-full">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">History</h2>
        <button type="button" onClick={onClose} className="text-sm underline">
          Close
        </button>
      </div>

      {history.isLoading && <p className="text-sm text-gray-500">Loading…</p>}

      <section className="flex flex-col gap-2">
        <h3 className="text-sm font-medium text-gray-500">Not worn in a while</h3>
        {history.data && history.data.items.length === 0 && (
          <p className="text-sm text-gray-500">No items yet.</p>
        )}
        <div className="grid grid-cols-3 gap-2">
          {history.data?.items.map((item) => (
            <div key={item.id} className="flex flex-col gap-1">
              <div className="aspect-square bg-gray-100 rounded overflow-hidden">
                <img
                  src={`/api/items/${item.id}/thumb`}
                  alt={item.name || item.category}
                  className="w-full h-full object-cover"
                  loading="lazy"
                />
              </div>
              <span className="text-[10px] text-gray-500 truncate">
                {item.last_worn_on ? `Last worn ${item.last_worn_on}` : 'Never worn'}
              </span>
            </div>
          ))}
        </div>
      </section>

      <section className="flex flex-col gap-2">
        <h3 className="text-sm font-medium text-gray-500">Wear log</h3>
        {history.data && history.data.wears.length === 0 && (
          <p className="text-sm text-gray-500">
            Nothing logged yet — log a look as worn from Looks or Suggest.
          </p>
        )}
        <div className="flex flex-col gap-2">
          {history.data?.wears.map((wear) => (
            <div key={wear.id} className="border rounded-lg p-3 flex items-center gap-2">
              <div className="flex gap-1 overflow-x-auto flex-1">
                {wear.items.map((item) => (
                  <img
                    key={item.id}
                    src={`/api/items/${item.id}/thumb`}
                    alt={item.name || item.category}
                    className="w-12 h-12 object-cover rounded flex-shrink-0"
                  />
                ))}
              </div>
              <div className="flex flex-col items-end gap-1">
                <span className="text-xs text-gray-500">
                  {wear.worn_on}
                  {wear.occasion ? ` — ${wear.occasion}` : ''}
                </span>
                <button
                  type="button"
                  onClick={() => remove.mutate(wear.id)}
                  disabled={remove.isPending}
                  className="text-xs text-red-600 underline disabled:opacity-50"
                >
                  Remove
                </button>
              </div>
            </div>
          ))}
        </div>
      </section>
    </main>
  )
}
