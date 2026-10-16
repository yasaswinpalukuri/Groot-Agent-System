import { test as base, expect } from '@playwright/test'

export const API = 'http://groot:8000'

/**
 * Every test starts with the network sealed off:
 * - unmocked calls to the agent API get a 501 and fail the test at teardown,
 * - any other external request (e.g. Google Fonts) is aborted,
 * - uncaught page errors fail the test, so "the UI didn't crash" is always asserted.
 * Specs register their own page.route() handlers, which take precedence over these.
 */
export const test = base.extend<{ sealedNetwork: void }>({
  sealedNetwork: [async ({ page }, use) => {
    const unmocked: string[] = []
    const pageErrors: Error[] = []
    page.on('pageerror', err => pageErrors.push(err))

    await page.route(url => !['localhost', 'groot'].includes(url.hostname), route => route.abort())
    await page.route(`${API}/**`, route => {
      unmocked.push(`${route.request().method()} ${route.request().url()}`)
      return route.fulfill({ status: 501, json: { error: 'not mocked' } })
    })

    await use()

    expect(pageErrors, 'uncaught errors in the page').toEqual([])
    expect(unmocked, 'API calls without a mock').toEqual([])
  }, { auto: true }],
})

/** Build an SSE body in the format the backend streams: one `data:` frame per token, then [DONE]. */
export function sseBody(tokens: string[]): string {
  return tokens.map(token => `data: ${JSON.stringify({ token })}\n\n`).join('') + 'data: [DONE]\n\n'
}

export { expect }
