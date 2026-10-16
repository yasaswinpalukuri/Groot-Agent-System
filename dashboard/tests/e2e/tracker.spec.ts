import { test, expect, API } from './fixtures/test'
import fixture from './fixtures/jobs.json' with { type: 'json' }

type Job = (typeof fixture.jobs)[number]

/** In-memory tracker backend so a status change is visible on the refetch that follows it. */
async function mockTracker(page: import('@playwright/test').Page) {
  const jobs: Job[] = structuredClone(fixture.jobs)
  const patches: { id: string; body: unknown }[] = []

  await page.route(`${API}/jobs/tracker`, route => route.fulfill({ json: { jobs } }))
  await page.route(`${API}/jobs/tracker/*/status`, route => {
    const id = route.request().url().split('/').at(-2)!
    const body = route.request().postDataJSON() as { status: string }
    patches.push({ id, body })
    const job = jobs.find(j => String(j.id) === id)
    if (!job) return route.fulfill({ status: 404, json: { detail: 'not found' } })
    job.status = body.status
    return route.fulfill({ json: { ok: true } })
  })
  return { patches }
}

test('job list renders with stats', async ({ page }) => {
  await mockTracker(page)
  await page.goto('/approvals')

  for (const company of ['Acme Corp', 'Globex', 'Initech']) {
    await expect(page.getByText(company, { exact: true })).toBeVisible()
  }
  await expect(page.getByText('2026-09-28')).toBeVisible()
  await expect(page.getByText('TOTAL TRACKED').locator('..')).toContainText('3')
  await expect(page.getByText('INTERVIEWS').locator('..')).toContainText('1')
})

test('filtering by status narrows the list', async ({ page }) => {
  await mockTracker(page)
  await page.goto('/approvals')
  await expect(page.getByText('Acme Corp', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'interview', exact: true }).click()
  await expect(page.getByText('Globex', { exact: true })).toBeVisible()
  await expect(page.getByText('Acme Corp', { exact: true })).toBeHidden()
  await expect(page.getByText('Initech', { exact: true })).toBeHidden()

  await page.getByRole('button', { name: 'offer', exact: true }).click()
  await expect(page.getByText('No jobs tracked yet')).toBeVisible()

  await page.getByRole('button', { name: 'All', exact: true }).click()
  await expect(page.getByText('Initech', { exact: true })).toBeVisible()
})

test('changing a status sends a PATCH and updates the stats', async ({ page }) => {
  const { patches } = await mockTracker(page)
  await page.goto('/approvals')

  await page.getByLabel('Update status for Globex').selectOption('offer')

  await expect(page.getByText('OFFERS').locator('..')).toContainText('1')
  expect(patches).toEqual([{ id: '2', body: { status: 'offer' } }])
})

test('a 500 from the tracker shows an error state, not an empty list', async ({ page }) => {
  await page.route(`${API}/jobs/tracker`, route => route.fulfill({ status: 500, json: { detail: 'db locked' } }))
  await page.goto('/approvals')

  await expect(page.getByText('Could not reach agent service')).toBeVisible()
  await expect(page.getByText('No jobs tracked yet')).toBeHidden()
  await expect(page.getByText('Loading tracker...')).toBeHidden()
})
