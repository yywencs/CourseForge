import axios from 'axios'
import type { AxiosResponse } from 'axios'

import { ApiError, type ApiEnvelope } from '@/types/api'

export const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  timeout: 10_000,
  headers: {
    'Content-Type': 'application/json',
  },
})

http.interceptors.response.use(
  (response: AxiosResponse<ApiEnvelope<unknown>>) => {
    const body = response.data
    if (typeof body?.code === 'number' && body.code !== 0) {
      throw new ApiError(body.code, body.info || '请求失败')
    }
    return response
  },
  (error: unknown) => Promise.reject(error),
)

export async function unwrap<T>(
  request: Promise<AxiosResponse<ApiEnvelope<T>>>,
): Promise<T> {
  const response = await request
  return response.data.data
}
