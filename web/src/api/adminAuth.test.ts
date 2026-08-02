import type { AxiosResponse } from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { getCurrentAdministratorSession, loginAdministrator } from './adminAuth'
import { adminHttp } from './adminHttp'
import type { ApiEnvelope } from '@/types/api'
import type { AdministratorLoginResponse } from '@/types/auth'

function okResponse<T>(data: T): AxiosResponse<ApiEnvelope<T>> {
  return {
    data: { code: 0, info: 'success', data },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: { headers: {} } as AxiosResponse['config'],
  }
}

describe('administrator authentication API contract', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('logs in with administrator credentials', async () => {
    const response: AdministratorLoginResponse = {
      access_token: 'administrator-token',
      token_type: 'Bearer',
      expires_at: '2026-08-02T14:00:00Z',
      administrator: { id: 30001, username: 'admin' },
    }
    const post = vi.spyOn(adminHttp, 'post').mockResolvedValue(okResponse(response))

    await expect(loginAdministrator({ username: 'admin', password: 'password' }))
      .resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/admin/v1/auth/login', {
      username: 'admin',
      password: 'password',
    })
  })

  it('loads the authenticated administrator session', async () => {
    const get = vi.spyOn(adminHttp, 'get').mockResolvedValue(okResponse({}))

    await getCurrentAdministratorSession()
    expect(get).toHaveBeenCalledWith('/admin/v1/auth/me')
  })
})
