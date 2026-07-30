import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { computed } from 'vue'

import {
  cancelWaitlist,
  dropEnrollment,
  listMyEnrollments,
  listMyWaitlist,
} from '@/api/enrollment'
import { useSessionStore } from '@/stores/session'
import type {
  StudentEnrollment,
  WaitlistEntry,
} from '@/types/enrollment'

export function useEnrollmentWorkspace() {
  const session = useSessionStore()
  const queryClient = useQueryClient()
  const termId = computed(() => session.context.termId)
  const queryEnabled = computed(
    () => session.isAuthenticated && termId.value > 0,
  )

  const enrollmentsQuery = useQuery({
    queryKey: computed(() => ['my-enrollments', session.studentId, termId.value]),
    queryFn: () => listMyEnrollments(termId.value),
    enabled: queryEnabled,
    staleTime: 2_000,
  })

  const waitlistQuery = useQuery({
    queryKey: computed(() => ['my-waitlist', session.studentId, termId.value]),
    queryFn: () => listMyWaitlist(termId.value),
    enabled: queryEnabled,
    staleTime: 2_000,
  })

  async function refreshAll(): Promise<void> {
    await Promise.all([
      enrollmentsQuery.refetch(),
      waitlistQuery.refetch(),
    ])
  }

  const dropMutation = useMutation({
    mutationFn: (record: StudentEnrollment) =>
      dropEnrollment(record.enrollment_id),
    onSuccess: (receipt) => {
      const projection = receipt.redis_released
        ? '名额与额度已返还'
        : '正式记录已退课，名额正在后台修复'
      ElMessage.success(projection)
      void queryClient.invalidateQueries({ queryKey: ['my-enrollments'] })
      void queryClient.invalidateQueries({ queryKey: ['my-waitlist'] })
    },
    onError: (error: Error) => {
      ElMessage.error(error.message || '退课失败')
    },
  })

  const cancelWaitlistMutation = useMutation({
    mutationFn: (entry: WaitlistEntry) => cancelWaitlist(entry.waitlist_id),
    onSuccess: () => {
      ElMessage.success('候补已取消')
      void queryClient.invalidateQueries({ queryKey: ['my-waitlist'] })
    },
    onError: (error: Error) => {
      ElMessage.error(error.message || '取消候补失败')
    },
  })

  return {
    termId,
    enrollmentsQuery,
    waitlistQuery,
    dropMutation,
    cancelWaitlistMutation,
    refreshAll,
  }
}
