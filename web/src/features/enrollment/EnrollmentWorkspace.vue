<script setup lang="ts">
import { RefreshCw } from '@lucide/vue'
import { ElMessageBox } from 'element-plus'
import { computed, shallowRef } from 'vue'

import { useEnrollmentWorkspace } from '@/composables/useEnrollmentWorkspace'
import { useTrackedApplications } from '@/composables/useTrackedApplications'
import { courseLabel } from '@/data/courseCatalog'
import type {
  StudentEnrollment,
  WaitlistEntry,
} from '@/types/enrollment'
import ApplicationLedger from './ApplicationLedger.vue'
import EnrollmentList from './EnrollmentList.vue'
import WaitlistList from './WaitlistList.vue'

const activeView = shallowRef<'records' | 'waitlist'>('records')
const {
  enrollmentsQuery,
  waitlistQuery,
  dropMutation,
  cancelWaitlistMutation,
  refreshAll,
} = useEnrollmentWorkspace()
const {
  items: trackedApplications,
  isRefreshing: applicationsRefreshing,
  refresh: refreshApplications,
} = useTrackedApplications()

const droppingId = computed(
  () => dropMutation.variables.value?.enrollment_id,
)
const cancellingId = computed(
  () => cancelWaitlistMutation.variables.value?.waitlist_id,
)
const enrollmentError = computed(() =>
  enrollmentsQuery.error.value instanceof Error
    ? enrollmentsQuery.error.value.message
    : enrollmentsQuery.isError.value
      ? '选课记录读取失败，请稍后刷新'
      : undefined,
)
const waitlistError = computed(() =>
  waitlistQuery.error.value instanceof Error
    ? waitlistQuery.error.value.message
    : waitlistQuery.isError.value
      ? '候补队列读取失败，请稍后刷新'
      : undefined,
)

async function confirmDrop(record: StudentEnrollment): Promise<void> {
  const courseName = courseLabel(record.teaching_class_id)
  try {
    await ElMessageBox.confirm(
      '退课后，这门课将从你的本学期课程和课表中移除。',
      `确定退掉《${courseName}》吗？`,
      {
        confirmButtonText: '确认退课',
        cancelButtonText: '暂不退课',
        type: 'warning',
      },
    )
    dropMutation.mutate(record)
  } catch {
    // 用户取消确认时无需产生提示。
  }
}

async function confirmCancelWaitlist(entry: WaitlistEntry): Promise<void> {
  const courseName = courseLabel(entry.teaching_class_id)
  try {
    await ElMessageBox.confirm(
      '取消后，你将不再参与这门课的候补；如有需要，可以重新加入。',
      `确定取消《${courseName}》的候补吗？`,
      {
        confirmButtonText: '确认取消',
        cancelButtonText: '继续候补',
        type: 'warning',
      },
    )
    cancelWaitlistMutation.mutate(entry)
  } catch {
    // 用户取消确认时无需产生提示。
  }
}

async function refresh(): Promise<void> {
  await Promise.all([refreshAll(), refreshApplications(true)])
}
</script>

<template>
  <div class="workspace">
    <div class="workspace-summary">
      <div>
        <span>本学期正式记录</span>
        <strong>{{ enrollmentsQuery.data.value?.total ?? 0 }}</strong>
      </div>
      <div>
        <span>有效候补</span>
        <strong>
          {{
            (waitlistQuery.data.value?.items ?? []).filter((item) =>
              ['waiting', 'promoting'].includes(item.state),
            ).length
          }}
        </strong>
      </div>
      <div>
        <span>最近申请</span>
        <strong>{{ trackedApplications.length }}</strong>
      </div>
      <button
        type="button"
        :disabled="enrollmentsQuery.isFetching.value || waitlistQuery.isFetching.value"
        @click="refresh"
      >
        <RefreshCw :size="16" />
        刷新全部
      </button>
    </div>

    <ApplicationLedger
      :items="trackedApplications"
      :refreshing="applicationsRefreshing"
      @refresh="refreshApplications(true)"
    />

    <div class="workspace-tabs" role="tablist" aria-label="选课记录分类">
      <button
        type="button"
        role="tab"
        :aria-selected="activeView === 'records'"
        :class="{ 'is-active': activeView === 'records' }"
        @click="activeView = 'records'"
      >
        正式选课
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeView === 'waitlist'"
        :class="{ 'is-active': activeView === 'waitlist' }"
        @click="activeView = 'waitlist'"
      >
        候补队列
      </button>
    </div>

    <EnrollmentList
      v-if="activeView === 'records'"
      :items="enrollmentsQuery.data.value?.items ?? []"
      :loading="enrollmentsQuery.isLoading.value"
      :dropping-id="droppingId"
      :error="enrollmentError"
      @drop="confirmDrop"
    />
    <WaitlistList
      v-else
      :items="waitlistQuery.data.value?.items ?? []"
      :loading="waitlistQuery.isLoading.value"
      :cancelling-id="cancellingId"
      :error="waitlistError"
      @cancel="confirmCancelWaitlist"
    />
  </div>
</template>

<style scoped>
.workspace {
  display: grid;
  gap: 18px;
}

.workspace-summary {
  display: grid;
  grid-template-columns: repeat(3, 1fr) auto;
  gap: 10px;
}

.workspace-summary > div {
  display: grid;
  gap: 4px;
  padding: 15px 17px;
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--ink);
  background: var(--surface);
}

.workspace-summary span {
  color: var(--muted);
  font-size: 10px;
}

.workspace-summary strong {
  font-family: var(--font-display);
  color: var(--ink);
  font-size: 24px;
  font-variation-settings: "wdth" 76, "wght" 680;
}

.workspace-summary > button {
  display: flex;
  align-items: center;
  align-self: stretch;
  gap: 7px;
  padding: 0 16px;
  border: 1px solid var(--line);
  border-radius: 10px;
  color: white;
  background: var(--brand);
  font-size: 12px;
  font-weight: 750;
  cursor: pointer;
}

.workspace-tabs {
  display: flex;
  width: fit-content;
  gap: 5px;
  padding: 4px;
  border: 1px solid var(--line);
  border-radius: 11px;
  background: var(--surface-muted);
}

.workspace-tabs button {
  padding: 8px 14px;
  border: 0;
  border-radius: 8px;
  color: var(--muted);
  background: transparent;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.workspace-tabs button.is-active {
  color: var(--ink);
  background: white;
  box-shadow: 0 4px 12px rgba(16, 48, 40, 0.08);
}

@media (max-width: 700px) {
  .workspace-summary {
    grid-template-columns: repeat(3, 1fr);
  }

  .workspace-summary > button {
    grid-column: 1 / -1;
    justify-content: center;
    padding-block: 12px;
  }
}
</style>
