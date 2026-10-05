import { useState } from 'react'

const Section = ({ title, children, defaultOpen = false }) => {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div style={{ border: '1px solid #30363D', borderRadius: '8px', overflow: 'hidden', marginBottom: '12px' }}>
      <button onClick={() => setOpen(!open)} style={{
        width: '100%', padding: '14px 20px', background: '#161B22',
        border: 'none', cursor: 'pointer', display: 'flex',
        justifyContent: 'space-between', alignItems: 'center',
        color: '#E6EDF3', fontSize: '14px', fontWeight: 600,
      }}>
        <span>{title}</span>
        <span style={{ color: '#6E7681', fontSize: '12px' }}>{open ? '▲' : '▼'}</span>
      </button>
      {open && (
        <div style={{ padding: '16px 20px', background: '#0D1117', borderTop: '1px solid #30363D' }}>
          {children}
        </div>
      )}
    </div>
  )
}

const Table = ({ headers, rows }) => (
  <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '13px', marginBottom: '8px' }}>
    <thead>
      <tr>
        {headers.map(h => (
          <th key={h} style={{ padding: '8px 12px', textAlign: 'left', color: '#6E7681',
            fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em',
            borderBottom: '1px solid #21262D' }}>{h}</th>
        ))}
      </tr>
    </thead>
    <tbody>
      {rows.map((row, i) => (
        <tr key={i} style={{ borderBottom: '1px solid #21262D' }}>
          {row.map((cell, j) => (
            <td key={j} style={{ padding: '8px 12px', color: j === 0 ? '#E6EDF3' : '#8B949E',
              fontFamily: j === 0 ? 'inherit' : 'JetBrains Mono, monospace', fontSize: '12px' }}>
              {cell}
            </td>
          ))}
        </tr>
      ))}
    </tbody>
  </table>
)

const Code = ({ children }) => (
  <pre style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '6px',
    padding: '12px 16px', fontFamily: 'JetBrains Mono, monospace', fontSize: '12px',
    color: '#58A6FF', overflowX: 'auto', marginBottom: '8px', whiteSpace: 'pre-wrap' }}>
    {children}
  </pre>
)

const Badge = ({ label, color = '#3FB950' }) => (
  <span style={{ display: 'inline-flex', alignItems: 'center', gap: '5px',
    padding: '2px 8px', borderRadius: '4px', fontSize: '11px', fontWeight: 600,
    background: color + '22', color, border: `1px solid ${color}44` }}>
    {label}
  </span>
)

export default function Operations() {
  return (
    <div style={{ padding: '24px', maxWidth: '900px', margin: '0 auto' }}>
      <div style={{ marginBottom: '24px' }}>
        <h1 style={{ fontSize: '24px', fontWeight: 700, color: '#E6EDF3', marginBottom: '4px' }}>
          Operations Guide
        </h1>
        <p style={{ color: '#6E7681', fontSize: '13px' }}>
          Everything you need to run, monitor, and interact with Groot OS
        </p>
      </div>

      <Section title="📅 Daily Automated Schedule" defaultOpen={true}>
        <Table
          headers={['Time', 'What happens', 'Where to see it']}
          rows={[
            ['5 AM UTC (midnight EST)', 'Tony updates portfolio uptime on GitHub Pages', '/var/log/groot/portfolio_update.log'],
            ['9 AM EST', 'Daily AI Digest — Einstein summarizes tech news', 'Slack #groot + Telegram'],
            ['9 AM EST', 'Approval reminder (pending tasks)', 'Slack #approvals + Telegram'],
            ['8 AM + 8 PM EST', 'Top 10 Jobs from Adzuna Canada', 'Slack #job-search + Telegram'],
            ['2 PM EST', 'Approval reminder', 'Slack #approvals + Telegram'],
            ['7 PM EST', 'Approval reminder', 'Slack #approvals + Telegram'],
            ['11 PM EST', 'Groot writes nightly diary to Notion', 'Notion + Slack + Telegram'],
            ['Every hour', 'Supervisor loop runs checklist', 'Slack #groot'],
            ['Every minute', 'Groot Inbox polls Telegram', 'Telegram'],
          ]}
        />
        <p style={{ color: '#6E7681', fontSize: '12px', marginTop: '8px' }}>
          Hourly checklist: job-search-daily · research-ai-companies (24h) · llmops-trends (48h)
        </p>
      </Section>

      <Section title="💬 Telegram Commands">
        <Table
          headers={['What you type', 'What happens']}
          rows={[
            ['What jobs are posted today?', 'Job Search hits Adzuna, returns top 5 scored jobs'],
            ['teach me hybrid rag', 'Siva teaches from real Groot code → Slack #tutor'],
            ['research Cohere hiring Toronto', 'Einstein researches → Slack #researcher'],
            ['find contacts at Shopify', 'Career agent finds LinkedIn contacts + drafts outreach'],
            ['write a Python script to...', 'Tony writes working code'],
            ['Interview at RBC for Data Engineer Tuesday', 'Einstein researches + Siva creates 48h prep plan'],
            ['APPROVE task-id', 'Approves a pending task'],
            ['REJECT task-id', 'Rejects a pending task'],
          ]}
        />
      </Section>

      <Section title="🔧 Health Checks">
        <p style={{ color: '#8B949E', fontSize: '13px', marginBottom: '12px' }}>Run these to verify everything is working:</p>
        <Code>{'# Quick health check\ncurl -s http://localhost:8000/health\n\n# All Docker services\ncd ~/agents && docker compose ps\n\n# Loaded Ollama models\ncurl -s http://localhost:11434/api/ps\n\n# Live status API\ncurl -s http://localhost:8787/status\n\n# Tailscale Funnel\ntailscale funnel status'}</Code>
      </Section>

      <Section title="🔄 Restart Commands">
        <Code>{'# Restart all Docker services\ncd ~/agents && docker compose restart\n\n# Restart specific service\ncd ~/agents && docker compose restart agent_service\n\n# Restart Ollama\nsudo systemctl restart ollama\n\n# Restart status API\nsudo systemctl restart groot-status\n\n# Re-enable Tailscale Funnel\nsudo tailscale funnel --bg 8787'}</Code>
        <p style={{ color: '#F85149', fontSize: '12px', marginTop: '8px' }}>
          ⚠️ Full emergency restart: docker compose down → docker compose up -d → restart ollama → restart groot-status
        </p>
      </Section>

      <Section title="🛠️ Aider (Tony's Code Editor)">
        <p style={{ color: '#F85149', fontSize: '13px', marginBottom: '8px', fontWeight: 600 }}>
          Always stop agent_service before running Aider!
        </p>
        <Code>{'# BEFORE Aider\ncd ~/agents && docker compose stop agent_service\n\n# Run Aider\ncd ~/code/tony\naider --message "..." --yes\n\n# AFTER Aider finishes\ncd ~/agents && docker compose start agent_service'}</Code>
        <p style={{ color: '#6E7681', fontSize: '12px' }}>
          Why: Supervisor loop competes for RAM with qwen2.5-coder:7b on 16GB system.
        </p>
      </Section>

      <Section title="📝 When to Rebuild">
        <Code>{'# After changing ~/agents/agent_service/\ncd ~/agents && docker compose up -d --build agent_service\n\n# After changing ~/agents/dashboard/src/\ncd ~/agents/dashboard && npm run build\ncd ~/agents && docker compose up -d --build dashboard\n\n# After changing ~/groot-status-api.py\nsudo systemctl restart groot-status'}</Code>
      </Section>

      <Section title="📊 Logs">
        <Table
          headers={['Log', 'Location', 'What it shows']}
          rows={[
            ['Portfolio updates', '/var/log/groot/portfolio_update.log', 'Nightly uptime push'],
            ['Backup', '/var/log/groot/backup.log', 'rsync backup status'],
            ['Agent service', 'docker compose logs agent_service', 'API requests, errors'],
            ['Ollama', 'journalctl -u ollama -n 20', 'Model loading, inference'],
            ['Status API', 'journalctl -u groot-status -n 20', 'Live status endpoint'],
          ]}
        />
      </Section>

      <Section title="🗄️ Notion Databases">
        <Table
          headers={['Database', 'ID', 'What it stores']}
          rows={[
            ['Groot OS Dairy', '3a6726bb...e99ff', 'Nightly diary entries'],
            ['Siva Learning Topics', '3af726bb...e2e9', '28+ learning topics'],
            ['Job Descriptions', '39b726bb...eb22', 'Extracted JDs'],
            ['Candidates', '39b726bb...3bcc', 'Resume extractions'],
            ['Invoices', '39b726bb...7af', 'Invoice extractions'],
          ]}
        />
      </Section>

      <Section title="⚙️ n8n Workflows (http://groot:5678)">
        <Table
          headers={['Workflow', 'Schedule', 'Status']}
          rows={[
            ['Daily AI Digest', '9 AM EST', '🟢 Active'],
            ['Top 10 Jobs', '8 AM + 8 PM EST', '🟢 Active'],
            ['Groot Nightly Diary', '11 PM EST', '🟢 Active'],
            ['Approval Reminder', '9AM/2PM/7PM EST', '🟢 Active'],
            ['Telegram Approval Handler', 'Every minute', '🟢 Active'],
            ['Slack Approval Polling', 'Every 4 hours', '🟢 Active'],
            ['Groot Inbox', 'Every minute', '🟢 Active'],
            ['Tony Portfolio Update', 'INACTIVE', '🔴 Replaced by cron'],
          ]}
        />
      </Section>

      <Section title="🌐 Key URLs">
        <Table
          headers={['Service', 'URL']}
          rows={[
            ['Portfolio', 'https://yasaswinpalukuri.github.io'],
            ['Case Study', 'https://yasaswinpalukuri.github.io/groot.html'],
            ['Live Status', 'https://yasaswinpalukuri.github.io/live.html'],
            ['Status API', 'https://groot.tailc6acbd.ts.net/status'],
            ['Dashboard', 'http://groot:3000'],
            ['n8n', 'http://groot:5678'],
            ['API', 'http://groot:8000'],
            ['GitHub', 'github.com/yasaswinpalukuri'],
            ['LinkedIn', 'linkedin.com/in/yasaswin-palukuri'],
          ]}
        />
      </Section>

      <Section title="💾 Structured Data Extractor (manual)">
        <Code>{'cd ~/projects/Structured-Data-Extractor\nsource .venv/bin/activate\nuvicorn api.main:app --port 8003\n\n# Test extraction\ncurl -X POST http://localhost:8003/extract \\\n  -H "Content-Type: application/json" \\\n  -d \'{"text": "paste job description here", "sync_to_notion": true}\''}</Code>
      </Section>

      <Section title="🔮 TODO — RAM Upgrade">
        <div style={{ background: '#161B22', border: '1px solid #D29922', borderRadius: '6px', padding: '12px 16px' }}>
          <p style={{ color: '#D29922', fontSize: '13px', fontWeight: 600, marginBottom: '8px' }}>
            Buy: 1× 16GB DDR4 SODIMM (PC4-21300, 2666MHz) — ~$80-100 CAD
          </p>
          <p style={{ color: '#8B949E', fontSize: '12px', marginBottom: '4px' }}>Where: Amazon.ca or Canada Computers</p>
          <p style={{ color: '#8B949E', fontSize: '12px', marginBottom: '4px' }}>Result: 32GB total — all models can load simultaneously</p>
          <p style={{ color: '#8B949E', fontSize: '12px' }}>After upgrade: no more stop/start for Aider, Einstein + Tony run simultaneously</p>
        </div>
      </Section>

    </div>
  )
}
