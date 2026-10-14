import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { ToastContext } from './useToast'

// One app-wide toast, matching the design's single flash(msg): a dark
// banner pinned above the tab bar that clears itself after ~2s. A new
// flash replaces whatever is showing (no queue — nothing here fires fast
// enough to need one) and restarts the timer, even for the same text.
const DURATION_MS = 2000

export function ToastProvider({ children }: { children: ReactNode }) {
  // `id` changes on every flash so repeating the same message still resets
  // the timer below.
  const [toast, setToast] = useState<{ id: number; message: string } | null>(null)

  const flash = useCallback((message: string) => {
    setToast((prev) => ({ id: (prev?.id ?? 0) + 1, message }))
  }, [])

  useEffect(() => {
    if (!toast) return
    const timer = setTimeout(() => setToast(null), DURATION_MS)
    return () => clearTimeout(timer)
  }, [toast])

  return (
    <ToastContext.Provider value={flash}>
      {children}
      {/* The live region stays mounted so screen readers announce each
          message; only its content comes and goes. z-30 keeps it above
          the full-screen Wardrobe overlays (z-20). */}
      <div
        role="status"
        aria-live="polite"
        className="fixed left-0 right-0 z-30 pointer-events-none"
        style={{
          bottom: 'calc(env(safe-area-inset-bottom) + 64px)',
          paddingLeft: 'max(var(--gutter-x), env(safe-area-inset-left))',
          paddingRight: 'max(var(--gutter-x), env(safe-area-inset-right))',
        }}
      >
        {toast && (
          <div className="wy-toast max-w-2xl mx-auto" key={toast.id}>
            {toast.message}
          </div>
        )}
      </div>
    </ToastContext.Provider>
  )
}
