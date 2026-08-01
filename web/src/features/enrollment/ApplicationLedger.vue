<script setup lang="ts">
import {
  CheckCircle2,
  RefreshCw,
  Search,
} from '@lucide/vue'
import { computed, shallowRef } from 'vue'

import { queryApplication } from '@/api/enrollment'
import StateBadge from '@/components/StateBadge.vue'
import type { TrackedApplicationView } from '@/composables/useTrackedApplications'
import type { SelectionApplication } from '@/types/enrollment'

const props = defineProps<{
  items: TrackedApplicationView[]
  refreshing?: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

const applicationId = shallowRef('')
const manualRecord = shallowRef<SelectionApplication>()
const lookupError = shallowRef('')
const lookupLoading = shallowRef(false)

const unsettledCount = computed(
  () =>
    props.items.filter(
      (item) => !(item.live?.mysql_persisted ?? item.mysqlPersisted),
    ).length,
)

async function lookup(): Promise<void> {
  const id = applicationId.value.trim()
  if (!id) return
  lookupLoading.value = true
  lookupError.value = ''
  try {
    manualRecord.value = await queryApplication(id)
  } catch (error) {
    manualRecord.value = undefined
    lookupError.value =
      error instanceof Error ? error.message : '申请单查询失败'
  } finally {
    lookupLoading.value = false
  }
}

function displayState(item: TrackedApplicationView): string {
  return item.live?.state ?? item.state
}

function displayFailure(item: TrackedApplicationView): string | undefined {
  return item.live?.failure?.message ?? item.failure?.message
}
</script>

<template>
  <section class="ledger-panel">
    <header class="ledger-panel__heading">
      <div>
        <h2>申请单追踪</h2>
        <p>{{ unsettledCount }} 个申请仍在等待结果确认</p>
      </div>
      <button type="button" :disabled="refreshing" @click="emit('refresh')">
        <RefreshCw :size="15" :class="{ 'is-spinning': refreshing }" />
        刷新状态
      </button>
    </header>

    <form class="application-lookup" @submit.prevent="lookup">
      <Search :size="17" />
      <input
        v-model="applicationId"
        placeholder="输入申请单编号"
        aria-label="申请单 ID"
      />
      <button type="submit" :disabled="lookupLoading">
        {{ lookupLoading ? '查询中…' : '查询' }}
      </button>
    </form>

    <div v-if="manualRecord" class="manual-result">
      <div>
        <strong>{{ manualRecord.application_id }}</strong>
        <span>教学班 #{{ manualRecord.teaching_class_id }} · {{ manualRecord.credits }} 学分</span>
      </div>
      <StateBadge :state="manualRecord.state" />
      <small>
        申请{{ manualRecord.broker_confirmed ? '已受理' : '确认中' }} ·
        结果{{ manualRecord.mysql_persisted ? '已完成' : '处理中' }}
      </small>
    </div>
    <p v-else-if="lookupError" class="lookup-error" role="alert">{{ lookupError }}</p>

    <div v-if="items.length" class="ledger-list">
      <article v-for="item in items" :key="item.applicationId" class="ledger-item">
        <div class="ledger-item__identity">
          <span>{{ item.courseCode }}</span>
          <strong>{{ item.courseName }}</strong>
          <code>{{ item.applicationId }}</code>
        </div>
        <StateBadge :state="displayState(item)" />
        <div class="ledger-item__checks">
          <span :class="{ 'is-done': item.live?.broker_confirmed ?? item.brokerConfirmed }">
            <CheckCircle2 :size="14" />
            申请已受理
          </span>
          <span :class="{ 'is-done': item.live?.mysql_persisted ?? item.mysqlPersisted }">
            <CheckCircle2 :size="14" />
            选课记录已生成
          </span>
        </div>
        <p v-if="displayFailure(item)" class="ledger-item__failure">
          {{ displayFailure(item) }}
        </p>
        <time>{{ new Date(item.submittedAt).toLocaleString('zh-CN') }}</time>
      </article>
    </div>
    <div v-else class="ledger-empty">
      <CheckCircle2 :size="27" />
      <strong>还没有选课申请</strong>
      <span>提交选课后，申请进度会显示在这里。</span>
    </div>
  </section>
</template>

<style scoped>
.ledger-panel {
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.ledger-panel__heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.ledger-panel__heading span {
  color: var(--signal);
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.11em;
  text-transform: uppercase;
}

.ledger-panel__heading h2 {
  margin: 5px 0;
  font-family: var(--font-display);
  font-size: 25px;
}

.ledger-panel__heading p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.ledger-panel__heading button,
.application-lookup button {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 9px 12px;
  border: 1px solid var(--line);
  border-radius: 9px;
  color: var(--brand);
  background: white;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.application-lookup {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 9px;
  margin: 22px 0 14px;
  padding: 7px 8px 7px 13px;
  border: 1px solid var(--line);
  border-radius: 12px;
  color: var(--muted);
  background: var(--surface-muted);
}

.application-lookup input {
  min-width: 0;
  padding: 7px 0;
  border: 0;
  outline: 0;
  background: transparent;
}

.manual-result {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px 18px;
  margin-bottom: 14px;
  padding: 14px;
  border-radius: 12px;
  background: var(--brand-pale);
}

.manual-result > div {
  display: grid;
  gap: 4px;
}

.manual-result span,
.manual-result small {
  color: var(--muted);
  font-size: 11px;
}

.manual-result small {
  grid-column: 1 / -1;
}

.lookup-error {
  margin: 0 0 14px;
  padding: 11px 13px;
  border-radius: 10px;
  color: #8b302b;
  background: #fff0ed;
  font-size: 12px;
}

.ledger-list {
  display: grid;
  gap: 10px;
}

.ledger-item {
  display: grid;
  grid-template-columns: minmax(220px, 1.2fr) auto 1fr auto;
  align-items: center;
  gap: 16px;
  padding: 16px;
  border: 1px solid var(--line-soft);
  border-radius: 13px;
  background: var(--surface);
}

.ledger-item__identity {
  display: grid;
  gap: 3px;
}

.ledger-item__identity > span {
  color: var(--brand);
  font-family: var(--font-mono);
  font-size: 10px;
}

.ledger-item__identity strong {
  font-size: 13px;
}

.ledger-item__identity code {
  overflow: hidden;
  color: var(--muted);
  font-size: 10px;
  text-overflow: ellipsis;
}

.ledger-item__checks {
  display: flex;
  gap: 12px;
}

.ledger-item__checks span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #95a29e;
  font-size: 10px;
}

.ledger-item__checks span.is-done {
  color: var(--success);
}

.ledger-item time {
  color: var(--muted);
  font-size: 10px;
}

.ledger-item__failure {
  grid-column: 1 / -1;
  margin: 0;
  color: var(--danger);
  font-size: 11px;
}

.ledger-empty {
  display: grid;
  justify-items: center;
  gap: 7px;
  padding: 36px 20px;
  color: var(--muted);
  text-align: center;
}

.ledger-empty svg {
  color: var(--brand-bright);
}

.ledger-empty strong {
  color: var(--ink);
}

.ledger-empty span {
  font-size: 12px;
}

.is-spinning {
  animation: spin 800ms linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 850px) {
  .ledger-item {
    grid-template-columns: 1fr auto;
  }

  .ledger-item__checks {
    grid-column: 1 / -1;
  }
}

@media (max-width: 560px) {
  .ledger-panel__heading {
    flex-direction: column;
  }

  .ledger-item {
    grid-template-columns: 1fr;
  }

  .ledger-item__checks,
  .ledger-item__failure {
    grid-column: auto;
  }
}

@media (prefers-reduced-motion: reduce) {
  .is-spinning {
    animation: none;
  }
}
</style>
