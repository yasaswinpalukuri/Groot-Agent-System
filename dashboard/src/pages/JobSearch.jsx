import { useState, useEffect } from 'react'
import { ChevronDown, ChevronUp, ExternalLink, RefreshCw, Send, Users } from 'lucide-react'

const API = 'http://groot:8000'

const LOCATIONS = ['Canada', 'British Columbia', 'Alberta', 'New Brunswick', 'Newfoundland and Labrador', 'Ontario']
const ROLES     = ['Data Engineer', 'ML Engineer', 'AI Engineer', 'Data Scientist', 'LLMOps Engineer']

function scoreJob(job) {
  let score = 5
  const title = job.title.toLowerCase()
  if (title.includes('senior')) score += 1
  if (title.includes('lead'))   score += 1
  if (job.salary !== 'Not specified') score += 1
  if (title.includes('ml') || title.includes('ai') || title.includes('machine learning')) score += 1
  if (title.includes('data engineer')) score += 1
  return Math.min(score, 10)
}

function scoreColor(s) {
  if (s >= 9) return '#3FB950'
  if (s >= 7) return '#58A6FF'
  if (s >= 5) return '#D29922'
  return '#6E7681'
}

function formatDate(dateStr) {
  if (!dateStr) return '—'
  return new Date(dateStr).toISOString().slice(0, 10)
}

export default function JobSearch() {
  const [jobs, setJobs]           = useState([])
  const [loading, setLoading]     = useState(false)
  const [applying, setApplying]   = useState(null)
  const [applied, setApplied]     = useState({})
  const [finding, setFinding]     = useState(null)
  const [found, setFound]         = useState({})
  const [expanded, setExpanded]   = useState(null)
  const [role, setRole]           = useState('Data Engineer')
  const [location, setLocation]   = useState('Canada')
  const [error, setError]         = useState(null)

  const loadTrackerState = async (jobList) => {
    try {
      const r = await fetch(`${API}/jobs/tracker`)
      const d = await r.json()
      const trackedCompanies = new Set(
        (d.jobs || []).map(j => j.company.toLowerCase().trim())
      )
      const newApplied = {}
      jobList.forEach((job, i) => {
        if (trackedCompanies.has(job.company.toLowerCase().trim())) {
          newApplied[i] = 'applied'
        }
      })
      setApplied(newApplied)
    } catch {}
  }

  const fetchJobs = async () => {
    setLoading(true)
    setError(null)
    try {
      const r = await fetch(`${API}/jobs/search?title=${encodeURIComponent(role)}&location=${encodeURIComponent(location)}`)
      const d = await r.json()
      const jobList = (d.jobs || []).map(j => ({ ...j, score: scoreJob(j) }))
      setJobs(jobList)
      await loadTrackerState(jobList)
    } catch {
      setError('Could not reach agent service')
    } finally {
      setLoading(false)
    }
  }

  const handleApply = async (job, i) => {
    setApplying(i)
    try {
      const r = await fetch(`${API}/jobs/apply`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(job),
      })
      const d = await r.json()
      setApplied(a => ({ ...a, [i]: d.status || 'applied' }))
    } catch {
      setApplied(a => ({ ...a, [i]: 'error' }))
    } finally {
      setApplying(null)
    }
  }

  const handleFindContacts = async (job, i) => {
    setFinding(i)
    try {
      await fetch(`${API}/career/outreach`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ company: job.company, role: job.title }),
      })
      setFound(f => ({ ...f, [i]: 'sent' }))
    } catch {
      setFound(f => ({ ...f, [i]: 'error' }))
    } finally {
      setFinding(null)
    }
  }

  useEffect(() => { fetchJobs() }, [])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>

      {/* Search bar */}
      <div style={{ display: 'flex', gap: '12px', alignItems: 'center', flexWrap: 'wrap',
        background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', padding: '12px 16px' }}>
        <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
          {ROLES.map(r => (
            <button key={r} onClick={() => setRole(r)} style={{
              padding: '4px 10px', borderRadius: '20px',
              border: `1px solid ${role === r ? '#58A6FF' : '#30363D'}`,
              background: role === r ? '#1C2F4A' : 'transparent',
              color: role === r ? '#58A6FF' : '#8B949E',
              cursor: 'pointer', fontSize: '11px',
            }}>{r}</button>
          ))}
        </div>
        <div style={{ display: 'flex', gap: '6px' }}>
          {LOCATIONS.map(l => (
            <button key={l} onClick={() => setLocation(l)} style={{
              padding: '4px 10px', borderRadius: '20px',
              border: `1px solid ${location === l ? '#3FB950' : '#30363D'}`,
              background: location === l ? '#1C2A1C' : 'transparent',
              color: location === l ? '#3FB950' : '#8B949E',
              cursor: 'pointer', fontSize: '11px',
            }}>{l}</button>
          ))}
        </div>
        <button onClick={fetchJobs} disabled={loading} style={{
          display: 'flex', alignItems: 'center', gap: '6px',
          padding: '6px 14px', borderRadius: '6px',
          background: '#58A6FF', border: 'none', color: '#fff',
          cursor: loading ? 'not-allowed' : 'pointer', fontSize: '12px',
          opacity: loading ? 0.6 : 1, marginLeft: 'auto',
        }}>
          <RefreshCw size={12} style={{ animation: loading ? 'spin 1s linear infinite' : 'none' }} />
          {loading ? 'Searching...' : 'Search'}
        </button>
      </div>

      {error && (
        <div style={{ background: '#2D1A1A', border: '1px solid #F85149',
          borderRadius: '8px', padding: '12px 16px', color: '#F85149', fontSize: '13px' }}>
          {error}
        </div>
      )}

      {/* Results table */}
      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ display: 'grid',
          gridTemplateColumns: '1.4fr 1.2fr 0.8fr 50px 0.9fr 90px 100px 90px 24px',
          padding: '10px 16px', borderBottom: '1px solid #30363D' }}>
          {['COMPANY','ROLE','LOCATION','SCORE','SALARY','DATE','APPLY','CONTACTS',''].map(h => (
            <span key={h} style={{ color: '#6E7681', fontSize: '10px',
              fontWeight: 600, letterSpacing: '0.06em' }}>{h}</span>
          ))}
        </div>

        {loading && (
          <div style={{ padding: '32px', textAlign: 'center', color: '#6E7681', fontSize: '13px' }}>
            Searching Adzuna Canada...
          </div>
        )}

        {!loading && jobs.map((job, i) => {
          const isOpen     = expanded === i
          const isApplied  = applied[i]
          const isApplying = applying === i
          const isFound    = found[i]
          const isFinding  = finding === i
          return (
            <div key={i}>
              <div style={{ display: 'grid',
                gridTemplateColumns: '1.4fr 1.2fr 0.8fr 50px 0.9fr 90px 100px 90px 24px',
                padding: '10px 16px', borderBottom: '1px solid #21262D', alignItems: 'center' }}>
                <span style={{ color: '#E6EDF3', fontSize: '13px', fontWeight: 500 }}>{job.company}</span>
                <span style={{ color: '#8B949E', fontSize: '12px' }}>{job.title}</span>
                <span style={{ color: '#8B949E', fontSize: '11px' }}>{job.location?.split(',')[0]}</span>
                <span style={{ color: scoreColor(job.score), fontSize: '14px',
                  fontWeight: 700, fontFamily: 'JetBrains Mono, monospace' }}>{job.score}</span>
                <span style={{ color: '#8B949E', fontSize: '11px',
                  fontFamily: 'JetBrains Mono, monospace' }}>{job.salary}</span>
                <span style={{ color: '#6E7681', fontSize: '11px',
                  fontFamily: 'JetBrains Mono, monospace' }}>{formatDate(job.date)}</span>

                {/* Apply button */}
                <button onClick={() => !isApplied && !isApplying && handleApply(job, i)}
                  disabled={!!isApplied || isApplying} style={{
                    display: 'flex', alignItems: 'center', gap: '4px',
                    padding: '4px 8px', borderRadius: '6px', border: 'none',
                    cursor: isApplied || isApplying ? 'not-allowed' : 'pointer',
                    fontSize: '10px', fontFamily: 'Inter, sans-serif',
                    background: isApplied === 'applied' ? '#1C2A1C' : isApplying ? '#21262D' : '#1C2F4A',
                    color: isApplied === 'applied' ? '#3FB950' : isApplying ? '#6E7681' : '#58A6FF',
                  }}>
                  <Send size={9} />
                  {isApplied === 'applied' ? '✓ Sent' : isApplying ? 'Wait...' : 'Interested'}
                </button>

                {/* Find Contacts button */}
                <button onClick={() => !isFound && !isFinding && handleFindContacts(job, i)}
                  disabled={!!isFound || isFinding} style={{
                    display: 'flex', alignItems: 'center', gap: '4px',
                    padding: '4px 8px', borderRadius: '6px', border: 'none',
                    cursor: isFound || isFinding ? 'not-allowed' : 'pointer',
                    fontSize: '10px', fontFamily: 'Inter, sans-serif',
                    background: isFound === 'sent' ? '#1C2A1C' : isFinding ? '#21262D' : '#21262D',
                    color: isFound === 'sent' ? '#3FB950' : isFinding ? '#6E7681' : '#BC8CFF',
                  }}>
                  <Users size={9} />
                  {isFound === 'sent' ? '✓ Found' : isFinding ? 'Wait...' : 'Contacts'}
                </button>

                <div onClick={() => setExpanded(isOpen ? null : i)} style={{ cursor: 'pointer' }}>
                  {isOpen ? <ChevronUp size={14} color="#6E7681" /> : <ChevronDown size={14} color="#6E7681" />}
                </div>
              </div>

              {isOpen && (
                <div style={{ padding: '16px 24px', background: '#1A1F27',
                  borderBottom: '1px solid #30363D', display: 'flex', gap: '16px', alignItems: 'flex-start' }}>
                  <div style={{ flex: 1 }}>
                    <div style={{ color: '#8B949E', fontSize: '10px', fontWeight: 600,
                      letterSpacing: '0.06em', marginBottom: '6px' }}>CATEGORY</div>
                    <div style={{ color: '#E6EDF3', fontSize: '13px', marginBottom: '12px' }}>{job.category}</div>
                    <div style={{ color: '#8B949E', fontSize: '10px', fontWeight: 600,
                      letterSpacing: '0.06em', marginBottom: '6px' }}>LOCATION</div>
                    <div style={{ color: '#E6EDF3', fontSize: '13px' }}>{job.location}</div>
                  </div>
                  <a href={job.url} target="_blank" rel="noopener noreferrer" style={{
                    display: 'flex', alignItems: 'center', gap: '6px',
                    padding: '8px 16px', borderRadius: '6px',
                    background: '#58A6FF', color: '#fff',
                    textDecoration: 'none', fontSize: '12px', flexShrink: 0,
                  }}>
                    <ExternalLink size={12} />
                    View on Adzuna
                  </a>
                </div>
              )}
            </div>
          )
        })}

        {!loading && jobs.length === 0 && !error && (
          <div style={{ padding: '32px', textAlign: 'center', color: '#6E7681', fontSize: '13px' }}>
            No jobs found. Try a different role or location.
          </div>
        )}
      </div>
      <style>{`@keyframes spin { from{transform:rotate(0deg)} to{transform:rotate(360deg)} }`}</style>
    </div>
  )
}
