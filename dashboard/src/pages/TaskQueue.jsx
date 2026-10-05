import { useState, useEffect } from 'react'
import { Clock, RefreshCw, Zap, Activity, XCircle } from 'lucide-react'

const API = 'http://groot:8000'

const PRIORITY_LABEL = { 0: 'URGENT', 1: 'HIGH', 2: 'MEDIUM', 3: 'LOW' }
const PRIORITY_COLOR = { 0: '#F85149', 1: '#D29922', 2: '#58A6FF', 3: '#8B949E' }
const PRIORITY_BG    = { 0: '#2D1A1A', 1: '#2D2208', 2: '#1C2F4A', 3: '#21262D' }

const AGENT_MODEL = {
  groot: 'qwen2.5:7b', einstein: 'deepseek-r1:14b',
  tony: 'qwen2.5-coder:7b', siva: 'phi3:medium',
  job_search: 'qwen2.5:7b', career: 'mistral:7b-instruct',
}

export default function TaskQueue() {
  const [queueData, setQueueData]     = useState({ queue_depth: 0, current_task: null, tasks: [] })
  const [history, setHistory]         = useState([])
  const [loading, setLoading]         = useState(false)
  const [lastUpdate, setLastUpdate]   = useState(null)
  const [lastTaskKey, setLastTaskKey] = useState(null)
  const [scoreboard, setScoreboard] = useState([])

  const fetchQueue = async () => {
    setLoading(true)
    try {
      const r = await fetch(`${API}/scheduler/queue`)
      const d = await r.json()
      setQueueData(d)
      setLastUpdate(new Date().toLocaleTimeString())
      // Fetch scoreboard
      try {
        const sb = await fetch(`${API}/tasks/scoreboard`).then(r => r.json())
        setScoreboard(sb.scoreboard || [])
      } catch {}
      if (d.current_task) {
        const key = d.current_task.agent + '::' + d.current_task.preview
        setLastTaskKey(prev => {
          if (prev !== key) {
            setHistory(h => [{
              ...d.current_task,
              timestamp: new Date().toISOString(),
              id: key + Date.now(),
            }, ...h].slice(0, 50))
          }
          return key
        })
      }
    } catch {}
    setLoading(false)
  }

  const terminateTask = async () => {
    try {
      await fetch(`${API}/scheduler/terminate`, { method: 'POST' })
      fetchQueue()
    } catch {}
  }

  const setPriority = async (val) => {
    try {
      await fetch(`${API}/scheduler/priority`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ priority: parseInt(val) }),
      })
      fetchQueue()
    } catch {}
  }

  useEffect(() => {
    fetchQueue()
    const interval = setInterval(fetchQueue, 3000)
    return () => clearInterval(interval)
  }, [])

  const allTasks = [
    ...(queueData.current_task ? [{ ...queueData.current_task, is_current: true }] : []),
    ...(queueData.tasks || []),
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.08em' }}>TASK QUEUE</span>
        <span style={{ color: '#6E7681', fontSize: '10px', fontFamily: 'JetBrains Mono, monospace' }}>
          {lastUpdate ? 'Updated ' + lastUpdate : 'Loading...'}
        </span>
        <button onClick={fetchQueue} style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center',
          gap: '4px', padding: '4px 10px', borderRadius: '6px', border: '1px solid #30363D',
          background: 'transparent', color: '#8B949E', cursor: 'pointer', fontSize: '11px' }}>
          <RefreshCw size={10} style={{ animation: loading ? 'spin 1s linear infinite' : 'none' }} />
          Refresh
        </button>
      </div>

      <div style={{ display: 'flex', gap: '16px' }}>
        {[
          { icon: Clock,    label: 'QUEUE DEPTH', value: queueData.queue_depth,
            color: queueData.queue_depth > 0 ? '#D29922' : '#3FB950' },
          { icon: Activity, label: 'STATUS',
            value: queueData.queue_depth > 0 ? 'BUSY' : 'IDLE',
            color: queueData.queue_depth > 0 ? '#D29922' : '#3FB950' },
          { icon: Zap,   label: 'RAM SAFETY', value: '3 GB', color: '#E6EDF3' },
          { icon: Clock, label: 'MAX WAIT',   value: '30s',  color: '#E6EDF3' },
        ].map(s => (
          <div key={s.label} style={{ flex: 1, background: '#161B22',
            border: '1px solid #30363D', borderRadius: '8px', padding: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
              <s.icon size={14} color="#8B949E" />
              <span style={{ color: '#8B949E', fontSize: '10px', fontWeight: 600, letterSpacing: '0.06em' }}>
                {s.label}
              </span>
            </div>
            <div style={{ color: s.color, fontSize: '22px', fontWeight: 700,
              fontFamily: 'JetBrains Mono, monospace' }}>{s.value}</div>
          </div>
        ))}
      </div>

      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D',
          display: 'flex', alignItems: 'center', gap: '8px' }}>
          <div style={{ width: '8px', height: '8px', borderRadius: '50%',
            background: queueData.queue_depth > 0 ? '#D29922' : '#3FB950',
            animation: queueData.queue_depth > 0 ? 'pulse 1.5s infinite' : 'none' }} />
          <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em' }}>
            PENDING TASKS — {queueData.queue_depth} in queue
          </span>
          {queueData.queue_depth > 0 && (
            <div style={{ marginLeft: 'auto', display: 'flex', gap: '8px', alignItems: 'center' }}>
              <select onChange={e => e.target.value && setPriority(e.target.value)}
                defaultValue=""
                style={{ background: '#21262D', border: '1px solid #30363D',
                  borderRadius: '4px', color: '#8B949E', fontSize: '11px',
                  padding: '4px 8px', cursor: 'pointer' }}>
                <option value="" disabled>Set Priority</option>
                <option value="0">URGENT</option>
                <option value="1">HIGH</option>
                <option value="2">MEDIUM</option>
                <option value="3">LOW</option>
              </select>
              <button onClick={terminateTask} style={{
                display: 'flex', alignItems: 'center', gap: '4px',
                padding: '4px 10px', borderRadius: '6px',
                border: '1px solid #F85149', background: '#2D1A1A',
                color: '#F85149', cursor: 'pointer', fontSize: '11px' }}>
                <XCircle size={12} />
                Terminate
              </button>
            </div>
          )}
        </div>

        {allTasks.length === 0 ? (
          <div style={{ padding: '48px', textAlign: 'center' }}>
            <div style={{ color: '#3FB950', fontSize: '13px',
              fontFamily: 'JetBrains Mono, monospace', marginBottom: '8px' }}>● IDLE</div>
            <div style={{ color: '#6E7681', fontSize: '12px' }}>No tasks pending.</div>
            <div style={{ color: '#444C56', fontSize: '11px', marginTop: '8px' }}>
              Tasks are created when you click Interested on a job, use Chat, or n8n workflows trigger agents.
            </div>
          </div>
        ) : (
          <div style={{ padding: '8px' }}>
            {allTasks.map((task, i) => (
              <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '12px',
                padding: '12px 16px', borderRadius: '6px',
                background: PRIORITY_BG[task.priority] || '#21262D',
                marginBottom: '6px',
                border: task.is_current ? '1px solid #D2992244' : '1px solid #30363D',
                position: 'relative' }}>
                {task.is_current && (
                  <div style={{ position: 'absolute', top: '6px', right: '10px',
                    fontSize: '9px', color: '#D29922',
                    fontFamily: 'JetBrains Mono, monospace' }}>RUNNING</div>
                )}
                <div style={{ width: '8px', height: '8px', borderRadius: '50%', flexShrink: 0,
                  background: PRIORITY_COLOR[task.priority] || '#8B949E',
                  animation: task.is_current ? 'pulse 1s infinite' : 'none' }} />
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
                    <span style={{ color: '#E6EDF3', fontSize: '13px', fontWeight: 600 }}>
                      {(task.agent || 'unknown').toUpperCase()}
                    </span>
                    <span style={{ color: '#6E7681', fontSize: '11px',
                      fontFamily: 'JetBrains Mono, monospace' }}>
                      {AGENT_MODEL[task.agent] || task.model || ''}
                    </span>
                  </div>
                  <div style={{ color: '#8B949E', fontSize: '12px' }}>
                    {task.preview || 'Processing...'}
                  </div>
                </div>
                <span style={{ fontSize: '10px', padding: '3px 10px', borderRadius: '4px',
                  background: PRIORITY_BG[task.priority], color: PRIORITY_COLOR[task.priority],
                  fontFamily: 'JetBrains Mono, monospace', fontWeight: 600, flexShrink: 0 }}>
                  {PRIORITY_LABEL[task.priority] || 'MEDIUM'}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      <div style={{ background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D',
          display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em' }}>
            TASK HISTORY (this session)
          </span>
          {history.length > 0 && (
            <button onClick={() => setHistory([])} style={{ background: 'transparent', border: 'none',
              color: '#6E7681', fontSize: '10px', cursor: 'pointer',
              fontFamily: 'JetBrains Mono, monospace' }}>Clear</button>
          )}
        </div>
        {history.length === 0 ? (
          <div style={{ padding: '32px', textAlign: 'center', color: '#6E7681', fontSize: '12px' }}>
            No tasks processed yet this session
          </div>
        ) : (
          history.map((task, i) => (
            <div key={task.id || i} style={{ display: 'flex', alignItems: 'center', gap: '12px',
              padding: '10px 16px', borderBottom: '1px solid #21262D' }}>
              <span style={{ color: '#6E7681', fontSize: '10px',
                fontFamily: 'JetBrains Mono, monospace', flexShrink: 0, width: '70px' }}>
                {new Date(task.timestamp).toLocaleTimeString()}
              </span>
              <span style={{ color: PRIORITY_COLOR[task.priority], fontSize: '11px',
                fontFamily: 'JetBrains Mono, monospace', width: '65px', flexShrink: 0 }}>
                {(task.agent || '').toUpperCase()}
              </span>
              <span style={{ color: '#8B949E', fontSize: '12px', flex: 1 }}>
                {task.preview || 'Task'}
              </span>
              <span style={{ color: PRIORITY_COLOR[task.priority], fontSize: '10px',
                fontFamily: 'JetBrains Mono, monospace', flexShrink: 0 }}>
                {PRIORITY_LABEL[task.priority] || 'MEDIUM'}
              </span>
            </div>
          ))
        )}
      </div>
      {/* Scoreboard */}
      <div style={{ background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D' }}>
          <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em' }}>
            AGENT SCOREBOARD (last 7 days)
          </span>
        </div>
        {scoreboard.length === 0 ? (
          <div style={{ padding: '24px', textAlign: 'center', color: '#6E7681', fontSize: '12px' }}>
            No tasks logged yet
          </div>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid #21262D' }}>
                {['AGENT','TASKS','COMPLETED','FAILED','AVG TIME','LAST ACTIVE'].map(h => (
                  <th key={h} style={{ padding: '8px 16px', textAlign: 'left',
                    color: '#6E7681', fontSize: '10px', fontWeight: 600, letterSpacing: '0.06em' }}>
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {scoreboard.map((row, i) => (
                <tr key={i} style={{ borderBottom: '1px solid #21262D' }}>
                  <td style={{ padding: '10px 16px', color: '#E6EDF3', fontSize: '13px', fontWeight: 500 }}>
                    {row.agent?.toUpperCase()}
                  </td>
                  <td style={{ padding: '10px 16px', color: '#58A6FF', fontSize: '13px',
                    fontFamily: 'JetBrains Mono, monospace' }}>
                    {row.total_tasks}
                  </td>
                  <td style={{ padding: '10px 16px', color: '#3FB950', fontSize: '13px',
                    fontFamily: 'JetBrains Mono, monospace' }}>
                    {row.completed}
                  </td>
                  <td style={{ padding: '10px 16px', color: row.failed > 0 ? '#F85149' : '#6E7681',
                    fontSize: '13px', fontFamily: 'JetBrains Mono, monospace' }}>
                    {row.failed}
                  </td>
                  <td style={{ padding: '10px 16px', color: '#8B949E', fontSize: '12px',
                    fontFamily: 'JetBrains Mono, monospace' }}>
                    {row.avg_duration_ms > 0 ? Math.round(row.avg_duration_ms) + 'ms' : '—'}
                  </td>
                  <td style={{ padding: '10px 16px', color: '#6E7681', fontSize: '11px',
                    fontFamily: 'JetBrains Mono, monospace' }}>
                    {row.last_active ? new Date(row.last_active).toLocaleTimeString() : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <style>{'@keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}} @keyframes pulse{0%,100%{opacity:1}50%{opacity:0.4}}'}</style>
    </div>
  )
}
