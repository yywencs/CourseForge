import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { shallowRef } from 'vue'

import { joinWaitlist, selectCourse } from '@/api/enrollment'
import { useEnrollmentTrackerStore } from '@/stores/enrollmentTracker'
import { ApiError } from '@/types/api'
import type { TeachingClassSummary } from '@/types/enrollment'

export function useCatalogActions() {
  const queryClient = useQueryClient()
  const tracker = useEnrollmentTrackerStore()
  const activeTeachingClassId = shallowRef<number | null>(null)
  const activeAction = shallowRef<'select' | 'waitlist' | null>(null)

  const selectionMutation = useMutation({
    mutationFn: async (course: TeachingClassSummary) => {
      activeTeachingClassId.value = course.id
      activeAction.value = 'select'
      const pending = tracker.begin(course)
      try {
        const receipt = await selectCourse({
          request_id: pending.requestId,
          round_id: course.roundId,
          teaching_class_id: course.id,
        })
        tracker.complete(course, pending, receipt)
        return receipt
      } catch (error) {
        const shouldKeepRequest =
          error instanceof ApiError &&
          (error.transport || error.message.includes('处理中'))
        if (!shouldKeepRequest) {
          tracker.discardSelection(pending.requestId)
        }
        throw error
      }
    },
    onSuccess: (receipt) => {
      const message = receipt.mysql_persisted
        ? '选课完成，正式记录已生成'
        : '选课申请已受理，结果确认中'
      ElMessage.success(message)
      void queryClient.invalidateQueries({ queryKey: ['my-enrollments'] })
    },
    onError: (error: Error) => {
      ElMessage.error(error.message || '选课提交失败')
    },
    onSettled: () => {
      activeTeachingClassId.value = null
      activeAction.value = null
    },
  })

  const waitlistMutation = useMutation({
    mutationFn: async (course: TeachingClassSummary) => {
      activeTeachingClassId.value = course.id
      activeAction.value = 'waitlist'
      const pending = tracker.beginWaitlist(course)
      try {
        const entry = await joinWaitlist({
          request_id: pending.requestId,
          round_id: course.roundId,
          teaching_class_id: course.id,
        })
        tracker.finishWaitlist(pending.requestId)
        return entry
      } catch (error) {
        if (!(error instanceof ApiError && error.transport)) {
          tracker.finishWaitlist(pending.requestId)
        }
        throw error
      }
    },
    onSuccess: (entry) => {
      ElMessage.success(`已加入候补，当前排位 ${entry.position}`)
      void queryClient.invalidateQueries({ queryKey: ['my-waitlist'] })
    },
    onError: (error: Error) => {
      ElMessage.error(error.message || '加入候补失败')
    },
    onSettled: () => {
      activeTeachingClassId.value = null
      activeAction.value = null
    },
  })

  return {
    activeTeachingClassId,
    activeAction,
    selectionMutation,
    waitlistMutation,
  }
}
