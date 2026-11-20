import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { logout, type Item, type Me } from './api'
import Today from './Today'
import Wardrobe from './Wardrobe'
import Build from './Build'
import Looks from './Looks'
import Plan from './Plan'
import History from './History'
import Inspo from './Inspo'
import Settings from './Settings'
import { Cross } from './Icons'
import Shape from './Shape'
import type { ShapeName } from './shapes'
import { ToastProvider } from './Toast'

// The persistent chrome: a sticky header (brand + the utility screens the
// design doesn't put in its 5-tab bar) and a sticky bottom tab bar, both
// restyled to the Modernist design system. Owns the top-level View switch —
// tab content below never renders its own "Close" affordance, the tab bar
// and header links are the only navigation.
type View = 'today' | 'wardrobe' | 'build' | 'looks' | 'plan' | 'inspo' | 'history' | 'settings'

// The screens that don't have a tab: behind the menu button.
const MENU: { view: View; label: string; shape: ShapeName; bg: string; fg: string }[] = [
  { view: 'inspo', label: 'inspo', shape: 'triangle', bg: 'var(--color-yellow)', fg: 'var(--color-block-ink)' },
  { view: 'history', label: 'history', shape: 'square', bg: 'var(--color-blue)', fg: 'var(--color-on-accent)' },
  { view: 'settings', label: 'settings', shape: 'half', bg: 'var(--color-block-neutral)', fg: 'var(--color-block-ink)' },
]

const TABS: { key: View; label: string; shape: ShapeName }[] = [
  { key: 'today', label: 'TODAY', shape: 'circle' },
  { key: 'wardrobe', label: 'WARDROBE', shape: 'square' },
  { key: 'build', label: 'BUILD', shape: 'circle' },
  { key: 'looks', label: 'LOOKS', shape: 'triangle' },
  { key: 'plan', label: 'PLAN', shape: 'half' },
]

export default function Shell({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const [view, setView] = useState<View>('today')
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    if (!menuOpen) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setMenuOpen(false)
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [menuOpen])
  // Set by Wardrobe's "Add to outfit" so Build can pre-fill the matching
  // slot on mount, then cleared by Build once consumed.
  const [pendingBuildItem, setPendingBuildItem] = useState<Item | null>(null)

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      // Tells the login screen not to sign straight back in via SSO (#51).
      window.history.replaceState(null, '', '/?signedout')
      onLogout()
    },
  })

  return (
    <ToastProvider>
      <div className="flex flex-col bg-(--color-bg) text-(--color-ink)">
        <header className="sticky top-0 z-10 bg-(--color-bg) border-b border-(--color-line) pt-[env(safe-area-inset-top)]">
          <div className="flex items-center gap-4 px-4 py-2">
            <span className="wy-display text-3xl mr-auto">wardy</span>
            <button
              type="button"
              aria-label="Menu"
              aria-expanded={menuOpen}
              aria-controls="app-menu"
              onClick={() => setMenuOpen(!menuOpen)}
              className="size-11 rounded-full border-2 border-(--color-ink) grid place-items-center"
            >
              {menuOpen ? (
                <Cross />
              ) : (
                <svg viewBox="0 0 20 20" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                  <path d="M3 6h14M3 14h14" />
                </svg>
              )}
            </button>
          </div>

        </header>

        {menuOpen && (
          <nav id="app-menu" aria-label="Menu" className="fixed inset-0 z-30 flex flex-col bg-(--color-bg) safe-area-overlay">
            <div className="w-full max-w-2xl mx-auto flex items-center gap-4 px-4 py-2">
              <span className="wy-display text-3xl">wardy</span>
              <span className="wy-lbl mr-auto">{me.display_name}</span>
              <button
                type="button"
                aria-label="Close menu"
                onClick={() => setMenuOpen(false)}
                className="size-11 rounded-full border-2 border-(--color-ink) grid place-items-center"
              >
                <Cross />
              </button>
            </div>
            <div className="flex-1 w-full max-w-2xl mx-auto flex flex-col gap-1.5 px-4 pb-4">
              {MENU.map((m) => (
                <button
                  key={m.view}
                  type="button"
                  onClick={() => {
                    setMenuOpen(false)
                    setView(m.view)
                  }}
                  aria-current={view === m.view ? 'page' : undefined}
                  className="flex-1 min-h-20 flex flex-col justify-between p-5 text-left"
                  style={{
                    background: m.bg,
                    color: m.fg,
                    boxShadow: view === m.view ? 'inset 8px 0 0 var(--color-accent)' : undefined,
                  }}
                >
                  <Shape name={m.shape} size={26} />
                  <span className="wy-display text-6xl">{m.label}</span>
                </button>
              ))}
              <button
                type="button"
                onClick={() => logoutMutation.mutate()}
                className="flex-1 min-h-20 flex flex-col justify-between p-5 text-left bg-(--color-ink) text-(--color-bg)"
              >
                <Cross />
                <span className="wy-display text-6xl">log out</span>
              </button>
            </div>
          </nav>
        )}

        {/* Clears the tab bar (60px + 12px); body padding covers the home indicator. */}
        <div className="flex-1 w-full max-w-2xl mx-auto pb-[72px]">
          {view === 'today' && <Today />}
          {view === 'wardrobe' && (
            <Wardrobe
              onAddToOutfit={(item) => {
                setPendingBuildItem(item)
                setView('build')
              }}
            />
          )}
          {view === 'build' && (
            <Build pendingItem={pendingBuildItem} onConsumePendingItem={() => setPendingBuildItem(null)} />
          )}
          {view === 'looks' && <Looks />}
          {view === 'plan' && <Plan />}
          {view === 'inspo' && <Inspo />}
          {view === 'history' && <History />}
          {view === 'settings' && <Settings />}
        </div>

        <nav className="fixed bottom-0 left-0 right-0 bg-(--color-bg) border-t border-(--color-line) z-10 safe-area-tabbar">
          <div className="max-w-2xl mx-auto grid grid-cols-[1fr_1fr_1.2fr_1fr_1fr]">
            {TABS.map((t) =>
              t.key === 'build' ? (
                <button
                  key={t.key}
                  type="button"
                  className="wy-tab"
                  data-on={view === t.key || undefined}
                  onClick={() => setView(t.key)}
                >
                  <span className="wy-fab text-3xl font-extrabold leading-none" aria-hidden="true">
                    +
                  </span>
                  {t.label}
                </button>
              ) : (
                <button
                  key={t.key}
                  type="button"
                  className="wy-tab"
                  data-on={view === t.key || undefined}
                  onClick={() => setView(t.key)}
                >
                  <Shape name={t.shape} size={22} />
                  {t.label}
                </button>
              ),
            )}
          </div>
        </nav>
      </div>
    </ToastProvider>
  )
}
