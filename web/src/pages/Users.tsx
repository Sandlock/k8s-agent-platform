import { useState, useEffect, FormEvent } from 'react'
import { api, User } from '../api'

export default function Users() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newIsAdmin, setNewIsAdmin] = useState(false)
  const [createError, setCreateError] = useState('')
  const [creating, setCreating] = useState(false)

  async function refresh() {
    try {
      setUsers(await api.listUsers())
    } catch {}
    setLoading(false)
  }

  useEffect(() => { refresh() }, [])

  async function handleCreate(e: FormEvent) {
    e.preventDefault()
    setCreating(true)
    setCreateError('')
    try {
      await api.createUser(newUsername, newPassword, newIsAdmin)
      setNewUsername('')
      setNewPassword('')
      setNewIsAdmin(false)
      setShowCreate(false)
      refresh()
    } catch (err: any) {
      setCreateError(err.message ?? 'Failed to create user')
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(id: string, username: string) {
    if (!confirm(`Delete user "${username}"?`)) return
    await api.deleteUser(id)
    refresh()
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <h2 style={{ fontSize: 18 }}>Users</h2>
        <button className="primary" onClick={() => setShowCreate(v => !v)}>
          {showCreate ? 'Cancel' : 'New user'}
        </button>
      </div>

      {showCreate && (
        <div className="card" style={{ marginBottom: 20 }}>
          <form onSubmit={handleCreate} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <h3 style={{ fontSize: 15, marginBottom: 4 }}>Create user</h3>
            <input
              placeholder="Username"
              value={newUsername}
              onChange={e => setNewUsername(e.target.value)}
              required
            />
            <input
              type="password"
              placeholder="Temporary password"
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              required
            />
            <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 14, cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={newIsAdmin}
                onChange={e => setNewIsAdmin(e.target.checked)}
                style={{ width: 'auto' }}
              />
              Admin
            </label>
            {createError && <p style={{ color: '#f85149', fontSize: 13 }}>{createError}</p>}
            <div>
              <button type="submit" className="primary" disabled={creating}>
                {creating ? 'Creating…' : 'Create'}
              </button>
            </div>
          </form>
        </div>
      )}

      {loading ? (
        <p style={{ color: '#8b949e' }}>Loading…</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {users.map(u => (
            <div key={u.id} className="card" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <span style={{ fontWeight: 500 }}>{u.username}</span>
                {u.isAdmin && (
                  <span style={{
                    marginLeft: 8, fontSize: 11, padding: '2px 6px',
                    background: '#1f3a1f', border: '1px solid #238636',
                    borderRadius: 4, color: '#3fb950'
                  }}>admin</span>
                )}
                <div style={{ fontSize: 12, color: '#8b949e', marginTop: 2 }}>
                  {new Date(u.createdAt).toLocaleDateString()}
                </div>
              </div>
              <button className="danger" onClick={() => handleDelete(u.id, u.username)}>Delete</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
