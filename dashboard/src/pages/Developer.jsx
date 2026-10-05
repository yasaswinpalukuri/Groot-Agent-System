import { useState, useEffect } from 'react'
import { CheckCircle, XCircle, GitMerge, ExternalLink, GitCommit, RefreshCw } from 'lucide-react'

const GITHUB_USER = 'yasaswinpalukuri'
const GROOT_REPO  = 'Groot-Agent-System'

const PROJECTS = [
  {
    name: 'Portfolio Website',
    url: 'https://yasaswinpalukuri.github.io',
    stack: ['HTML', 'CSS', 'JavaScript'],
    status: 'complete',
    desc: 'Personal portfolio — dark theme, all projects, live at yasaswinpalukuri.github.io',
  },
  {
    name: 'Groot Multi-Agent System',
    url: 'https://github.com/yasaswinpalukuri/Groot-Agent-System',
    stack: ['Python', 'FastAPI', 'LangGraph', 'Ollama', 'Docker'],
    status: 'active',
    desc: 'Self-hosted 6-agent AI OS running 24/7 on bare metal',
  },
  {
    name: 'Multi-Turn Conversational Data Assistant',
    url: 'https://github.com/yasaswinpalukuri/Multi-Turn-Conversational-Data-Assistant',
    stack: ['LangChain', 'ChromaDB', 'Streamlit', 'Ollama', 'Pandas'],
    status: 'complete',
    desc: 'Natural language queries on NYC Taxi dataset. 74 passing tests + CI.',
  },
  {
    name: 'LLM-Powered Resume Screener',
    url: 'https://github.com/yasaswinpalukuri/LLM-Powered-Resume-Screener',
    stack: ['Python', 'LangChain', 'FastAPI', 'Pydantic'],
    status: 'complete',
    desc: 'Scores and ranks resumes against job descriptions using structured LLM outputs.',
  },
  {
    name: 'Structured Data Extractor',
    url: 'https://github.com/yasaswinpalukuri/Structured-Data-Extractor',
    stack: ['Python', 'Pydantic', 'Notion API', 'LangChain'],
    status: 'complete',
    desc: 'Multi-source document parser with Notion integration. Classifier delivered.',
  },
  {
    name: 'LLM Eval / Red-Teaming Framework',
    url: 'https://github.com/yasaswinpalukuri',
    stack: ['DeepEval', 'Ragas', 'Llama Guard', 'Python'],
    status: 'planned',
    desc: 'Automated red-teaming and hallucination detection pipeline.',
  },
  {
    name: 'Text-to-SQL (qwen2.5-coder)',
    url: 'https://github.com/yasaswinpalukuri',
    stack: ['Python', 'SQLite', 'FastAPI', 'qwen2.5-coder:7b'],
    status: 'planned',
    desc: 'Natural language to SQL using local LLM. Targets Data Engineer JDs.',
  },
]

function statusColor(s) {
  if (s === 'active')   return '#3FB950'
  if (s === 'building') return '#D29922'
  if (s === 'complete') return '#58A6FF'
  return '#6E7681'
}

function Check({ ok, label }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
      {ok ? <CheckCircle size={12} color="#3FB950" /> : <XCircle size={12} color="#F85149" />}
      <span style={{ color: ok ? '#3FB950' : '#F85149', fontSize: '10px',
        fontFamily: 'JetBrains Mono, monospace' }}>{label}</span>
    </div>
  )
}

export default function Developer() {
  const [commits, setCommits]   = useState([])
  const [loading, setLoading]   = useState(true)
  const [ciStatus, setCiStatus] = useState({ ruff: true, bandit: true, pytest: true, score: 87 })

  useEffect(() => {
    const fetchCommits = async () => {
      try {
        const r = await fetch(
          `https://api.github.com/repos/${GITHUB_USER}/${GROOT_REPO}/commits?per_page=5`
        )
        const data = await r.json()
        if (Array.isArray(data)) {
          setCommits(data.map(c => ({
            sha:     c.sha?.slice(0, 7),
            message: c.commit?.message?.split('\n')[0] || '',
            author:  c.commit?.author?.name || '',
            date:    c.commit?.author?.date?.slice(0, 10) || '',
            url:     c.html_url,
          })))
        }
      } catch {}
      finally { setLoading(false) }
    }
    fetchCommits()
  }, [])

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>

      {/* Tony's recent commits */}
      <div style={{ background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D',
          display: 'flex', alignItems: 'center', gap: '8px' }}>
          <GitCommit size={14} color="#8B949E" />
          <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
            letterSpacing: '0.06em' }}>TONY'S RECENT COMMITS — {GROOT_REPO}</span>
          <a href={`https://github.com/${GITHUB_USER}/${GROOT_REPO}`}
            target="_blank" rel="noopener noreferrer" style={{ marginLeft: 'auto' }}>
            <ExternalLink size={12} color="#6E7681" />
          </a>
        </div>
        {loading && (
          <div style={{ padding: '24px', textAlign: 'center', color: '#6E7681', fontSize: '12px' }}>
            Loading commits from GitHub...
          </div>
        )}
        {!loading && commits.map((c, i) => (
          <div key={i} style={{ padding: '12px 16px', borderBottom: '1px solid #21262D',
            display: 'flex', alignItems: 'center', gap: '12px' }}>
            <span style={{ color: '#6E7681', fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace', flexShrink: 0 }}>{c.sha}</span>
            <span style={{ color: '#E6EDF3', fontSize: '13px', flex: 1 }}>{c.message}</span>
            <span style={{ color: '#6E7681', fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace', flexShrink: 0 }}>{c.date}</span>
            <a href={c.url} target="_blank" rel="noopener noreferrer">
              <ExternalLink size={12} color="#6E7681" />
            </a>
          </div>
        ))}
        {!loading && commits.length === 0 && (
          <div style={{ padding: '24px', textAlign: 'center', color: '#6E7681', fontSize: '12px' }}>
            No commits found
          </div>
        )}
      </div>

      {/* CI gate status */}
      <div style={{ background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', overflow: 'hidden' }}>
        <div style={{ padding: '12px 16px', borderBottom: '1px solid #30363D',
          display: 'flex', alignItems: 'center', gap: '8px' }}>
          <GitMerge size={14} color="#8B949E" />
          <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
            letterSpacing: '0.06em' }}>CI GATE STATUS</span>
        </div>
        <div style={{ padding: '16px', display: 'flex', gap: '24px', alignItems: 'center' }}>
          <Check ok={ciStatus.ruff}   label="ruff" />
          <Check ok={ciStatus.bandit} label="bandit" />
          <Check ok={ciStatus.pytest} label="pytest (3 tests)" />
          <div style={{ marginLeft: 'auto', padding: '4px 12px', borderRadius: '4px',
            background: '#1C2A1C', color: '#3FB950', fontSize: '12px',
            fontFamily: 'JetBrains Mono, monospace', fontWeight: 700 }}>
            {ciStatus.score}/100
          </div>
          <div style={{ padding: '6px 14px', borderRadius: '6px',
            background: '#58A6FF', color: '#fff', fontSize: '12px' }}>
            CI Passing
          </div>
        </div>
      </div>

      {/* Portfolio projects */}
      <div>
        <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
          letterSpacing: '0.06em', marginBottom: '12px' }}>PORTFOLIO PROJECTS</div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '12px' }}>
          {PROJECTS.map(p => (
            <div key={p.name} style={{
              background: '#161B22', border: '1px solid #30363D',
              borderRadius: '8px', padding: '16px',
              borderLeft: `3px solid ${statusColor(p.status)}`,
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between',
                alignItems: 'flex-start', marginBottom: '6px' }}>
                <span style={{ color: '#E6EDF3', fontSize: '13px', fontWeight: 500 }}>{p.name}</span>
                <a href={p.url} target="_blank" rel="noopener noreferrer">
                  <ExternalLink size={12} color="#6E7681" style={{ cursor: 'pointer', flexShrink: 0 }} />
                </a>
              </div>
              <p style={{ color: '#6E7681', fontSize: '11px', marginBottom: '10px', lineHeight: '1.5' }}>
                {p.desc}
              </p>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', marginBottom: '10px' }}>
                {p.stack.map(t => (
                  <span key={t} style={{ padding: '2px 8px', borderRadius: '4px',
                    background: '#21262D', color: '#8B949E',
                    fontSize: '10px', fontFamily: 'JetBrains Mono, monospace' }}>{t}</span>
                ))}
              </div>
              <span style={{ fontSize: '10px', padding: '2px 8px', borderRadius: '4px',
                background: '#21262D', color: statusColor(p.status),
                fontFamily: 'JetBrains Mono, monospace' }}>{p.status}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
