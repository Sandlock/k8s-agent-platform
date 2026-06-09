import { useState, useEffect } from 'react'
import { api, Sandbox } from '../api'
import Terminal from '../components/Terminal'
import CreateSandbox from '../components/CreateSandbox'

interface Props { onLogout: () => void }

export default function Dashboard({ onLogout }: Props) {
  const [sandboxes, setSandboxes] = useState<Sandbox[]>([])
  const [activeSandbox, setActiveSandbox] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [loading, setLoading] = useState(true)

  async function refresh() {
    try {
      const list = await api.listSandboxes()
      setSandboxes(list ?? [])
    } catch {}
    setLoading(false)
  }

  useEffect(() => {
    refresh()
    const t = setInterval(refresh, 5000)
    return () => clearInterval(t)
  }, [])

  async function stop(id: string) {
    await api.stopSandbox(id)
    if (activeSandbox === id) setActiveSandbox(null)
    refresh()
  }

  if (activeSandbox) {
    return (
      <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
        <div style={{ padding: '8px 16px', background: '#161b22', borderBottom: '1px solid #30363d', display: 'flex', gap: 8, alignItems: 'center' }}>
          <span style={{ fontSize: 13, color: '#8b949e' }}>Sandbox: {activeSandbox}</span>
          <button onClick={() => setActiveSandbox(null)}>← Back</button>
          <button className="danger" onClick={() => stop(activeSandbox)}>Stop</button>
        </div>
        <Terminal sandboxId={activeSandbox} />
      </div>
    )
  }

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '32px 16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 22 }}>Sandlock</h1>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="primary" onClick={() => setShowCreate(true)}>New sandbox</button>
          <button onClick={onLogout}>Log out</button>
        </div>
      </div>

      {showCreate && (
        <CreateSandbox
          onCreated={(id) => { setShowCreate(false); setActiveSandbox(id); refresh() }}
          onCancel={() => setShowCreate(false)}
        />
      )}

      {loading ? (
        <p style={{ color: '#8b949e' }}>Loading…</p>
      ) : sandboxes.length === 0 ? (
        <p style={{ color: '#8b949e' }}>No sandboxes yet. Create one to get started.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {sandboxes.map(sb => (
            <div key={sb.id} className="card" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <code style={{ fontSize: 13 }}>{sb.id}</code>
                <div style={{ fontSize: 12, color: '#8b949e', marginTop: 2 }}>{sb.harness} · {sb.status}</div>
              </div>
              <button onClick={() => setActiveSandbox(sb.id)} disabled={sb.status !== 'running'}>Open</button>
              <button className="danger" onClick={() => stop(sb.id)}>Stop</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
