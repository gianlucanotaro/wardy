export type Me = { email: string; display_name: string }

export type Item = {
  id: number
  name: string
  photo_path: string
  thumb_path: string
  created_at: string
  [key: string]: unknown
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
