import { useState, useEffect } from 'react'
import { ExternalLink, Clock, CheckCircle, XCircle, RefreshCw } from 'lucide-react'

const API = 'http://groot:8000'

const STATUSES = ['All', 'interested', 'applied', 'contacted', 'interview', 'offer', 'rejected']

function statusColor(s) {
  if (s === 'offer')     return { bg: '#1C2A1C', color: '#3FB950' }
  if (s === 'interview') return { bg: '#1C2F4A', color: '#58A6FF' }
  if (s === 'applied')   return { bg: '#2D2208', color: '#D29922' }
  if (s === 'contacted') return { bg: '#1C2F4A', color: '#BC8CFF' }
  if (s === 'interested') return { bg: '#21262D', color: '#8B949E' }
  return { bg: '#21262D', color: '#6E7681' }
}

function formatDate(dateStr) {
  if (!dateStr) return '—'
  try { return new Date(dateStr).toISOString().slice(0, 10) }
  catch { return dateStr.slice(0, 10) }
}

export default function Approvals() {
  const [jobs, setJobs]       = useState([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter]   = useState('All')
  const [error, setError]     = useState(null)

  const fetchTracker = async () => {
    setLoading(true)
    setError(null)
    try {
      const r = await fetch(`${API}/jobs/tracker`)
      const d = await r.json()
      setJobs(d.jobs || [])
    } catch {
      setError('Could not reach agent service')
    } finally {
      setLoading(false)
    }
  }

  const updateStatus = async (jobId, newStatus) => {
    try {
      await fetch(`${API}/jobs/tracker/${jobId}/status`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: newStatus }),
      })
      fetchTracker()
    } catch {}
  }

  useEffect(() => { fetchTracker() }, [])

  const filtered = filter === 'All' ? jobs : jobs.filter(j => j.status === filter)

  const stats = {
    total:     jobs.length,
    applied:   jobs.filter(j => ['applied','contacted','interview','offer'].includes(j.status)).length,
    interview: jobs.filter(j => j.status === 'interview').length,
    offer:     jobs.filter(j => j.status === 'offer').length,
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>

      {/* Stats */}
      <div style={{ display: 'flex', gap: '12px' }}>
        {[
          { label: 'TOTAL TRACKED', value: stats.total,     color: '#E6EDF3' },
          { label: 'APPLIED',        value: stats.applied,   color: '#D29922' },
          { label: 'INTERVIEWS',     value: stats.interview, color: '#58A6FF' },
          { label: 'OFFERS',         value: stats.offer,     color: '#3FB950' },
        ].map(s => (
          <div key={s.label} style={{
            flex: 1, background: '#161B22', border: '1px solid #30363D',
            borderRadius: '8px', padding: '16px',
          }}>
            <div style={{ color: '#6E7681', fontSize: '10px', fontWeight: 600,
              letterSpacing: '0.06em', marginBottom: '8px' }}>{s.label}</div>
            <div style={{ color: s.color, fontSize: '28px', fontWeight: 700,
              fontFamily: 'JetBrains Mono, monospace' }}>{s.value}</div>
          </div>
        ))}
      </div>

      {/* Filter */}
      <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap',
        background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', padding: '12px 16px', alignItems: 'center' }}>
        <span style={{ color: '#6E7681', fontSize: '10px', fontWeight: 600,
          letterSpacing: '0.06em', marginRight: '8px' }}>FILTER</span>
        {STATUSES.map(s => (
          <button key={s} onClick={() => setFilter(s)} style={{
            padding: '4px 10px', borderRadius: '20px',
            border: `1px solid ${filter === s ? '#58A6FF' : '#30363D'}`,
            background: filter === s ? '#1C2F4A' : 'transparent',
            color: filter === s ? '#58A6FF' : '#8B949E',
            cursor: 'pointer', fontSize: '11px',
          }}>{s}</button>
        ))}
        <button onClick={fetchTracker} style={{
          marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '4px',
          padding: '4px 10px', borderRadius: '6px', border: '1px solid #30363D',
          background: 'transparent', color: '#8B949E', cursor: 'pointer', fontSize: '11px',
        }}>
          <RefreshCw size={10} /> Refresh
        </button>
      </div>

      {/* Table */}
      <div style={{ background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ display: 'grid',
          gridTemplateColumns: '1.5fr 1.5fr 1fr 120px 110px 100px 32px',
          padding: '10px 16px', borderBottom: '1px solid #30363D' }}>
          {['COMPANY','ROLE','LOCATION','DATE','STATUS','UPDATE',''].map(h => (
            <span key={h} style={{ color: '#6E7681', fontSize: '10px',
              fontWeight: 600, letterSpacing: '0.06em' }}>{h}</span>
          ))}
        </div>

        {loading && (
          <div style={{ padding: '32px', textAlign: 'center',
            color: '#6E7681', fontSize: '13px' }}>
            Loading tracker...
          </div>
        )}

        {error && (
          <div style={{ padding: '32px', textAlign: 'center',
            color: '#F85149', fontSize: '13px' }}>{error}</div>
        )}

        {!loading && filtered.length === 0 && !error && (
          <div style={{ padding: '48px', textAlign: 'center' }}>
            <div style={{ color: '#6E7681', fontSize: '13px', marginBottom: '8px' }}>
              No jobs tracked yet
            </div>
            <div style={{ color: '#444C56', fontSize: '12px' }}>
              Click "Interested" on a job in the Job Search page
            </div>
          </div>
        )}

        {!loading && filtered.map((job, i) => {
          const sc = statusColor(job.status)
          return (
            <div key={i} style={{ display: 'grid',
              gridTemplateColumns: '1.5fr 1.5fr 1fr 120px 110px 100px 32px',
              padding: '12px 16px', borderBottom: '1px solid #21262D',
              alignItems: 'center' }}>
              <span style={{ color: '#E6EDF3', fontSize: '13px', fontWeight: 500 }}>
                {job.company}
              </span>
              <span style={{ color: '#8B949E', fontSize: '13px' }}>{job.role}</span>
              <span style={{ color: '#8B949E', fontSize: '12px' }}>{job.location}</span>
              <span style={{ color: '#6E7681', fontSize: '11px',
                fontFamily: 'JetBrains Mono, monospace' }}>
                {formatDate(job.date)}
              </span>
              <span style={{ fontSize: '11px', padding: '2px 8px', borderRadius: '4px',
                background: sc.bg, color: sc.color,
                fontFamily: 'JetBrains Mono, monospace', display: 'inline-block' }}>
                {job.status}
              </span>
              <select
                onChange={e => updateStatus(job.id, e.target.value)}
                defaultValue={job.status}
                style={{
                  background: '#21262D', border: '1px solid #30363D',
                  borderRadius: '4px', color: '#8B949E',
                  fontSize: '10px', padding: '3px 4px', cursor: 'pointer',
                }}>
                {STATUSES.filter(s => s !== 'All').map(s => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
              <a href={job.url} target="_blank" rel="noopener noreferrer">
                <ExternalLink size={12} color="#6E7681" />
              </a>
            </div>
          )
        })}
      </div>
    </div>
  )
}
