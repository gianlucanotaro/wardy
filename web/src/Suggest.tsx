import { useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { createLook, getSuggestions } from './api'
import { OCCASIONS } from './taxonomy'

// Three states per docs/frontend.md: normal (source "ai" — outfit.note is
// the model's rationale), rules-only-fallback (source "rules_fallback" —
// outfit.note is the plain factual note, shown with a quiet banner), and
// nothing-matches (no outfits, no_match_reason set).
export default function Suggest({ onClose }: { onClose: () => void }) {
  const [occasion, setOccasion] = useState<string>('casual')
  // Keyed by occasion+index so switching occasions never shows a stale
  // "Saved" state for a different outfit that happens to share an index.
  const [savedKeys, setSavedKeys] = useState<Set<string>>(new Set())

  const suggestions = useQuery({
    queryKey: ['suggest', occasion],
    queryFn: () => getSuggestions(occasion),
  })

  const saveAsLook = useMutation({
    mutationFn: (input: { key: string; itemIds: number[] }) =>
      createLook({
        name: `${occasion.charAt(0).toUpperCase()}${occasion.slice(1)} look`,
        item_ids: input.itemIds,
      }),
    onSuccess: (_look, input) => setSavedKeys((prev) => new Set(prev).add(input.key)),
  })

  return (
    <div className="flex flex-col gap-4 p-4 max-w-2xl mx-auto w-full">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">Suggest an outfit</h2>
        <button type="button" onClick={onClose} className="text-sm underline">
          Close
        </button>
      </div>

      <div className="flex flex-wrap gap-1">
        {OCCASIONS.map((o) => (
          <button
            type="button"
            key={o}
            onClick={() => setOccasion(o)}
            className={`border rounded px-3 py-1.5 text-sm capitalize ${
              o === occasion ? 'bg-black text-white' : 'bg-white'
            }`}
          >
            {o}
          </button>
        ))}
      </div>

      {suggestions.isLoading && <p className="text-sm text-gray-500">Finding outfits…</p>}
      {suggestions.isError && <p className="text-sm text-red-600">Couldn't load suggestions.</p>}

      {suggestions.data && !suggestions.data.weather_available && (
        <p className="text-xs text-gray-400">
          Weather unavailable — suggestions don't account for today's temperature.
        </p>
      )}

      {suggestions.data && suggestions.data.source === 'rules_fallback' && (
        <p className="text-xs text-gray-400">Showing simple matches — AI suggestions are unavailable right now.</p>
      )}

      {suggestions.data && suggestions.data.outfits.length === 0 && (
        <p className="text-sm text-gray-500">
          {suggestions.data.no_match_reason || 'Nothing matches yet — tag a few more items.'}
        </p>
      )}

      <div className="flex flex-col gap-3">
        {suggestions.data?.outfits.map((outfit, i) => {
          const key = `${occasion}-${i}`
          const saved = savedKeys.has(key)
          return (
            <div key={i} className="border rounded-lg p-3 flex flex-col gap-2">
              <div className="flex gap-2 overflow-x-auto">
                {outfit.items.map((item) => (
                  <img
                    key={item.id}
                    src={`/api/items/${item.id}/thumb`}
                    alt={item.name || item.category}
                    className="w-20 h-20 object-cover rounded flex-shrink-0"
                  />
                ))}
              </div>
              <p className="text-xs text-gray-400">{outfit.note}</p>
              <button
                type="button"
                disabled={saved || saveAsLook.isPending}
                onClick={() => saveAsLook.mutate({ key, itemIds: outfit.items.map((it) => it.id) })}
                className="self-start text-sm underline disabled:no-underline disabled:text-gray-400"
              >
                {saved ? 'Saved' : 'Save as look'}
              </button>
            </div>
          )
        })}
      </div>
    </div>
  )
}
