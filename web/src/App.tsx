import { useEffect, useState, type FormEvent } from 'react'
import { MantineProvider } from '@mantine/core'
import { api, ApiError, type UserContext } from './api'
import { THEMES, applyTheme, savedTheme } from './theme'
import PlanningWeek from './PlanningWeek'
import Resources from './Resources'
import Students from './Students'
import Settings from './Settings'
import Attendance from './Attendance'
import Exams from './Exams'
import Tasks from './Tasks'

type Tab = 'planning' | 'tasks' | 'attendance' | 'exams' | 'resources' | 'students' | 'settings'

// Mantine est la bibliothèque de composants retenue (banc d'essai
// Mantine / DaisyUI / AntD, arbitrage Angelo). Le provider suit le thème
// maison : couleur primaire approchante, mode sombre sur « Nuit ».
const MANTINE_PRIMARY: Record<string, string> = {
  '': 'blue',
  craie: 'blue',
  ardoise: 'indigo',
  lagune: 'teal',
  nuit: 'indigo',
}

// F-17: real login. The session is an HttpOnly cookie; on load we ask the
// server who we are, and the permissions it computed drive the UI.
export default function App() {
  const [user, setUser] = useState<UserContext | null>(null)
  const [checking, setChecking] = useState(true)
  const [tab, setTab] = useState<Tab>('tasks')
  const [theme, setTheme] = useState(savedTheme)

  useEffect(() => {
    api
      .me()
      .then(setUser)
      .catch(() => setUser(null))
      .finally(() => setChecking(false))
  }, [])

  const pickTheme = (id: string) => {
    applyTheme(id)
    setTheme(id)
  }

  const shell = (children: React.ReactNode) => (
    <MantineProvider
      forceColorScheme={theme === 'nuit' ? 'dark' : 'light'}
      theme={{ primaryColor: MANTINE_PRIMARY[theme] ?? 'blue' }}
    >
      {children}
    </MantineProvider>
  )

  if (checking) return null
  if (!user) return shell(<LoginForm onLoggedIn={setUser} />)

  const logout = () => {
    void api.logout().finally(() => setUser(null))
  }

  return shell(
    <>
      <nav className="mainnav">
        <span className="brand">CAMO-EDUCASER</span>
        <button className={tab === 'tasks' ? 'active' : ''} onClick={() => setTab('tasks')}>
          Tableau de bord
        </button>
        <button className={tab === 'planning' ? 'active' : ''} onClick={() => setTab('planning')}>
          Planning
        </button>
        <button className={tab === 'attendance' ? 'active' : ''} onClick={() => setTab('attendance')}>
          Présences
        </button>
        <button className={tab === 'exams' ? 'active' : ''} onClick={() => setTab('exams')}>
          Examens
        </button>
        <button className={tab === 'resources' ? 'active' : ''} onClick={() => setTab('resources')}>
          Ressources
        </button>
        <button className={tab === 'students' ? 'active' : ''} onClick={() => setTab('students')}>
          Élèves
        </button>
        <button className={tab === 'settings' ? 'active' : ''} onClick={() => setTab('settings')}>
          Paramétrage
        </button>
        <div className="spacer" />
        <ThemePicker value={theme} onPick={pickTheme} />
        <span className="muted">{user.name}</span>
        <button onClick={logout}>Déconnexion</button>
      </nav>
      {tab === 'planning' && <PlanningWeek user={user} />}
      {tab === 'tasks' && <Tasks user={user} onGo={(t) => setTab(t)} />}
      {tab === 'attendance' && <Attendance user={user} />}
      {tab === 'exams' && <Exams user={user} />}
      {tab === 'resources' && <Resources user={user} />}
      {tab === 'students' && <Students user={user} />}
      {tab === 'settings' && <Settings user={user} />}
    </>
  )
}

function ThemePicker({ value, onPick }: { value: string; onPick: (id: string) => void }) {
  return (
    <select
      className="theme-pick"
      value={value}
      onChange={(e) => onPick(e.target.value)}
      title="Thème d'affichage"
      aria-label="Thème d'affichage"
    >
      {THEMES.map((t) => (
        <option key={t.id} value={t.id}>
          {t.label}
        </option>
      ))}
    </select>
  )
}

function LoginForm({ onLoggedIn }: { onLoggedIn: (u: UserContext) => void }) {
  const [login, setLogin] = useState('')
  const [secret, setSecret] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      onLoggedIn(await api.login(login.trim(), secret))
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 401
          ? 'Identifiant ou mot de passe invalide.'
          : 'Connexion impossible. Réessayez.',
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="gate" onSubmit={(e) => void submit(e)}>
      <h1>CAMO-EDUCASER — Planning</h1>
      <div className="form-row">
        <label>Identifiant</label>
        <input value={login} onChange={(e) => setLogin(e.target.value)} autoFocus />
      </div>
      <div className="form-row">
        <label>Mot de passe</label>
        <input type="password" value={secret} onChange={(e) => setSecret(e.target.value)} />
      </div>
      {error && <div className="error-box">{error}</div>}
      <button className="primary" type="submit" disabled={busy || !login.trim() || !secret}>
        Se connecter
      </button>
    </form>
  )
}
