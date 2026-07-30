<script setup lang="ts">
import { BookOpenCheck, CalendarClock, LogOut } from '@lucide/vue'

import StateBadge from '@/components/StateBadge.vue'
import { courseLabel } from '@/data/courseCatalog'
import type { StudentEnrollment } from '@/types/enrollment'

defineProps<{
  items: StudentEnrollment[]
  loading?: boolean
  droppingId?: string
  error?: string
}>()

const emit = defineEmits<{
  drop: [record: StudentEnrollment]
}>()
</script>

<template>
  <section class="records-panel">
    <header>
      <div>
        <span>Official records</span>
        <h2>正式选课</h2>
      </div>
      <strong>{{ items.filter((item) => item.state === 'enrolled').length }} 门修读中</strong>
    </header>

    <div v-if="loading" class="records-state">正在读取正式选课记录…</div>
    <div v-else-if="error" class="records-state records-state--error" role="alert">
      {{ error }}
    </div>
    <div v-else-if="items.length" class="record-list">
      <article v-for="item in items" :key="item.enrollment_id" class="record-item">
        <div class="record-item__icon"><BookOpenCheck :size="20" /></div>
        <div class="record-item__main">
          <strong>{{ courseLabel(item.teaching_class_id) }}</strong>
          <code>{{ item.enrollment_id }}</code>
        </div>
        <div class="record-item__meta">
          <span>{{ item.credits }} 学分</span>
          <span><CalendarClock :size="13" /> {{ new Date(item.enrolled_at).toLocaleString('zh-CN') }}</span>
        </div>
        <StateBadge :state="item.state" />
        <button
          v-if="item.state === 'enrolled'"
          type="button"
          :disabled="droppingId === item.enrollment_id"
          @click="emit('drop', item)"
        >
          <LogOut :size="14" />
          {{ droppingId === item.enrollment_id ? '退课中…' : '退课' }}
        </button>
      </article>
    </div>
    <div v-else class="records-state">
      当前学期还没有正式选课记录。选课结果异步落库后会出现在这里。
    </div>
  </section>
</template>

<style scoped>
.records-panel {
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.records-panel header {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 18px;
  margin-bottom: 18px;
}

.records-panel header span {
  color: #98630c;
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.11em;
  text-transform: uppercase;
}

.records-panel h2 {
  margin: 5px 0 0;
  font-family: var(--font-display);
  font-size: 25px;
}

.records-panel header > strong {
  color: var(--brand);
  font-size: 12px;
}

.record-list {
  display: grid;
  gap: 9px;
}

.record-item {
  display: grid;
  grid-template-columns: auto minmax(180px, 1fr) 1fr auto auto;
  align-items: center;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--line-soft);
  border-radius: 12px;
}

.record-item__icon {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 10px;
  color: var(--brand);
  background: #def2eb;
}

.record-item__main {
  display: grid;
  gap: 4px;
}

.record-item__main strong {
  font-size: 13px;
}

.record-item__main code {
  color: var(--muted);
  font-size: 9px;
}

.record-item__meta {
  display: grid;
  gap: 5px;
  color: var(--muted);
  font-size: 10px;
}

.record-item__meta span {
  display: flex;
  align-items: center;
  gap: 5px;
}

.record-item button {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 8px 10px;
  border: 1px solid #e4c9c5;
  border-radius: 8px;
  color: #9a3b34;
  background: #fff7f5;
  font-size: 11px;
  cursor: pointer;
}

.record-item button:disabled {
  opacity: 0.55;
}

.records-state {
  padding: 40px 18px;
  color: var(--muted);
  font-size: 12px;
  text-align: center;
}

.records-state--error {
  color: var(--danger);
  background: #fff4f1;
}

@media (max-width: 820px) {
  .record-item {
    grid-template-columns: auto 1fr auto;
  }

  .record-item__meta {
    grid-column: 2 / -1;
  }
}

@media (max-width: 560px) {
  .record-item {
    grid-template-columns: auto 1fr;
  }

  .record-item > .state-badge,
  .record-item button {
    grid-column: 2;
    width: fit-content;
  }
}
</style>
