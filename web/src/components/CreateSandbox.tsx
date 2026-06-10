import { useState, FormEvent } from 'react'
import { api } from '../api'

interface Props {
  onCreated: (id: string) => void
  onCancel: () => void
}

export default function CreateSandbox({ onCreated, onCancel }: Props) {
  const [apiKey, setApiKey] = useState('')
  const [repoUrl, setRepoUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const { sandboxId } = await api.createSandbox({
        harness: 'claude-code',
        anthropicKey: apiKey || undefined,
        repoUrl: repoUrl || undefined,
      })
      onCreated(sandboxId)
    } catch (err: any) {
      setError(err.message ?? 'Failed to create sandbox')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="card" style={{ marginBottom: 24 }}>
      <h2 style={{ fontSize: 16, marginBottom: 16 }}>New sandbox</h2>
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <input
          placeholder="Anthropic API key (optional — uses stored key if blank)"
          value={apiKey}
          onChange={e => setApiKey(e.target.value)}
        />
        <input
          placeholder="Repo URL (optional)"
          value={repoUrl}
          onChange={e => setRepoUrl(e.target.value)}
        />
        {error && <p style={{ color: '#f85149', fontSize: 13 }}>{error}</p>}
        <div style={{ display: 'flex', gap: 8 }}>
          <button type="submit" className="primary" disabled={loading}>
            {loading ? 'Creating…' : 'Create'}
          </button>
          <button type="button" onClick={onCancel}>Cancel</button>
        </div>
      </form>
    </div>
  )
}
