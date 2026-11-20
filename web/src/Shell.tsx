import { useState } from 'react'
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
import Shape from './Shape'
import type { ShapeName } from './shapes'
import { ToastProvider } from './Toast'

// The persistent chrome: a sticky header (brand + the utility screens the
// design doesn't put in its 5-tab bar) and a sticky bottom tab bar, both
// restyled to the Modernist design system. Owns the top-level View switch —
// tab content below never renders its own "Close" affordance, the tab bar
// and header links are the only navigation.
type View = 'today' | 'wardrobe' | 'build' | 'looks' | 'plan' | 'inspo' | 'history' | 'settings'

const TABS: { key: View; label: string; shape: ShapeName }[] = [
  { key: 'today', label: 'TODAY', shape: 'circle' },
  { key: 'wardrobe', label: 'WARDROBE', shape: 'square' },
  { key: 'build', label: 'BUILD', shape: 'circle' },
  { key: 'looks', label: 'LOOKS', shape: 'triangle' },
  { key: 'plan', label: 'PLAN', shape: 'half' },
]

export default function Shell({ me, onLogout }: { me: Me; onLogout: () => void }) {
  const [view, setView] = useState<View>('today')
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
        <header className="sticky top-0 z-10 border-b-2 border-(--color-line-strong) bg-(--color-bg) pt-[env(safe-area-inset-top)]">
          <div className="flex items-center gap-4 px-4 py-3">
            <span className="wy-display text-2xl mr-auto">wardy</span>
            <span className="hidden sm:inline wy-lbl">{me.display_name}</span>
            <button
              type="button"
              onClick={() => setView('inspo')}
              className="wy-lbl"
              style={view === 'inspo' ? { color: 'var(--color-accent-700)' } : undefined}
            >
              INSPO
            </button>
            <button type="button" onClick={() => setView('history')} className="wy-lbl">
              HISTORY
            </button>
            <button type="button" onClick={() => setView('settings')} className="wy-lbl">
              SETTINGS
            </button>
            <button type="button" onClick={() => logoutMutation.mutate()} className="wy-lbl">
              LOG OUT
            </button>
          </div>
        </header>

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

        <nav className="fixed bottom-0 left-0 right-0 bg-(--color-bg) border-t-2 border-(--color-line-strong) z-10 safe-area-tabbar">
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
