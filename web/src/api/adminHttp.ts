import axios, { AxiosError } from 'axios'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'

import { readStoredAdministratorAccessToken } from '@/stores/adminSession'
import { ApiError, type ApiEnvelope } from '@/types/api'

export const adminHttp = axios.create({
  baseURL: import.meta.env.VITE_ADMIN_API_BASE_URL || '/admin-api',
  timeout: 10_000,
  headers: { 'Content-Type': 'application/json' },
})

adminHttp.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const accessToken = readStoredAdministratorAccessToken()
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`
  }
  return config
})

adminHttp.interceptors.response.use(
  (response: AxiosResponse<ApiEnvelope<unknown>>) => {
    const body = response.data
    if (typeof body?.code === 'number' && body.code !== 0) {
      throw new ApiError(body.code, body.info || '请求失败')
    }
    return response
  },
  (error: unknown) => {
    if (error instanceof ApiError) return Promise.reject(error)
    if (error instanceof AxiosError) {
      return Promise.reject(
        new ApiError(
          error.response?.status ?? 0,
          error.code === 'ECONNABORTED'
            ? '请求超时，请稍后重试'
            : '教务服务暂时不可用',
          true,
        ),
      )
    }
    return Promise.reject(new ApiError(0, '发生未知请求错误', true))
  },
)

export async function unwrapAdmin<T>(
  request: Promise<AxiosResponse<ApiEnvelope<T>>>,
): Promise<T> {
  const response = await request
  return response.data.data
}
