import { useState, useEffect, FormEvent } from 'react'
import { api, MCPServer, UpsertMCPServerBody } from '../api'

type MCPType = 'http' | 'sse' | 'stdio'

function KVEditor({
  label,
  value,
  onChange,
  secretKeys,
}: {
  label: string
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
  secretKeys?: string[]
}) {
  const entries = Object.entries(value)
  function set(k: string, v: string) {
    onChange({ ...value, [k]: v })
  }
  function remove(k: string) {
    const next = { ...value }
    delete next[k]
    onChange(next)
  }
  function add() {
    onChange({ ...value, '': '' })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <label style={{ fontSize: 13, color: '#8b949e' }}>{label}</label>
      {secretKeys && secretKeys.length > 0 && (
        <p style={{ fontSize: 12, color: '#8b949e', margin: 0 }}>
          Existing secret keys (values hidden): {secretKeys.map(k => <code key={k} style={{ marginRight: 6 }}>{k}</code>)}
          <br />Re-enter values below to update them.
        </p>
      )}
      {entries.map(([k, v], i) => (
        <div key={i} style={{ display: 'flex', gap: 6 }}>
          <input
            placeholder="KEY"
            value={k}
            onChange={e => {
              const next = Object.fromEntries(entries.map(([ek, ev], j) => j === i ? [e.target.value, ev] : [ek, ev]))
              onChange(next)
            }}
            style={{ flex: 1, fontFamily: 'monospace', fontSize: 13 }}
          />
          <input
            placeholder="value"
            value={v}
            onChange={e => set(k, e.target.value)}
            style={{ flex: 2, fontFamily: 'monospace', fontSize: 13 }}
          />
          <button type="button" onClick={() => remove(k)} style={{ flexShrink: 0 }}>✕</button>
        </div>
      ))}
      <button type="button" onClick={add} style={{ alignSelf: 'flex-start', fontSize: 12 }}>+ Add entry</button>
    </div>
  )
}

export default function MCPServers() {
  const [servers, setServers] = useState<MCPServer[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [editName, setEditName] = useState('')
  const [editServer, setEditServer] = useState<MCPServer | null>(null)

  const [formName, setFormName] = useState('')
  const [formType, setFormType] = useState<MCPType>('http')
  const [formURL, setFormURL] = useState('')
  const [formCommand, setFormCommand] = useState('')
  const [formArgs, setFormArgs] = useState('')
  const [formEnv, setFormEnv] = useState<Record<string, string>>({})
  const [formSecretEnv, setFormSecretEnv] = useState<Record<string, string>>({})
  const [formHeaders, setFormHeaders] = useState<Record<string, string>>({})
  const [formSecretHeaders, setFormSecretHeaders] = useState<Record<string, string>>({})
  const [formDisplayName, setFormDisplayName] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function refresh() {
    try {
      setServers(await api.listMCPServers())
    } catch {}
    setLoading(false)
  }

  useEffect(() => { refresh() }, [])

  function openCreate() {
    setEditName('')
    setEditServer(null)
    setFormName('')
    setFormType('http')
    setFormURL('')
    setFormCommand('')
    setFormArgs('')
    setFormEnv({})
    setFormSecretEnv({})
    setFormHeaders({})
    setFormSecretHeaders({})
    setFormDisplayName('')
    setFormError('')
    setShowForm(true)
  }

  function openEdit(srv: MCPServer) {
    setEditName(srv.name)
    setEditServer(srv)
    setFormName(srv.name)
    setFormType(srv.type)
    setFormURL(srv.url ?? '')
    setFormCommand(srv.command ?? '')
    setFormArgs((srv.args ?? []).join('\n'))
    setFormEnv(srv.env ?? {})
    setFormSecretEnv({})
    setFormHeaders(srv.headers ?? {})
    setFormSecretHeaders({})
    setFormDisplayName(srv.displayName ?? '')
    setFormError('')
    setShowForm(true)
  }

  function cancelForm() {
    setShowForm(false)
    setEditName('')
    setEditServer(null)
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError('')
    const name = editName || formName
    const body: UpsertMCPServerBody = {
      type: formType,
      displayName: formDisplayName || undefined,
    }
    if (formType === 'http' || formType === 'sse') {
      body.url = formURL
      if (Object.keys(formHeaders).length > 0) body.headers = formHeaders
      if (Object.keys(formSecretHeaders).length > 0) body.secretHeaders = formSecretHeaders
    } else {
      body.command = formCommand
      const args = formArgs.split('\n').map(s => s.trim()).filter(Boolean)
      if (args.length > 0) body.args = args
    }
    if (Object.keys(formEnv).length > 0) body.env = formEnv
    if (Object.keys(formSecretEnv).length > 0) body.secretEnv = formSecretEnv

    try {
      await api.putMCPServer(name, body)
      cancelForm()
      refresh()
    } catch (err: any) {
      setFormError(err.message ?? 'Failed to save MCP server')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(name: string) {
    if (!confirm(`Delete MCP server "${name}"?`)) return
    await api.deleteMCPServer(name)
    refresh()
  }

  function serverSummary(srv: MCPServer) {
    if (srv.type === 'stdio') return `stdio: ${srv.command}`
    return `${srv.type}: ${srv.url}`
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h2 style={{ fontSize: 18, marginBottom: 4 }}>MCP Servers</h2>
          <p style={{ fontSize: 13, color: '#8b949e', margin: 0 }}>
            MCP servers are registered in <code>~/.claude/settings.json</code> in every new sandbox, making them available as tools inside Claude Code.
          </p>
        </div>
        {!showForm && (
          <button className="primary" onClick={openCreate}>Add server</button>
        )}
      </div>

      {showForm && (
        <div className="card" style={{ marginBottom: 20 }}>
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <h3 style={{ fontSize: 15, marginBottom: 4 }}>{editName ? `Edit ${editName}` : 'New MCP server'}</h3>

            {!editName && (
              <input
                placeholder="Name (e.g. github-mcp)"
                value={formName}
                onChange={e => setFormName(e.target.value.toLowerCase())}
                pattern="^[a-z0-9][a-z0-9_-]{0,63}$"
                title="Lowercase letters, digits, hyphens, underscores. Must start with a letter or digit."
                required
              />
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              <label style={{ fontSize: 13, color: '#8b949e' }}>Type</label>
              <select value={formType} onChange={e => setFormType(e.target.value as MCPType)}
                style={{ padding: '6px 8px', background: '#0d1117', border: '1px solid #30363d', borderRadius: 6, color: '#e6edf3', fontSize: 14 }}>
                <option value="http">HTTP</option>
                <option value="sse">SSE</option>
                <option value="stdio">stdio</option>
              </select>
            </div>

            {(formType === 'http' || formType === 'sse') && (
              <input
                placeholder="URL (e.g. https://my-mcp-server.example.com/mcp)"
                value={formURL}
                onChange={e => setFormURL(e.target.value)}
                required
              />
            )}

            {formType === 'stdio' && (
              <>
                <input
                  placeholder="Command (e.g. npx)"
                  value={formCommand}
                  onChange={e => setFormCommand(e.target.value)}
                  required
                />
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                  <label style={{ fontSize: 13, color: '#8b949e' }}>Args (one per line)</label>
                  <textarea
                    placeholder="-y&#10;@modelcontextprotocol/server-github"
                    value={formArgs}
                    onChange={e => setFormArgs(e.target.value)}
                    rows={3}
                    style={{ fontFamily: 'monospace', fontSize: 13, resize: 'vertical' }}
                  />
                </div>
                <KVEditor label="Environment variables" value={formEnv} onChange={setFormEnv} />
                <KVEditor
                  label="Secret environment variables (stored encrypted, values not returned by API)"
                  value={formSecretEnv}
                  onChange={setFormSecretEnv}
                  secretKeys={editServer?.secretEnvKeys}
                />
              </>
            )}

            {(formType === 'http' || formType === 'sse') && (
              <>
                <KVEditor label="HTTP headers" value={formHeaders} onChange={setFormHeaders} />
                <KVEditor
                  label="Secret HTTP headers (stored encrypted, values not returned by API)"
                  value={formSecretHeaders}
                  onChange={setFormSecretHeaders}
                  secretKeys={editServer?.secretHeaderKeys}
                />
                <KVEditor label="Environment variables" value={formEnv} onChange={setFormEnv} />
                <KVEditor
                  label="Secret environment variables (stored encrypted)"
                  value={formSecretEnv}
                  onChange={setFormSecretEnv}
                  secretKeys={editServer?.secretEnvKeys}
                />
              </>
            )}

            <input
              placeholder="Display name (optional)"
              value={formDisplayName}
              onChange={e => setFormDisplayName(e.target.value)}
            />

            {formError && <p style={{ color: '#f85149', fontSize: 13 }}>{formError}</p>}

            <div style={{ display: 'flex', gap: 8 }}>
              <button type="submit" className="primary" disabled={saving}>
                {saving ? 'Saving…' : 'Save'}
              </button>
              <button type="button" onClick={cancelForm}>Cancel</button>
            </div>
          </form>
        </div>
      )}

      {loading ? (
        <p style={{ color: '#8b949e' }}>Loading…</p>
      ) : servers.length === 0 && !showForm ? (
        <p style={{ color: '#8b949e' }}>No MCP servers yet. Add one to make it available in every sandbox.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {servers.map(srv => (
            <div key={srv.id} className="card" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <code style={{ fontSize: 14 }}>{srv.name}</code>
                {srv.displayName && (
                  <span style={{ fontSize: 13, color: '#8b949e', marginLeft: 8 }}>{srv.displayName}</span>
                )}
                <div style={{ fontSize: 12, color: '#8b949e', marginTop: 2 }}>{serverSummary(srv)}</div>
              </div>
              <button onClick={() => openEdit(srv)}>Edit</button>
              <button className="danger" onClick={() => handleDelete(srv.name)}>Delete</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
