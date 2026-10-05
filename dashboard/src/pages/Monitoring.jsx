import { useState, useEffect } from 'react'
import { Activity, Cpu, HardDrive, Box } from 'lucide-react'

const API = 'http://groot:8000'

export default function Monitoring() {
  const [data, setData]       = useState({ ram_used_pct: 0, queue_depth: 0 })
  const [health, setHealth]   = useState('checking...')
  const [containers, setContainers] = useState([
    { name: 'agent_service', port: '8000', status: 'up' },
    { name: 'chromadb',      port: '8001', status: 'up' },
    { name: 'n8n',           port: '5678', status: 'up' },
    { name: 'dashboard',     port: '3000', status: 'up' },
  ])

  useEffect(() => {
    fetch(`${API}/health`).then(r => r.json())
      .then(d => setHealth(d.status)).catch(() => setHealth('error'))

    const ws = new WebSocket('ws://groot:8000/ws/live')
    ws.onmessage = e => setData(JSON.parse(e.data))
    return () => ws.close()
  }, [])

  const ramColor = data.ram_used_pct < 60 ? '#3FB950' :
                   data.ram_used_pct < 80 ? '#D29922' : '#F85149'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.08em' }}>
        SYSTEM MONITORING
      </div>

      {/* Live metrics */}
      <div style={{ display: 'flex', gap: '16px' }}>
        {[
          { icon: Cpu,      label: 'RAM USED',   value: data.ram_used_pct + '%', color: ramColor },
          { icon: Activity, label: 'QUEUE DEPTH', value: data.queue_depth,       color: '#E6EDF3' },
          { icon: Box,      label: 'API STATUS',  value: health, color: health === 'ok' ? '#3FB950' : '#F85149' },
          { icon: HardDrive,label: 'DISK',        value: '21%',  color: '#3FB950' },
        ].map(m => (
          <div key={m.label} style={{ flex: 1, background: '#161B22',
            border: '1px solid #30363D', borderRadius: '8px', padding: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
              <m.icon size={14} color="#8B949E" />
              <span style={{ color: '#8B949E', fontSize: '10px', fontWeight: 600, letterSpacing: '0.06em' }}>
                {m.label}
              </span>
            </div>
            <div style={{ color: m.color, fontSize: '22px', fontWeight: 700,
              fontFamily: 'JetBrains Mono, monospace' }}>{m.value}</div>
          </div>
        ))}
      </div>

      {/* Container status */}
      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D' }}>
          <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em' }}>
            DOCKER CONTAINERS
          </span>
        </div>
        {containers.map(c => (
          <div key={c.name} style={{ display: 'flex', alignItems: 'center', gap: '16px',
            padding: '12px 16px', borderBottom: '1px solid #21262D' }}>
            <div style={{ width: '8px', height: '8px', borderRadius: '50%', background: '#3FB950' }} />
            <span style={{ color: '#E6EDF3', fontSize: '13px', fontFamily: 'JetBrains Mono, monospace',
              flex: 1 }}>{c.name}</span>
            <span style={{ color: '#6E7681', fontSize: '11px', fontFamily: 'JetBrains Mono, monospace' }}>
              :{c.port}
            </span>
            <span style={{ fontSize: '10px', padding: '2px 8px', borderRadius: '4px',
              background: '#1C2A1C', color: '#3FB950', fontFamily: 'JetBrains Mono, monospace' }}>
              running
            </span>
          </div>
        ))}
      </div>

      {/* Ollama models */}
      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D' }}>
          <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em' }}>
            OLLAMA MODELS (7 LOADED)
          </span>
        </div>
        {[
          { name: 'qwen2.5:7b',        size: '4.7 GB', agents: 'Groot · Job Search' },
          { name: 'deepseek-r1:14b',   size: '9.0 GB', agents: 'Einstein' },
          { name: 'qwen2.5-coder:7b',  size: '4.7 GB', agents: 'Tony' },
          { name: 'phi3:medium',        size: '7.9 GB', agents: 'Siva' },
          { name: 'mistral:7b-instruct',size: '4.4 GB', agents: 'Career' },
          { name: 'llama3.2:3b',       size: '2.0 GB', agents: 'Routing' },
          { name: 'nomic-embed-text',  size: '274 MB', agents: 'RAG Embeddings' },
        ].map(m => (
          <div key={m.name} style={{ display: 'flex', alignItems: 'center', gap: '16px',
            padding: '10px 16px', borderBottom: '1px solid #21262D' }}>
            <span style={{ color: '#E6EDF3', fontSize: '12px', fontFamily: 'JetBrains Mono, monospace',
              flex: 1 }}>{m.name}</span>
            <span style={{ color: '#6E7681', fontSize: '11px', fontFamily: 'JetBrains Mono, monospace',
              width: '70px' }}>{m.size}</span>
            <span style={{ color: '#58A6FF', fontSize: '11px' }}>{m.agents}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
