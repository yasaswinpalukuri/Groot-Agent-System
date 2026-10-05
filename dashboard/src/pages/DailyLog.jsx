import { useState, useEffect } from 'react'

const API = 'http://groot:8000'

const agentColors = {
  groot: '#58A6FF',
  einstein: '#BC8CFF',
  tony: '#3FB950',
  siva: '#F0883E',
  job_search: '#D29922',
  career: '#F85149',
}

const statusColors = {
  completed: '#3FB950',
  failed: '#F85149',
  running: '#58A6FF',
  pending: '#D29922',
}

export default function DailyLog() {
  const [tasks, setTasks] = useState([])
  const [scoreboard, setScoreboard] = useState([])
  const [hours, setHours] = useState(24)
  const todayStr = new Date().toLocaleDateString('en-CA')
  const [agentFilter, setAgentFilter] = useState('all')
  const [loading, setLoading] = useState(true)
  const [apiCounts, setApiCounts] = useState({ total: 0, completed: 0, failed: 0 })
  const [lastUpdated, setLastUpdated] = useState(null)

  const fetchData = async () => {
    try {
      const [actRes, scoreRes] = await Promise.all([
        fetch(`${API}/activity/today`),
        fetch(`${API}/tasks/scoreboard`)
      ])
      const act = await actRes.json()
      const score = await scoreRes.json()
      setTasks(act.activities || [])
      setScoreboard(score.scoreboard || [])
      setLastUpdated(new Date())
    } catch (e) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [hours])
  useEffect(() => {
    const interval = setInterval(fetchData, 60000)
    return () => clearInterval(interval)
  }, [hours])

  const filtered = agentFilter === 'all'
    ? tasks
    : tasks.filter(t => t.agent === agentFilter)

  const agents = [...new Set(tasks.map(t => t.agent))]

  const groupByDate = (tasks) => {
    const groups = {}
    tasks.forEach(t => {
      const date = new Date(t.timestamp).toLocaleDateString('en-CA')
      if (!groups[date]) groups[date] = []
      groups[date].push(t)
    })
    return groups
  }

  const grouped = groupByDate(filtered)
  const dates = Object.keys(grouped).sort().reverse()

  // API returns only today's tasks already
  const successToday = tasks.filter(t => t.status === 'completed').length
  const failedToday = tasks.filter(t => t.status === 'failed').length

  return (
    <div style={{ padding: '24px', maxWidth: '1000px', margin: '0 auto' }}>

      {/* Header */}
      <div style={{ marginBottom: '24px', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '12px' }}>
        <div>
          <h1 style={{ fontSize: '24px', fontWeight: 700, color: '#E6EDF3', marginBottom: '4px' }}>Daily Task Log</h1>
          <p style={{ color: '#6E7681', fontSize: '13px' }}>
            {lastUpdated ? `Last updated: ${lastUpdated.toLocaleTimeString()}` : 'Loading...'}
          </p>
        </div>
        <button onClick={fetchData} style={{
          padding: '8px 16px', background: 'transparent', border: '1px solid #30363D',
          borderRadius: '6px', color: '#E6EDF3', cursor: 'pointer', fontSize: '13px'
        }}>↻ Refresh</button>
      </div>

      {/* Summary cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '12px', marginBottom: '24px' }}>
        {[
          { label: 'Tasks Today', value: tasks.length, color: '#58A6FF' },
          { label: 'Completed', value: successToday, color: '#3FB950' },
          { label: 'Failed', value: failedToday, color: '#F85149' },
          { label: 'Success Rate', value: tasks.length ? Math.round(successToday / tasks.length * 100) + '%' : '—', color: '#BC8CFF' },
        ].map(card => (
          <div key={card.label} style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', padding: '16px' }}>
            <div style={{ fontSize: '24px', fontWeight: 700, color: card.color }}>{card.value}</div>
            <div style={{ fontSize: '12px', color: '#6E7681', marginTop: '4px' }}>{card.label}</div>
          </div>
        ))}
      </div>

      {/* Scoreboard */}
      {scoreboard.length > 0 && (
        <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', padding: '16px', marginBottom: '24px' }}>
          <div style={{ fontSize: '13px', fontWeight: 600, color: '#8B949E', marginBottom: '12px', textTransform: 'uppercase', letterSpacing: '0.06em' }}>7-Day Agent Scoreboard</div>
          <div style={{ display: 'flex', gap: '16px', flexWrap: 'wrap' }}>
            {scoreboard.map(s => (
              <div key={s.agent} style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ width: '8px', height: '8px', borderRadius: '50%', background: agentColors[s.agent] || '#8B949E', display: 'inline-block' }}></span>
                <span style={{ color: agentColors[s.agent] || '#8B949E', fontSize: '13px', fontWeight: 600 }}>{s.agent}</span>
                <span style={{ color: '#3FB950', fontSize: '13px' }}>{s.completed}✓</span>
                {s.failed > 0 && <span style={{ color: '#F85149', fontSize: '13px' }}>{s.failed}✗</span>}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Filters */}
      <div style={{ display: 'flex', gap: '8px', marginBottom: '20px', flexWrap: 'wrap', alignItems: 'center' }}>
        <span style={{ color: '#6E7681', fontSize: '13px' }}>Filter:</span>
        {['all', ...agents].map(a => (
          <button key={a} onClick={() => setAgentFilter(a)} style={{
            padding: '5px 12px', borderRadius: '100px', fontSize: '12px', cursor: 'pointer',
            background: agentFilter === a ? (agentColors[a] || '#58A6FF') + '22' : 'transparent',
            border: `1px solid ${agentFilter === a ? (agentColors[a] || '#58A6FF') : '#30363D'}`,
            color: agentFilter === a ? (agentColors[a] || '#58A6FF') : '#8B949E',
          }}>{a}</button>
        ))}
        <select value={hours} onChange={e => setHours(Number(e.target.value))} style={{
          marginLeft: 'auto', padding: '5px 10px', background: '#161B22', border: '1px solid #30363D',
          borderRadius: '6px', color: '#E6EDF3', fontSize: '13px', cursor: 'pointer'
        }}>
          <option value={24}>Last 24 hours</option>
          <option value={48}>Last 48 hours</option>
          <option value={168}>Last 7 days</option>
          <option value={720}>Last 30 days</option>
        </select>
      </div>

      {/* Task Timeline */}
      {loading ? (
        <div style={{ color: '#6E7681', textAlign: 'center', padding: '40px' }}>Loading tasks...</div>
      ) : filtered.length === 0 ? (
        <div style={{ color: '#6E7681', textAlign: 'center', padding: '40px', background: '#161B22', borderRadius: '8px', border: '1px solid #30363D' }}>
          No tasks recorded yet. Tasks will appear here as agents run.
        </div>
      ) : (
        dates.map(date => (
          <div key={date} style={{ marginBottom: '24px' }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: '#8B949E', marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '10px' }}>
              <span>{date === new Date().toLocaleDateString('en-CA') ? '📅 Today' : `📅 ${date}`}</span>
              <span style={{ color: '#30363D' }}>—</span>
              <span>{grouped[date].length} tasks</span>
            </div>
            <div style={{ borderLeft: '2px solid #21262D', paddingLeft: '20px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {grouped[date].map((task, i) => (
                <div key={i} style={{
                  background: '#161B22', border: '1px solid #30363D', borderRadius: '8px',
                  padding: '12px 16px', position: 'relative'
                }}>
                  {/* Timeline dot */}
                  <div style={{
                    position: 'absolute', left: '-27px', top: '16px',
                    width: '10px', height: '10px', borderRadius: '50%',
                    background: statusColors[task.status] || '#8B949E',
                    border: '2px solid #0D1117'
                  }}></div>

                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '12px', flexWrap: 'wrap' }}>
                    <div style={{ flex: 1 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '6px', flexWrap: 'wrap' }}>
                        <span style={{
                          fontSize: '11px', fontWeight: 700, padding: '2px 8px', borderRadius: '100px',
                          background: (agentColors[task.agent] || '#8B949E') + '22',
                          color: agentColors[task.agent] || '#8B949E',
                          border: `1px solid ${(agentColors[task.agent] || '#8B949E')}44`
                        }}>{task.agent?.toUpperCase()}</span>
                        <span style={{
                          fontSize: '11px', padding: '2px 8px', borderRadius: '100px',
                          background: (statusColors[task.status] || '#8B949E') + '22',
                          color: statusColors[task.status] || '#8B949E',
                          border: `1px solid ${(statusColors[task.status] || '#8B949E')}44`
                        }}>{task.status}</span>
                        {task.duration_seconds && (
                          <span style={{ fontSize: '11px', color: '#6E7681' }}>{Math.round(task.duration_seconds)}s</span>
                        )}
                      </div>
                      <div style={{ fontSize: '13px', color: '#E6EDF3', marginBottom: '4px', fontWeight: 500 }}>
                        {task.task?.slice(0, 120)}{task.task?.length > 120 ? '...' : ''}
                      </div>
                      {task.response && (
                        <div style={{ fontSize: '12px', color: '#6E7681', marginTop: '4px', fontFamily: 'monospace' }}>
                          ↳ {task.response?.slice(0, 150)}{task.response?.length > 150 ? '...' : ''}
                        </div>
                      )}
                    </div>
                    <div style={{ fontSize: '11px', color: '#6E7681', whiteSpace: 'nowrap', fontFamily: 'monospace' }}>
                      {new Date(task.timestamp).toLocaleTimeString()}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))
      )}
    </div>
  )
}
