import { useState, useEffect, FormEvent } from 'react'
import { api, Skill } from '../api'

export default function Skills() {
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [editName, setEditName] = useState('')
  const [formName, setFormName] = useState('')
  const [formContent, setFormContent] = useState('')
  const [formError, setFormError] = useState('')
  const [saving, setSaving] = useState(false)

  async function refresh() {
    try {
      setSkills(await api.listSkills())
    } catch {}
    setLoading(false)
  }

  useEffect(() => { refresh() }, [])

  function openCreate() {
    setEditName('')
    setFormName('')
    setFormContent('')
    setFormError('')
    setShowForm(true)
  }

  function openEdit(sk: Skill) {
    setEditName(sk.name)
    setFormName(sk.name)
    setFormContent(sk.content)
    setFormError('')
    setShowForm(true)
  }

  function cancelForm() {
    setShowForm(false)
    setEditName('')
    setFormName('')
    setFormContent('')
    setFormError('')
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError('')
    const name = editName || formName
    try {
      await api.putSkill(name, formContent)
      cancelForm()
      refresh()
    } catch (err: any) {
      setFormError(err.message ?? 'Failed to save skill')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(name: string) {
    if (!confirm(`Delete skill "${name}"?`)) return
    await api.deleteSkill(name)
    refresh()
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
        <div>
          <h2 style={{ fontSize: 18, marginBottom: 4 }}>Skills</h2>
          <p style={{ fontSize: 13, color: '#8b949e', margin: 0 }}>
            Skills are markdown files written to <code>~/.claude/commands/</code> in every new sandbox, making them available as <code>/name</code> commands.
          </p>
        </div>
        {!showForm && (
          <button className="primary" onClick={openCreate}>New skill</button>
        )}
      </div>

      {showForm && (
        <div className="card" style={{ marginBottom: 20 }}>
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <h3 style={{ fontSize: 15, marginBottom: 4 }}>{editName ? `Edit /${editName}` : 'New skill'}</h3>
            {!editName && (
              <input
                placeholder="Name (e.g. deploy-check)"
                value={formName}
                onChange={e => setFormName(e.target.value.toLowerCase())}
                pattern="^[a-z0-9][a-z0-9_-]{0,63}$"
                title="Lowercase letters, digits, hyphens, underscores. Must start with a letter or digit."
                required
              />
            )}
            <textarea
              placeholder="Skill content (markdown prompt)"
              value={formContent}
              onChange={e => setFormContent(e.target.value)}
              required
              rows={10}
              style={{ fontFamily: 'monospace', fontSize: 13, resize: 'vertical' }}
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
      ) : skills.length === 0 && !showForm ? (
        <p style={{ color: '#8b949e' }}>No skills yet. Add one to make it available in every sandbox.</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {skills.map(sk => (
            <div key={sk.id} className="card" style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <code style={{ fontSize: 14 }}>/{sk.name}</code>
                <div style={{ fontSize: 12, color: '#8b949e', marginTop: 2, whiteSpace: 'pre', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 500 }}>
                  {sk.content.split('\n')[0]}
                </div>
              </div>
              <button onClick={() => openEdit(sk)}>Edit</button>
              <button className="danger" onClick={() => handleDelete(sk.name)}>Delete</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
