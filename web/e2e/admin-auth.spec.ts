import { expect, test } from '@playwright/test'

function administratorToken(): string {
  const encode = (value: object) =>
    Buffer.from(JSON.stringify(value)).toString('base64url')
  return `${encode({ alg: 'HS256' })}.${encode({
    sub: '30001',
    actor_type: 'administrator',
    exp: Math.floor(Date.now() / 1000) + 3600,
  })}.signature`
}

test('unauthenticated visitors are redirected to the administrator login', async ({ page }) => {
  await page.goto('/admin/courses')

  await expect(page).toHaveURL(/\/admin\/login\?redirect=\/admin\/courses$/)
  await expect(page.getByRole('heading', { name: '管理员登录' })).toBeVisible()
})

test('administrator signs in and the admin API receives its token', async ({ page }) => {
  const token = administratorToken()
  await page.route('**/admin-api/admin/v1/auth/login', async (route) => {
    await route.fulfill({
      json: {
        code: 0,
        info: 'success',
        data: {
          access_token: token,
          token_type: 'Bearer',
          expires_at: new Date(Date.now() + 3_600_000).toISOString(),
          administrator: { id: 30001, username: 'admin' },
        },
      },
    })
  })
  await page.route('**/admin-api/admin/v1/status', async (route) => {
    await route.fulfill({
      json: {
        code: 0,
        info: 'success',
        data: { service: 'courseforge-admin', status: 'ok' },
      },
    })
  })
  await page.route('**/admin-api/readyz', async (route) => {
    await route.fulfill({ json: { status: 'ready' } })
  })
  await page.route('**/healthz', async (route) => {
    await route.fulfill({ json: { status: 'ok' } })
  })
  await page.route('**/readyz', async (route) => {
    await route.fulfill({ json: { status: 'ready' } })
  })
  const statusRequest = page.waitForRequest('**/admin-api/admin/v1/status')

  await page.goto('/admin/login')
  await page.getByLabel('用户名').fill('admin')
  await page.getByLabel('密码').fill('correct-password')
  await page.getByRole('button', { name: '进入教务管理台' }).click()

  await expect(page).toHaveURL(/\/admin$/)
  await expect(page.getByRole('heading', { name: '运行概览' })).toBeVisible()
  await expect(page.getByText('admin', { exact: true })).toBeVisible()
  expect((await statusRequest).headers().authorization).toBe(`Bearer ${token}`)
})
