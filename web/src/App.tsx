import { useState } from 'react'
import { getToken, setToken, clearToken, api } from './api'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'

export default function App() {
  const [authed, setAuthed] = useState(!!getToken())

  function handleLogin(token: string) {
    setToken(token)
    setAuthed(true)
  }

  function handleLogout() {
    api.logout().catch(() => {})
    clearToken()
    setAuthed(false)
  }

  if (!authed) return <Login onLogin={handleLogin} />
  return <Dashboard onLogout={handleLogout} />
}
