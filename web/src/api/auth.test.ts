import type { AxiosResponse } from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { getCurrentSession, login } from './auth'
import { http } from './http'
import type { ApiEnvelope } from '@/types/api'
import type { LoginResponse } from '@/types/auth'

function okResponse<T>(data: T): AxiosResponse<ApiEnvelope<T>> {
  return {
    data: { code: 0, info: 'success', data },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: { headers: {} } as AxiosResponse['config'],
  }
}

describe('authentication API contract', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('logs in with student credentials', async () => {
    const response: LoginResponse = {
      access_token: 'signed-token',
      token_type: 'Bearer',
      expires_at: '2026-07-31T14:00:00Z',
      student: {
        id: 10001,
        student_no: '2026001001',
        student_name: '林知夏',
      },
      selection_context: { term_id: 1, round_id: 2 },
    }
    const post = vi.spyOn(http, 'post').mockResolvedValue(okResponse(response))

    await expect(
      login({ student_no: '2026001001', password: 'password' }),
    ).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/api/v1/auth/login', {
      student_no: '2026001001',
      password: 'password',
    })
  })

  it('loads the authenticated student session', async () => {
    const get = vi.spyOn(http, 'get').mockResolvedValue(okResponse({}))

    await getCurrentSession()
    expect(get).toHaveBeenCalledWith('/api/v1/auth/me')
  })
})
