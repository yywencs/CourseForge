import axios from 'axios'
import type { AxiosResponse } from 'axios'

import { ApiError, type ApiEnvelope } from '@/types/api'

export interface ApiErrorMessages {
  timeout: string
  http: (status: number) => string
  network: string
  unknown: string
}

// validateApiResponse 兼容旧契约：HTTP 200 中的非零业务 code 仍按错误处理。
export function validateApiResponse<T>(
  response: AxiosResponse<ApiEnvelope<T>>,
): AxiosResponse<ApiEnvelope<T>> {
  const body = response.data
  if (typeof body?.code === 'number' && body.code !== 0) {
    throw new ApiError(body.code, body.info || '请求失败')
  }
  return response
}

// toApiError 兼容新契约：真实非 2xx 响应如果仍携带标准响应信封，
// 优先保留后端业务 code 和提示；没有信封时才按 HTTP 或传输错误处理。
export function toApiError(
  error: unknown,
  messages: ApiErrorMessages,
): ApiError {
  if (error instanceof ApiError) return error

  if (axios.isAxiosError(error)) {
    const response = error.response
    if (response) {
      const envelope = errorEnvelope(response.data)
      if (envelope) {
        return new ApiError(
          envelope.code,
          envelope.info || messages.http(response.status),
        )
      }
    }
    if (error.code === 'ECONNABORTED') {
      return new ApiError(0, messages.timeout, true)
    }
    if (response) {
      return new ApiError(response.status, messages.http(response.status))
    }
    return new ApiError(0, messages.network, true)
  }

  return new ApiError(0, messages.unknown, true)
}

function errorEnvelope(
  value: unknown,
): Pick<ApiEnvelope<unknown>, 'code' | 'info'> | undefined {
  if (typeof value !== 'object' || value === null) return undefined
  const candidate = value as Partial<ApiEnvelope<unknown>>
  if (typeof candidate.code !== 'number' || candidate.code === 0) return undefined
  return {
    code: candidate.code,
    info: typeof candidate.info === 'string' ? candidate.info : '',
  }
}
