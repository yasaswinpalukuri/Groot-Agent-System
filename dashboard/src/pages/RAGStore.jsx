import { useState, useEffect } from 'react'
import { Database, RefreshCw } from 'lucide-react'

const API = 'http://groot:8000'

const COLLECTIONS = [
  { name: 'learning_notes',     access: ['groot', 'einstein', 'siva'],  max: 500, color: '#58A6FF' },
  { name: 'research_reports',   access: ['groot', 'einstein'],           max: 200, color: '#BC8CFF' },
  { name: 'job_descriptions',   access: ['groot', 'job_search', 'einstein'], max: 300, color: '#3FB950' },
  { name: 'applied_jobs',       access: ['groot', 'job_search'],         max: 100, color: '#D29922' },
  { name: 'documentation_cache',access: ['groot', 'tony', 'einstein'],   max: 200, color: '#F85149' },
  { name: 'code_patterns',      access: ['groot', 'tony'],               max: 200, color: '#8B949E' },
]

export default function RAGStore() {
  const [auditLog, setAuditLog] = useState([])
  const [loading, setLoading]   = useState(false)

  const fetchAudit = async () => {
    setLoading(true)
    try {
      const r = await fetch(`${API}/rag/audit`)
      const d = await r.json()
      setAuditLog(d.entries || [])
    } catch {}
    setLoading(false)
  }

  useEffect(() => { fetchAudit() }, [])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.08em' }}>
        RAG STORE — CHROMADB
      </div>

      {/* Collections */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '12px' }}>
        {COLLECTIONS.map(c => (
          <div key={c.name} style={{ background: '#161B22', border: '1px solid #30363D',
            borderRadius: '8px', padding: '16px', borderLeft: `3px solid ${c.color}` }}>
            <div style={{ color: '#E6EDF3', fontSize: '13px', fontFamily: 'JetBrains Mono, monospace',
              marginBottom: '8px' }}>{c.name}</div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginBottom: '10px' }}>
              {c.access.map(a => (
                <span key={a} style={{ padding: '1px 6px', borderRadius: '3px',
                  background: '#21262D', color: '#8B949E', fontSize: '10px',
                  fontFamily: 'JetBrains Mono, monospace' }}>{a}</span>
              ))}
            </div>
            <div style={{ height: '3px', background: '#21262D', borderRadius: '2px' }}>
              <div style={{ height: '100%', borderRadius: '2px', background: c.color, width: '2%' }} />
            </div>
            <div style={{ color: '#6E7681', fontSize: '10px', marginTop: '4px',
              fontFamily: 'JetBrains Mono, monospace' }}>0 / {c.max} docs</div>
          </div>
        ))}
      </div>

      {/* Config */}
      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', padding: '16px' }}>
        <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
          letterSpacing: '0.06em', marginBottom: '12px' }}>CONFIGURATION</div>
        <div style={{ display: 'flex', gap: '32px' }}>
          {[
            { label: 'CHUNK SIZE',    value: '800 tokens' },
            { label: 'OVERLAP',       value: '100 tokens' },
            { label: 'EMBEDDING',     value: 'nomic-embed-text' },
            { label: 'SEARCH MODE',   value: 'Hybrid BM25(40%) + Dense(60%)' },
            { label: 'RBAC',          value: 'agent_access metadata' },
            { label: 'AUDIT LOG',     value: '/var/log/groot/rag_audit.jsonl' },
          ].map(c => (
            <div key={c.label}>
              <div style={{ color: '#6E7681', fontSize: '10px', marginBottom: '4px' }}>{c.label}</div>
              <div style={{ color: '#E6EDF3', fontSize: '12px',
                fontFamily: 'JetBrains Mono, monospace' }}>{c.value}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
