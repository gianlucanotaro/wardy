import { useEffect, useRef, useState } from 'react'
import type { Look } from './api'

// Above this many looks the sheet gets a search box (#76: "from 9 looks").
const SEARCH_FROM = 9
// Dragging the sheet's top down by this much closes it.
const SWIPE_CLOSE_PX = 80

const WEEKDAY = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT']
const MONTH = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']

// The look picker sheet (#76), per frame 3 of docs/mockups/plan.html: tap a
// look to plan it, CLEAR DAY to unplan, CANCEL / ✕ / the scrim / Escape /
// a swipe down on the top of the sheet to close. A native modal <dialog>
// gives the focus trap, Escape and an inert page behind it.
export default function PlanPicker({
  date,
  looks,
  currentLookId,
  onPick,
  onClear,
  onClose,
}: {
  date: Date
  looks: Look[]
  currentLookId: number | null
  onPick: (lookId: number) => void
  onClear: () => void
  onClose: () => void
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const closeButton = useRef<HTMLButtonElement>(null)
  const [query, setQuery] = useState('')
  const [dragY, setDragY] = useState(0)
  const dragStart = useRef<number | null>(null)

  // Open modally, stop the page scrolling underneath, and hand focus back
  // to whatever opened the sheet (the day card) when it goes away.
  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null
    dialog.current?.showModal()
    // Not the search box: that would pop the keyboard over the list.
    closeButton.current?.focus()
    const overflow = document.documentElement.style.overflow
    document.documentElement.style.overflow = 'hidden'
    return () => {
      document.documentElement.style.overflow = overflow
      opener?.focus()
    }
  }, [])

  // The day's current look first, then the rest in the usual order.
  const q = query.trim().toLowerCase()
  const shown = looks
    .filter((l) => !q || l.name.toLowerCase().includes(q))
    .sort((a, b) => Number(b.id === currentLookId) - Number(a.id === currentLookId))
  const spoken = date.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })

  function onGrabDown(e: React.PointerEvent<HTMLDivElement>) {
    dragStart.current = e.clientY
    e.currentTarget.setPointerCapture(e.pointerId)
  }
  function onGrabMove(e: React.PointerEvent<HTMLDivElement>) {
    if (dragStart.current === null) return
    setDragY(Math.max(0, e.clientY - dragStart.current))
  }
  function onGrabUp() {
    if (dragStart.current === null) return
    dragStart.current = null
    if (dragY > SWIPE_CLOSE_PX) onClose()
    else setDragY(0)
  }

  return (
    <dialog
      ref={dialog}
      aria-label={`Choose a look for ${spoken}`}
      // Escape: let React unmount us rather than the browser closing it.
      onCancel={(e) => {
        e.preventDefault()
        onClose()
      }}
      className="fixed inset-0 z-20 m-0 w-full h-full max-w-none max-h-none p-0 bg-transparent flex flex-col justify-end backdrop:bg-transparent"
    >
      <button
        type="button"
        tabIndex={-1}
        aria-hidden="true"
        onClick={onClose}
        className="absolute inset-0 bg-(--color-ink)/45"
      />
      <div
        className="relative flex flex-col max-h-[85dvh] bg-(--color-bg) text-(--color-ink) border-t-2 border-(--color-line-strong) safe-area-overlay pt-0!"
        style={{ transform: dragY ? `translateY(${dragY}px)` : undefined }}
      >
        {/* Grab handle and title: the drag-down-to-close area. */}
        <div
          className="touch-none select-none"
          onPointerDown={onGrabDown}
          onPointerMove={onGrabMove}
          onPointerUp={onGrabUp}
          onPointerCancel={onGrabUp}
        >
          <div className="h-5 grid place-items-center" aria-hidden="true">
            <i className="block w-10 h-1 bg-(--color-ink)" />
          </div>
          <div className="flex items-start justify-between px-1 pb-2.5 pt-0.5 border-b-2 border-(--color-line-strong)">
            <div>
              <span className="wy-lbl">
                {WEEKDAY[date.getDay()]} {date.getDate()} {MONTH[date.getMonth()]} · {looks.length} LOOK
                {looks.length === 1 ? '' : 'S'}
              </span>
              <h2 className="font-extrabold text-2xl leading-7 tracking-[-0.02em]">CHOOSE A LOOK</h2>
            </div>
            <button
              type="button"
              ref={closeButton}
              onClick={onClose}
              aria-label="Close"
              className="w-11 h-11 shrink-0 grid place-items-center border border-(--color-line)"
            >
              <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="square">
                <path d="M5 5l14 14M19 5 5 19" />
              </svg>
            </button>
          </div>
        </div>

        {looks.length >= SEARCH_FROM && (
          <label className="flex items-center gap-2 h-11 mx-1 my-2.5 px-3 border border-(--color-line-strong) shrink-0">
            <svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
              <circle cx="10.5" cy="10.5" r="6.5" />
              <path d="M15.5 15.5 21 21" />
            </svg>
            {/* 16px so iOS doesn't zoom the page on focus. */}
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search looks"
              aria-label="Search looks"
              className="flex-1 min-w-0 bg-transparent text-base outline-none placeholder:text-(--color-neutral-700)"
            />
          </label>
        )}

        {/* Occasion chips (#82) go here, between the search and the list. */}

        <ul className="flex-1 min-h-0 overflow-y-auto overscroll-contain border-t border-(--color-line)">
          {shown.map((look) => (
            <li key={look.id}>
              <LookRow look={look} on={look.id === currentLookId} onClick={() => (look.id === currentLookId ? onClose() : onPick(look.id))} />
            </li>
          ))}
          {shown.length === 0 && <li className="px-1 py-6 text-sm text-(--color-neutral-700)">No looks match "{query.trim()}".</li>}
        </ul>

        <div className="grid grid-cols-2 border-t-2 border-(--color-line-strong) -mx-(--gutter-x) shrink-0">
          <button type="button" onClick={onClear} disabled={currentLookId == null} className="wy-act text-(--color-accent-700)">
            CLEAR DAY
          </button>
          <button type="button" onClick={onClose} className="wy-act border-l-2 border-(--color-line-strong)">
            CANCEL
          </button>
        </div>
      </div>
    </dialog>
  )
}

function LookRow({ look, on, onClick }: { look: Look; on: boolean; onClick: () => void }) {
  // Five slots, like the day cards: up to five thumbs, or four and "+N".
  const shown = look.items.length > 5 ? look.items.slice(0, 4) : look.items
  const more = look.items.length - shown.length
  const thumb = 'w-[30px] h-[40px] shrink-0 border border-(--color-line-strong)'
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={on}
      className="w-full min-h-[76px] grid items-center gap-2.5 px-1 py-2 border-b border-(--color-line) text-left focus-visible:outline-2 focus-visible:outline-(--color-line-strong) focus-visible:-outline-offset-2"
      style={{
        gridTemplateColumns: '1fr auto',
        ...(on ? { background: 'var(--color-accent-100)', boxShadow: 'inset 4px 0 0 var(--color-accent)' } : {}),
      }}
    >
      <span className={`min-w-0 flex flex-col ${on ? 'pl-2' : ''}`}>
        <span className={`font-extrabold text-sm leading-[18px] uppercase tracking-[-0.01em] truncate ${on ? 'text-(--color-accent-700)' : ''}`}>
          {look.name}
        </span>
        <span className="text-[11.5px] text-(--color-neutral-700) mb-1.5">
          {look.items.length} piece{look.items.length === 1 ? '' : 's'}
          {on ? ' · planned for this day' : ''}
        </span>
        <span className="h-10 flex gap-[3px] overflow-hidden">
          {shown.map((it) => (
            <img key={it.id} src={`/api/items/${it.id}/thumb`} alt="" loading="lazy" className={`${thumb} object-cover bg-(--color-surface)`} />
          ))}
          {more > 0 && (
            <span className={`${thumb} flex items-center justify-center bg-(--color-bg) font-extrabold text-[11px]`}>+{more}</span>
          )}
        </span>
      </span>
      <span className="w-11 h-11 grid place-items-center text-(--color-accent-700)" aria-hidden="true">
        {on && (
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="square">
            <path d="M20 6 9 17l-5-5" />
          </svg>
        )}
      </span>
    </button>
  )
}
