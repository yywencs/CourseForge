<script setup lang="ts">
import {
  Activity,
  CheckCircle2,
  CircleAlert,
  CircleHelp,
  RefreshCw,
  Server,
} from '@lucide/vue'

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
      <div>
        <span>Live backend probes</span>
        <h1>运行概览</h1>
        <p>直接探测 API、核心依赖和 Admin 服务，不展示前端伪造业务指标。</p>
      </div>
      <button type="button" :disabled="isRefreshing" @click="refresh">
        <RefreshCw :size="16" :class="{ 'is-spinning': isRefreshing }" />
        立即检测
      </button>
    </header>

    <div class="health-summary">
      <div>
        <Server :size="21" />
        <span>已检测服务</span>
        <strong>{{ probes.length }}</strong>
      </div>
      <div>
        <Activity :size="21" />
        <span>健康项</span>
        <strong>{{ healthyCount }}</strong>
      </div>
      <p>
        就绪检查会并行验证 MySQL、Redis 和 RabbitMQ。若某项失败，后端
        <code>/readyz</code> 会返回具体依赖名称。
      </p>
    </div>

    <div class="probe-grid">
      <article
        v-for="probe in probes"
        :key="probe.name"
        class="probe-card"
        :class="`is-${probe.status}`"
      >
        <component :is="statusIcon(probe.status)" :size="21" />
        <div>
          <span>{{ probe.description }}</span>
          <h2>{{ probe.name }}</h2>
          <p>{{ probe.detail }}</p>
        </div>
        <time v-if="probe.checkedAt">
          {{ new Date(probe.checkedAt).toLocaleTimeString('zh-CN') }}
        </time>
      </article>
    </div>

    <section class="pipeline-panel">
      <header>
        <span>Selection pipeline</span>
        <h2>选课处理链</h2>
      </header>
      <ol>
        <li><i>01</i><strong>资格预检</strong><span>MySQL 读取学生、专业、年级、先修与课表</span></li>
        <li><i>02</i><strong>原子预占</strong><span>Redis Lua 同时占用额度、课程门数与名额</span></li>
        <li><i>03</i><strong>结果投递</strong><span>Redis Stream 保存结果，RabbitMQ Publisher Confirm</span></li>
        <li><i>04</i><strong>异步落库</strong><span>消费者幂等写入 MySQL，失败由恢复任务补发</span></li>
      </ol>
    </section>
  </section>
</template>

<style scoped>
.admin-page-heading {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 24px;
  margin-bottom: 22px;
}

.admin-page-heading span,
.pipeline-panel header span {
  color: #98630c;
  font-family: var(--font-mono);
  font-size: 9px;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.admin-page-heading h1 {
  margin: 5px 0;
  font-family: var(--font-display);
  font-size: 36px;
}

.admin-page-heading p {
  margin: 0;
  color: var(--muted);
  font-size: 11px;
}

.admin-page-heading button {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 9px 12px;
  border: 1px solid var(--line);
  border-radius: 9px;
  color: var(--brand);
  background: white;
  font-size: 11px;
  font-weight: 750;
  cursor: pointer;
}

.health-summary {
  display: grid;
  grid-template-columns: 180px 180px 1fr;
  gap: 12px;
  margin-bottom: 16px;
}

.health-summary > div,
.health-summary > p,
.pipeline-panel {
  border: 1px solid var(--line);
  border-radius: 14px;
  background: white;
}

.health-summary > div {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 4px 10px;
  padding: 16px;
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
  font-family: var(--font-display);
  font-size: 24px;
}

.health-summary > p {
  margin: 0;
  padding: 16px;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.6;
}

.probe-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.probe-card {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 13px;
  padding: 18px;
  border: 1px solid var(--line);
  border-radius: 14px;
  color: var(--muted);
  background: white;
}

.probe-card.is-healthy > svg {
  color: var(--success);
}

.probe-card.is-degraded > svg {
  color: var(--danger);
}

.probe-card span,
.probe-card time {
  color: var(--muted);
  font-size: 9px;
}

.probe-card h2 {
  margin: 4px 0;
  font-size: 15px;
}

.probe-card p {
  margin: 0;
  font-size: 10px;
}

.pipeline-panel {
  margin-top: 16px;
  padding: 22px;
}

.pipeline-panel h2 {
  margin: 5px 0 18px;
  font-family: var(--font-display);
  font-size: 25px;
}

.pipeline-panel ol {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1px;
  margin: 0;
  padding: 0;
  border-radius: 10px;
  background: var(--line);
  list-style: none;
}

.pipeline-panel li {
  display: grid;
  gap: 6px;
  min-height: 130px;
  padding: 16px;
  background: var(--surface-muted);
}

.pipeline-panel li i {
  color: var(--brand);
  font-family: var(--font-mono);
  font-size: 10px;
  font-style: normal;
}

.pipeline-panel li strong {
  font-size: 13px;
}

.pipeline-panel li span {
  color: var(--muted);
  font-size: 10px;
  line-height: 1.5;
}

.is-spinning {
  animation: spin 800ms linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1000px) {
  .probe-grid,
  .pipeline-panel ol {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 680px) {
  .health-summary,
  .probe-grid,
  .pipeline-panel ol {
    grid-template-columns: 1fr;
  }

  .admin-page-heading {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
