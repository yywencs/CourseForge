import { adminHttp, unwrapAdmin } from '@/api/adminHttp'
import { http, unwrap } from '@/api/http'
import type {
  Course,
  CourseDraft,
  RoundClassBinding,
  SelectionRound,
  SelectionRoundDraft,
  TeachingClass,
  TeachingClassDraft,
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
