import { useQuery } from '@tanstack/vue-query'
import { computed, watch } from 'vue'

import { listStudentCatalog } from '@/api/catalog'
import { courseCatalog, replaceCourseCatalog } from '@/data/courseCatalog'
import { useSessionStore } from '@/stores/session'

// useStudentCatalog 统一加载学生端课程目录，并维护跨学生页面共享的教学班展示信息。
export function useStudentCatalog() {
  const session = useSessionStore()
  const roundId = computed(() => session.context.roundId)
  const catalogQuery = useQuery({
    queryKey: computed(() => ['course-catalog', roundId.value]),
    queryFn: () => listStudentCatalog(roundId.value),
    enabled: computed(() => session.isAuthenticated && roundId.value > 0),
  })

  watch(
    [() => catalogQuery.data.value?.items, roundId],
    ([items, currentRoundId]) => replaceCourseCatalog(items ?? [], currentRoundId),
    { immediate: true },
  )

  const courses = computed(() => courseCatalog(roundId.value))

  return {
    catalogQuery,
    courses,
  }
}
