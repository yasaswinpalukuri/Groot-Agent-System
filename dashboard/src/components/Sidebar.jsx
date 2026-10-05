import { useNavigate, useLocation } from 'react-router-dom'
import { LayoutDashboard, MessageSquare, CheckSquare, ListTodo,
         Bot, Search, Briefcase, Code, BookOpen, Activity,
         Database, Shield } from 'lucide-react'

const SECTIONS = [
  { label: 'COMMAND', items: [
    { icon: LayoutDashboard, label: 'Overview',    path: '/overview' },
    { icon: ListTodo,        label: 'Task Queue',  path: '/queue' },
    { icon: MessageSquare,   label: 'Ask Groot',   path: '/chat' },
    { icon: CheckSquare,     label: 'Job Tracker', path: '/approvals' },

  ]},
  { label: 'AGENTS', items: [
    { icon: Bot,       label: 'Groot',      path: '/agent/groot' },
    { icon: Search,    label: 'Einstein',   path: '/agent/einstein' },
    { icon: Briefcase, label: 'Job Search', path: '/jobs' },
    { icon: Code,      label: 'Tony',       path: '/developer' },
    { icon: BookOpen,   label: 'Daily Log',  path: '/dailylog' },
    { icon: BookOpen,   label: 'Operations', path: '/operations' },
    { icon: BookOpen,  label: 'Siva',       path: '/agent/siva' },
  ]},
  { label: 'SYSTEM', items: [
    { icon: Activity,  label: 'Monitoring', path: '/monitoring' },
    { icon: Database,  label: 'RAG Store',  path: '/rag' },
    { icon: Shield,    label: 'Security',   path: '/security' },
  ]},
]

export default function Sidebar() {
  const navigate = useNavigate()
  const location = useLocation()

  return (
    <div style={{
      width: '200px', background: '#161B22', flexShrink: 0,
      borderRight: '1px solid #30363D', padding: '16px 8px',
      overflowY: 'auto',
    }}>
      {SECTIONS.map(s => (
        <div key={s.label} style={{ marginBottom: '24px' }}>
          <div style={{ color: '#6E7681', fontSize: '10px', fontWeight: 600,
            letterSpacing: '0.08em', padding: '0 8px', marginBottom: '4px' }}>
            {s.label}
          </div>
          {s.items.map(item => {
            const Icon   = item.icon
            const active = item.path && location.pathname === item.path
            return (
              <button key={item.label}
                onClick={() => item.path && navigate(item.path)}
                style={{
                  display: 'flex', alignItems: 'center', gap: '8px',
                  width: '100%', padding: '6px 8px', borderRadius: '6px',
                  border: 'none', textAlign: 'left',
                  cursor: item.path ? 'pointer' : 'default',
                  background: active ? '#21262D' : 'transparent',
                  color: active ? '#E6EDF3' : item.path ? '#8B949E' : '#444C56',
                  fontSize: '12px', fontFamily: 'Inter, sans-serif',
                  marginBottom: '2px',
                }}>
                <Icon size={14} />
                {item.label}
              </button>
            )
          })}
        </div>
      ))}
    </div>
  )
}
