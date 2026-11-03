import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateItemTags, type Item, type TagsPayload } from './api'
import {
  CATEGORIES,
  SUBCATEGORIES,
  PATTERNS,
  MATERIALS,
  FITS,
  COLORS,
  SEASONS,
  OCCASIONS,
  STYLES,
  MAX_ITEM_STYLES,
  SCALE_MIN,
  SCALE_MAX,
  FORMALITY_LABELS,
  WARMTH_LABELS,
  type Category,
} from './taxonomy'

function draftFromItem(item: Item): TagsPayload {
  const category = (item.category || CATEGORIES[0]) as Category
  return {
    category,
    subcategory: item.subcategory || SUBCATEGORIES[category][0],
    color_primary: item.color_primary || COLORS[0],
    color_secondary: item.color_secondary || '',
    pattern: item.pattern || PATTERNS[0],
    material: item.material || 'unknown',
    fit: item.fit || 'regular',
    formality_min: item.formality_min || 3,
    formality_max: item.formality_max || 3,
    warmth_min: item.warmth_min || 1,
    warmth_max: item.warmth_max || 5,
    seasons: item.seasons ?? [],
    occasions: item.occasions ?? [],
    custom_tags: item.custom_tags ?? [],
    styles: item.styles ?? [],
  }
}

// Every field here is a picker, never a text input — free text anywhere in
// this form defeats the whole point of the closed vocabulary. Each change
// saves immediately (optimistic update), matching the "one-tap save" and
// "edits must feel instant" requirements for the screen used most often in
// the app.
//
// markAiSuggested is only ever true from the add-item review flow, right
// after a tagging job lands on an item nobody has touched yet — the AI sets
// every field in one shot, so the marker is all-or-nothing rather than
// diffed per field. The moment any field is edited here, UpdateItemTags
// sets user_edited_at and the marker disappears on its own next render
// (the caller re-derives markAiSuggested from the live item, not a flag
// that outlives the edit).
export default function TagEditor({ item, markAiSuggested }: { item: Item; markAiSuggested?: boolean }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<TagsPayload>(() => draftFromItem(item))

  // Re-sync the draft when a tagging job lands while this component stays
  // mounted across the whole poll (the add-item review screen's whole
  // premise) — but never once the user has made their own edit, since
  // UpdateItemAITags's user_edited_at guard means the item's tags can't
  // change server-side after that anyway, so there's nothing to catch up
  // to.
  const prevAiTaggedAt = useRef(item.ai_tagged_at)
  useEffect(() => {
    if (item.ai_tagged_at !== prevAiTaggedAt.current && !item.user_edited_at) {
      setDraft(draftFromItem(item))
    }
    prevAiTaggedAt.current = item.ai_tagged_at
  }, [item])

  const mutation = useMutation({
    mutationFn: (next: TagsPayload) => updateItemTags(item.id, next),
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey: ['items'] })
      const previous = queryClient.getQueryData<Item[]>(['items'])
      queryClient.setQueryData<Item[]>(['items'], (old) =>
        old?.map((it) => (it.id === item.id ? { ...it, ...next } : it)),
      )
      return { previous }
    },
    onError: (_err, _next, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['items'], context.previous)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['items'] })
    },
  })

  function save(next: TagsPayload) {
    setDraft(next)
    mutation.mutate(next)
  }

  const subcategories = SUBCATEGORIES[draft.category as Category] ?? []

  return (
    <div className="flex flex-col gap-4 text-left">
      <Field label="CATEGORY" aiSuggested={markAiSuggested}>
        <ChipGroup
          value={draft.category}
          options={[...CATEGORIES]}
          onChange={(category) =>
            save({ ...draft, category, subcategory: SUBCATEGORIES[category as Category][0] })
          }
        />
      </Field>

      <Field label="SUBCATEGORY" aiSuggested={markAiSuggested}>
        <ChipGroup value={draft.subcategory} options={subcategories} onChange={(subcategory) => save({ ...draft, subcategory })} />
      </Field>

      <Field label="COLOR" aiSuggested={markAiSuggested}>
        <ChipGroup value={draft.color_primary} options={[...COLORS]} onChange={(color_primary) => save({ ...draft, color_primary })} />
      </Field>

      <Field label="SECONDARY COLOR">
        <ChipGroup
          value={draft.color_secondary}
          options={['', ...COLORS]}
          labels={{ '': 'NONE' }}
          onChange={(color_secondary) => save({ ...draft, color_secondary })}
        />
      </Field>

      <Field label="PATTERN" aiSuggested={markAiSuggested}>
        <ChipGroup value={draft.pattern} options={[...PATTERNS]} onChange={(pattern) => save({ ...draft, pattern })} />
      </Field>

      <Field label="MATERIAL" aiSuggested={markAiSuggested} strongCallout="LEAST RELIABLE — CHECK THIS">
        <ChipGroup value={draft.material} options={[...MATERIALS]} onChange={(material) => save({ ...draft, material })} />
      </Field>

      <Field label="FIT" aiSuggested={markAiSuggested}>
        <ChipGroup value={draft.fit} options={[...FITS]} onChange={(fit) => save({ ...draft, fit })} />
      </Field>

      <ScaleField
        label="FORMALITY"
        aiSuggested={markAiSuggested}
        min={draft.formality_min}
        max={draft.formality_max}
        labels={FORMALITY_LABELS}
        onChange={(formality_min, formality_max) => save({ ...draft, formality_min, formality_max })}
      />

      <ScaleField
        label="WARMTH"
        aiSuggested={markAiSuggested}
        min={draft.warmth_min}
        max={draft.warmth_max}
        labels={WARMTH_LABELS}
        onChange={(warmth_min, warmth_max) => save({ ...draft, warmth_min, warmth_max })}
      />

      <MultiField
        label="SEASONS"
        aiSuggested={markAiSuggested}
        value={draft.seasons}
        options={[...SEASONS]}
        onChange={(seasons) => save({ ...draft, seasons })}
      />

      <MultiField
        label="OCCASIONS"
        aiSuggested={markAiSuggested}
        value={draft.occasions}
        options={[...OCCASIONS]}
        onChange={(occasions) => save({ ...draft, occasions })}
      />

      <MultiField
        label="STYLE"
        aiSuggested={markAiSuggested}
        value={draft.styles}
        options={[...STYLES]}
        max={MAX_ITEM_STYLES}
        onChange={(styles) => save({ ...draft, styles })}
      />

      {mutation.isError && <p className="text-sm text-(--color-accent-700)">Couldn't save that change.</p>}

      <CustomTagsField value={draft.custom_tags} onChange={(custom_tags) => save({ ...draft, custom_tags })} />
    </div>
  )
}

// Deliberately its own bordered section, not another MultiField above —
// these aren't picked from a closed vocabulary, they're free text the
// user (or, initially, the AI's best guess) attaches for their own
// organization. Visually distinct so it's never mistaken for one of the
// validated fields above it.
export function CustomTagsField({ value, onChange }: { value: string[]; onChange: (value: string[]) => void }) {
  const [draft, setDraft] = useState('')

  function addTag() {
    const tag = draft.trim()
    if (tag && !value.includes(tag)) {
      onChange([...value, tag])
    }
    setDraft('')
  }

  return (
    <div className="flex flex-col gap-2 border border-(--color-line) bg-(--color-surface) p-3">
      <span className="wy-lbl">YOUR TAGS</span>

      <div className="flex flex-wrap gap-1.5">
        {value.map((tag) => (
          <span
            key={tag}
            className="flex items-center gap-1.5 border border-(--color-line-strong) px-2 py-1 text-xs font-extrabold uppercase tracking-wide bg-(--color-bg)"
          >
            {tag}
            <button
              type="button"
              onClick={() => onChange(value.filter((t) => t !== tag))}
              aria-label={`Remove tag ${tag}`}
              className="text-(--color-neutral-500)"
            >
              ×
            </button>
          </span>
        ))}
      </div>

      <input
        type="text"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ',') {
            e.preventDefault()
            addTag()
          }
        }}
        onBlur={addTag}
        placeholder="ADD A TAG…"
        className="border border-(--color-line) bg-(--color-bg) px-2 py-1.5 text-sm focus:outline-2 focus:outline-(--color-accent)"
      />
    </div>
  )
}

export function Field({
  label,
  aiSuggested,
  strongCallout,
  children,
}: {
  label: string
  aiSuggested?: boolean
  strongCallout?: string
  children: ReactNode
}) {
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <span className="wy-lbl flex items-center gap-1.5">
        {label}
        {aiSuggested && <AiDot />}
      </span>
      {aiSuggested && strongCallout && (
        <span className="text-xs font-extrabold text-(--color-accent-700)">● {strongCallout}</span>
      )}
      {children}
    </div>
  )
}

// The small "AI suggested" marker from Wardy Prototype.dc.html's isAdd
// review stage — a dot rather than a text label so it doesn't compete with
// the field's own label, but named for assistive tech since it's otherwise
// pure decoration.
function AiDot() {
  return (
    <span className="text-(--color-accent)" aria-label="AI suggested">
      ●
    </span>
  )
}

export function ChipGroup({
  value,
  options,
  labels,
  onChange,
}: {
  value: string
  options: string[]
  labels?: Record<string, string>
  onChange: (value: string) => void
}) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {options.map((opt) => (
        <button
          type="button"
          key={opt}
          className="wy-chip"
          data-on={opt === value || undefined}
          onClick={() => onChange(opt)}
        >
          {(labels?.[opt] ?? opt).toUpperCase()}
        </button>
      ))}
    </div>
  )
}

// A range on a 1-5 scale, set with two taps: the first tap picks a single
// step (and saves it), the second makes the range between the two. A
// third tap starts over. Ranges, not single values, because a shirt can
// be formality 2-4 and warmth 1-5 (worn layered in winter).
export function ScaleField({
  label,
  aiSuggested,
  min,
  max,
  labels,
  onChange,
}: {
  label: string
  aiSuggested?: boolean
  min: number
  max: number
  labels: Record<number, string>
  onChange: (min: number, max: number) => void
}) {
  const values = Array.from({ length: SCALE_MAX - SCALE_MIN + 1 }, (_, i) => SCALE_MIN + i)
  // The first tap of a pair, waiting for the second.
  const [anchor, setAnchor] = useState<number | null>(null)

  function tap(v: number) {
    if (anchor === null) {
      setAnchor(v)
      onChange(v, v)
    } else {
      setAnchor(null)
      onChange(Math.min(anchor, v), Math.max(anchor, v))
    }
  }

  // 0-0 is an unset range (an inspo fit nobody has rated yet).
  const rangeLabel = !min ? '—' : min === max ? labels[min] : `${labels[min]} – ${labels[max]}`
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <div className="flex justify-between gap-2">
        <span className="wy-lbl flex items-center gap-1.5">
          {label}
          {aiSuggested && <AiDot />}
        </span>
        <span className="wy-lbl text-right">{rangeLabel.toUpperCase()}</span>
      </div>
      <div className="grid grid-cols-5 gap-1">
        {values.map((v) => {
          const on = v >= min && v <= max
          return (
            <button
              type="button"
              key={v}
              onClick={() => tap(v)}
              aria-label={`${label} ${v}: ${labels[v]}`}
              aria-pressed={on}
              className={`min-h-11 border font-extrabold text-xs ${
                v === anchor ? 'border-(--color-ink) border-2' : 'border-(--color-line)'
              }`}
              style={{
                background: on ? 'var(--color-accent)' : 'transparent',
                color: on ? 'var(--color-bg)' : 'var(--color-ink)',
              }}
            >
              {v}
            </button>
          )
        })}
      </div>
    </div>
  )
}

// max caps how many can be on at once: picking one more drops the oldest.
export function MultiField({
  label,
  aiSuggested,
  value,
  options,
  max,
  onChange,
}: {
  label: string
  aiSuggested?: boolean
  value: string[]
  options: string[]
  max?: number
  onChange: (value: string[]) => void
}) {
  function toggle(opt: string) {
    if (value.includes(opt)) {
      onChange(value.filter((v) => v !== opt))
    } else {
      onChange([...value, opt].slice(max ? -max : 0))
    }
  }

  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <span className="wy-lbl flex items-center gap-1.5">
        {label}
        {aiSuggested && <AiDot />}
      </span>
      <div className="flex flex-wrap gap-1.5">
        {options.map((opt) => (
          <button type="button" key={opt} className="wy-chip" data-on={value.includes(opt) || undefined} onClick={() => toggle(opt)}>
            {opt.toUpperCase()}
          </button>
        ))}
      </div>
    </div>
  )
}
