import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Wardy',
        short_name: 'Wardy',
        description: 'A self-hosted digital wardrobe and lookbook.',
        display: 'standalone',
        // The paper colour of the light theme. A manifest can't follow dark
        // mode, so a phone set to dark sees this on the splash screen only;
        // index.html and theme.ts set the live status bar colour.
        background_color: '#f1ece2',
        theme_color: '#f1ece2',
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // Navigations to /api must reach the server, not the cached app
        // shell — the SSO start and callback (#51) are full-page redirects.
        navigateFallbackDenylist: [/^\/api\//],
        // Cache the app shell only — never API responses. This is
        // authenticated, per-user data; NetworkOnly (not NetworkFirst)
        // is deliberate so nothing from /api ever lands in a cache that
        // could serve one session's wardrobe to another (docs/frontend.md).
        runtimeCaching: [
          // The one exception: the cut-out model and its runtime (models.go,
          // about 59 MB). They are the same for everyone and are not user
          // data, and a phone should download them once, not on every photo.
          {
            urlPattern: ({ url }) => url.pathname.startsWith('/api/models/'),
            handler: 'CacheFirst',
            options: {
              cacheName: 'cutout-model',
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 8 },
            },
          },
          {
            urlPattern: ({ url }) => url.pathname.startsWith('/api/'),
            handler: 'NetworkOnly',
          },
        ],
      },
    }),
  ],
  server: {
    port: 5173,
    proxy: {
      '/healthz': 'http://localhost:8080',
      '/api': 'http://localhost:8080',
    },
  },
})
