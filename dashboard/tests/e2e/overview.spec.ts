import { test, expect, API } from './fixtures/test'
import overview from './fixtures/overview.json' with { type: 'json' }

test('overview loads and shows live stats', async ({ page }) => {
  await page.route(`${API}/health`, route => route.fulfill({ json: overview.health }))
  await page.route(`${API}/scheduler/status`, route => route.fulfill({ json: overview.schedulerStatus }))
  await page.route(`${API}/scheduler/queue`, route => route.fulfill({ json: overview.schedulerQueue }))
  // RAM usage only arrives over the live WebSocket, so mock that too.
  await page.routeWebSocket('ws://groot:8000/ws/live', ws => {
    ws.send(JSON.stringify(overview.live))
  })

  await page.goto('/')

  await expect(page).toHaveURL(/\/overview$/)
  await expect(page.getByRole('group', { name: 'API STATUS' })).toContainText('ok')
  await expect(page.getByRole('group', { name: 'QUEUE DEPTH' })).toContainText('2')
  await expect(page.getByRole('group', { name: 'RAM USED' })).toContainText('42%')
  await expect(page.getByText('Einstein — Research vector database')).toBeVisible()
  await expect(page.getByText('2 tasks')).toBeVisible()
})
