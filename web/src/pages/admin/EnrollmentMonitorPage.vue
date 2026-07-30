<script setup lang="ts">
import { CheckCircle2, CircleDashed } from '@lucide/vue'

const endpoints = [
  ['POST', '/api/v1/enrollments', '提交选课并等待 Broker Confirm'],
  ['GET', '/api/v1/enrollments/applications/:id', '查询本人申请与异步落库状态'],
  ['GET', '/api/v1/enrollments/me', '分页查询本人正式选课'],
  ['DELETE', '/api/v1/enrollments/:id', '退课并触发 Redis 投影返还'],
  ['POST', '/api/v1/enrollments/waitlist', '加入已满教学班候补'],
  ['GET', '/api/v1/enrollments/waitlist/me', '分页查询本人候补'],
  ['GET', '/api/v1/enrollments/waitlist/:id', '查询单个候补状态'],
  ['DELETE', '/api/v1/enrollments/waitlist/:id', '取消候补'],
]
</script>

<template>
  <section>
    <header class="admin-page-heading">
      <span>Enrollment capability map</span>
      <h1>选课能力</h1>
      <p>学生端已经对接全部 8 个选课 HTTP 接口；管理端聚合查询仍需后端提供只读 API。</p>
    </header>

    <section class="endpoint-panel">
      <header>
        <div><CheckCircle2 :size="20" /><strong>学生端接口覆盖</strong></div>
        <span>8 / 8</span>
      </header>
      <div class="endpoint-list">
        <div v-for="endpoint in endpoints" :key="endpoint[1]">
          <code>{{ endpoint[0] }}</code>
          <strong>{{ endpoint[1] }}</strong>
          <span>{{ endpoint[2] }}</span>
          <i>已接入</i>
        </div>
      </div>
    </section>

    <section class="boundary-panel">
      <CircleDashed :size="21" />
      <div>
        <strong>教务全局申请监控尚无查询接口</strong>
        <p>
          当前 API 强制按 JWT 学生身份校验数据所有权，不应让管理端复用学生接口绕过权限。
          后续应新增独立 Admin 鉴权、全局申请分页、候补积压和投影修复统计接口。
        </p>
      </div>
    </section>
  </section>
</template>

<style scoped>
.admin-page-heading {
  margin-bottom: 22px;
}

.admin-page-heading > span {
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

.endpoint-panel {
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: white;
}

.endpoint-panel > header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 17px 19px;
  border-bottom: 1px solid var(--line-soft);
}

.endpoint-panel > header div {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--success);
}

.endpoint-panel > header strong {
  color: var(--ink);
  font-size: 13px;
}

.endpoint-panel > header > span {
  color: var(--success);
  font-family: var(--font-display);
  font-size: 21px;
}

.endpoint-list > div {
  display: grid;
  grid-template-columns: 70px minmax(280px, 1fr) 1fr auto;
  align-items: center;
  gap: 12px;
  padding: 13px 19px;
  border-bottom: 1px solid var(--line-soft);
}

.endpoint-list > div:last-child {
  border-bottom: 0;
}

.endpoint-list code {
  color: var(--brand);
  font-size: 10px;
  font-weight: 800;
}

.endpoint-list strong {
  font-family: var(--font-mono);
  font-size: 10px;
}

.endpoint-list span {
  color: var(--muted);
  font-size: 10px;
}

.endpoint-list i {
  padding: 4px 7px;
  border-radius: 999px;
  color: var(--success);
  background: #daf3e9;
  font-size: 9px;
  font-style: normal;
}

.boundary-panel {
  display: flex;
  gap: 12px;
  margin-top: 14px;
  padding: 17px;
  border: 1px solid #e7d6aa;
  border-radius: 12px;
  color: #80570f;
  background: #fff6dd;
}

.boundary-panel strong {
  font-size: 12px;
}

.boundary-panel p {
  margin: 5px 0 0;
  font-size: 10px;
  line-height: 1.6;
}

@media (max-width: 760px) {
  .endpoint-list > div {
    grid-template-columns: auto 1fr auto;
  }

  .endpoint-list span {
    grid-column: 2 / -1;
  }
}
</style>
