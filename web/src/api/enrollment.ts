import { http, unwrap } from './http'
import type {
  SelectCourseRequest,
  SelectionReceipt,
} from '@/types/enrollment'
import type { ApiEnvelope } from '@/types/api'

export function selectCourse(
  request: SelectCourseRequest,
): Promise<SelectionReceipt> {
  return unwrap(
    http.post<ApiEnvelope<SelectionReceipt>>('/api/v1/enrollments', request),
  )
}
