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
    material: item.material || MATERIALS[MATERIALS.length - 1], // unknown
    fit: item.fit || FITS[1], // regular
    formality: item.formality || 3,
    warmth: item.warmth || 3,
    seasons: item.seasons ?? [],
    occasions: item.occasions ?? [],
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
    <div className="flex flex-col gap-3 text-left">
      <Field label="Category">
        <Select
          value={draft.category}
          options={[...CATEGORIES]}
          onChange={(category) =>
            save({ ...draft, category, subcategory: SUBCATEGORIES[category as Category][0] })
          }
        />
      </Field>

      <Field label="Subcategory">
        <Select
          value={draft.subcategory}
          options={subcategories}
          onChange={(subcategory) => save({ ...draft, subcategory })}
        />
      </Field>

      <Field label="Color">
        <Select
          value={draft.color_primary}
          options={[...COLORS]}
          onChange={(color_primary) => save({ ...draft, color_primary })}
        />
      </Field>

      <Field label="Secondary color">
        <Select
          value={draft.color_secondary}
          options={['', ...COLORS]}
          labels={{ '': 'None' }}
          onChange={(color_secondary) => save({ ...draft, color_secondary })}
        />
      </Field>

      <Field label="Pattern">
        <Select
          value={draft.pattern}
          options={[...PATTERNS]}
          onChange={(pattern) => save({ ...draft, pattern })}
        />
      </Field>

      <Field label="Material">
        <Select
          value={draft.material}
          options={[...MATERIALS]}
          onChange={(material) => save({ ...draft, material })}
        />
      </Field>

      <Field label="Fit">
        <Select
          value={draft.fit}
          options={[...FITS]}
          onChange={(fit) => save({ ...draft, fit })}
        />
      </Field>

      <ScaleField
        label="Formality"
        value={draft.formality}
        labels={FORMALITY_LABELS}
        onChange={(formality) => save({ ...draft, formality })}
      />

      <ScaleField
        label="Warmth"
        value={draft.warmth}
        labels={WARMTH_LABELS}
        onChange={(warmth) => save({ ...draft, warmth })}
      />

      <MultiField
        label="Seasons"
        value={draft.seasons}
        options={[...SEASONS]}
        onChange={(seasons) => save({ ...draft, seasons })}
      />

      <MultiField
        label="Occasions"
        value={draft.occasions}
        options={[...OCCASIONS]}
        onChange={(occasions) => save({ ...draft, occasions })}
      />

      {mutation.isError && <p className="text-sm text-red-600">Couldn't save that change.</p>}
    </div>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-gray-500">{label}</span>
      {children}
    </label>
  )
}

function Select({
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
    <select
      className="border rounded px-2 py-2 bg-white"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {options.map((opt) => (
        <option key={opt} value={opt}>
          {labels?.[opt] ?? opt}
        </option>
      ))}
    </select>
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
    <div className="flex flex-col gap-1 text-sm">
      <span className="text-gray-500">
        {label}: {labels[value]}
      </span>
      <div className="flex gap-1">
        {values.map((v) => (
          <button
            type="button"
            key={v}
            onClick={() => onChange(v)}
            aria-label={`${label} ${v}: ${labels[v]}`}
            className={`flex-1 border rounded py-2 text-xs ${
              v === value ? 'bg-black text-white' : 'bg-white'
            }`}
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
    <div className="flex flex-col gap-1 text-sm">
      <span className="text-gray-500">{label}</span>
      <div className="flex flex-wrap gap-1">
        {options.map((opt) => (
          <button
            type="button"
            key={opt}
            onClick={() => toggle(opt)}
            className={`border rounded px-2 py-1 text-xs ${
              value.includes(opt) ? 'bg-black text-white' : 'bg-white'
            }`}
          >
            {opt}
          </button>
        ))}
      </div>
    </div>
  )
}
