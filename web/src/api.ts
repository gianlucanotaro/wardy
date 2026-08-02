export type Me = { email: string; display_name: string }

export type TagsPayload = {
  category: string
  subcategory: string
  color_primary: string
  color_secondary: string
  pattern: string
  material: string
  fit: string
  formality: number
  warmth: number
  seasons: string[]
  occasions: string[]
}

export type Item = TagsPayload & {
  id: number
  name: string
  photo_path: string
  thumb_path: string
  user_edited_at: string | null
  created_at: string
  // Whether a tag_item AI job is still pending for this item. Poll while
  // true; stop once it's false (tagged, permanently failed, or AI was
  // never attempted all read the same from here — the tag editor already
  // handles an untagged item gracefully either way).
  tagging_pending: boolean
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

export function listItems(): Promise<Item[]> {
  return request('/api/items')
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

export function getWeather(): Promise<Weather> {
  return request('/api/weather')
}
