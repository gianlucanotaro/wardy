import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  deleteInspo,
  getInspo,
  setUseInspo,
  updateInspoTags,
  uploadInspo,
  type InspoBoard,
  type InspoCount,
  type InspoFit,
  type InspoTags,
} from './api'
import { ChipGroup, CustomTagsField, Field, MultiField, ScaleField } from './TagEditor'
import {
  CATEGORIES,
  COLORS,
  COLOR_SWATCH,
  FITS,
  FORMALITY_LABELS,
  MAX_INSPO_COLORS,
  MAX_INSPO_PIECES,
  MAX_INSPO_STYLES,
  OCCASIONS,
  SEASONS,
  STYLES,
  SUBCATEGORIES,
  type Color,
} from './taxonomy'
import { useToast } from './useToast'

// The INSPO screen (#49): photos of other people's outfits, tagged, adding
// up to the style profile on top — which outfit suggestions use as a
// preference once there are enough tagged fits. Same patterns as the
// wardrobe: plain file input, tagging as a polled background job, every
// tag a picker (only YOUR TAGS is free text).

const OVERLAY_BAR = 'sticky top-[env(safe-area-inset-top)] z-10 bg-(--color-bg)'

function tagsOf(fit: InspoFit): InspoTags {
  return {
    styles: fit.styles ?? [],
    colors: fit.colors ?? [],
    pieces: fit.pieces ?? [],
    fit: fit.fit,
    formality_min: fit.formality_min,
    formality_max: fit.formality_max,
    seasons: fit.seasons ?? [],
    occasions: fit.occasions ?? [],
    custom_tags: fit.custom_tags ?? [],
  }
}

function Swatch({ color }: { color: string }) {
  return (
    <span
      className="inline-block w-4 h-4 border border-(--color-line-strong) align-middle"
      style={{ background: COLOR_SWATCH[color as Color] ?? 'transparent' }}
      title={color}
    />
  )
}

export default function Inspo() {
  const queryClient = useQueryClient()
  const flash = useToast()
  const board = useQuery({
    queryKey: ['inspo'],
    queryFn: getInspo,
    // Poll while fits are being tagged or the summary is being rewritten.
    refetchInterval: (query) =>
      query.state.data?.fits.some((f) => f.tagging_pending) || query.state.data?.summary_pending ? 3000 : false,
  })
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const fileInput = useRef<HTMLInputElement>(null)

  const upload = useMutation({
    mutationFn: uploadInspo,
    onSuccess: (fit) => {
      queryClient.setQueryData<InspoBoard>(['inspo'], (old) => (old ? { ...old, fits: [fit, ...old.fits] } : old))
      queryClient.invalidateQueries({ queryKey: ['inspo'] })
      setSelectedId(fit.id)
    },
    onError: (err) => flash(err.message === 'BOARD FULL' ? 'BOARD FULL — DELETE A FEW FITS' : 'UPLOAD FAILED'),
  })

  const toggleUse = useMutation({
    mutationFn: setUseInspo,
    onMutate: async (use) => {
      await queryClient.cancelQueries({ queryKey: ['inspo'] })
      const previous = queryClient.getQueryData<InspoBoard>(['inspo'])
      queryClient.setQueryData<InspoBoard>(['inspo'], (old) => (old ? { ...old, use_in_suggestions: use } : old))
      return { previous }
    },
    onError: (_err, _use, context) => {
      if (context?.previous) queryClient.setQueryData(['inspo'], context.previous)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['inspo'] }),
  })

  const data = board.data
  const selected = data?.fits.find((f) => f.id === selectedId) ?? null
  const profile = data?.profile

  return (
    <div className="flex flex-col">
      <div className="flex items-baseline justify-between px-4 py-3 border-b-2 border-(--color-line-strong)">
        <h1 className="font-extrabold text-3xl tracking-tight">INSPO</h1>
        {data && <span className="wy-lbl">{data.fits.length} FITS</span>}
      </div>

      {board.isLoading && <p className="px-4 py-4 wy-lbl">LOADING…</p>}

      {data && profile && profile.tagged > 0 && (
        <section className="px-4 py-3 border-b-2 border-(--color-line-strong) flex flex-col gap-2">
          <div className="wy-lbl">YOUR STYLE · FROM {profile.tagged} FITS</div>
          {data.summary && <p className="text-sm leading-relaxed">{data.summary}</p>}
          {profile.styles.length > 0 && (
            <div className="flex flex-col gap-1">
              {profile.styles.map((s) => (
                <ShareBar key={s.value} count={s} of={profile.tagged} />
              ))}
            </div>
          )}
          {profile.colors.length > 0 && (
            <div className="flex items-center gap-1">
              <span className="wy-lbl mr-2">COLOURS</span>
              {profile.colors.map((c) => (
                <Swatch key={c.value} color={c.value} />
              ))}
            </div>
          )}
          {profile.pieces.length > 0 && (
            <div className="wy-lbl">
              <span className="mr-2">PIECES</span>
              <span className="text-(--color-ink)">{profile.pieces.map((p) => p.value).join(' · ').toUpperCase()}</span>
            </div>
          )}
          <label className="flex items-center justify-between gap-3 pt-2 mt-1 border-t border-(--color-line) min-h-11">
            <span className="wy-lbl text-(--color-ink)">
              USE IN SUGGESTIONS
              {profile.tagged < data.min_fits && (
                <span className="text-(--color-neutral-700)">
                  {' '}
                  · {profile.tagged}/{data.min_fits}
                </span>
              )}
            </span>
            <input
              type="checkbox"
              className="w-5 h-5 accent-(--color-ink)"
              checked={data.use_in_suggestions}
              onChange={(e) => toggleUse.mutate(e.target.checked)}
            />
          </label>
        </section>
      )}

      {data && data.fits.length === 0 && <p className="px-4 py-4 text-sm">No fits yet.</p>}

      {data && data.fits.length > 0 && (
        <div className="grid grid-cols-3">
          {data.fits.map((fit) => (
            <button type="button" key={fit.id} onClick={() => setSelectedId(fit.id)} className="wy-tile">
              <div className="relative aspect-3/4 border border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
                <img src={`/api/inspo/${fit.id}/thumb`} alt="inspiration fit" className="w-full h-full object-cover" loading="lazy" />
                {fit.styles?.[0] && (
                  <span className="absolute top-1 left-1 bg-(--color-bg) text-[9px] font-extrabold uppercase tracking-wide px-1.5 py-0.5">
                    {fit.styles[0]}
                  </span>
                )}
                {fit.tagging_pending && (
                  <span className="absolute bottom-1 right-1 bg-(--color-ink) text-(--color-bg) text-[9px] font-extrabold uppercase tracking-wide px-1.5 py-0.5">
                    Tagging…
                  </span>
                )}
              </div>
            </button>
          ))}
        </div>
      )}

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
        data-primary="1"
        onClick={() => fileInput.current?.click()}
        disabled={upload.isPending}
        className="wy-act flex justify-between border-b-2 border-(--color-line-strong)"
      >
        <span>{upload.isPending ? 'UPLOADING…' : '+ ADD INSPO'}</span>
        <span>→</span>
      </button>

      {selected && <FitDetail fit={selected} onClose={() => setSelectedId(null)} />}
    </div>
  )
}

// One style's share of the tagged fits, as the design's bar.
function ShareBar({ count, of }: { count: InspoCount; of: number }) {
  const pct = Math.round((count.count / of) * 100)
  return (
    <div className="grid grid-cols-[92px_1fr_34px] items-center gap-2 font-extrabold text-[9.5px] tracking-[0.08em] uppercase">
      <span className="truncate">{count.value}</span>
      <span className="h-2 bg-(--color-surface) relative" aria-hidden="true">
        <span className="absolute inset-y-0 left-0 bg-(--color-ink)" style={{ width: `${pct}%` }} />
      </span>
      <span className="text-right">{pct}%</span>
    </div>
  )
}

// The fit overlay: photo and its tags. Every change saves straight away,
// as in the item tag editor.
function FitDetail({ fit, onClose }: { fit: InspoFit; onClose: () => void }) {
  const queryClient = useQueryClient()
  const flash = useToast()
  const [draft, setDraft] = useState<InspoTags>(() => tagsOf(fit))

  // Pick up AI tags that land while the overlay is open — unless the user
  // already edited, after which AI never overwrites (see TagEditor).
  const prevAiTaggedAt = useRef(fit.ai_tagged_at)
  useEffect(() => {
    if (fit.ai_tagged_at !== prevAiTaggedAt.current && !fit.user_edited_at) {
      setDraft(tagsOf(fit))
    }
    prevAiTaggedAt.current = fit.ai_tagged_at
  }, [fit])

  const save = useMutation({
    mutationFn: (next: InspoTags) => updateInspoTags(fit.id, next),
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey: ['inspo'] })
      const previous = queryClient.getQueryData<InspoBoard>(['inspo'])
      queryClient.setQueryData<InspoBoard>(['inspo'], (old) =>
        old ? { ...old, fits: old.fits.map((f) => (f.id === fit.id ? { ...f, ...next } : f)) } : old,
      )
      return { previous }
    },
    onError: (_err, _next, context) => {
      if (context?.previous) queryClient.setQueryData(['inspo'], context.previous)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['inspo'] }),
  })

  const remove = useMutation({
    mutationFn: () => deleteInspo(fit.id),
    onSuccess: () => {
      onClose()
      queryClient.invalidateQueries({ queryKey: ['inspo'] })
      flash('FIT DELETED')
    },
  })

  function update(next: InspoTags) {
    setDraft(next)
    save.mutate(next)
  }

  const ai = Boolean(fit.ai_tagged_at && !fit.user_edited_at)

  return (
    <div className="fixed inset-0 z-20 bg-(--color-ink)/60 overflow-y-auto safe-area-overlay" onClick={onClose}>
      <div
        className="bg-(--color-bg) min-h-full sm:min-h-0 sm:max-w-lg sm:mx-auto sm:my-8 border-(--color-line-strong) sm:border-2"
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`${OVERLAY_BAR} flex items-center justify-between gap-3 px-4 py-3 border-b-2 border-(--color-line-strong)`}>
          <span className="font-extrabold text-xl tracking-tight">FIT</span>
          <button type="button" onClick={onClose} className="wy-chip">
            ✕ CLOSE
          </button>
        </div>

        <div className="border-b-2 border-(--color-line-strong) p-4">
          <div className="aspect-3/4 border-2 border-(--color-line-strong) bg-(--color-surface) overflow-hidden">
            <img src={`/api/inspo/${fit.id}/photo`} alt="inspiration fit" className="w-full h-full object-cover" />
          </div>
        </div>

        {fit.tagging_pending && <p className="px-4 pt-3 wy-lbl">TAGGING…</p>}

        <div className="p-4 flex flex-col gap-4 text-left">
          <MultiField
            label="STYLE"
            aiSuggested={ai}
            value={draft.styles}
            options={[...STYLES]}
            max={MAX_INSPO_STYLES}
            onChange={(styles) => update({ ...draft, styles })}
          />
          <MultiField
            label="COLOURS"
            aiSuggested={ai}
            value={draft.colors}
            options={[...COLORS]}
            max={MAX_INSPO_COLORS}
            onChange={(colors) => update({ ...draft, colors })}
          />
          <PiecesField ai={ai} value={draft.pieces} onChange={(pieces) => update({ ...draft, pieces })} />
          <Field label="FIT" aiSuggested={ai}>
            <ChipGroup
              value={draft.fit}
              options={['', ...FITS.filter((f) => f !== 'not-applicable')]}
              labels={{ '': 'ANY' }}
              onChange={(f) => update({ ...draft, fit: f })}
            />
          </Field>
          <ScaleField
            label="FORMALITY"
            aiSuggested={ai}
            min={draft.formality_min}
            max={draft.formality_max}
            labels={FORMALITY_LABELS}
            onChange={(formality_min, formality_max) => update({ ...draft, formality_min, formality_max })}
          />
          <MultiField
            label="SEASONS"
            aiSuggested={ai}
            value={draft.seasons}
            options={[...SEASONS]}
            onChange={(seasons) => update({ ...draft, seasons })}
          />
          <MultiField
            label="OCCASIONS"
            aiSuggested={ai}
            value={draft.occasions}
            options={[...OCCASIONS]}
            onChange={(occasions) => update({ ...draft, occasions })}
          />
          {save.isError && <p className="text-sm text-(--color-accent-700)">Couldn't save that change.</p>}
          <CustomTagsField value={draft.custom_tags} onChange={(custom_tags) => update({ ...draft, custom_tags })} />
        </div>

        <div className="grid grid-cols-2 border-t-2 border-(--color-line-strong)">
          <button type="button" data-primary="1" onClick={onClose} className="wy-act">
            DONE
          </button>
          <button
            type="button"
            onClick={() => remove.mutate()}
            disabled={remove.isPending}
            className="wy-act border-l-2 border-(--color-line-strong) text-(--color-accent-700)"
          >
            {remove.isPending ? 'DELETING…' : 'DELETE'}
          </button>
        </div>
      </div>
    </div>
  )
}

// The pieces in a fit: chips to remove, and a picker grouped by category
// to add — the full subcategory list is too long for chips.
function PiecesField({ ai, value, onChange }: { ai: boolean; value: string[]; onChange: (value: string[]) => void }) {
  const full = value.length >= MAX_INSPO_PIECES
  return (
    <Field label="PIECES" aiSuggested={ai}>
      <div className="flex flex-wrap gap-1.5">
        {value.map((p) => (
          <button
            type="button"
            key={p}
            className="wy-chip"
            data-on
            onClick={() => onChange(value.filter((v) => v !== p))}
            aria-label={`Remove ${p}`}
          >
            {p.toUpperCase()} ×
          </button>
        ))}
      </div>
      <select
        value=""
        disabled={full}
        onChange={(e) => e.target.value && onChange([...value, e.target.value])}
        aria-label="Add a piece"
        className="min-h-11 border border-(--color-line) bg-(--color-surface) px-2 text-sm font-extrabold uppercase tracking-wide disabled:opacity-45"
      >
        <option value="">{full ? `MAX ${MAX_INSPO_PIECES}` : '+ ADD A PIECE'}</option>
        {CATEGORIES.map((cat) => (
          <optgroup key={cat} label={cat.toUpperCase()}>
            {SUBCATEGORIES[cat]
              .filter((p) => !value.includes(p))
              .map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
          </optgroup>
        ))}
      </select>
    </Field>
  )
}
