import axios from 'axios'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'

import { toApiError, validateApiResponse } from '@/api/error'
import { readStoredAdministratorAccessToken } from '@/stores/adminSession'
import type { ApiEnvelope } from '@/types/api'

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
  (response: AxiosResponse<ApiEnvelope<unknown>>) =>
    validateApiResponse(response),
  (error: unknown) =>
    Promise.reject(
      toApiError(error, {
        timeout: '请求超时，请稍后重试',
        http: () => '教务服务暂时不可用',
        network: '教务服务暂时不可用',
        unknown: '发生未知请求错误',
      }),
    ),
)

export async function unwrapAdmin<T>(
  request: Promise<AxiosResponse<ApiEnvelope<T>>>,
): Promise<T> {
  const response = await request
  return response.data.data
}
