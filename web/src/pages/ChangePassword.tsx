import { useState, FormEvent } from 'react'
import { api } from '../api'

interface Props {
  onChanged: () => void
  onLogout: () => void
}

export default function ChangePassword({ onChanged, onLogout }: Props) {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (next !== confirm) { setError('Passwords do not match'); return }
    if (next.length < 8) { setError('Password must be at least 8 characters'); return }
    setLoading(true)
    setError('')
    try {
      await api.changePassword(current, next)
      onChanged()
    } catch (err: any) {
      setError(err.message ?? 'Failed to change password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
      <div className="card" style={{ width: 400 }}>
        <h1 style={{ fontSize: 22, marginBottom: 8 }}>Change your password</h1>
        <p style={{ fontSize: 13, color: '#8b949e', marginBottom: 24 }}>
          You're using a temporary password. Please set a new one before continuing.
        </p>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <input
            type="password"
            placeholder="Current password"
            value={current}
            onChange={e => setCurrent(e.target.value)}
            required
          />
          <input
            type="password"
            placeholder="New password"
            value={next}
            onChange={e => setNext(e.target.value)}
            required
          />
          <input
            type="password"
            placeholder="Confirm new password"
            value={confirm}
            onChange={e => setConfirm(e.target.value)}
            required
          />
          {error && <p style={{ color: '#f85149', fontSize: 13 }}>{error}</p>}
          <div style={{ display: 'flex', gap: 8 }}>
            <button type="submit" className="primary" disabled={loading} style={{ flex: 1 }}>
              {loading ? 'Saving…' : 'Set password'}
            </button>
            <button type="button" onClick={onLogout}>Log out</button>
          </div>
        </form>
      </div>
    </div>
  )
}
