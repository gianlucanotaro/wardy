import { type Coords } from './location'

export type Me = { email: string; display_name: string }

export type TagsPayload = {
  category: string
  subcategory: string
  color_primary: string
  color_secondary: string
  pattern: string
  material: string
  fit: string
  // Ranges on the 1-5 scales, min <= max: a shirt can be formality 2-4
  // and warmth 1-5 (worn layered in winter).
  formality_min: number
  formality_max: number
  warmth_min: number
  warmth_max: number
  seasons: string[]
  occasions: string[]
  // Free-form, user- or AI-authored — not part of the closed taxonomy
  // above, so it's edited separately (see CustomTagsField in TagEditor).
  custom_tags: string[]
}

export type Item = TagsPayload & {
  id: number
  name: string
  photo_path: string
  thumb_path: string
  // Set once a tag_item job (AI or, going forward, any future automated
  // tagger) has written its result — null if the item was only ever
  // manually tagged. Paired with user_edited_at to tell "AI's untouched
  // suggestion" apart from "someone already reviewed this" (see
  // TagEditor's markAiSuggested).
  ai_tagged_at: string | null
  user_edited_at: string | null
  created_at: string
  // Whether a tag_item AI job is still pending for this item. Poll while
  // true; stop once it's false (tagged, permanently failed, or AI was
  // never attempted all read the same from here — the tag editor already
  // handles an untagged item gracefully either way).
  tagging_pending: boolean
  // How many times, and when most recently, this item has been logged as
  // worn — computed by the same /api/items call, not a separate fetch.
  wear_count: number
  last_worn_on: string | null
  archived: boolean
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  if (!res.ok) {
    throw new Error(`${init?.method ?? 'GET'} ${path}: ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export function getMe(): Promise<Me> {
  return request('/api/me')
}

export function login(email: string, password: string): Promise<Me> {
  return request('/api/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export function logout(): Promise<void> {
  return request('/api/logout', { method: 'POST' })
}

export function listItems(opts?: { archived?: boolean }): Promise<Item[]> {
  return request(opts?.archived ? '/api/items?archived=true' : '/api/items')
}

export async function uploadItem(file: File): Promise<Item> {
  const form = new FormData()
  form.append('photo', file)
  const res = await fetch('/api/items', { method: 'POST', body: form })
  if (!res.ok) {
    throw new Error(`upload failed: ${res.status}`)
  }
  return res.json()
}

export function deleteItem(id: number): Promise<void> {
  return request(`/api/items/${id}`, { method: 'DELETE' })
}

export function archiveItem(id: number, archived: boolean): Promise<Item> {
  return request(`/api/items/${id}/archive`, { method: 'PATCH', body: JSON.stringify({ archived }) })
}

export function updateItemTags(id: number, tags: TagsPayload): Promise<Item> {
  return request(`/api/items/${id}/tags`, {
    method: 'PATCH',
    body: JSON.stringify(tags),
  })
}

export type Settings = { home_lat: number | null; home_lon: number | null }

export function getSettings(): Promise<Settings> {
  return request('/api/settings')
}

export function updateSettings(homeLat: number, homeLon: number): Promise<Settings> {
  return request('/api/settings', {
    method: 'PATCH',
    body: JSON.stringify({ home_lat: homeLat, home_lon: homeLon }),
  })
}

export type Weather =
  | { available: true; temp_high_c: number; temp_low_c: number; precip_probability: number; wind_speed_kph: number; condition: string }
  | { available: false; reason: string }

// coords is this device's current position (location.ts); without it the
// server uses the home location.
function withCoords(params: URLSearchParams, coords: Coords | null) {
  if (coords) {
    params.set('lat', String(coords.lat))
    params.set('lon', String(coords.lon))
  }
}

export function getWeather(coords: Coords | null): Promise<Weather> {
  const params = new URLSearchParams()
  withCoords(params, coords)
  return request(`/api/weather?${params.toString()}`)
}

export type SuggestOutfit = { items: Item[]; note: string }

export type SuggestSource = 'ai' | 'rules_fallback'

export type SuggestResponse = {
  outfits: SuggestOutfit[]
  source?: SuggestSource
  no_match_reason?: string
  weather_available: boolean
}

// mood is optional prompt context for the AI rationale only — it never
// changes which items are eligible (the "context not filter" design;
// see internal/http/suggest.go).
export function getSuggestions(occasion: string, mood: string | undefined, coords: Coords | null): Promise<SuggestResponse> {
  const params = new URLSearchParams({ occasion })
  if (mood) params.set('mood', mood)
  withCoords(params, coords)
  return request(`/api/suggest?${params.toString()}`)
}

export type Look = {
  id: number
  name: string
  notes: string
  cover_item_id: number | null
  items: Item[]
  created_at: string
}

export function listLooks(): Promise<Look[]> {
  return request('/api/looks')
}

export function getLook(id: number): Promise<Look> {
  return request(`/api/looks/${id}`)
}

export function createLook(input: { name: string; notes?: string; item_ids: number[] }): Promise<Look> {
  return request('/api/looks', { method: 'POST', body: JSON.stringify(input) })
}

export function updateLook(id: number, input: { name?: string; item_ids?: number[] }): Promise<Look> {
  return request(`/api/looks/${id}`, { method: 'PATCH', body: JSON.stringify(input) })
}

export function deleteLook(id: number): Promise<void> {
  return request(`/api/looks/${id}`, { method: 'DELETE' })
}

export type Wear = {
  id: number
  look_id: number | null
  worn_on: string
  occasion: string
  weather_summary: string
  notes: string
  items: Item[]
  created_at: string
}

export type ItemLastWorn = Item & { last_worn_on: string | null }

export type History = {
  wears: Wear[]
  items: ItemLastWorn[]
}

export function listWears(): Promise<Wear[]> {
  return request('/api/wears')
}

export function getWear(id: number): Promise<Wear> {
  return request(`/api/wears/${id}`)
}

export function logWear(input: { look_id?: number; item_ids?: number[]; worn_on: string; occasion?: string; notes?: string }): Promise<Wear> {
  return request('/api/wears', { method: 'POST', body: JSON.stringify(input) })
}

export function deleteWear(id: number): Promise<void> {
  return request(`/api/wears/${id}`, { method: 'DELETE' })
}

export function getHistory(): Promise<History> {
  return request('/api/history')
}

export type PlanDay = { date: string; look: Look | null }

export function getPlan(start: string, end: string): Promise<PlanDay[]> {
  return request(`/api/plan?start=${start}&end=${end}`)
}

export function setPlanDay(date: string, lookId: number | null): Promise<PlanDay> {
  return request(`/api/plan/${date}`, { method: 'PUT', body: JSON.stringify({ look_id: lookId }) })
}

// todayISODate is the local calendar date (not UTC — worn_on means "today"
// to the person tapping the button, and UTC can disagree near midnight).
export function todayISODate(): string {
  const d = new Date()
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${mm}-${dd}`
}
