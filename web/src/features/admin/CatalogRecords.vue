<script setup lang="ts">
import { Link2, LockKeyhole, Pencil, Trash2, Video } from '@lucide/vue'

import type { Course, SelectionRound, TeachingClass } from '@/types/catalog'

defineProps<{
  mode: 'courses' | 'classes' | 'rounds'
  courses: readonly Course[]
  teachingClasses: readonly TeachingClass[]
  rounds: readonly SelectionRound[]
}>()
const emit = defineEmits<{
  editCourse: [item: Course]
  deleteCourse: [item: Course]
  manageCourseVideo: [item: Course]
  editClass: [item: TeachingClass]
  deleteClass: [item: TeachingClass]
  editRound: [item: SelectionRound]
  deleteRound: [item: SelectionRound]
  manageRound: [item: SelectionRound]
}>()

function stateLabel(state: string): string {
  return ({ planned: '计划中', open: '开放中', closed: '已结束', cancelled: '已取消' })[state] ?? state
}

function scheduleLabel(item: TeachingClass): string {
  const schedule = item.schedules[0]
  return schedule ? `星期${schedule.day_of_week} · ${schedule.start_section}–${schedule.end_section} 节` : '时间待排'
}
</script>

<template>
  <div class="record-table">
    <div v-if="mode === 'courses'" class="record-list">
      <article v-for="item in courses" :key="item.id">
        <div class="record-code"><strong>{{ item.course_code }}</strong><span>#{{ item.id }}</span></div>
        <div class="record-main"><h3>{{ item.course_name }}</h3><p>{{ item.introduction || '尚未填写课程简介' }}</p><div><span v-for="tag in item.tags" :key="tag">{{ tag }}</span></div></div>
        <strong class="record-number">{{ item.credits }}<small>学分</small></strong>
        <div class="record-actions"><button type="button" aria-label="管理课程视频" @click="emit('manageCourseVideo', item)"><Video :size="16" /></button><button type="button" aria-label="编辑课程" @click="emit('editCourse', item)"><Pencil :size="16" /></button><button type="button" class="danger" aria-label="删除课程" @click="emit('deleteCourse', item)"><Trash2 :size="16" /></button></div>
      </article>
      <p v-if="!courses.length" class="empty">还没有课程，先新增一门课程。</p>
    </div>

    <div v-else-if="mode === 'classes'" class="record-list">
      <article v-for="item in teachingClasses" :key="item.id">
        <div class="record-code"><strong>{{ item.class_code }}</strong><span>学期 {{ item.term_id }}</span></div>
        <div class="record-main"><h3>{{ item.course_name }}</h3><p>{{ item.teacher_name || '教师待公布' }} · {{ scheduleLabel(item) }} · {{ item.location || '地点待公布' }}</p><div><span class="state">{{ stateLabel(item.state) }}</span></div></div>
        <strong class="record-number">{{ item.selected_count }}/{{ item.capacity }}<small>人数</small></strong>
        <div class="record-actions">
          <template v-if="item.state === 'planned'"><button type="button" aria-label="编辑教学班" @click="emit('editClass', item)"><Pencil :size="16" /></button><button type="button" class="danger" aria-label="删除教学班" @click="emit('deleteClass', item)"><Trash2 :size="16" /></button></template>
          <span v-else class="locked" title="已进入选课流程，不能通过 CRUD 修改"><LockKeyhole :size="16" /></span>
        </div>
      </article>
      <p v-if="!teachingClasses.length" class="empty">当前筛选下没有教学班。</p>
    </div>

    <div v-else class="record-list">
      <article v-for="item in rounds" :key="item.id">
        <div class="record-code"><strong>{{ item.round_code }}</strong><span>学期 {{ item.term_id }}</span></div>
        <div class="record-main"><h3>{{ item.round_name }}</h3><p>{{ new Date(item.start_time).toLocaleString() }} 至 {{ new Date(item.end_time).toLocaleString() }}</p><div><span class="state">{{ stateLabel(item.state) }}</span><span>{{ item.class_count }} 个教学班</span></div></div>
        <div class="record-actions is-round">
          <button type="button" aria-label="管理教学班绑定" @click="emit('manageRound', item)"><Link2 :size="16" /></button>
          <template v-if="item.state === 'planned'"><button type="button" aria-label="编辑轮次" @click="emit('editRound', item)"><Pencil :size="16" /></button><button type="button" class="danger" aria-label="删除轮次" @click="emit('deleteRound', item)"><Trash2 :size="16" /></button></template>
          <span v-else class="locked"><LockKeyhole :size="16" /></span>
        </div>
      </article>
      <p v-if="!rounds.length" class="empty">当前筛选下没有选课轮次。</p>
    </div>
  </div>
</template>

<style scoped>
.record-table { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); }
.record-list article { display: grid; grid-template-columns: 150px minmax(260px, 1fr) 90px 96px; align-items: center; gap: 18px; min-height: 105px; padding: 17px 19px; border-bottom: 1px solid var(--line-soft); }.record-list article:last-of-type { border-bottom: 0; }
.record-code, .record-main { display: grid; gap: 5px; }.record-code strong { font-family: var(--font-mono); color: var(--brand); font-size: 11px; }.record-code span { color: var(--muted); font-size: 9px; }
.record-main h3 { margin: 0; font-family: var(--font-display); font-size: 18px; font-variation-settings: "wdth" 82, "wght" 680; }.record-main p { overflow: hidden; margin: 0; color: var(--muted); font-size: 10px; line-height: 1.6; text-overflow: ellipsis; white-space: nowrap; }.record-main > div { display: flex; gap: 5px; }.record-main > div span { padding: 3px 6px; border: 1px solid var(--line-soft); border-radius: 5px; color: var(--muted); font-size: 8px; }.record-main > div .state { color: var(--brand); background: var(--brand-pale); }
.record-number { display: grid; justify-items: center; font-family: var(--font-display); font-size: 19px; }.record-number small { color: var(--muted); font-family: inherit; font-size: 8px; font-weight: 500; }
.record-actions { display: flex; justify-content: flex-end; gap: 6px; }.record-actions.is-round { min-width: 128px; }.record-actions button, .locked { display: grid; width: 34px; height: 34px; place-items: center; border: 1px solid var(--line); border-radius: 7px; color: var(--ink-soft); background: white; cursor: pointer; }.record-actions button:hover { border-color: var(--brand); color: var(--brand); }.record-actions .danger:hover { border-color: var(--danger); color: var(--danger); }.locked { color: var(--muted); background: var(--surface-muted); cursor: default; }
.empty { margin: 0; padding: 54px 20px; color: var(--muted); text-align: center; font-size: 12px; }
@media (max-width: 800px) { .record-list article { grid-template-columns: 120px minmax(180px, 1fr) auto; }.record-number { display: none; } } @media (max-width: 580px) { .record-list article { grid-template-columns: 1fr auto; }.record-main { grid-column: 1 / -1; grid-row: 2; }.record-actions { grid-column: 2; grid-row: 1; } }
</style>
