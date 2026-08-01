<script setup lang="ts">
import { Plus, Trash2 } from '@lucide/vue'
import { reactive, watch } from 'vue'

import type { ClassSchedule, Course, TeachingClass, TeachingClassDraft } from '@/types/catalog'

const props = defineProps<{ courses: readonly Course[]; teachingClass?: TeachingClass }>()
const emit = defineEmits<{ save: [draft: TeachingClassDraft]; cancel: [] }>()

const form = reactive({
  class_code: '', term_id: 0, course_id: 0, teacher_name: '', location: '', capacity: 30,
  minimum_grade_year: '', maximum_grade_year: '',
  schedules: [] as ClassSchedule[],
})

function emptySchedule(): ClassSchedule {
  return { day_of_week: 1, start_week: 1, end_week: 16, start_section: 1, end_section: 2 }
}

watch(
  () => props.teachingClass,
  (item) => {
    Object.assign(form, {
      class_code: item?.class_code ?? '', term_id: item?.term_id ?? 0,
      course_id: item?.course_id ?? props.courses[0]?.id ?? 0,
      teacher_name: item?.teacher_name ?? '', location: item?.location ?? '',
      capacity: item?.capacity ?? 30,
      minimum_grade_year: item?.minimum_grade_year ? String(item.minimum_grade_year) : '',
      maximum_grade_year: item?.maximum_grade_year ? String(item.maximum_grade_year) : '',
      schedules: item?.schedules.length ? item.schedules.map((schedule) => ({ ...schedule })) : [emptySchedule()],
    })
  },
  { immediate: true },
)

function optionalYear(value: string): number | undefined {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
}

function submit(): void {
  emit('save', {
    class_code: form.class_code.trim(), term_id: Number(form.term_id), course_id: Number(form.course_id),
    teacher_name: form.teacher_name.trim(), location: form.location.trim(), capacity: Number(form.capacity),
    minimum_grade_year: optionalYear(form.minimum_grade_year),
    maximum_grade_year: optionalYear(form.maximum_grade_year),
    schedules: form.schedules.map((schedule) => ({
      day_of_week: Number(schedule.day_of_week), start_week: Number(schedule.start_week), end_week: Number(schedule.end_week),
      start_section: Number(schedule.start_section), end_section: Number(schedule.end_section),
    })),
  })
}
</script>

<template>
  <form class="class-form" @submit.prevent="submit">
    <header><h2>{{ teachingClass ? '编辑教学班' : '新增教学班' }}</h2><p>这里只维护计划中的教学班，上课时间先支持一组常规安排。</p></header>
    <div class="form-grid">
      <label><span>教学班代码</span><input v-model="form.class_code" required /></label>
      <label><span>学期 ID</span><input v-model.number="form.term_id" type="number" min="1" required /></label>
      <label><span>课程</span><select v-model.number="form.course_id" required><option v-for="course in courses" :key="course.id" :value="course.id">{{ course.course_code }} · {{ course.course_name }}</option></select></label>
      <label><span>任课教师</span><input v-model="form.teacher_name" /></label>
      <label><span>上课地点</span><input v-model="form.location" /></label>
      <label><span>教学班容量</span><input v-model.number="form.capacity" type="number" min="1" required /></label>
      <label><span>最低入学年份</span><input v-model="form.minimum_grade_year" inputmode="numeric" placeholder="不限" /></label>
      <label><span>最高入学年份</span><input v-model="form.maximum_grade_year" inputmode="numeric" placeholder="不限" /></label>
    </div>
    <section class="schedules">
      <header><strong>上课安排</strong><button type="button" @click="form.schedules.push(emptySchedule())"><Plus :size="15" />添加时间</button></header>
      <fieldset v-for="(schedule, index) in form.schedules" :key="index">
        <legend>安排 {{ index + 1 }}</legend>
        <label><span>星期</span><select v-model.number="schedule.day_of_week"><option v-for="day in 7" :key="day" :value="day">星期{{ ['一','二','三','四','五','六','日'][day - 1] }}</option></select></label>
        <label><span>起始周</span><input v-model.number="schedule.start_week" type="number" min="1" /></label>
        <label><span>结束周</span><input v-model.number="schedule.end_week" type="number" min="1" /></label>
        <label><span>起始节</span><input v-model.number="schedule.start_section" type="number" min="1" /></label>
        <label><span>结束节</span><input v-model.number="schedule.end_section" type="number" min="1" /></label>
        <button type="button" class="remove-schedule" :disabled="form.schedules.length === 1" aria-label="删除这组上课安排" @click="form.schedules.splice(index, 1)"><Trash2 :size="15" /></button>
      </fieldset>
    </section>
    <footer><button type="button" class="quiet" @click="emit('cancel')">取消</button><button type="submit" class="primary">保存教学班</button></footer>
  </form>
</template>

<style scoped>
.class-form { display: grid; gap: 21px; }
.class-form header h2 { margin: 0; font-size: 22px; }.class-form header p { margin: 5px 0 0; color: var(--muted); font-size: 12px; }
.form-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; }.form-grid label, fieldset label { display: grid; gap: 6px; color: var(--muted); font-size: 10px; }
input, select { min-width: 0; width: 100%; padding: 10px 11px; border: 1px solid var(--line); border-radius: 8px; outline: 0; color: var(--ink); background: white; } input:focus, select:focus { border-color: var(--brand); }
.schedules { display: grid; gap: 9px; }.schedules > header { display: flex; justify-content: space-between; align-items: center; }.schedules > header strong { font-size: 11px; }.schedules > header button { display: flex; align-items: center; gap: 5px; min-height: 32px; padding: 0 9px; border: 1px solid var(--line); border-radius: 7px; color: var(--brand); background: white; cursor: pointer; }
fieldset { display: grid; grid-template-columns: 1.2fr repeat(4, 1fr) auto; align-items: end; gap: 12px; margin: 0; padding: 16px; border: 1px solid var(--line-soft); border-radius: 10px; } legend { padding: 0 6px; color: var(--ink); font-size: 11px; font-weight: 700; }.remove-schedule { display: grid; width: 38px; height: 38px; place-items: center; padding: 0 !important; color: var(--danger); background: white; }.remove-schedule:disabled { color: var(--muted); opacity: .4; cursor: not-allowed; }
.class-form footer { display: flex; justify-content: flex-end; gap: 9px; padding-top: 16px; border-top: 1px solid var(--line-soft); }.class-form button { min-height: 40px; padding: 0 16px; border: 1px solid var(--line); border-radius: 8px; cursor: pointer; }.quiet { background: white; }.primary { border-color: var(--brand) !important; color: white; background: var(--brand); }
@media (max-width: 720px) { .form-grid { grid-template-columns: 1fr 1fr; } fieldset { grid-template-columns: 1fr 1fr; } } @media (max-width: 480px) { .form-grid, fieldset { grid-template-columns: 1fr; } }
</style>
