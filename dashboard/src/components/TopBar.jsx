import { useNavigate, useLocation } from 'react-router-dom'
import { Bell, Activity } from 'lucide-react'
import { useState, useEffect } from 'react'

const NAV = [
  { label: 'Overview',  path: '/overview' },
  { label: 'Chat',      path: '/chat' },
  { label: 'Jobs',      path: '/jobs' },
  { label: 'Developer', path: '/developer' },
]

export default function TopBar() {
  const navigate  = useNavigate()
  const location  = useLocation()
  const [time, setTime] = useState(new Date())

  useEffect(() => {
    const t = setInterval(() => setTime(new Date()), 1000)
    return () => clearInterval(t)
  }, [])

  return (
    <div style={{
      height: '48px', background: '#161B22',
      borderBottom: '1px solid #30363D',
      display: 'flex', alignItems: 'center',
      padding: '0 16px', gap: '16px', flexShrink: 0,
    }}>
      <span style={{ color: '#3FB950', fontSize: '14px', fontWeight: 700, marginRight: '8px' }}>
        GROOT · Agent OS
      </span>

      <div style={{ display: 'flex', gap: '4px' }}>
        {NAV.map(n => (
          <button key={n.path} onClick={() => navigate(n.path)} style={{
            padding: '4px 12px', borderRadius: '6px', border: 'none',
            cursor: 'pointer', fontSize: '12px', fontFamily: 'Inter, sans-serif',
            background: location.pathname === n.path ? '#21262D' : 'transparent',
            color: location.pathname === n.path ? '#E6EDF3' : '#8B949E',
          }}>
            {n.label}
          </button>
        ))}
      </div>

      <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '16px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <div style={{ width: '6px', height: '6px', borderRadius: '50%', background: '#3FB950' }} />
          <span style={{ color: '#8B949E', fontSize: '11px' }}>All systems nominal</span>
        </div>
        <span style={{ color: '#8B949E', fontSize: '11px', fontFamily: 'JetBrains Mono, monospace' }}>
          {time.toUTCString().slice(17, 25)} UTC
        </span>
        <div style={{ position: 'relative', cursor: 'pointer' }}>
          <Bell size={16} color="#8B949E" />
          <div style={{
            position: 'absolute', top: '-4px', right: '-4px',
            width: '12px', height: '12px', borderRadius: '50%',
            background: '#F85149', fontSize: '8px', color: '#fff',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>0</div>
        </div>
      </div>
    </div>
  )
}
