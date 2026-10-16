// Typed client for the Groot agent service used by the Chat and Job Tracker pages.
// Response shapes mirror what the backend at API_BASE returns today; the backend
// itself lives outside this repo, so keep these in sync by hand.

export const API_BASE = 'http://groot:8000'

// ---------- Chat ----------

export type Role = 'user' | 'assistant'

export interface ChatMessage {
  role: Role
  content: string
  model?: string
}

export interface ChatSession {
  id: string
  name: string
}

export interface ChatSessionsResponse {
  sessions?: ChatSession[]
}

export interface ChatSessionDetail {
  messages?: ChatMessage[]
}

export interface ChatRequest {
  message: string
  model: string
  history: Pick<ChatMessage, 'role' | 'content'>[]
  session_id: string
  session_name: string
}

/** One SSE frame from POST /chat: `data: {"token": "..."}`. The stream ends with `data: [DONE]`. */
export interface ChatStreamChunk {
  token: string
}

export interface UploadResponse {
  response?: string
}

// ---------- Job tracker ----------

export const JOB_STATUSES = ['interested', 'applied', 'contacted', 'interview', 'offer', 'rejected'] as const
export type JobStatus = (typeof JOB_STATUSES)[number]

export interface JobRecord {
  id: number | string
  company: string
  role: string
  location: string
  date: string | null
  status: JobStatus
  url: string
}

export interface JobTrackerResponse {
  jobs?: JobRecord[]
}

// ---------- Transport ----------

export class ApiError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(`${API_BASE}${path}`, init)
  if (!res.ok) throw new ApiError(res.status, `${init?.method ?? 'GET'} ${path} failed with ${res.status}`)
  return res
}

async function requestJson<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await request(path, init)
  return (await res.json()) as T
}

const jsonBody = (body: unknown): RequestInit => ({
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
})

export const listChatSessions = () => requestJson<ChatSessionsResponse>('/chat/sessions')

export const getChatSession = (id: string) => requestJson<ChatSessionDetail>(`/chat/sessions/${id}`)

export const deleteChatSession = (id: string) =>
  request(`/chat/sessions/${id}`, { method: 'DELETE' }).then(() => undefined)

export const uploadChatFile = (form: FormData) =>
  requestJson<UploadResponse>('/chat/upload', { method: 'POST', body: form })

/**
 * POST /chat and call onToken for every token in the SSE stream.
 * Resolves when the stream closes; rejects on a non-2xx response or network error.
 */
export async function streamChat(req: ChatRequest, onToken: (token: string) => void): Promise<void> {
  const res = await request('/chat', { method: 'POST', ...jsonBody(req) })
  if (!res.body) throw new ApiError(res.status, 'POST /chat returned no body')

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''
    for (const line of lines) {
      if (!line.startsWith('data: ') || line === 'data: [DONE]') continue
      try {
        const { token } = JSON.parse(line.slice(6)) as ChatStreamChunk
        if (typeof token === 'string') onToken(token)
      } catch {
        // Skip malformed frames rather than abort the whole reply.
      }
    }
  }
}

export const getJobTracker = () => requestJson<JobTrackerResponse>('/jobs/tracker')

export const updateJobStatus = (id: JobRecord['id'], status: JobStatus) =>
  request(`/jobs/tracker/${id}/status`, { method: 'PATCH', ...jsonBody({ status }) }).then(() => undefined)
