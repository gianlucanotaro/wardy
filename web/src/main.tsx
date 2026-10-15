import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import './index.css'
import App from './App.tsx'
import { registerSW } from 'virtual:pwa-register'

const queryClient = new QueryClient()

// iOS Safari ignores user-scalable=no and touch-action for pinch on some
// versions; cancelling its proprietary gesture event is what reliably stops it.
document.addEventListener('gesturestart', (e) => e.preventDefault())

// Get a new deploy onto an open app without the user having to force-quit
// it. The service worker only looks for a new version on a full page load,
// and iOS resumes a home-screen app rather than reloading it, so also check
// whenever the app comes back to the foreground. registerType 'autoUpdate'
// (vite.config.ts) then reloads the page once the new version takes over.
registerSW({
  immediate: true,
  onRegisteredSW(_url, registration) {
    if (!registration) return
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible') registration.update()
    })
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
