import { useState, useEffect } from 'react'
import { Activity, Cpu, Database, Briefcase, Clock } from 'lucide-react'

const API = 'http://groot:8000'

const AGENTS = [
  { name: 'Groot',      model: 'qwen2.5:7b',       status: 'idle',   demo: true },
  { name: 'Einstein',   model: 'deepseek-r1:14b',   status: 'idle',   demo: true },
  { name: 'Tony',       model: 'qwen2.5-coder:7b',  status: 'idle',   demo: true },
  { name: 'Siva',       model: 'phi3:medium',        status: 'idle',   demo: true },
  { name: 'Job Search', model: 'qwen2.5:7b',        status: 'idle',   demo: true },
  { name: 'Career',     model: 'mistral:7b-instruct',status: 'idle',   demo: true },
  { name: 'Content',    model: '—',                  status: 'absent', demo: false },
  { name: 'Finance',    model: '—',                  status: 'absent', demo: false },
]

const RAG = [
  { name: 'learning_notes',    docs: 2, max: 500 },
  { name: 'research_reports',  docs: 0, max: 200 },
  { name: 'job_descriptions',  docs: 1, max: 300 },
  { name: 'applied_jobs',      docs: 0, max: 100 },
  { name: 'documentation_cache', docs: 0, max: 200 },
  { name: 'code_patterns',     docs: 0, max: 200 },
]

const PRIORITY_LABEL = { 0: 'URGENT', 1: 'HIGH', 2: 'MEDIUM', 3: 'LOW' }
const PRIORITY_COLOR = { 0: '#F85149', 1: '#D29922', 2: '#58A6FF', 3: '#8B949E' }

function StatCard({ icon: Icon, label, value, color }) {
  return (
    <div style={{ background: '#161B22', border: '1px solid #30363D',
      borderRadius: '8px', padding: '16px', flex: 1 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
        <Icon size={14} color="#8B949E" />
        <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em' }}>
          {label}
        </span>
      </div>
      <div style={{ color: color || '#E6EDF3', fontSize: '22px', fontWeight: 700,
        fontFamily: 'JetBrains Mono, monospace' }}>{value}</div>
    </div>
  )
}

function AgentRow({ agent }) {
  const dotColor = agent.status === 'idle' ? '#3FB950' :
                   agent.status === 'absent' ? '#444C56' : '#D29922'
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '12px',
      padding: '10px 12px', borderBottom: '1px solid #21262D' }}>
      <div style={{ width: '8px', height: '8px', borderRadius: '50%',
        background: dotColor, flexShrink: 0 }} />
      <span style={{ color: agent.status === 'absent' ? '#6E7681' : '#E6EDF3',
        fontSize: '13px', width: '110px' }}>{agent.name}</span>
      <span style={{ color: '#6E7681', fontSize: '11px',
        fontFamily: 'JetBrains Mono, monospace', flex: 1 }}>{agent.model}</span>
      <span style={{
        fontSize: '10px', padding: '2px 8px', borderRadius: '4px',
        background: agent.status === 'absent' ? '#21262D' : '#1C2A1C',
        color: agent.status === 'absent' ? '#6E7681' : '#3FB950',
        fontFamily: 'JetBrains Mono, monospace',
      }}>
        {agent.status === 'absent' ? 'production only' : agent.status}
      </span>
    </div>
  )
}

export default function Overview() {
  const [scheduler, setScheduler] = useState({ queue_depth: 0, status: 'idle' })
  const [ram, setRam]             = useState(0)
  const [health, setHealth]       = useState('checking...')
  const [queue, setQueue]         = useState([])
  const [queueHistory, setQueueHistory] = useState([])

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [s, h, q] = await Promise.all([
          fetch(`${API}/scheduler/status`).then(r => r.json()),
          fetch(`${API}/health`).then(r => r.json()),
          fetch(`${API}/scheduler/queue`).then(r => r.json()),
        ])
        setScheduler(s)
        setHealth(h.status)
        setQueue(q.tasks || [])
      } catch { setHealth('error') }
    }
    fetchData()
    const queueInterval = setInterval(async () => {
      try {
        const q = await fetch(`${API}/scheduler/queue`).then(r => r.json())
        const tasks = []
        if (q.current_task) tasks.push({ ...q.current_task, is_current: true })
        tasks.push(...(q.tasks || []))
        setQueue(tasks)
        setScheduler(s => ({ ...s, queue_depth: q.queue_depth }))
      } catch {}
    }, 3000)

    const ws = new WebSocket(`ws://groot:8000/ws/live`)
    ws.onmessage = e => {
      const d = JSON.parse(e.data)
      setRam(d.ram_used_pct)
      setScheduler(s => ({ ...s, queue_depth: d.queue_depth }))

      // Track queue history for the mini chart
      setQueueHistory(h => {
        const next = [...h, { time: new Date().toLocaleTimeString(), depth: d.queue_depth }]
        return next.slice(-20)
      })
    }
    return () => { ws.close(); clearInterval(queueInterval) }
  }, [])

  const ramColor = ram < 60 ? '#3FB950' : ram < 80 ? '#D29922' : '#F85149'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>

      {/* Stat cards */}
      <div style={{ display: 'flex', gap: '16px' }}>
        <StatCard icon={Activity}  label="QUEUE DEPTH"     value={scheduler.queue_depth} />
        <StatCard icon={Cpu}       label="RAM USED"        value={`${ram}%`} color={ramColor} />
        <StatCard icon={Database}  label="RAG COLLECTIONS" value="6" />
        <StatCard icon={Briefcase} label="API STATUS"      value={health}
          color={health === 'ok' ? '#3FB950' : '#F85149'} />
      </div>

      {/* Agent status + RAG */}
      <div style={{ display: 'flex', gap: '16px' }}>
        <div style={{ flex: 2, background: '#161B22', border: '1px solid #30363D',
          borderRadius: '8px', overflow: 'hidden' }}>
          <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D' }}>
            <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
              letterSpacing: '0.06em' }}>AGENT STATUS · 6 OF 8 ACTIVE</span>
          </div>
          {AGENTS.map(a => <AgentRow key={a.name} agent={a} />)}
        </div>

        <div style={{ flex: 1, background: '#161B22', border: '1px solid #30363D',
          borderRadius: '8px', overflow: 'hidden' }}>
          <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D' }}>
            <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
              letterSpacing: '0.06em' }}>RAG STORE</span>
          </div>
          <div style={{ padding: '16px' }}>
            {RAG.map(r => (
              <div key={r.name} style={{ marginBottom: '14px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '5px' }}>
                  <span style={{ color: '#E6EDF3', fontSize: '11px',
                    fontFamily: 'JetBrains Mono, monospace' }}>{r.name}</span>
                  <span style={{ color: '#6E7681', fontSize: '10px',
                    fontFamily: 'JetBrains Mono, monospace' }}>{r.docs}/{r.max}</span>
                </div>
                <div style={{ height: '3px', background: '#21262D', borderRadius: '2px' }}>
                  <div style={{ height: '100%', borderRadius: '2px', background: '#58A6FF',
                    width: `${Math.max(2, (r.docs / r.max) * 100)}%` }} />
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Load Scheduler + Queue */}
      <div style={{ display: 'flex', gap: '16px' }}>

        {/* Scheduler stats */}
        <div style={{ flex: 1, background: '#161B22', border: '1px solid #30363D',
          borderRadius: '8px', padding: '16px' }}>
          <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
            letterSpacing: '0.06em', marginBottom: '16px' }}>LOAD SCHEDULER</div>
          <div style={{ display: 'flex', gap: '24px', alignItems: 'center', flexWrap: 'wrap' }}>
            <div>
              <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '4px' }}>STATUS</div>
              <div style={{ color: scheduler.status === 'idle' ? '#3FB950' : '#D29922',
                fontSize: '13px', fontFamily: 'JetBrains Mono, monospace' }}>
                {scheduler.status.toUpperCase()}
              </div>
            </div>
            <div>
              <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '4px' }}>QUEUE</div>
              <div style={{ color: '#E6EDF3', fontSize: '28px', fontWeight: 700,
                fontFamily: 'JetBrains Mono, monospace' }}>{scheduler.queue_depth}</div>
            </div>
            <div>
              <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '4px' }}>RAM SAFETY</div>
              <div style={{ color: '#E6EDF3', fontSize: '13px',
                fontFamily: 'JetBrains Mono, monospace' }}>3 GB reserved</div>
            </div>
            <div>
              <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '4px' }}>MAX LARGE MODELS</div>
              <div style={{ color: '#E6EDF3', fontSize: '13px',
                fontFamily: 'JetBrains Mono, monospace' }}>1 at a time</div>
            </div>
          </div>

          {/* Queue depth history sparkline */}
          {queueHistory.length > 1 && (
            <div style={{ marginTop: '16px' }}>
              <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '6px' }}>
                QUEUE DEPTH (last 20 readings)
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-end', gap: '2px', height: '32px' }}>
                {queueHistory.map((h, i) => (
                  <div key={i} style={{
                    flex: 1, background: h.depth > 0 ? '#58A6FF' : '#21262D',
                    height: `${Math.max(4, (h.depth / Math.max(...queueHistory.map(x => x.depth), 1)) * 32)}px`,
                    borderRadius: '2px', transition: 'height 0.3s ease',
                  }} title={`${h.time}: ${h.depth}`} />
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Live queue list */}
        <div style={{ flex: 1, background: '#161B22', border: '1px solid #30363D',
          borderRadius: '8px', overflow: 'hidden' }}>
          <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D',
            display: 'flex', alignItems: 'center', gap: '8px' }}>
            <Clock size={12} color="#8B949E" />
            <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
              letterSpacing: '0.06em' }}>PENDING QUEUE</span>
            <span style={{ marginLeft: 'auto', color: '#6E7681', fontSize: '10px',
              fontFamily: 'JetBrains Mono, monospace' }}>
              {scheduler.queue_depth} task{scheduler.queue_depth !== 1 ? 's' : ''}
            </span>
          </div>

          {scheduler.queue_depth === 0 ? (
            <div style={{ padding: '32px', textAlign: 'center' }}>
              <div style={{ color: '#3FB950', fontSize: '12px',
                fontFamily: 'JetBrains Mono, monospace', marginBottom: '4px' }}>● IDLE</div>
              <div style={{ color: '#6E7681', fontSize: '11px' }}>No tasks in queue</div>
            </div>
          ) : (
            <div style={{ padding: '8px' }}>
              {queue.length === 0 ? (
                Array.from({ length: scheduler.queue_depth }).map((_, i) => (
                  <div key={i} style={{ padding: '10px 12px', borderRadius: '6px',
                    background: '#21262D', marginBottom: '6px',
                    display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <div style={{ width: '6px', height: '6px', borderRadius: '50%',
                      background: '#D29922' }} />
                    <span style={{ color: '#8B949E', fontSize: '12px' }}>
                      Task {i + 1} — processing...
                    </span>
                    <span style={{ marginLeft: 'auto', color: '#D29922', fontSize: '10px',
                      fontFamily: 'JetBrains Mono, monospace' }}>QUEUED</span>
                  </div>
                ))
              ) : (
                queue.map((task, i) => (
                  <div key={i} style={{ padding: '10px 12px', borderRadius: '6px',
                    background: '#21262D', marginBottom: '6px',
                    display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <div style={{ width: '6px', height: '6px', borderRadius: '50%',
                      background: PRIORITY_COLOR[task.priority] || '#8B949E' }} />
                    <div style={{ flex: 1 }}>
                      <div style={{ color: '#E6EDF3', fontSize: '12px' }}>
                        {task.agent} — {task.task?.slice(0, 40)}...
                      </div>
                    </div>
                    <span style={{ color: PRIORITY_COLOR[task.priority],
                      fontSize: '10px', fontFamily: 'JetBrains Mono, monospace' }}>
                      {PRIORITY_LABEL[task.priority] || 'MEDIUM'}
                    </span>
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      </div>

      {/* Eval scores */}
      <div style={{ background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', padding: '16px' }}>
        <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
          letterSpacing: '0.06em', marginBottom: '12px' }}>RAGAS EVALUATION</div>
        <div style={{ display: 'flex', gap: '32px' }}>
          {[
            { label: 'FAITHFULNESS',     value: 1.000, color: '#3FB950' },
            { label: 'ANSWER RELEVANCY', value: 0.676, color: '#58A6FF' },
            { label: 'OVERALL',          value: 0.838, color: '#D29922' },
          ].map(m => (
            <div key={m.label}>
              <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '4px' }}>{m.label}</div>
              <div style={{ color: m.color, fontSize: '22px', fontWeight: 700,
                fontFamily: 'JetBrains Mono, monospace' }}>{m.value.toFixed(3)}</div>
              <div style={{ marginTop: '6px', height: '3px', background: '#21262D',
                borderRadius: '2px', width: '80px' }}>
                <div style={{ height: '100%', borderRadius: '2px', background: m.color,
                  width: `${m.value * 100}%` }} />
              </div>
            </div>
          ))}
          <div style={{ marginLeft: 'auto', alignSelf: 'center' }}>
            <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '4px' }}>JUDGE MODEL</div>
            <div style={{ color: '#8B949E', fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace' }}>qwen2.5:7b</div>
            <div style={{ color: '#6E7681', fontSize: '10px', marginTop: '8px', marginBottom: '4px' }}>GUARDRAILS</div>
            <div style={{ color: '#3FB950', fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace' }}>active</div>
          </div>
        </div>
      </div>

    </div>
  )
}
