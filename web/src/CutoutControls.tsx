import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { removeItemMask, setItemMask, type Item } from './api'
import { cutoutInstalled, cutoutSupported, makeMask } from './cutout'
import Notice from './Notice'
import { useToast } from './useToast'

// In the item view: cut the photo out (or again), or take the cut-out off.
// Only where this phone can do it and the server has the model.
export default function CutoutControls({ item }: { item: Item }) {
  const queryClient = useQueryClient()
  const flash = useToast()
  const installed = useQuery({ queryKey: ['cutout-installed'], queryFn: cutoutInstalled, staleTime: Infinity })
  const [status, setStatus] = useState<string | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['items'] })

  const cut = useMutation({
    mutationFn: async () => {
      const photo = await (await fetch(`/api/items/${item.id}/photo`)).blob()
      return setItemMask(item.id, await makeMask(photo, setStatus))
    },
    onSuccess: () => {
      refresh()
      flash('CUT OUT')
    },
    onError: () => flash("COULDN'T CUT OUT"),
    onSettled: () => setStatus(null),
  })
  const clear = useMutation({
    mutationFn: () => removeItemMask(item.id),
    onSuccess: () => {
      refresh()
      flash('CUT-OUT REMOVED')
    },
  })

  if (!cutoutSupported() || !installed.data) return null
  const busy = cut.isPending || clear.isPending
  return (
    <div className="px-4 py-3 border-t border-(--color-line) flex flex-col gap-2">
      <span className="wy-lbl">CUT-OUT</span>
      <div className="flex flex-wrap gap-2">
        <button type="button" className="wy-chip" disabled={busy} onClick={() => cut.mutate()}>
          {item.mask_path ? 'redo the cut-out' : 'cut out the background'}
        </button>
        {item.mask_path && (
          <button type="button" className="wy-chip" disabled={busy} onClick={() => clear.mutate()}>
            remove it
          </button>
        )}
      </div>
      {status && <Notice kind="loading">{status}…</Notice>}
    </div>
  )
}
