<script setup lang="ts">
import { Activity, CheckCircle2, CircleAlert, CircleHelp, RefreshCw, Server } from '@lucide/vue'
import { useServiceProbes } from '@/composables/useServiceProbes'

const { probes, healthyCount, isRefreshing, refresh } = useServiceProbes()
function statusIcon(status: string) {
  if (status === 'healthy') return CheckCircle2
  if (status === 'degraded') return CircleAlert
  return CircleHelp
}
</script>

<template>
  <section>
    <header class="admin-page-heading">
      <div><h1>运行概览</h1><p>查看学生端、教务端和相关服务的当前状态。</p></div>
      <button type="button" :disabled="isRefreshing" @click="refresh">
        <RefreshCw :size="16" :class="{ 'is-spinning': isRefreshing }" />立即检测
      </button>
    </header>

    <div class="health-summary">
      <div><Server :size="22" /><span>检测项目</span><strong>{{ probes.length }}</strong></div>
      <div><Activity :size="22" /><span>状态正常</span><strong>{{ healthyCount }}</strong></div>
      <p>系统会定期更新状态。出现异常时，可立即重新检测并根据提示处理。</p>
    </div>

    <section class="probe-section">
      <header><h2>服务状态</h2><span>最近一次检测结果</span></header>
      <div class="probe-grid">
        <article v-for="(probe, index) in probes" :key="probe.name" class="probe-card" :class="`is-${probe.status}`">
          <b>{{ String(index + 1).padStart(2, '0') }}</b>
          <component :is="statusIcon(probe.status)" :size="21" />
          <div><span>{{ probe.description }}</span><h3>{{ probe.name }}</h3><p>{{ probe.detail }}</p></div>
          <time v-if="probe.checkedAt">{{ new Date(probe.checkedAt).toLocaleTimeString('zh-CN') }}</time>
        </article>
      </div>
    </section>
  </section>
</template>

<style scoped>
.admin-page-heading {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 24px;
  margin-bottom: 28px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--line);
}

.admin-page-heading button {
  display: flex;
  align-items: center;
  gap: 7px;
  min-height: 42px;
  padding: 9px 14px;
  border: 1px solid var(--brand);
  border-radius: 8px;
  color: white;
  background: var(--brand);
  font-size: 11px;
  font-weight: 750;
  cursor: pointer;
}

.health-summary {
  display: grid;
  grid-template-columns: 190px 190px 1fr;
  overflow: hidden;
  margin-bottom: 18px;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  color: var(--ink);
  background: var(--surface);
}

.health-summary > div {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 2px 10px;
  padding: 20px;
  border-right: 1px solid var(--line-soft);
  color: var(--brand);
}

.health-summary > div svg {
  grid-row: 1 / 3;
}

.health-summary span {
  color: var(--muted);
  font-size: 10px;
}

.health-summary strong {
  color: var(--ink);
  font-family: var(--font-display);
  font-size: 30px;
  font-variation-settings: "wdth" 65, "wght" 800;
}

.health-summary > p {
  align-self: center;
  margin: 0;
  padding: 20px;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.7;
}

.probe-section {
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
}

.probe-section > header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 18px;
  border-bottom: 1px solid var(--line-soft);
}

.probe-section h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 20px;
  font-variation-settings: "wdth" 74, "wght" 760;
}

.probe-section > header span,
.probe-card span,
.probe-card time {
  color: var(--muted);
  font-size: 9px;
}

.probe-card {
  display: grid;
  grid-template-columns: 48px auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  border-bottom: 1px solid var(--line-soft);
}

.probe-card:last-child {
  border-bottom: 0;
}

.probe-card > b {
  color: var(--brand);
  font-family: var(--font-display);
  font-size: 23px;
  font-variation-settings: "wdth" 62, "wght" 800;
}

.probe-card.is-healthy > svg { color: var(--success); }
.probe-card.is-degraded > svg { color: var(--danger); }

.probe-card h3 {
  margin: 3px 0;
  font-size: 13px;
}

.probe-card p {
  margin: 0;
  color: var(--muted);
  font-size: 10px;
}

.is-spinning { animation: spin 800ms linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

@media (max-width: 720px) {
  .health-summary { grid-template-columns: 1fr 1fr; }
  .health-summary > p { grid-column: 1 / -1; border-top: 1px solid var(--line-soft); }
  .admin-page-heading { align-items: flex-start; flex-direction: column; }
}

@media (max-width: 520px) {
  .probe-card { grid-template-columns: 35px auto 1fr; padding-inline: 12px; }
  .probe-card time { grid-column: 3; }
}
</style>
