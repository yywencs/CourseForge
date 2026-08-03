import axios from 'axios'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'

import { toApiError, validateApiResponse } from '@/api/error'
import { readStoredAccessToken } from '@/stores/session'
import type { ApiEnvelope } from '@/types/api'

export const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  timeout: 10_000,
  headers: {
    'Content-Type': 'application/json',
  },
})

http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const accessToken = readStoredAccessToken()
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`
  }
  return config
})

http.interceptors.response.use(
  (response: AxiosResponse<ApiEnvelope<unknown>>) =>
    validateApiResponse(response),
  (error: unknown) =>
    Promise.reject(
      toApiError(error, {
        timeout: '请求超时，请检查服务状态后重试',
        http: (status) => `服务返回 ${status}`,
        network: '服务暂时不可用，请稍后重试',
        unknown: '发生未知请求错误',
      }),
    ),
)

export async function unwrap<T>(
  request: Promise<AxiosResponse<ApiEnvelope<T>>>,
): Promise<T> {
  const response = await request
  return response.data.data
}
