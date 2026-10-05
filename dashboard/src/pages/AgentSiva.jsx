import { useState, useRef, useEffect } from 'react'
import { Send } from 'lucide-react'

const API  = 'http://groot:8000'
const NAME = 'Siva'
const MODEL = 'phi3:medium'

const GREETINGS = {
  Groot:    'I am Groot. What do you need?',
  Einstein: 'Einstein here. What shall we research?',
  Siva:     'Siva here. What would you like to learn today?',
}

export default function AgentSiva() {
  const [messages, setMessages] = useState([
    { role: 'assistant', content: GREETINGS[NAME] || 'Ready.', model: MODEL }
  ])
  const [input, setInput]       = useState('')
  const [streaming, setStreaming] = useState(false)
  const bottomRef = useRef(null)

  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages])

  const send = async () => {
    if (!input.trim() || streaming) return
    const userMsg   = { role: 'user', content: input }
    const assistMsg = { role: 'assistant', content: '', model: MODEL }
    setMessages(m => [...m, userMsg, assistMsg])
    setInput('')
    setStreaming(true)

    const history = messages.filter(m => m.content)
      .map(m => ({ role: m.role, content: m.content }))
    history.push({ role: 'user', content: input })

    try {
      const res = await fetch(API + '/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: input, model: MODEL, history }),
      })
      const reader  = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer    = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop()
        for (const line of lines) {
          if (line.startsWith('data: ') && line !== 'data: [DONE]') {
            try {
              const token = JSON.parse(line.slice(6)).token || ''
              setMessages(m => {
                const u = [...m]
                u[u.length - 1] = { ...u[u.length - 1], content: u[u.length - 1].content + token }
                return u
              })
            } catch {}
          }
        }
      }
    } catch {
      setMessages(m => {
        const u = [...m]
        u[u.length - 1].content = 'Error reaching agent service.'
        return u
      })
    } finally { setStreaming(false) }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 160px)', gap: '16px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        <span style={{ color: '#8B949E', fontSize: '11px', fontWeight: 600, letterSpacing: '0.08em' }}>
          {NAME.toUpperCase()} AGENT
        </span>
        <span style={{ color: '#6E7681', fontSize: '11px', fontFamily: 'JetBrains Mono, monospace' }}>
          {MODEL}
        </span>
      </div>

      <div style={{ flex: 1, background: '#161B22', border: '1px solid #30363D',
        borderRadius: '8px', padding: '16px', overflowY: 'auto',
        display: 'flex', flexDirection: 'column', gap: '16px' }}>
        {messages.map((msg, i) => (
          <div key={i} style={{ display: 'flex', flexDirection: 'column',
            alignItems: msg.role === 'user' ? 'flex-end' : 'flex-start' }}>
            {msg.role === 'assistant' && (
              <span style={{ color: '#6E7681', fontSize: '10px',
                fontFamily: 'JetBrains Mono, monospace', marginBottom: '4px' }}>
                {NAME} · {msg.model}
              </span>
            )}
            <div style={{ maxWidth: '75%', padding: '10px 14px', borderRadius: '8px',
              background: msg.role === 'user' ? '#1C2F4A' : '#21262D',
              color: '#E6EDF3', fontSize: '13px', lineHeight: '1.6',
              border: '1px solid ' + (msg.role === 'user' ? '#58A6FF33' : '#30363D'),
              whiteSpace: 'pre-wrap' }}>
              {msg.content}
              {streaming && i === messages.length - 1 && msg.role === 'assistant' && (
                <span style={{ display: 'inline-block', width: '8px', height: '14px',
                  background: '#58A6FF', marginLeft: '2px', animation: 'blink 1s infinite' }} />
              )}
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      <div style={{ display: 'flex', gap: '8px', background: '#161B22',
        border: '1px solid #30363D', borderRadius: '8px', padding: '8px 12px', alignItems: 'center' }}>
        <input value={input} onChange={e => setInput(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && !e.shiftKey && send()}
          placeholder={'Message ' + NAME + '...'}
          style={{ flex: 1, background: 'transparent', border: 'none',
            color: '#E6EDF3', fontSize: '13px', fontFamily: 'Inter, sans-serif', outline: 'none' }} />
        <button onClick={send} disabled={streaming || !input.trim()} style={{
          background: streaming ? '#21262D' : '#58A6FF', border: 'none',
          borderRadius: '6px', padding: '6px 12px',
          cursor: streaming ? 'not-allowed' : 'pointer', color: '#fff',
          display: 'flex', alignItems: 'center' }}>
          <Send size={14} />
        </button>
      </div>
      <style>{'@keyframes blink { 0%,100%{opacity:1} 50%{opacity:0} }'}</style>
    </div>
  )
}
