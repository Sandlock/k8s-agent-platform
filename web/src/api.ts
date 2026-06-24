const BASE = ''  // same origin via vite proxy in dev; empty string = relative in prod

let token = localStorage.getItem('sandlock_token') ?? ''
let unauthorizedHandler: (() => void) | null = null

export function setToken(t: string) {
  token = t
  localStorage.setItem('sandlock_token', t)
}

export function clearToken() {
  token = ''
  localStorage.removeItem('sandlock_token')
  localStorage.removeItem('sandlock_admin')
}

export function getToken() { return token }

export function setStoredAdmin(v: boolean) {
  localStorage.setItem('sandlock_admin', String(v))
}

export function getStoredAdmin() {
  return localStorage.getItem('sandlock_admin') === 'true'
}

export function onUnauthorized(handler: () => void) {
  unauthorizedHandler = handler
}

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
  if (res.status === 401) {
    clearToken()
    unauthorizedHandler?.()
    throw new Error('Session expired — please log in again')
  }
  if (!res.ok) throw new Error(await res.text())
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  login: (username: string, password: string) =>
    req<{ token: string; isAdmin: boolean; mustChangePassword: boolean }>('POST', '/v1/auth/login', { username, password }),
  logout: () => req<void>('POST', '/v1/auth/logout'),
  changePassword: (currentPassword: string, newPassword: string) =>
    req<void>('PUT', '/v1/auth/password', { currentPassword, newPassword }),

  listSandboxes: () => req<Sandbox[]>('GET', '/v1/sandboxes'),
  createSandbox: (body: CreateSandboxBody) => req<{ sandboxId: string; attachUrl: string }>('POST', '/v1/sandboxes', body),
  stopSandbox: (id: string) => req<void>('DELETE', `/v1/sandboxes/${id}`),
  storeKey: (anthropicKey: string) => req<{ hint: string }>('POST', '/v1/keys', { anthropicKey }),

  listSkills: () => req<Skill[]>('GET', '/v1/skills'),
  putSkill: (name: string, content: string) => req<{ id: string; name: string }>('PUT', `/v1/skills/${name}`, { content }),
  deleteSkill: (name: string) => req<void>('DELETE', `/v1/skills/${name}`),

  listMCPServers: () => req<MCPServer[]>('GET', '/v1/mcp'),
  putMCPServer: (name: string, body: UpsertMCPServerBody) => req<{ id: string; name: string }>('PUT', `/v1/mcp/${name}`, body),
  deleteMCPServer: (name: string) => req<void>('DELETE', `/v1/mcp/${name}`),

  listUsers: () => req<User[]>('GET', '/v1/users'),
  createUser: (username: string, password: string, isAdmin: boolean) =>
    req<{ userId: string }>('POST', '/v1/users', { username, password, isAdmin }),
  deleteUser: (id: string) => req<void>('DELETE', `/v1/users/${id}`),
}

export interface User {
  id: string
  username: string
  isAdmin: boolean
  createdAt: string
}

export interface Sandbox {
  id: string
  harness: string
  status: string
  providerRef: string
  createdAt: string
  repoUrl?: string
  branch?: string
}

export interface Skill {
  id: string
  name: string
  content: string
  createdAt: string
}

export interface MCPServer {
  id: string
  name: string
  type: 'http' | 'sse' | 'stdio'
  url?: string
  command?: string
  args?: string[]
  env?: Record<string, string>
  headers?: Record<string, string>
  secretEnvKeys?: string[]
  secretHeaderKeys?: string[]
  displayName?: string
  createdAt: string
  updatedAt: string
}

export interface UpsertMCPServerBody {
  type: 'http' | 'sse' | 'stdio'
  url?: string
  command?: string
  args?: string[]
  env?: Record<string, string>
  secretEnv?: Record<string, string>
  headers?: Record<string, string>
  secretHeaders?: Record<string, string>
  displayName?: string
}

export interface CreateSandboxBody {
  harness?: string
  anthropicKey?: string
  useStoredKey?: boolean
  repoUrl?: string
  branch?: string
}

export function terminalWsUrl(id: string) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const t = token ? `?token=${encodeURIComponent(token)}` : ''
  return `${proto}://${location.host}/v1/sandboxes/${id}/terminal${t}`
}
