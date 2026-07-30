import axios from 'axios'

import type {
  AdminStatus,
  ApiEnvelope,
  HealthStatus,
  ReadinessStatus,
} from '@/types/api'

const apiSystemHttp = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  timeout: 6_000,
})

const adminSystemHttp = axios.create({
  baseURL: import.meta.env.VITE_ADMIN_API_BASE_URL || '/admin-api',
  timeout: 6_000,
})

export async function queryApiHealth(): Promise<HealthStatus> {
  const response = await apiSystemHttp.get<HealthStatus>('/healthz')
  return response.data
}

export async function queryApiReadiness(): Promise<ReadinessStatus> {
  try {
    const response = await apiSystemHttp.get<ReadinessStatus>('/readyz')
    return response.data
  } catch (error) {
    if (axios.isAxiosError<ReadinessStatus>(error) && error.response?.data) {
      return error.response.data
    }
    throw error
  }
}

export async function queryAdminStatus(): Promise<AdminStatus> {
  const response =
    await adminSystemHttp.get<ApiEnvelope<AdminStatus>>('/admin/v1/status')
  if (response.data.code !== 0) {
    throw new Error(response.data.info)
  }
  return response.data.data
}

export async function queryAdminReadiness(): Promise<ReadinessStatus> {
  try {
    const response = await adminSystemHttp.get<ReadinessStatus>('/readyz')
    return response.data
  } catch (error) {
    if (axios.isAxiosError<ReadinessStatus>(error) && error.response?.data) {
      return error.response.data
    }
    throw error
  }
}
