import { describe, expect, it } from 'vitest'
import type { AxiosResponse } from 'axios'

import { toApiError, validateApiResponse } from './error'
import { ApiError, type ApiEnvelope } from '@/types/api'

const messages = {
  timeout: '请求超时',
  http: (status: number) => `HTTP ${status}`,
  network: '网络不可用',
  unknown: '未知错误',
}

describe('API error compatibility', () => {
  it('keeps handling business errors returned with HTTP 200', () => {
    const response = {
      status: 200,
      data: { code: 409, info: '教学班名额已满', data: null },
    } as AxiosResponse<ApiEnvelope<unknown>>

    expect(() => validateApiResponse(response)).toThrowError(
      expect.objectContaining({
        name: 'ApiError',
        code: 409,
        message: '教学班名额已满',
        transport: false,
      }),
    )
  })

  it('reads the same business envelope from a real non-2xx response', () => {
    const error = {
      isAxiosError: true,
      response: {
        status: 409,
        data: { code: 409, info: '教学班名额已满', data: null },
      },
    }

    expect(toApiError(error, messages)).toEqual(
      expect.objectContaining({
        code: 409,
        message: '教学班名额已满',
        transport: false,
      }),
    )
  })

  it('classifies an unstructured gateway response as an HTTP error', () => {
    const error = {
      isAxiosError: true,
      response: { status: 502, data: '<html>Bad Gateway</html>' },
    }

    expect(toApiError(error, messages)).toEqual(
      expect.objectContaining({
        code: 502,
        message: 'HTTP 502',
        transport: false,
      }),
    )
  })

  it('classifies a missing response as a transport error', () => {
    const error = { isAxiosError: true, code: 'ERR_NETWORK' }

    expect(toApiError(error, messages)).toEqual(
      expect.objectContaining({
        code: 0,
        message: '网络不可用',
        transport: true,
      }),
    )
  })

  it('preserves an existing ApiError', () => {
    const original = new ApiError(429, '请求过于频繁')
    expect(toApiError(original, messages)).toBe(original)
  })
})
