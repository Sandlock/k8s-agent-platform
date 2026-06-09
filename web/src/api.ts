const BASE = ''  // same origin via vite proxy in dev; empty string = relative in prod

let token = localStorage.getItem('sandlock_token') ?? ''

export function setToken(t: string) {
  token = t
  localStorage.setItem('sandlock_token', t)
}

export function clearToken() {
  token = ''
  localStorage.removeItem('sandlock_token')
}

export function getToken() { return token }

function headers(extra: Record<string, string> = {}) {
  const h: Record<string, string> = { 'Content-Type': 'application/json', ...extra }
  if (token) h['Authorization'] = `Bearer ${token}`
  return h
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: headers(),
    body: body != null ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) throw new Error(await res.text())
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  login: (username: string, password: string) =>
    req<{ token: string }>('POST', '/v1/auth/login', { username, password }),
  logout: () => req<void>('POST', '/v1/auth/logout'),
  listSandboxes: () => req<Sandbox[]>('GET', '/v1/sandboxes'),
  createSandbox: (body: CreateSandboxBody) => req<{ sandboxId: string; attachUrl: string }>('POST', '/v1/sandboxes', body),
  stopSandbox: (id: string) => req<void>('DELETE', `/v1/sandboxes/${id}`),
  storeKey: (anthropicKey: string) => req<{ hint: string }>('POST', '/v1/keys', { anthropicKey }),
}

export interface Sandbox {
  id: string
  harness: string
  status: string
  providerRef: string
  createdAt: string
}

export interface CreateSandboxBody {
  harness?: string
  anthropicKey?: string
  useStoredKey?: boolean
  repoUrl?: string
}

export function terminalWsUrl(id: string) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/v1/sandboxes/${id}/terminal`
}
