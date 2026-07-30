import { http, unwrap } from './http'
import type { ApiEnvelope } from '@/types/api'
import type {
  DropEnrollmentReceipt,
  JoinWaitlistRequest,
  PageResult,
  SelectCourseRequest,
  SelectionApplication,
  SelectionReceipt,
  StudentEnrollment,
  WaitlistEntry,
} from '@/types/enrollment'

const enrollmentBasePath = '/api/v1/enrollments'

export function selectCourse(
  request: SelectCourseRequest,
): Promise<SelectionReceipt> {
  return unwrap(
    http.post<ApiEnvelope<SelectionReceipt>>(enrollmentBasePath, request),
  )
}

export function queryApplication(
  applicationId: string,
): Promise<SelectionApplication> {
  return unwrap(
    http.get<ApiEnvelope<SelectionApplication>>(
      `${enrollmentBasePath}/applications/${applicationId}`,
    ),
  )
}

export function listMyEnrollments(
  termId: number,
  limit = 100,
  offset = 0,
): Promise<PageResult<StudentEnrollment>> {
  return unwrap(
    http.get<ApiEnvelope<PageResult<StudentEnrollment>>>(
      `${enrollmentBasePath}/me`,
      { params: { term_id: termId, limit, offset } },
    ),
  )
}

export function dropEnrollment(
  enrollmentId: string,
): Promise<DropEnrollmentReceipt> {
  return unwrap(
    http.delete<ApiEnvelope<DropEnrollmentReceipt>>(
      `${enrollmentBasePath}/${enrollmentId}`,
    ),
  )
}

export function joinWaitlist(
  request: JoinWaitlistRequest,
): Promise<WaitlistEntry> {
  return unwrap(
    http.post<ApiEnvelope<WaitlistEntry>>(
      `${enrollmentBasePath}/waitlist`,
      request,
    ),
  )
}

export function queryWaitlist(waitlistId: string): Promise<WaitlistEntry> {
  return unwrap(
    http.get<ApiEnvelope<WaitlistEntry>>(
      `${enrollmentBasePath}/waitlist/${waitlistId}`,
    ),
  )
}

export function listMyWaitlist(
  termId: number,
  limit = 100,
  offset = 0,
): Promise<PageResult<WaitlistEntry>> {
  return unwrap(
    http.get<ApiEnvelope<PageResult<WaitlistEntry>>>(
      `${enrollmentBasePath}/waitlist/me`,
      { params: { term_id: termId, limit, offset } },
    ),
  )
}

export function cancelWaitlist(waitlistId: string): Promise<WaitlistEntry> {
  return unwrap(
    http.delete<ApiEnvelope<WaitlistEntry>>(
      `${enrollmentBasePath}/waitlist/${waitlistId}`,
    ),
  )
}
