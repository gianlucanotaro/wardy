import { useQuery } from '@tanstack/react-query'
import { getWeather, type DayForecast } from './api'
import { useLocationSource, useWeatherCoords } from './location'

// The forecast for one plan day (#80), or undefined when there's none:
// weather down, no location, or the date beyond the 7-day forecast. Every
// card asks with the same ['weather', coords] key as Build and Settings,
// so the whole week (and those screens) share one request.
export function useDayForecast(date: string, enabled: boolean): DayForecast | undefined {
  const [source] = useLocationSource()
  const { coords, ready } = useWeatherCoords(source)
  const weather = useQuery({
    queryKey: ['weather', coords],
    queryFn: () => getWeather(coords),
    enabled: enabled && ready,
  })
  if (!enabled || !weather.data?.available) return undefined
  return weather.data.days?.find((d) => d.date === date)
}

export function forecastLabel(f: DayForecast): string {
  return `high ${Math.round(f.temp_high_c)}°, low ${Math.round(f.temp_low_c)}°, ${f.precip_probability}% chance of rain`
}
