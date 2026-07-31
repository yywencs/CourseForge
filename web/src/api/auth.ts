import { http, unwrap } from './http'
import type { ApiEnvelope } from '@/types/api'
import type {
  CurrentSessionResponse,
  LoginRequest,
  LoginResponse,
} from '@/types/auth'

const authBasePath = '/api/v1/auth'

export function login(request: LoginRequest): Promise<LoginResponse> {
  return unwrap(
    http.post<ApiEnvelope<LoginResponse>>(`${authBasePath}/login`, request),
  )
}

export function getCurrentSession(): Promise<CurrentSessionResponse> {
  return unwrap(
    http.get<ApiEnvelope<CurrentSessionResponse>>(`${authBasePath}/me`),
  )
}
