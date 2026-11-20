import { useState } from 'react'
import ColorSwatches from './ColorSwatches'
import {
  activeFields,
  COLOR_SWATCHES,
  FILTER_FIELDS,
  fieldValues,
  narrowedCategories,
  SHORTCUT_FIELDS,
  triggerField,
  withoutField,
  type FilterField,
  type Filters,
} from './searchFilters'

// The wardrobe's search box (#14): free text searches custom tags; typing
// a field name and a colon ("color:") or tapping a + FIELD button opens a
// checklist of that field's values. Picked values become a chip under the
// box — tap it to change the picks, ✕ to drop it.
export default function WardrobeSearch({
  query,
  onQueryChange,
  filters,
  onFiltersChange,
}: {
  query: string
  onQueryChange: (q: string) => void
  filters: Filters
  onFiltersChange: (f: Filters) => void
}) {
  // Opened by a button or chip. Typing "field:" opens it too, derived
  // from the text rather than stored, so backspacing closes it.
  const [pickerField, setPickerField] = useState<FilterField | null>(null)
  const [showMore, setShowMore] = useState(false)
  const typed = triggerField(query)
  const open = pickerField ?? typed
  const active = activeFields(filters)

  function close() {
    setPickerField(null)
    if (typed) onQueryChange('')
  }

  function setValues(field: FilterField, values: string[]) {
    onFiltersChange(values.length ? { ...filters, [field]: values } : withoutField(filters, field))
  }

  const offered = (showMore ? FILTER_FIELDS : SHORTCUT_FIELDS).filter((f) => !active.includes(f))
  const hasMore = !showMore && FILTER_FIELDS.some((f) => !SHORTCUT_FIELDS.includes(f) && !active.includes(f))

  return (
    <div className="flex flex-col gap-2.5" onKeyDown={(e) => e.key === 'Escape' && open && close()}>
      {open && <div className="fixed inset-0 z-20 bg-(--color-bg)/60" onClick={close} aria-hidden="true" />}
      <div className={`relative ${open ? 'z-30' : ''}`}>
        <svg
          aria-hidden="true"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2.2"
          strokeLinecap="square"
          className="absolute left-[11px] top-3.5 text-(--color-neutral-700)"
        >
          <circle cx="10.5" cy="10.5" r="6.5" />
          <path d="M15.5 15.5 21 21" />
        </svg>
        <input
          type="text"
          value={query}
          onChange={(e) => onQueryChange(e.target.value)}
          placeholder="SEARCH TAGS, OR TYPE COLOR: …"
          aria-label="Search your wardrobe"
          className="wy-input w-full h-11 pl-[34px] pr-3 text-sm"
        />
        {open && (
          <Picker
            field={open}
            filters={filters}
            onChange={(values) => setValues(open, values)}
            onDone={close}
          />
        )}
      </div>

      {active.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5">
          {active.map((f) => (
            <div
              key={f}
              className="inline-flex items-stretch max-w-full min-h-11 rounded-full overflow-hidden bg-(--color-ink) text-(--color-bg)"
            >
              <button
                type="button"
                onClick={() => setPickerField(f)}
                className="pl-4 pr-2 py-1.5 text-left font-bold text-xs flex items-center gap-1.5"
              >
                {f === 'color' &&
                  filters[f]!.map((c) => (
                    <span
                      key={c}
                      aria-hidden="true"
                      className="size-3.5 shrink-0 rounded-full border border-(--color-bg)/40"
                      style={{ background: COLOR_SWATCHES[c as keyof typeof COLOR_SWATCHES] }}
                    />
                  ))}
                <span>
                  <span className="opacity-70">{f}:</span> {filters[f]!.join(', ')}
                </span>
              </button>
              <button
                type="button"
                onClick={() => setValues(f, [])}
                aria-label={`Remove ${f} filter`}
                className="w-11 shrink-0 flex items-center justify-center border-l border-(--color-bg)/25"
              >
                <svg
                  aria-hidden="true"
                  width="12"
                  height="12"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="3"
                  strokeLinecap="square"
                >
                  <path d="M5 5l14 14M19 5 5 19" />
                </svg>
              </button>
            </div>
          ))}
          <button
            type="button"
            onClick={() => {
              onFiltersChange({})
              onQueryChange('')
            }}
            className="ml-auto min-h-11 px-1 font-bold text-xs text-(--color-accent-700) underline underline-offset-3 lowercase"
          >
            CLEAR ALL
          </button>
        </div>
      )}

      {(offered.length > 0 || hasMore) && (
        <div className="flex flex-col gap-1.5">
          {active.length === 0 && <div className="wy-lbl">FILTER BY</div>}
          <div className="flex flex-wrap gap-1.5">
            {offered.map((f) => (
              <button key={f} type="button" onClick={() => setPickerField(f)} className="wy-chip">
                + {f.toUpperCase()}
              </button>
            ))}
            {hasMore && (
              <button type="button" onClick={() => setShowMore(true)} className="wy-chip">
                + MORE
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function Picker({
  field,
  filters,
  onChange,
  onDone,
}: {
  field: FilterField
  filters: Filters
  onChange: (values: string[]) => void
  onDone: () => void
}) {
  const picked = filters[field] ?? []
  const narrowed = field === 'subcategory' ? narrowedCategories(filters) : []
  const label = field.toUpperCase()
  const toggle = (v: string) => onChange(picked.includes(v) ? picked.filter((p) => p !== v) : [...picked, v])

  return (
    <div
      role="group"
      aria-label={`Pick ${field}`}
      className="absolute left-0 right-0 top-full mt-2 z-30 bg-(--color-bg) border-2 border-(--color-line-strong) flex flex-col"
    >
      <div className="flex flex-col gap-1 px-2.5 pt-2.5 pb-2 border-b border-(--color-line)">
        <div className="flex items-baseline justify-between gap-2">
          <span className="font-extrabold text-[13px] tracking-[0.06em]">{label}</span>
          {field === 'color' && <span className="wy-lbl">MAIN OR SECOND COLOR</span>}
          {narrowed.length > 0 && (
            <span className="wy-lbl text-(--color-accent-700)">{narrowed.join(', ').toUpperCase()} ONLY</span>
          )}
        </div>
        {narrowed.length > 0 && (
          <span className="text-[11.5px] leading-snug text-(--color-neutral-700)">
            Narrowed by your category filter. Remove it to see every subcategory.
          </span>
        )}
      </div>

      <div className="max-h-[50vh] overflow-y-auto px-2.5 py-3">
        {field === 'color' ? (
          <ColorSwatches value={picked} options={[...fieldValues(field, filters)]} onPick={toggle} showNames={false} />
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {fieldValues(field, filters).map((v) => {
              const on = picked.includes(v)
              return (
                <button
                  key={v}
                  type="button"
                  className="wy-chip"
                  data-on={on || undefined}
                  aria-pressed={on}
                  onClick={() => toggle(v)}
                >
                  {v}
                </button>
              )
            })}
          </div>
        )}
      </div>

      <div className="flex items-center justify-between gap-2 px-2.5 py-2 border-t border-(--color-line)">
        <span className="wy-lbl text-(--color-accent-700) truncate">
          {picked.length} SELECTED{picked.length > 0 && ` · ${picked.join(', ')}`}
        </span>
        <div className="flex gap-1.5 shrink-0">
          {picked.length > 0 && (
            <button type="button" onClick={() => onChange([])} className="wy-chip">
              CLEAR
            </button>
          )}
          <button
            type="button"
            onClick={onDone}
            className="wy-chip min-w-[72px]"
            style={{ background: 'var(--color-accent)', color: 'var(--color-on-accent)', borderColor: 'var(--color-accent)' }}
          >
            DONE
          </button>
        </div>
      </div>
    </div>
  )
}
