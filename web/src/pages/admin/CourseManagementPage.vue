<script setup lang="ts">
import { Check, CircleDashed } from '@lucide/vue'
const capabilities = [
  { title: '课程与教学班', detail: '课程、教学班、上课安排和专业范围的基础信息已经就绪。', state: 'ready' },
  { title: '课程目录', detail: '课程目录的统一查询与维护功能待开放。', state: 'pending' },
  { title: '课程维护', detail: '新增、编辑和停开课程等教务功能待开放。', state: 'pending' },
  { title: '课程视频与直播', detail: '课程介绍视频、直播宣讲和内容维护功能待开放。', state: 'pending' },
]
</script>

<template>
  <section>
    <header class="admin-page-heading"><h1>课程管理</h1><p>课程、教学班及课程内容的开放情况。</p></header>
    <div class="capability-list">
      <article v-for="(item, index) in capabilities" :key="item.title">
        <b>{{ String(index + 1).padStart(2, '0') }}</b>
        <div><h2>{{ item.title }}</h2><p>{{ item.detail }}</p></div>
        <span :class="`is-${item.state}`">
          <Check v-if="item.state === 'ready'" :size="16" />
          <CircleDashed v-else :size="16" />
          {{ item.state === 'ready' ? '已开放' : '待开放' }}
        </span>
      </article>
    </div>
  </section>
</template>

<style scoped>
.admin-page-heading { margin-bottom: 28px; padding-bottom: 24px; border-bottom: 1px solid var(--line); }
.capability-list { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); }
.capability-list article { display: grid; grid-template-columns: 60px minmax(0,1fr) auto; align-items: center; gap: 18px; min-height: 112px; padding: 20px; border-bottom: 1px solid var(--line); }
.capability-list article:last-child { border-bottom: 0; }
.capability-list b { color: var(--brand); font-family: var(--font-display); font-size: 28px; font-variation-settings: "wdth" 72, "wght" 680; }
.capability-list h2 { margin: 0 0 6px; font-family: var(--font-display); font-size: 20px; font-variation-settings: "wdth" 82, "wght" 660; }
.capability-list p { margin: 0; color: var(--muted); font-size: 11px; line-height: 1.7; }
.capability-list span { display: flex; align-items: center; gap: 6px; min-width: 82px; padding: 7px 9px; border: 1px solid var(--line); border-radius: 7px; color: var(--muted); font-size: 10px; }
.capability-list span.is-ready { border-color: var(--success); color: var(--success); }
@media (max-width: 580px) { .capability-list article { grid-template-columns: 42px 1fr; gap: 10px; padding: 15px; } .capability-list article > span { grid-column: 2; width: fit-content; } }
</style>
