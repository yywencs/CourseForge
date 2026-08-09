import { defineStore } from 'pinia'
import { shallowRef } from 'vue'

import type {
  PendingSelection,
  PendingWaitlist,
  SelectionApplication,
  SelectionReceipt,
  TeachingClassSummary,
  TrackedApplication,
} from '@/types/enrollment'
import { createRequestId } from '@/utils/requestId'

const trackedStorageKey = 'courseforge.tracked-applications'
const pendingStorageKey = 'courseforge.pending-selections'
const pendingWaitlistStorageKey = 'courseforge.pending-waitlist'

function readArray<T>(key: string): T[] {
  if (typeof window === 'undefined') return []
  try {
    const value = JSON.parse(window.localStorage.getItem(key) ?? '[]')
    return Array.isArray(value) ? (value as T[]) : []
  } catch {
    return []
  }
}

function persist<T>(key: string, value: T[]): void {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(key, JSON.stringify(value))
}

type StoredTrackedApplication = Omit<TrackedApplication, 'streamRecorded'> & {
  brokerConfirmed?: boolean
  streamRecorded?: boolean
}

function readTrackedApplications(): TrackedApplication[] {
  return readArray<StoredTrackedApplication>(trackedStorageKey).map((stored) => {
    const { brokerConfirmed, ...application } = stored
    return {
      ...application,
      streamRecorded: stored.streamRecorded ?? brokerConfirmed ?? false,
    }
  })
}

export const useEnrollmentTrackerStore = defineStore(
  'enrollment-tracker',
  () => {
    const applications = shallowRef<TrackedApplication[]>(
      readTrackedApplications(),
    )
    const pendingSelections = shallowRef<PendingSelection[]>(
      readArray<PendingSelection>(pendingStorageKey),
    )
    const pendingWaitlist = shallowRef<PendingWaitlist[]>(
      readArray<PendingWaitlist>(pendingWaitlistStorageKey),
    )

    function begin(course: TeachingClassSummary): PendingSelection {
      const existing = pendingSelections.value.find(
        (item) =>
          item.teachingClassId === course.id &&
          item.roundId === course.roundId,
      )
      if (existing) return existing

      const pending: PendingSelection = {
        requestId: createRequestId(),
        teachingClassId: course.id,
        roundId: course.roundId,
        courseName: course.courseName,
        createdAt: new Date().toISOString(),
      }
      pendingSelections.value = [pending, ...pendingSelections.value].slice(0, 30)
      persist(pendingStorageKey, pendingSelections.value)
      return pending
    }

    function complete(
      course: TeachingClassSummary,
      pending: PendingSelection,
      receipt: SelectionReceipt,
    ): void {
      const tracked: TrackedApplication = {
        applicationId: receipt.application_id,
        requestId: pending.requestId,
        teachingClassId: course.id,
        courseName: course.courseName,
        courseCode: course.courseCode,
        state: receipt.state,
        streamRecorded: receipt.stream_recorded,
        mysqlPersisted: receipt.mysql_persisted,
        submittedAt: pending.createdAt,
      }
      applications.value = [
        tracked,
        ...applications.value.filter(
          (item) => item.applicationId !== tracked.applicationId,
        ),
      ].slice(0, 30)
      pendingSelections.value = pendingSelections.value.filter(
        (item) => item.requestId !== pending.requestId,
      )
      persist(trackedStorageKey, applications.value)
      persist(pendingStorageKey, pendingSelections.value)
    }

    function discardSelection(requestId: string): void {
      pendingSelections.value = pendingSelections.value.filter(
        (item) => item.requestId !== requestId,
      )
      persist(pendingStorageKey, pendingSelections.value)
    }

    function beginWaitlist(course: TeachingClassSummary): PendingWaitlist {
      const existing = pendingWaitlist.value.find(
        (item) =>
          item.teachingClassId === course.id &&
          item.roundId === course.roundId,
      )
      if (existing) return existing

      const pending: PendingWaitlist = {
        requestId: createRequestId(),
        teachingClassId: course.id,
        roundId: course.roundId,
        courseName: course.courseName,
        createdAt: new Date().toISOString(),
      }
      pendingWaitlist.value = [pending, ...pendingWaitlist.value].slice(0, 30)
      persist(pendingWaitlistStorageKey, pendingWaitlist.value)
      return pending
    }

    function finishWaitlist(requestId: string): void {
      pendingWaitlist.value = pendingWaitlist.value.filter(
        (item) => item.requestId !== requestId,
      )
      persist(pendingWaitlistStorageKey, pendingWaitlist.value)
    }

    function sync(application: SelectionApplication): void {
      applications.value = applications.value.map((item) =>
        item.applicationId === application.application_id
          ? {
              ...item,
              state: application.state,
              streamRecorded: application.stream_recorded,
              mysqlPersisted: application.mysql_persisted,
              failure: application.failure,
            }
          : item,
      )
      persist(trackedStorageKey, applications.value)
    }

    function clear(): void {
      applications.value = []
      pendingSelections.value = []
      pendingWaitlist.value = []
      persist(trackedStorageKey, [])
      persist(pendingStorageKey, [])
      persist(pendingWaitlistStorageKey, [])
    }

    return {
      applications,
      pendingSelections,
      pendingWaitlist,
      begin,
      complete,
      discardSelection,
      beginWaitlist,
      finishWaitlist,
      sync,
      clear,
    }
  },
)
