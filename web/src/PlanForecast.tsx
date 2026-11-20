import type { DayForecast } from './api'

// High/low and rain chance at the right end of a day card's title row
// (#80, mock-up frame 4). Nothing renders without a forecast, and the row
// keeps its height either way.
export default function PlanForecast({ forecast }: { forecast: DayForecast }) {
  return (
    <span
      aria-hidden="true"
      className="shrink-0 inline-flex items-center gap-[5px] font-extrabold text-[11px] tracking-[0.02em] tabular-nums"
    >
      {Math.round(forecast.temp_high_c)}°/{Math.round(forecast.temp_low_c)}°
      <span className="inline-flex items-center gap-0.5 font-semibold text-(--color-neutral-700)">
        <svg width="9" height="9" viewBox="0 0 24 24" aria-hidden="true">
          <path fill="currentColor" d="M12 3s-6 7-6 11a6 6 0 0 0 12 0c0-4-6-11-6-11z" />
        </svg>
        {forecast.precip_probability}%
      </span>
    </span>
  )
}
