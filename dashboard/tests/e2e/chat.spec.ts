import { test, expect, API, sseBody } from './fixtures/test'
import chat from './fixtures/chat.json' with { type: 'json' }

test.beforeEach(async ({ page }) => {
  await page.route(`${API}/chat/sessions`, route => route.fulfill({ json: chat.sessions }))
})

test('sending a message renders the streamed reply', async ({ page }) => {
  let sent: Record<string, unknown> | undefined
  await page.route(`${API}/chat`, route => {
    sent = route.request().postDataJSON()
    return route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache' },
      body: sseBody(chat.replyTokens),
    })
  })

  await page.goto('/chat')
  await expect(page.getByText('Earlier conversation')).toBeVisible()

  const input = page.getByPlaceholder(/Message Groot/)
  await input.fill('What is on my queue?')
  await page.getByRole('button', { name: 'Send message' }).click()

  await expect(page.getByText('What is on my queue?')).toBeVisible()
  // Tokens arrive as separate SSE frames and must be concatenated in order.
  await expect(page.getByText('Hello from Groot.', { exact: true })).toBeVisible()
  await expect(input).toHaveValue('')

  expect(sent).toMatchObject({
    message: 'What is on my queue?',
    model: 'qwen2.5:7b',
    session_name: 'What is on my queue?',
    history: [
      { role: 'assistant', content: 'I am Groot. What do you need?' },
      { role: 'user', content: 'What is on my queue?' },
    ],
  })
  expect(sent?.session_id).toMatch(/^sess_\d+_[a-z0-9]+$/)
})

test('a 500 from the chat endpoint shows an error instead of an empty reply', async ({ page }) => {
  await page.route(`${API}/chat`, route => route.fulfill({ status: 500, json: { detail: 'Ollama is down' } }))

  await page.goto('/chat')
  await page.getByPlaceholder(/Message Groot/).fill('Hello?')
  await page.getByRole('button', { name: 'Send message' }).click()

  await expect(page.getByText('Error reaching agent service.')).toBeVisible()
})
