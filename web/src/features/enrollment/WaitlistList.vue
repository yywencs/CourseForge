<script setup lang="ts">
import { ListOrdered, TimerReset, XCircle } from '@lucide/vue'

import StateBadge from '@/components/StateBadge.vue'
import { courseLabel } from '@/data/courseCatalog'
import type { WaitlistEntry } from '@/types/enrollment'

defineProps<{
  items: WaitlistEntry[]
  loading?: boolean
  cancellingId?: string
  error?: string
}>()

const emit = defineEmits<{
  cancel: [entry: WaitlistEntry]
}>()
</script>

<template>
  <section class="waitlist-panel">
    <header>
      <div>
        <span>Waitlist queue</span>
        <h2>候补队列</h2>
      </div>
      <strong>{{ items.filter((item) => item.state === 'waiting').length }} 个等待中</strong>
    </header>

    <div v-if="loading" class="waitlist-state">正在读取候补队列…</div>
    <div v-else-if="error" class="waitlist-state waitlist-state--error" role="alert">
      {{ error }}
    </div>
    <div v-else-if="items.length" class="waitlist-list">
      <article v-for="item in items" :key="item.waitlist_id" class="waitlist-item">
        <div class="waitlist-position">
          <ListOrdered :size="15" />
          <strong>{{ item.position }}</strong>
        </div>
        <div class="waitlist-main">
          <strong>{{ courseLabel(item.teaching_class_id) }}</strong>
          <span>{{ item.credits }} 学分 · {{ item.waitlist_id }}</span>
        </div>
        <StateBadge :state="item.state" />
        <time><TimerReset :size="13" /> {{ new Date(item.joined_at).toLocaleString('zh-CN') }}</time>
        <button
          v-if="['waiting', 'promoting'].includes(item.state)"
          type="button"
          :disabled="cancellingId === item.waitlist_id"
          @click="emit('cancel', item)"
        >
          <XCircle :size="14" />
          {{ cancellingId === item.waitlist_id ? '取消中…' : '取消候补' }}
        </button>
        <p v-if="item.failure">{{ item.failure.message }}</p>
      </article>
    </div>
    <div v-else class="waitlist-state">当前学期没有候补申请。</div>
  </section>
</template>

<style scoped>
.waitlist-panel {
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.waitlist-panel header {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 18px;
  margin-bottom: 18px;
}

.waitlist-panel header span {
  color: #98630c;
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.11em;
  text-transform: uppercase;
}

.waitlist-panel h2 {
  margin: 5px 0 0;
  font-family: var(--font-display);
  font-size: 25px;
}

.waitlist-panel header > strong {
  color: #8b5a08;
  font-size: 12px;
}

.waitlist-list {
  display: grid;
  gap: 9px;
}

.waitlist-item {
  display: grid;
  grid-template-columns: auto minmax(180px, 1fr) auto auto auto;
  align-items: center;
  gap: 14px;
  padding: 14px;
  border: 1px solid var(--line-soft);
  border-radius: 12px;
}

.waitlist-position {
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  border-radius: 11px;
  color: #80530b;
  background: #ffedbd;
}

.waitlist-position strong {
  margin-top: -7px;
  font-family: var(--font-display);
}

.waitlist-main {
  display: grid;
  gap: 4px;
}

.waitlist-main strong {
  font-size: 13px;
}

.waitlist-main span,
.waitlist-item time {
  color: var(--muted);
  font-size: 10px;
}

.waitlist-item time {
  display: flex;
  align-items: center;
  gap: 5px;
}

.waitlist-item button {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: 8px;
  color: #5f6e69;
  background: white;
  font-size: 11px;
  cursor: pointer;
}

.waitlist-item > p {
  grid-column: 2 / -1;
  margin: 0;
  color: var(--danger);
  font-size: 11px;
}

.waitlist-state {
  padding: 40px 18px;
  color: var(--muted);
  font-size: 12px;
  text-align: center;
}

.waitlist-state--error {
  color: var(--danger);
  background: #fff4f1;
}

@media (max-width: 850px) {
  .waitlist-item {
    grid-template-columns: auto 1fr auto;
  }

  .waitlist-item time {
    grid-column: 2 / -1;
  }
}

@media (max-width: 560px) {
  .waitlist-item {
    grid-template-columns: auto 1fr;
  }

  .waitlist-item > .state-badge,
  .waitlist-item button,
  .waitlist-item time {
    grid-column: 2;
    width: fit-content;
  }
}
</style>
