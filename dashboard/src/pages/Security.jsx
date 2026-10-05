import { useState, useEffect } from 'react'
import { Shield, CheckCircle, XCircle } from 'lucide-react'

const API = 'http://groot:8000'

export default function Security() {
  const [stats, setStats] = useState({ total: 0, blocked: 0, safe: 0 })

  const testGuardrails = async (text, label) => {
    try {
      const r = await fetch(`${API}/run`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agent: 'groot', task: text, priority: 3 }),
      })
      const d = await r.json()
      return d.blocked ? 'BLOCKED' : 'PASSED'
    } catch { return 'ERROR' }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.08em' }}>
        SECURITY & GUARDRAILS
      </div>

      {/* Guardrails status */}
      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', padding: '16px' }}>
        <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
          letterSpacing: '0.06em', marginBottom: '16px' }}>ACTIVE GUARDRAILS</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          {[
            { name: 'PII Detection',         desc: 'SSN, credit cards, API keys, SIN numbers', status: true },
            { name: 'Prompt Injection',       desc: 'Jailbreak attempts, instruction override', status: true },
            { name: 'Toxic Content Filter',   desc: 'Harmful language and dangerous requests',  status: true },
            { name: 'Input Length Limit',     desc: 'Max 32,000 characters per request',        status: true },
            { name: 'Audit Logging',          desc: '/var/log/groot/guardrails_audit.jsonl',    status: true },
          ].map(g => (
            <div key={g.name} style={{ display: 'flex', alignItems: 'center', gap: '12px',
              padding: '10px 12px', background: '#0D1117', borderRadius: '6px' }}>
              <CheckCircle size={14} color="#3FB950" />
              <div style={{ flex: 1 }}>
                <div style={{ color: '#E6EDF3', fontSize: '13px' }}>{g.name}</div>
                <div style={{ color: '#6E7681', fontSize: '11px' }}>{g.desc}</div>
              </div>
              <span style={{ fontSize: '10px', padding: '2px 8px', borderRadius: '4px',
                background: '#1C2A1C', color: '#3FB950',
                fontFamily: 'JetBrains Mono, monospace' }}>ACTIVE</span>
            </div>
          ))}
        </div>
      </div>

      {/* Ragas eval scores */}
      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', padding: '16px' }}>
        <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
          letterSpacing: '0.06em', marginBottom: '16px' }}>RAGAS EVALUATION SCORES</div>
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
            </div>
          ))}
        </div>
      </div>

      {/* Infrastructure security */}
      <div style={{ background: '#161B22', border: '1px solid #30363D', borderRadius: '8px', padding: '16px' }}>
        <div style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600,
          letterSpacing: '0.06em', marginBottom: '16px' }}>INFRASTRUCTURE</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {[
            { label: 'Disk Encryption',    value: 'LUKS full-disk (AES-256)' },
            { label: 'SSH Auth',           value: 'Ed25519 keys only (password disabled)' },
            { label: 'Firewall',           value: 'UFW active — deny incoming by default' },
            { label: 'Brute Force',        value: 'fail2ban active' },
            { label: 'VPN Access',         value: 'Tailscale (WireGuard)' },
            { label: 'Dropbear',           value: 'Port 2222 — LUKS remote unlock' },
          ].map(i => (
            <div key={i.label} style={{ display: 'flex', gap: '16px', padding: '6px 0',
              borderBottom: '1px solid #21262D' }}>
              <span style={{ color: '#6E7681', fontSize: '12px', width: '160px',
                flexShrink: 0 }}>{i.label}</span>
              <span style={{ color: '#E6EDF3', fontSize: '12px',
                fontFamily: 'JetBrains Mono, monospace' }}>{i.value}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
