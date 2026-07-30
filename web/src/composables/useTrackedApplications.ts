import { computed, onScopeDispose, shallowRef, watch } from 'vue'

import { queryApplication } from '@/api/enrollment'
import { useEnrollmentTrackerStore } from '@/stores/enrollmentTracker'
import type {
  SelectionApplication,
  TrackedApplication,
} from '@/types/enrollment'

export interface TrackedApplicationView extends TrackedApplication {
  live?: SelectionApplication
}

export function useTrackedApplications() {
  const tracker = useEnrollmentTrackerStore()
  const liveRecords = shallowRef<Record<string, SelectionApplication>>({})
  const isRefreshing = shallowRef(false)

  const items = computed<TrackedApplicationView[]>(() =>
    tracker.applications.map((tracked) => ({
      ...tracked,
      live: liveRecords.value[tracked.applicationId],
    })),
  )

  async function refresh(force = false): Promise<void> {
    const targets = tracker.applications.filter(
      (item) => force || !item.mysqlPersisted,
    )
    if (targets.length === 0) return
    isRefreshing.value = true
    try {
      const settled = await Promise.allSettled(
        targets.map((item) => queryApplication(item.applicationId)),
      )
      const next = { ...liveRecords.value }
      settled.forEach((result, index) => {
        if (result.status !== 'fulfilled') return
        const target = targets[index]
        if (!target) return
        next[target.applicationId] = result.value
        tracker.sync(result.value)
      })
      liveRecords.value = next
    } finally {
      isRefreshing.value = false
    }
  }

  watch(
    () => tracker.applications.map((item) => item.applicationId).join(','),
    () => {
      void refresh(true)
    },
    { immediate: true },
  )

  const timer = window.setInterval(() => {
    void refresh(false)
  }, 2_500)
  onScopeDispose(() => window.clearInterval(timer))

  return {
    items,
    isRefreshing,
    refresh,
  }
}
