import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useEnrollmentTrackerStore } from './enrollmentTracker'
import type { TeachingClassSummary } from '@/types/enrollment'

const course: TeachingClassSummary = {
  id: 20001,
  roundId: 1,
  courseId: 10001,
  courseCode: 'CS-304',
  courseName: '分布式系统设计',
  teacherName: '周老师',
  credits: 3.5,
  schedule: '周二',
  location: 'A308',
  capacity: 60,
  selectedCount: 42,
  tags: [],
  introduction: '',
  hasVideo: false,
}

describe('enrollment tracker', () => {
  beforeEach(() => {
    window.localStorage.clear()
    setActivePinia(createPinia())
  })

  it('reuses the same selection request id until a result is received', () => {
    const store = useEnrollmentTrackerStore()

    const first = store.begin(course)
    const retried = store.begin(course)

    expect(retried.requestId).toBe(first.requestId)
    expect(store.pendingSelections).toHaveLength(1)
  })

  it('reuses the same waitlist request id after an uncertain transport result', () => {
    const store = useEnrollmentTrackerStore()

    const first = store.beginWaitlist(course)
    const retried = store.beginWaitlist(course)

    expect(retried.requestId).toBe(first.requestId)
    store.finishWaitlist(first.requestId)
    expect(store.pendingWaitlist).toHaveLength(0)
  })
})
