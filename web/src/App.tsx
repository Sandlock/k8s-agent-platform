import { useState } from 'react'
import { getToken, setToken, clearToken, api } from './api'
import Login from './pages/Login'
import ChangePassword from './pages/ChangePassword'
import Dashboard from './pages/Dashboard'

type Screen = 'login' | 'change-password' | 'dashboard'

export default function App() {
  const [screen, setScreen] = useState<Screen>(getToken() ? 'dashboard' : 'login')
  const [isAdmin, setIsAdmin] = useState(false)

  function handleLogin(token: string, admin: boolean, mustChange: boolean) {
    setToken(token)
    setIsAdmin(admin)
    setScreen(mustChange ? 'change-password' : 'dashboard')
  }

  function handleLogout() {
    api.logout().catch(() => {})
    clearToken()
    setIsAdmin(false)
    setScreen('login')
  }

  if (screen === 'login') return <Login onLogin={handleLogin} />
  if (screen === 'change-password') return <ChangePassword onChanged={() => setScreen('dashboard')} onLogout={handleLogout} />
  return <Dashboard isAdmin={isAdmin} onLogout={handleLogout} />
}
