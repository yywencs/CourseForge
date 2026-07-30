import axios, { AxiosError } from 'axios'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'

import { readStoredAccessToken } from '@/stores/session'
import { ApiError, type ApiEnvelope } from '@/types/api'

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
  (response: AxiosResponse<ApiEnvelope<unknown>>) => {
    const body = response.data
    if (typeof body?.code === 'number' && body.code !== 0) {
      throw new ApiError(body.code, body.info || '请求失败')
    }
    return response
  },
  (error: unknown) => {
    if (error instanceof ApiError) {
      return Promise.reject(error)
    }
    if (error instanceof AxiosError) {
      const message =
        error.code === 'ECONNABORTED'
          ? '请求超时，请检查服务状态后重试'
          : error.response
            ? `服务返回 ${error.response.status}`
            : '无法连接后端服务'
      return Promise.reject(
        new ApiError(error.response?.status ?? 0, message, true),
      )
    }
    return Promise.reject(new ApiError(0, '发生未知请求错误', true))
  },
)

export async function unwrap<T>(
  request: Promise<AxiosResponse<ApiEnvelope<T>>>,
): Promise<T> {
  const response = await request
  return response.data.data
}
