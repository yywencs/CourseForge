import { adminHttp, unwrapAdmin } from '@/api/adminHttp'
import { http, unwrap } from '@/api/http'
import type {
  Course,
  CourseDraft,
  CourseVideo,
  RoundClassBinding,
  SelectionRound,
  SelectionRoundDraft,
  TeachingClass,
  TeachingClassDraft,
  UploadedVideoPart,
  VideoPlaybackTicket,
  VideoUploadPartTicket,
  VideoUploadTicket,
} from '@/types/catalog'

interface ItemList<T> {
  items: T[]
}

export function listStudentCatalog(roundId: number, keyword = ''): Promise<ItemList<TeachingClass>> {
  return unwrap(http.get('/api/v1/catalog/teaching-classes', {
    params: { round_id: roundId, keyword: keyword || undefined },
  }))
}

export function listCourses(keyword = ''): Promise<ItemList<Course>> {
  return unwrapAdmin(adminHttp.get('/admin/v1/courses', { params: { keyword: keyword || undefined } }))
}

export function createCourse(input: CourseDraft): Promise<Course> {
  return unwrapAdmin(adminHttp.post('/admin/v1/courses', input))
}

export function updateCourse(id: number, input: CourseDraft): Promise<Course> {
  return unwrapAdmin(adminHttp.put(`/admin/v1/courses/${id}`, input))
}

export function deleteCourse(id: number): Promise<{ deleted: boolean }> {
  return unwrapAdmin(adminHttp.delete(`/admin/v1/courses/${id}`))
}

export function listCourseVideos(courseId: number): Promise<ItemList<CourseVideo>> {
  return unwrapAdmin(adminHttp.get(`/admin/v1/courses/${courseId}/videos`))
}

export async function startCourseVideoUpload(courseId: number, input: {
  video_kind: 'preview' | 'lesson'
  title: string
  file_name: string
  content_type: string
  file_size: number
  sort_order: number
}): Promise<VideoUploadTicket> {
  const ticket = await unwrapAdmin<VideoUploadTicket>(
    adminHttp.post(`/admin/v1/courses/${courseId}/videos/uploads`, input),
  )
  if (!Number.isSafeInteger(ticket?.upload_id) || ticket.upload_id <= 0 ||
    !ticket.multipart_upload_id || !Number.isSafeInteger(ticket.part_size_bytes) ||
    ticket.part_size_bytes <= 0 || !Array.isArray(ticket.parts)) {
    throw new Error('视频上传接口返回不完整，请确认后端已更新并重新启动')
  }
  return ticket
}

export function presignCourseVideoUploadParts(
  uploadId: number,
  multipartUploadId: string,
  partNumbers: number[],
): Promise<{ parts: VideoUploadPartTicket[] }> {
  return unwrapAdmin(adminHttp.post(`/admin/v1/course-video-uploads/${uploadId}/parts/presign`, {
    multipart_upload_id: multipartUploadId,
    part_numbers: partNumbers,
  }))
}

export function listCourseVideoUploadParts(uploadId: number): Promise<{ parts: UploadedVideoPart[] }> {
  return unwrapAdmin(adminHttp.get(`/admin/v1/course-video-uploads/${uploadId}/parts`))
}

export function completeCourseVideoUpload(uploadId: number, durationMs?: number): Promise<CourseVideo> {
  return unwrapAdmin(adminHttp.post(`/admin/v1/course-video-uploads/${uploadId}/complete`, {
    duration_ms: durationMs,
  }))
}

export function getCourseVideoPlayback(videoId: number): Promise<VideoPlaybackTicket> {
  return unwrap(http.get(`/api/v1/course-videos/${videoId}/playback`))
}

export function listTeachingClasses(termId?: number, keyword = ''): Promise<ItemList<TeachingClass>> {
  return unwrapAdmin(adminHttp.get('/admin/v1/teaching-classes', {
    params: { term_id: termId || undefined, keyword: keyword || undefined },
  }))
}

export function createTeachingClass(input: TeachingClassDraft): Promise<TeachingClass> {
  return unwrapAdmin(adminHttp.post('/admin/v1/teaching-classes', input))
}

export function updateTeachingClass(id: number, input: TeachingClassDraft): Promise<TeachingClass> {
  return unwrapAdmin(adminHttp.put(`/admin/v1/teaching-classes/${id}`, input))
}

export function deleteTeachingClass(id: number): Promise<{ deleted: boolean }> {
  return unwrapAdmin(adminHttp.delete(`/admin/v1/teaching-classes/${id}`))
}

export function listSelectionRounds(termId?: number): Promise<ItemList<SelectionRound>> {
  return unwrapAdmin(adminHttp.get('/admin/v1/selection-rounds', {
    params: { term_id: termId || undefined },
  }))
}

export function createSelectionRound(input: SelectionRoundDraft): Promise<SelectionRound> {
  return unwrapAdmin(adminHttp.post('/admin/v1/selection-rounds', input))
}

export function updateSelectionRound(id: number, input: SelectionRoundDraft): Promise<SelectionRound> {
  return unwrapAdmin(adminHttp.put(`/admin/v1/selection-rounds/${id}`, input))
}

export function deleteSelectionRound(id: number): Promise<{ deleted: boolean }> {
  return unwrapAdmin(adminHttp.delete(`/admin/v1/selection-rounds/${id}`))
}

export function listRoundClasses(roundId: number): Promise<ItemList<RoundClassBinding>> {
  return unwrapAdmin(adminHttp.get(`/admin/v1/selection-rounds/${roundId}/teaching-classes`))
}

export function bindRoundClass(roundId: number, teachingClassId: number): Promise<{ bound: boolean }> {
  return unwrapAdmin(adminHttp.post(`/admin/v1/selection-rounds/${roundId}/teaching-classes/${teachingClassId}`))
}

export function unbindRoundClass(roundId: number, teachingClassId: number): Promise<{ unbound: boolean }> {
  return unwrapAdmin(adminHttp.delete(`/admin/v1/selection-rounds/${roundId}/teaching-classes/${teachingClassId}`))
}
