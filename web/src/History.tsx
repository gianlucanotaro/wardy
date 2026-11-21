import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import ItemImg from './ItemImg'
import Notice from './Notice'
import Shape from './Shape'
import { blockOf, shapeOf } from './shapes'
import { deleteWear, getHistory } from './api'
import { useToast } from './useToast'

// One screen, two views over the same data: a chronological wear log, and
// every item sorted by staleness (never-worn first) — /api/history returns
// both in one call so the screen has a single loading state.
export default function History() {
  const queryClient = useQueryClient()
  const flash = useToast()
  const history = useQuery({ queryKey: ['history'], queryFn: getHistory })

  const remove = useMutation({
    mutationFn: deleteWear,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['history'] })
      flash('WEAR REMOVED')
    },
  })

  return (
    <div className="flex flex-col">
      <div className="px-4 pt-4 pb-3">
        <span className="wy-display text-5xl">HISTORY</span>
      </div>

      {history.isLoading && <Notice kind="loading">loading…</Notice>}

      <div className="wy-sec flex justify-between px-4 py-2.5 border-b border-(--color-line)">
        <span className="wy-lbl">NOT WORN IN A WHILE</span>
      </div>
      {history.data && history.data.items.length === 0 && <p className="px-4 py-4 text-sm">No items yet.</p>}
      <div className="grid grid-cols-3 border-b border-(--color-line)">
        {history.data?.items.map((item) => (
          <div key={item.id} className="wy-tile">
            <div className="wy-block aspect-3/4" style={{ background: blockOf(item.category) }}>
              <ItemImg item={item} loading="lazy" />
              <span className="absolute left-2 top-2 text-(--color-block-ink)">
                <Shape name={shapeOf(item.category)} />
              </span>
            </div>
            <div className="text-[9.5px] text-(--color-neutral-700) tracking-wide mt-1.5 truncate uppercase">
              {item.last_worn_on ? `Last worn ${item.last_worn_on}` : 'Never worn'}
            </div>
          </div>
        ))}
      </div>

      <div className="wy-sec flex justify-between px-4 py-2.5 border-b border-(--color-line)">
        <span className="wy-lbl">WEAR LOG</span>
      </div>
      {history.data && history.data.wears.length === 0 && (
        <p className="px-4 py-4 text-sm">Nothing logged yet.</p>
      )}
      {history.data?.wears.map((wear) => (
        <div key={wear.id} className="flex items-center gap-3 px-4 py-3 border-b border-(--color-line)">
          <div className="flex gap-1 overflow-x-auto flex-1">
            {wear.items.map((item) => (
              <div key={item.id} className="wy-block w-12 h-12 flex-shrink-0" style={{ background: blockOf(item.category) }}>
                <ItemImg item={item} tile={1} />
              </div>
            ))}
          </div>
          <div className="flex flex-col items-end gap-1">
            <span className="wy-lbl">
              {wear.worn_on}
              {wear.occasion ? ` — ${wear.occasion.toUpperCase()}` : ''}
            </span>
            <button
              type="button"
              onClick={() => remove.mutate(wear.id)}
              disabled={remove.isPending}
              className="text-xs text-(--color-accent-700) font-extrabold uppercase tracking-wide"
            >
              Remove
            </button>
          </div>
        </div>
      ))}
    </div>
  )
}
