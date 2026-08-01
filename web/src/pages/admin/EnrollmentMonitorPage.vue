<script setup lang="ts">
import { CheckCircle2, CircleDashed } from '@lucide/vue'
const capabilities = [
  ['提交选课', '学生可以提交教学班选课申请'],
  ['申请进度', '学生可以查看自己的申请结果'],
  ['已选课程', '学生可以查看和退出已选课程'],
  ['候补管理', '学生可以加入、查看和取消候补'],
]
</script>

<template>
  <section>
    <header class="admin-page-heading"><h1>选课管理</h1><p>当前学生选课功能与后续管理边界。</p></header>
    <section class="endpoint-panel">
      <header><div><CheckCircle2 :size="20" /><strong>学生端选课功能</strong></div><span>4 项已开放</span></header>
      <div class="endpoint-list">
        <div v-for="(item, index) in capabilities" :key="item[0]">
          <b>{{ String(index + 1).padStart(2, '0') }}</b><strong>{{ item[0] }}</strong><span>{{ item[1] }}</span><i>已开放</i>
        </div>
      </div>
    </section>
    <section class="boundary-panel"><CircleDashed :size="21" /><div><strong>全校选课监控暂未开放</strong><p>全局申请查询、候补统计和异常处理功能将在后续开放。</p></div></section>
  </section>
</template>

<style scoped>
.admin-page-heading { margin-bottom: 28px; padding-bottom: 24px; border-bottom: 1px solid var(--line); }
.endpoint-panel { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); }
.endpoint-panel > header { display: flex; justify-content: space-between; align-items: center; padding: 16px 18px; border-bottom: 1px solid var(--line-soft); color: var(--ink); background: var(--surface-muted); }
.endpoint-panel > header div { display: flex; align-items: center; gap: 8px; }
.endpoint-panel > header strong { font-size: 12px; }
.endpoint-panel > header > span { color: var(--muted); font-size: 10px; }
.endpoint-list > div { display: grid; grid-template-columns: 48px minmax(130px,.45fr) 1fr auto; align-items: center; gap: 14px; padding: 15px 18px; border-bottom: 1px solid var(--line-soft); }
.endpoint-list > div:last-child { border-bottom: 0; }
.endpoint-list b { color: var(--brand); font-family: var(--font-display); font-size: 23px; font-variation-settings: "wdth" 62, "wght" 800; }
.endpoint-list strong { font-size: 12px; }
.endpoint-list span { color: var(--muted); font-size: 10px; }
.endpoint-list i { padding: 5px 7px; border: 1px solid var(--success); border-radius: 6px; color: var(--success); font-size: 9px; font-style: normal; }
.boundary-panel { display: flex; gap: 12px; margin-top: 14px; padding: 17px; border: 1px solid #e2ae63; border-radius: 10px; color: #714300; background: #fff4db; }
.boundary-panel strong { font-size: 12px; }
.boundary-panel p { margin: 5px 0 0; font-size: 10px; line-height: 1.6; }
@media (max-width: 650px) { .endpoint-list > div { grid-template-columns: 35px 1fr auto; } .endpoint-list span { grid-column: 2 / -1; } }
</style>
