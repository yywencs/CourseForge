<script setup lang="ts">
import { ArrowRight, Bell, CalendarDays, Play, Radio } from '@lucide/vue'
import { computed } from 'vue'
import { RouterLink } from 'vue-router'

import type { TeachingClassSummary } from '@/types/enrollment'

const props = defineProps<{
  courses: TeachingClassSummary[]
  selectedCount: number
  waitlistCount: number
  termId: number
  roundId: number
}>()

const emit = defineEmits<{
  playVideo: [course: TeachingClassSummary]
}>()

const previewCourses = computed(() =>
  props.courses
    .filter((course) => course.hasVideo && Boolean(course.videoUrl))
    .slice(0, 2),
)
</script>

<template>
  <aside class="media-rail" aria-label="课程内容与选课摘要">
    <section class="term-board">
      <CalendarDays />
      <div>
        <span>当前选课</span>
        <strong>学期 {{ termId }}</strong>
      </div>
      <b>第 {{ roundId }} 轮</b>
    </section>

    <section class="live-board">
      <header>
        <Radio :size="20" />
        <strong>课程直播宣讲</strong>
      </header>
      <div class="live-board__body">
        <span>功能预留</span>
        <h2>在选课前，<br />先听老师讲清楚。</h2>
        <p>直播服务接入后，这里会显示课程宣讲、开始时间和提醒入口。</p>
        <button type="button" disabled><Bell :size="16" />直播提醒即将开放</button>
      </div>
    </section>

    <section class="preview-board">
      <header>
        <strong>课程预览</strong>
        <span>{{ previewCourses.length }} 门已配置</span>
      </header>
      <button
        v-for="course in previewCourses"
        :key="course.id"
        type="button"
        @click="emit('playVideo', course)"
      >
        <span class="preview-board__play"><Play :size="16" fill="currentColor" /></span>
        <span><strong>{{ course.courseName }}</strong><small>{{ course.teacherName }}</small></span>
        <ArrowRight :size="16" />
      </button>
      <p v-if="!previewCourses.length">课程预览尚未配置，接入真实内容后会显示在这里。</p>
    </section>

    <section class="selection-board">
      <header><strong>我的本轮进度</strong></header>
      <div>
        <span><b>{{ selectedCount }}</b><small>已选课程</small></span>
        <span><b>{{ waitlistCount }}</b><small>候补课程</small></span>
      </div>
      <RouterLink to="/student/enrollments">
        查看选课记录
        <ArrowRight :size="16" />
      </RouterLink>
    </section>
  </aside>
</template>

<style scoped>
.media-rail {
  position: sticky;
  top: 28px;
  display: grid;
  align-self: start;
  gap: 14px;
}

.term-board,
.live-board,
.preview-board,
.selection-board {
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
}

.term-board {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 12px;
  min-height: 96px;
  padding: 18px;
  color: var(--ink);
  background: var(--brand-pale);
}

.term-board > svg {
  width: 31px;
  height: 31px;
}

.term-board div {
  display: grid;
  gap: 3px;
}

.term-board span {
  color: var(--muted);
  font-size: 10px;
}

.term-board strong {
  font-size: 16px;
}

.term-board b {
  align-self: stretch;
  display: grid;
  min-width: 68px;
  place-items: center;
  border-left: 1px solid var(--line);
  font-family: var(--font-display);
  font-size: 18px;
  font-variation-settings: "wdth" 72, "wght" 760;
}

.live-board {
  border-color: #ddc8bc;
}

.live-board > header,
.preview-board > header,
.selection-board > header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 45px;
  padding: 11px 15px;
  border-bottom: 1px solid var(--line-soft);
  font-size: 12px;
}

.live-board > header {
  justify-content: flex-start;
  gap: 8px;
  border-color: #ddc8bc;
  color: var(--signal);
  background: var(--signal-pale);
}

.live-board__body {
  padding: 18px;
}

.live-board__body > span {
  color: var(--signal);
  font-family: var(--font-display);
  font-size: 11px;
  font-variation-settings: "wdth" 72, "wght" 760;
  letter-spacing: 0.08em;
}

.live-board h2 {
  margin: 9px 0;
  font-family: var(--font-display);
  font-size: 22px;
  font-variation-settings: "wdth" 82, "wght" 660;
  letter-spacing: -0.02em;
  line-height: 1.2;
}

.live-board p,
.preview-board > p {
  margin: 0;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.7;
}

.live-board button {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: 15px;
  padding: 10px;
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--muted);
  background: var(--surface-muted);
}

.preview-board > header span {
  color: var(--muted);
  font-size: 10px;
}

.preview-board > button {
  display: grid;
  width: 100%;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 11px;
  padding: 13px 15px;
  border: 0;
  border-bottom: 1px solid var(--line-soft);
  color: var(--ink);
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.preview-board > button:hover {
  background: var(--brand-pale);
}

.preview-board > button > span:nth-child(2) {
  display: grid;
  gap: 2px;
}

.preview-board > button strong {
  overflow: hidden;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-board > button small {
  color: var(--muted);
  font-size: 9px;
}

.preview-board__play {
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  color: var(--brand);
  background: var(--brand-pale);
}

.preview-board > p {
  padding: 15px;
}

.selection-board > div {
  display: grid;
  grid-template-columns: 1fr 1fr;
}

.selection-board > div > span {
  display: grid;
  gap: 2px;
  padding: 16px;
  border-right: 1px solid var(--line-soft);
}

.selection-board > div > span:last-child {
  border-right: 0;
}

.selection-board b {
  font-family: var(--font-display);
  font-size: 24px;
  font-variation-settings: "wdth" 78, "wght" 680;
}

.selection-board small {
  color: var(--muted);
  font-size: 10px;
}

.selection-board > a {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 15px;
  border-top: 1px solid var(--line-soft);
  color: var(--brand);
  background: var(--brand-pale);
  font-size: 11px;
  font-weight: 700;
}

@media (max-width: 1100px) {
  .media-rail {
    position: static;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 680px) {
  .media-rail {
    grid-template-columns: 1fr;
  }
}
</style>
