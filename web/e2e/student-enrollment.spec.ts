import { expect, test } from '@playwright/test'

function sessionToken(): string {
  const encode = (value: object) =>
    Buffer.from(JSON.stringify(value)).toString('base64url')
  return `${encode({ alg: 'HS256' })}.${encode({
    sub: '10001',
    exp: Math.floor(Date.now() / 1000) + 3600,
  })}.signature`
}

test('student signs in with a student number and password', async ({ page }) => {
  await page.route('**/api/v1/auth/login', async (route) => {
    await route.fulfill({
      json: {
        code: 0,
        info: 'success',
        data: {
          access_token: sessionToken(),
          token_type: 'Bearer',
          expires_at: new Date(Date.now() + 3_600_000).toISOString(),
          student: {
            id: 10001,
            student_no: '2026001001',
            student_name: '林知夏',
          },
          selection_context: { term_id: 1, round_id: 1 },
        },
      },
    })
  })
  await page.route('**/api/v1/enrollments**', async (route) => {
    await route.fulfill({
      json: {
        code: 0,
        info: 'success',
        data: { items: [], limit: 100, offset: 0, total: 0 },
      },
    })
  })

  await page.goto('/login')
  await page.getByLabel('学号').fill('2026001001')
  await page.getByLabel('密码').fill('correct-password')
  await page.getByRole('button', { name: '登录并进入选课系统' }).click()

  await expect(page).toHaveURL(/\/student\/courses$/)
  await expect(page.getByText('林知夏')).toBeVisible()
})

test('student submits a selection and keeps tracking its application', async ({
  page,
}) => {
  await page.addInitScript(
    ({ token }) => {
      window.sessionStorage.setItem(
        'courseforge.student-session',
        JSON.stringify({
          accessToken: token,
          studentId: 10001,
          studentName: '林知夏',
          studentNo: '2026001001',
          termId: 1,
          roundId: 1,
        }),
      )
    },
    { token: sessionToken() },
  )

  await page.route('**/api/v1/enrollments**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    if (
      request.method() === 'GET' &&
      url.pathname.endsWith('/enrollments/me')
    ) {
      await route.fulfill({
        json: { code: 0, info: 'success', data: { items: [], limit: 100, offset: 0, total: 0 } },
      })
      return
    }
    if (
      request.method() === 'GET' &&
      url.pathname.endsWith('/waitlist/me')
    ) {
      await route.fulfill({
        json: { code: 0, info: 'success', data: { items: [], limit: 100, offset: 0, total: 0 } },
      })
      return
    }
    if (
      request.method() === 'GET' &&
      url.pathname.includes('/applications/application-1')
    ) {
      await route.fulfill({
        json: {
          code: 0,
          info: 'success',
          data: {
            application_id: 'application-1',
            request_id: 'request-1',
            round_id: 1,
            term_id: 1,
            course_id: 10001,
            teaching_class_id: 20001,
            credits: '3.5',
            state: 'selected',
            applied_at: '2026-07-30T10:00:00Z',
            completed_at: '2026-07-30T10:00:01Z',
            broker_confirmed: true,
            mysql_persisted: true,
          },
        },
      })
      return
    }
    if (request.method() === 'POST' && url.pathname.endsWith('/enrollments')) {
      await route.fulfill({
        json: {
          code: 0,
          info: 'success',
          data: {
            application_id: 'application-1',
            state: 'selected',
            broker_confirmed: true,
            mysql_persisted: false,
          },
        },
      })
      return
    }
    await route.fallback()
  })

  await page.goto('/student/courses')
  await page
    .getByTestId('course-card')
    .filter({ hasText: '分布式系统设计' })
    .getByRole('button', { name: '选择这门课' })
    .click()

  await expect(page.getByText('名额已锁定，正在异步写入正式记录')).toBeVisible()
  await page.getByRole('link', { name: '我的选课' }).click()
  await expect(page.getByText('application-1')).toBeVisible()
  await expect(page.getByText('MySQL 落库')).toBeVisible()
})
