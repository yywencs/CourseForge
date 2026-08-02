import { adminHttp, unwrapAdmin } from '@/api/adminHttp'
import type { ApiEnvelope } from '@/types/api'
import type {
  AdministratorCurrentSessionResponse,
  AdministratorLoginRequest,
  AdministratorLoginResponse,
} from '@/types/auth'

const administratorAuthBasePath = '/admin/v1/auth'

export function loginAdministrator(
  request: AdministratorLoginRequest,
): Promise<AdministratorLoginResponse> {
  return unwrapAdmin(
    adminHttp.post<ApiEnvelope<AdministratorLoginResponse>>(
      `${administratorAuthBasePath}/login`,
      request,
    ),
  )
}

export function getCurrentAdministratorSession(): Promise<AdministratorCurrentSessionResponse> {
  return unwrapAdmin(
    adminHttp.get<ApiEnvelope<AdministratorCurrentSessionResponse>>(
      `${administratorAuthBasePath}/me`,
    ),
  )
}
