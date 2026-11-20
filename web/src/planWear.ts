import { useQuery } from '@tanstack/react-query'
import { listLooks, type PlanDay, type Wear } from './api'

// What a past plan day says about what was worn (#78). With several wears
// on one day, the last one logged wins.

export function lastWear(day: PlanDay): Wear | undefined {
  return day.wears.at(-1)
}

export function asPlanned(day: PlanDay, wear: Wear | undefined): boolean {
  return wear != null && day.look != null && wear.look_id === day.look.id
}

// The name a wear goes by: its look's name, or "N pieces" for an outfit
// put together on the day. Shares the ['looks'] cache with the rest of the
// app, so it costs no extra request on the Plan page.
export function useWearName(wear: Wear | undefined): string | undefined {
  const looks = useQuery({ queryKey: ['looks'], queryFn: listLooks, enabled: wear?.look_id != null })
  if (!wear) return undefined
  const look = wear.look_id == null ? undefined : looks.data?.find((l) => l.id === wear.look_id)
  if (look) return look.name
  return `${wear.items.length} piece${wear.items.length === 1 ? '' : 's'}`
}

// The spoken version, for the card's aria-label.
export function useWornLabel(day: PlanDay): string {
  const wear = lastWear(day)
  const name = useWearName(wear)
  if (asPlanned(day, wear)) return `wore ${day.look!.name}, as planned`
  if (wear && day.look) return `planned ${day.look.name}, wore ${name}`
  if (wear) return `wore ${name}`
  if (day.look) return `planned ${day.look.name}, nothing logged`
  return 'nothing planned'
}
