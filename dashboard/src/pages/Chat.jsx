import { useState, useRef, useEffect } from 'react'
import { Send, Plus, Trash2, MessageSquare, Paperclip, X } from 'lucide-react'

const API = 'http://groot:8000'

const AGENTS = [
  { id: 'groot',      label: 'Groot',    model: 'qwen2.5:7b',      greeting: 'I am Groot. What do you need?' },
  { id: 'einstein',   label: 'Einstein', model: 'deepseek-r1:14b',  greeting: 'Einstein here. What shall we research?' },
  { id: 'tony',       label: 'Tony',     model: 'qwen2.5-coder:7b', greeting: 'Tony online. Show me the code.' },
  { id: 'siva',       label: 'Siva',     model: 'phi3:medium',      greeting: 'Siva here. What would you like to learn?' },
]

function genSessionId() {
  return 'sess_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7)
}

export default function Chat() {
  const [agent, setAgent]         = useState(AGENTS[0])
  const [sessions, setSessions]   = useState([])
  const [sessionId, setSessionId] = useState(() => genSessionId())
  const [sessionName, setSessionName] = useState('')
  const [messages, setMessages]   = useState([
    { role: 'assistant', content: AGENTS[0].greeting, model: AGENTS[0].model }
  ])
  const [input, setInput]         = useState('')
  const [streaming, setStreaming] = useState(false)
  const [file, setFile]           = useState(null)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef(null)
  const bottomRef = useRef(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  useEffect(() => {
    loadSessions()
    const interval = setInterval(loadSessions, 10000)
    return () => clearInterval(interval)
  }, [])

  const loadSessions = async () => {
    try {
      const r = await fetch(`${API}/chat/sessions`)
      const d = await r.json()
      setSessions(d.sessions || [])
    } catch {}
  }

  const loadSession = async (sess) => {
    try {
      const r = await fetch(`${API}/chat/sessions/${sess.id}`)
      const d = await r.json()
      setSessionId(sess.id)
      setSessionName(sess.name)
      const msgs = (d.messages || []).map(m => ({
        role:    m.role,
        content: m.content,
        model:   m.model || agent.model,
      }))
      setMessages(msgs.length > 0 ? msgs : [{ role: 'assistant', content: agent.greeting, model: agent.model }])
    } catch {}
  }

  const newChat = () => {
    const newId = genSessionId()
    setSessionId(newId)
    setSessionName('')
    setMessages([{ role: 'assistant', content: agent.greeting, model: agent.model }])
    loadSessions()
  }

  const deleteSession = async (sess, e) => {
    e.stopPropagation()
    try {
      await fetch(`${API}/chat/sessions/${sess.id}`, { method: 'DELETE' })
      loadSessions()
      if (sess.id === sessionId) newChat()
    } catch {}
  }

  const switchAgent = (a) => {
    if (!streaming) {
      setAgent(a)
      setMessages([{ role: 'assistant', content: a.greeting, model: a.model }])
    }
  }

  const handleFile = (e) => {
    const f = e.target.files[0]
    if (f) setFile(f)
  }

  const uploadFile = async () => {
    if (!file) return
    setUploading(true)
    const userMsg   = { role: 'user',      content: `📎 ${file.name}${input ? ' — ' + input : ''}`, model: agent.model }
    const assistMsg = { role: 'assistant', content: '', model: agent.model }
    setMessages(m => [...m, userMsg, assistMsg])
    const q = input
    setInput('')
    setFile(null)

    const form = new FormData()
    form.append('file',       file)
    form.append('message',    q || 'Analyze this and help me understand it')
    form.append('model',      agent.model)
    form.append('session_id', sessionId)

    try {
      const r = await fetch(`${API}/chat/upload`, { method: 'POST', body: form })
      const d = await r.json()
      setMessages(m => {
        const u = [...m]
        u[u.length - 1] = { ...u[u.length - 1], content: d.response || 'No response' }
        return u
      })
      loadSessions()
    } catch {
      setMessages(m => {
        const u = [...m]
        u[u.length - 1].content = 'Error processing file.'
        return u
      })
    } finally {
      setUploading(false)
    }
  }

  const send = async () => {
    if (!input.trim() || streaming) return
    const userMsg   = { role: 'user', content: input }
    const assistMsg = { role: 'assistant', content: '', model: agent.model }
    const isFirst   = messages.length <= 1
    const name      = isFirst ? input.slice(0, 40) : sessionName

    setMessages(m => [...m, userMsg, assistMsg])
    setInput('')
    setStreaming(true)
    if (isFirst) setSessionName(name)

    const history = messages
      .filter(m => m.content)
      .map(m => ({ role: m.role, content: m.content }))
    history.push({ role: 'user', content: input })

    try {
      const res = await fetch(`${API}/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          message:      input,
          model:        agent.model,
          history,
          session_id:   sessionId,
          session_name: name,
        }),
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
              const { token } = JSON.parse(line.slice(6))
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
    } finally {
      setStreaming(false)
      loadSessions()
    }
  }

  return (
    <div style={{ display: 'flex', height: 'calc(100vh - 100px)', gap: '0' }}>

      {/* Sessions sidebar */}
      <div style={{ width: '220px', background: '#161B22', borderRight: '1px solid #30363D',
        display: 'flex', flexDirection: 'column', flexShrink: 0 }}>

        {/* New chat button */}
        <button onClick={newChat} style={{
          display: 'flex', alignItems: 'center', gap: '8px',
          margin: '12px 8px', padding: '8px 12px', borderRadius: '6px',
          border: '1px solid #30363D', background: '#21262D',
          color: '#E6EDF3', cursor: 'pointer', fontSize: '12px',
        }}>
          <Plus size={14} />
          New Chat
        </button>

        {/* Agent selector */}
        <div style={{ padding: '0 8px 8px', borderBottom: '1px solid #30363D' }}>
          {AGENTS.map(a => (
            <button key={a.id} onClick={() => switchAgent(a)} style={{
              display: 'flex', alignItems: 'center', gap: '6px',
              width: '100%', padding: '5px 8px', borderRadius: '4px',
              border: 'none', background: agent.id === a.id ? '#21262D' : 'transparent',
              color: agent.id === a.id ? '#58A6FF' : '#8B949E',
              cursor: 'pointer', fontSize: '11px', marginBottom: '2px',
            }}>
              <div style={{ width: '6px', height: '6px', borderRadius: '50%',
                background: agent.id === a.id ? '#58A6FF' : '#444C56' }} />
              {a.label}
            </button>
          ))}
        </div>

        {/* Session list */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
          <div style={{ color: '#6E7681', fontSize: '10px', fontWeight: 600,
            letterSpacing: '0.06em', padding: '4px 8px', marginBottom: '4px' }}>
            RECENT
          </div>
          {sessions.length === 0 && (
            <div style={{ color: '#444C56', fontSize: '11px', padding: '8px', textAlign: 'center' }}>
              No conversations yet
            </div>
          )}
          {sessions.map(sess => (
            <div key={sess.id}
              onClick={() => loadSession(sess)}
              style={{
                display: 'flex', alignItems: 'center', gap: '6px',
                padding: '7px 8px', borderRadius: '6px', cursor: 'pointer',
                background: sess.id === sessionId ? '#30363D' : 'transparent',
                marginBottom: '2px', group: true,
              }}
              onMouseEnter={e => e.currentTarget.style.background = '#1A1F27'}
              onMouseLeave={e => e.currentTarget.style.background = sess.id === sessionId ? '#21262D' : 'transparent'}
            >
              <MessageSquare size={11} color="#6E7681" style={{ flexShrink: 0 }} />
              <span style={{ color: sess.id === sessionId ? '#E6EDF3' : '#8B949E',
                fontSize: '11px', flex: 1, overflow: 'hidden',
                textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {sess.name || 'New Conversation'}
              </span>
              <button onClick={e => deleteSession(sess, e)} style={{
                background: 'none', border: 'none', cursor: 'pointer',
                padding: '2px', opacity: 0.5, flexShrink: 0,
              }}>
                <Trash2 size={10} color="#F85149" />
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Main chat area */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '12px', padding: '16px' }}>

        {/* Messages */}
        <div style={{ flex: 1, background: '#161B22', border: '1px solid #30363D',
          borderRadius: '8px', padding: '16px', overflowY: 'auto',
          display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {messages.map((msg, i) => (
            <div key={i} style={{ display: 'flex', flexDirection: 'column',
              alignItems: msg.role === 'user' ? 'flex-end' : 'flex-start' }}>
              {msg.role === 'assistant' && (
                <span style={{ color: '#6E7681', fontSize: '10px',
                  fontFamily: 'JetBrains Mono, monospace', marginBottom: '4px' }}>
                  {agent.label} · {msg.model}
                </span>
              )}
              <div style={{ maxWidth: '75%', padding: '10px 14px', borderRadius: '8px',
                background: msg.role === 'user' ? '#1C2F4A' : '#21262D',
                color: '#E6EDF3', fontSize: '13px', lineHeight: '1.6',
                border: `1px solid ${msg.role === 'user' ? '#58A6FF33' : '#30363D'}`,
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

        {/* Input */}
        <div style={{ display: 'flex', gap: '8px', background: '#161B22',
          border: '1px solid #30363D', borderRadius: '8px',
          padding: '8px 12px', alignItems: 'center' }}>
          <input ref={fileRef} type="file"
            accept=".pdf,.jpg,.jpeg,.png,.gif,.webp"
            onChange={handleFile}
            style={{ display: 'none' }} />
          <button onClick={() => fileRef.current?.click()} style={{
            background: 'transparent', border: 'none', cursor: 'pointer',
            color: '#8B949E', padding: '4px', display: 'flex', alignItems: 'center',
          }}>
            <Paperclip size={16} />
          </button>
          {file && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '4px',
              background: '#21262D', borderRadius: '4px', padding: '2px 8px',
              fontSize: '11px', color: '#58A6FF', maxWidth: '120px' }}>
              <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {file.name}
              </span>
              <button onClick={() => setFile(null)} style={{
                background: 'none', border: 'none', cursor: 'pointer',
                color: '#6E7681', padding: '0', flexShrink: 0,
              }}>
                <X size={10} />
              </button>
            </div>
          )}
          <textarea value={input} onChange={e => setInput(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                file ? uploadFile() : send()
              }
            }}
            placeholder={file ? `Ask about ${file.name}...` : `Message ${agent.label}... (Shift+Enter for new line)`}
            rows={1}
            style={{ flex: 1, background: 'transparent', border: 'none',
              color: '#E6EDF3', fontSize: '13px', fontFamily: 'Inter, sans-serif',
              outline: 'none', resize: 'none', overflow: 'hidden',
              lineHeight: '1.5', maxHeight: '120px', overflowY: 'auto' }}
            onInput={e => {
              e.target.style.height = 'auto'
              e.target.style.height = Math.min(e.target.scrollHeight, 120) + 'px'
            }} />
          <button
            onClick={file ? uploadFile : send}
            disabled={streaming || uploading || (!input.trim() && !file)}
            style={{
              background: streaming || uploading ? '#21262D' : '#58A6FF', border: 'none',
              borderRadius: '6px', padding: '6px 12px',
              cursor: streaming || uploading ? 'not-allowed' : 'pointer', color: '#fff',
              display: 'flex', alignItems: 'center' }}>
            <Send size={14} />
          </button>
        </div>
      </div>
      <style>{'@keyframes blink{0%,100%{opacity:1}50%{opacity:0}}'}</style>
    </div>
  )
}
