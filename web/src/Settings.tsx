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

export default function Settings({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
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
    <div className="flex flex-col gap-4 p-4 max-w-sm mx-auto w-full">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">Settings</h2>
        <button type="button" onClick={onClose} className="text-sm underline">
          Close
        </button>
      </div>

      <section className="flex flex-col gap-2">
        <h3 className="text-sm font-medium text-gray-700">Home location</h3>
        <p className="text-xs text-gray-500">Used to fetch the weather for outfit suggestions.</p>

        <form
          className="flex flex-col gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            save.mutate()
          }}
        >
          <label className="flex flex-col gap-1 text-sm" htmlFor="lat">
            Latitude
            <input
              id="lat"
              type="number"
              step="any"
              required
              className="border rounded px-2 py-2"
              value={lat}
              onChange={(e) => setLat(e.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm" htmlFor="lon">
            Longitude
            <input
              id="lon"
              type="number"
              step="any"
              required
              className="border rounded px-2 py-2"
              value={lon}
              onChange={(e) => setLon(e.target.value)}
            />
          </label>

          <button type="button" onClick={useMyLocation} className="text-sm underline text-left">
            Use my current location
          </button>
          {geoError && <p className="text-sm text-red-600">{geoError}</p>}

          <button
            type="submit"
            disabled={save.isPending}
            className="bg-black text-white rounded px-3 py-2 text-sm disabled:opacity-50"
          >
            {save.isPending ? 'Saving…' : 'Save location'}
          </button>
          {save.isError && <p className="text-sm text-red-600">Couldn't save — try again.</p>}
        </form>
      </section>

      {hasLocation && (
        <section className="flex flex-col gap-1">
          <h3 className="text-sm font-medium text-gray-700">Current weather</h3>
          {weather.isLoading && <p className="text-sm text-gray-500">Loading…</p>}
          {weather.data?.available === false && (
            <p className="text-sm text-gray-500">
              Weather unavailable right now — suggestions will work without it.
            </p>
          )}
          {weather.data?.available === true && (
            <p className="text-sm text-gray-700">
              {CONDITION_LABELS[weather.data.condition] ?? weather.data.condition},{' '}
              {Math.round(weather.data.temp_low_c)}–{Math.round(weather.data.temp_high_c)}°C,{' '}
              {weather.data.precip_probability}% chance of rain, wind{' '}
              {Math.round(weather.data.wind_speed_kph)} km/h
            </p>
          )}
        </section>
      )}
    </div>
  )
}
