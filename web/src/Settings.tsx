import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getSettings, getWeather, updateSettings } from './api'

const CONDITION_LABELS: Record<string, string> = {
  clear: 'Clear',
  cloudy: 'Cloudy',
  fog: 'Foggy',
  drizzle: 'Drizzle',
  rain: 'Rain',
  snow: 'Snow',
  showers: 'Showers',
  thunderstorm: 'Thunderstorm',
  unknown: 'Unknown',
}

// iOS has no beforeinstallprompt event, so there's no programmatic install
// button to offer — this is a static instruction instead, shown only when
// it's actually actionable (on iOS, and not already installed).
function isIOSInstallable(): boolean {
  const isIOS =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  const isStandalone =
    window.matchMedia('(display-mode: standalone)').matches ||
    (window.navigator as unknown as { standalone?: boolean }).standalone === true
  return isIOS && !isStandalone
}

export default function Settings() {
  const queryClient = useQueryClient()
  const [showIOSInstallHint] = useState(isIOSInstallable)
  const settings = useQuery({ queryKey: ['settings'], queryFn: getSettings })
  const hasLocation = settings.data?.home_lat != null && settings.data?.home_lon != null

  const weather = useQuery({
    queryKey: ['weather'],
    queryFn: getWeather,
    enabled: hasLocation,
  })

  const [lat, setLat] = useState('')
  const [lon, setLon] = useState('')
  const [geoError, setGeoError] = useState<string | null>(null)

  // Pre-fill from the saved location once it loads, without clobbering
  // whatever the user's already typed.
  useEffect(() => {
    if (hasLocation && lat === '' && lon === '') {
      setLat(String(settings.data!.home_lat))
      setLon(String(settings.data!.home_lon))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settings.data])

  const save = useMutation({
    mutationFn: () => updateSettings(parseFloat(lat), parseFloat(lon)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['weather'] })
    },
  })

  // Deliberately not requested automatically — the user taps this, we
  // never demand browser geolocation (docs/ai.md).
  function useMyLocation() {
    setGeoError(null)
    if (!navigator.geolocation) {
      setGeoError('Geolocation is not available in this browser.')
      return
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setLat(pos.coords.latitude.toFixed(4))
        setLon(pos.coords.longitude.toFixed(4))
      },
      () => setGeoError('Could not get your location — enter it manually below.'),
    )
  }

  return (
    <div className="flex flex-col">
      <div className="px-4 py-3 border-b-2 border-(--color-line-strong)">
        <span className="font-extrabold text-3xl tracking-tight">SETTINGS</span>
      </div>

      {showIOSInstallHint && (
        <section className="px-4 py-3 border-b-2 border-(--color-line-strong)">
          <div className="wy-lbl mb-1">ADD TO HOME SCREEN</div>
          <p className="text-sm">
            Tap the Share icon in Safari, then "Add to Home Screen" — Wardy works better full-screen, without the
            browser bar.
          </p>
        </section>
      )}

      <section className="px-4 py-3 border-b-2 border-(--color-line-strong) flex flex-col gap-2">
        <div className="wy-lbl">HOME LOCATION</div>
        <p className="text-sm text-(--color-neutral-700)">Used to fetch the weather for outfit suggestions.</p>

        <form
          className="flex flex-col gap-2 mt-1"
          onSubmit={(e) => {
            e.preventDefault()
            save.mutate()
          }}
        >
          <label className="flex flex-col gap-1 text-sm" htmlFor="lat">
            <span className="wy-lbl">LATITUDE</span>
            <input
              id="lat"
              type="number"
              step="any"
              required
              className="border border-(--color-line) bg-(--color-surface) px-3 py-2 focus:outline-2 focus:outline-(--color-accent)"
              value={lat}
              onChange={(e) => setLat(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm" htmlFor="lon">
            <span className="wy-lbl">LONGITUDE</span>
            <input
              id="lon"
              type="number"
              step="any"
              required
              className="border border-(--color-line) bg-(--color-surface) px-3 py-2 focus:outline-2 focus:outline-(--color-accent)"
              value={lon}
              onChange={(e) => setLon(e.target.value)}
            />
          </label>

          <button type="button" onClick={useMyLocation} className="wy-lbl text-left mt-1">
            USE MY CURRENT LOCATION
          </button>
          {geoError && <p className="text-sm text-(--color-accent-700)">{geoError}</p>}

          <button type="submit" disabled={save.isPending} data-primary="1" className="wy-act mt-1 text-center">
            {save.isPending ? 'SAVING…' : 'SAVE LOCATION'}
          </button>
          {save.isError && <p className="text-sm text-(--color-accent-700)">Couldn't save — try again.</p>}
        </form>
      </section>

      {hasLocation && (
        <section className="px-4 py-3 border-b-2 border-(--color-line-strong) flex flex-col gap-1">
          <div className="wy-lbl">CURRENT WEATHER</div>
          {weather.isLoading && <p className="text-sm text-(--color-neutral-700)">Loading…</p>}
          {weather.data?.available === false && (
            <p className="text-sm text-(--color-neutral-700)">
              Weather unavailable right now — suggestions will work without it.
            </p>
          )}
          {weather.data?.available === true && (
            <p className="text-sm">
              {CONDITION_LABELS[weather.data.condition] ?? weather.data.condition},{' '}
              {Math.round(weather.data.temp_low_c)}–{Math.round(weather.data.temp_high_c)}°C,{' '}
              {weather.data.precip_probability}% chance of rain, wind {Math.round(weather.data.wind_speed_kph)} km/h
            </p>
          )}
        </section>
      )}
    </div>
  )
}
