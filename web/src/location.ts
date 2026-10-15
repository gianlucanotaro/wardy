import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'

// Where the weather comes from: this device's current position (the
// default — travelling, or no home location typed in) or the saved home
// location. Per device, so it lives in localStorage rather than the user's
// settings; a phone and a laptop can differ.
export type Coords = { lat: number; lon: number }
export type LocationSource = 'current' | 'home'

const SOURCE_KEY = 'wy-weather-location'

function readSource(): LocationSource {
  try {
    return localStorage.getItem(SOURCE_KEY) === 'home' ? 'home' : 'current'
  } catch {
    return 'current'
  }
}

export function useLocationSource(): [LocationSource, (source: LocationSource) => void] {
  const [source, setSource] = useState<LocationSource>(readSource)
  function set(next: LocationSource) {
    try {
      localStorage.setItem(SOURCE_KEY, next)
    } catch {
      // Private mode or blocked storage: the choice just lasts this visit.
    }
    setSource(next)
  }
  return [source, set]
}

// Resolves null when the browser can't or won't share a position (denied,
// unsupported, timed out) — the server then falls back to the home
// location. Rounded to ~1 km: plenty for a forecast, and no more precise
// than it needs to be on the way to the server.
function getPosition(): Promise<Coords | null> {
  return new Promise((resolve) => {
    if (!navigator.geolocation) return resolve(null)
    navigator.geolocation.getCurrentPosition(
      (pos) =>
        resolve({
          lat: Math.round(pos.coords.latitude * 100) / 100,
          lon: Math.round(pos.coords.longitude * 100) / 100,
        }),
      () => resolve(null),
      { enableHighAccuracy: false, maximumAge: 30 * 60_000, timeout: 10_000 },
    )
  })
}

// The coordinates to send with weather and suggestion requests, or null to
// let the server use the home location. `ready` is false while the browser
// is still deciding (or showing its permission prompt), so callers can
// hold their requests instead of fetching home weather first and then
// again for here.
export function useWeatherCoords(source: LocationSource): { coords: Coords | null; ready: boolean } {
  const position = useQuery({
    queryKey: ['position'],
    queryFn: getPosition,
    enabled: source === 'current',
    staleTime: 30 * 60_000,
    retry: false,
  })
  if (source === 'home') return { coords: null, ready: true }
  return { coords: position.data ?? null, ready: !position.isPending }
}
