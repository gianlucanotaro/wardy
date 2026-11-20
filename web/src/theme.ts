// Light or dark: the saved choice, else the phone's setting. index.html runs
// the same rule before first paint; this keeps it in step afterwards.
export type ThemePref = 'auto' | 'light' | 'dark'

const KEY = 'wardy-theme'
const BG = { light: '#f1ece2', dark: '#121110' }

export function getThemePref(): ThemePref {
  const saved = localStorage.getItem(KEY)
  return saved === 'light' || saved === 'dark' ? saved : 'auto'
}

export function applyTheme(pref: ThemePref = getThemePref()) {
  const dark = pref === 'dark' || (pref === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches)
  const theme = dark ? 'dark' : 'light'
  document.documentElement.dataset.theme = theme
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', BG[theme])
}

export function setThemePref(pref: ThemePref) {
  if (pref === 'auto') localStorage.removeItem(KEY)
  else localStorage.setItem(KEY, pref)
  applyTheme(pref)
}

// "Auto" follows the phone when it switches at sunset.
export function watchSystemTheme() {
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => applyTheme())
}
