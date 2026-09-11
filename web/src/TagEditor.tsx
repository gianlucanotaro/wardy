import { useState, type ReactNode } from 'react'
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
    formality: item.formality || 3,
    warmth: item.warmth || 3,
    seasons: item.seasons ?? [],
    occasions: item.occasions ?? [],
    custom_tags: item.custom_tags ?? [],
  }
}

// Every field here is a picker, never a text input — free text anywhere in
// this form defeats the whole point of the closed vocabulary. Each change
// saves immediately (optimistic update), matching the "one-tap save" and
// "edits must feel instant" requirements for the screen used most often in
// the app.
export default function TagEditor({ item }: { item: Item }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<TagsPayload>(() => draftFromItem(item))

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
      <Field label="CATEGORY">
        <ChipGroup
          value={draft.category}
          options={[...CATEGORIES]}
          onChange={(category) =>
            save({ ...draft, category, subcategory: SUBCATEGORIES[category as Category][0] })
          }
        />
      </Field>

      <Field label="SUBCATEGORY">
        <ChipGroup value={draft.subcategory} options={subcategories} onChange={(subcategory) => save({ ...draft, subcategory })} />
      </Field>

      <Field label="COLOR">
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

      <Field label="PATTERN">
        <ChipGroup value={draft.pattern} options={[...PATTERNS]} onChange={(pattern) => save({ ...draft, pattern })} />
      </Field>

      <Field label="MATERIAL">
        <ChipGroup value={draft.material} options={[...MATERIALS]} onChange={(material) => save({ ...draft, material })} />
      </Field>

      <Field label="FIT">
        <ChipGroup value={draft.fit} options={[...FITS]} onChange={(fit) => save({ ...draft, fit })} />
      </Field>

      <ScaleField
        label="FORMALITY"
        value={draft.formality}
        labels={FORMALITY_LABELS}
        onChange={(formality) => save({ ...draft, formality })}
      />

      <ScaleField
        label="WARMTH"
        value={draft.warmth}
        labels={WARMTH_LABELS}
        onChange={(warmth) => save({ ...draft, warmth })}
      />

      <MultiField label="SEASONS" value={draft.seasons} options={[...SEASONS]} onChange={(seasons) => save({ ...draft, seasons })} />

      <MultiField
        label="OCCASIONS"
        value={draft.occasions}
        options={[...OCCASIONS]}
        onChange={(occasions) => save({ ...draft, occasions })}
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
function CustomTagsField({ value, onChange }: { value: string[]; onChange: (value: string[]) => void }) {
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
      <p className="text-xs text-(--color-neutral-700)">
        Free-form — "favorite", "needs mending", whatever helps you find it later. Not used for outfit matching
        beyond flavor in AI rationale.
      </p>

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

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <span className="wy-lbl">{label}</span>
      {children}
    </div>
  )
}

function ChipGroup({
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

function ScaleField({
  label,
  value,
  labels,
  onChange,
}: {
  label: string
  value: number
  labels: Record<number, string>
  onChange: (value: number) => void
}) {
  const values = Array.from({ length: SCALE_MAX - SCALE_MIN + 1 }, (_, i) => SCALE_MIN + i)
  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <div className="flex justify-between">
        <span className="wy-lbl">{label}</span>
        <span className="wy-lbl">{labels[value].toUpperCase()}</span>
      </div>
      <div className="grid grid-cols-5 gap-1">
        {values.map((v) => (
          <button
            type="button"
            key={v}
            onClick={() => onChange(v)}
            aria-label={`${label} ${v}: ${labels[v]}`}
            className="min-h-11 border border-(--color-line) font-extrabold text-xs"
            style={{
              background: v === value ? 'var(--color-accent)' : 'transparent',
              color: v === value ? 'var(--color-bg)' : 'var(--color-ink)',
            }}
          >
            {v}
          </button>
        ))}
      </div>
    </div>
  )
}

function MultiField({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string[]
  options: string[]
  onChange: (value: string[]) => void
}) {
  function toggle(opt: string) {
    if (value.includes(opt)) {
      onChange(value.filter((v) => v !== opt))
    } else {
      onChange([...value, opt])
    }
  }

  return (
    <div className="flex flex-col gap-1.5 text-sm">
      <span className="wy-lbl">{label}</span>
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
