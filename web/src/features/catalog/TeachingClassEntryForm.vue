<script setup lang="ts">
import { CornerDownLeft, Hash } from '@lucide/vue'
import { reactive } from 'vue'

import type { TeachingClassSummary } from '@/types/enrollment'

const props = defineProps<{
  roundId: number
}>()

const emit = defineEmits<{
  submit: [course: TeachingClassSummary]
}>()

const form = reactive({
  teachingClassId: '',
  courseName: '',
})

function submit(): void {
  const teachingClassId = Number(form.teachingClassId)
  if (!Number.isSafeInteger(teachingClassId) || teachingClassId <= 0) return
  emit('submit', {
    id: teachingClassId,
    roundId: props.roundId,
    courseId: teachingClassId,
    courseCode: `CLASS-${teachingClassId}`,
    courseName: form.courseName.trim() || `教学班 #${teachingClassId}`,
    teacherName: '教师待公布',
    credits: 0,
    schedule: '时间待公布',
    location: '地点待公布',
    capacity: 1,
    selectedCount: 0,
    tags: ['教学班直达'],
    introduction: '输入教学班编号即可提交选课。',
    hasVideo: false,
  })
}
</script>

<template>
  <form class="class-entry" @submit.prevent="submit">
    <div class="class-entry__label">
      <Hash :size="17" />
      <span>
        <strong>教学班直达</strong>
        <small>输入教学班编号快速选课</small>
      </span>
    </div>
    <label>
      <span>教学班编号</span>
      <input
        v-model="form.teachingClassId"
        inputmode="numeric"
        placeholder="例如 20001"
        required
      />
    </label>
    <label>
      <span>显示名称</span>
      <input v-model="form.courseName" placeholder="可选" />
    </label>
    <button type="submit">
      进入选课
      <CornerDownLeft :size="16" />
    </button>
  </form>
</template>

<style scoped>
.class-entry {
  display: grid;
  grid-template-columns: minmax(210px, 1.2fr) 1fr 1fr auto;
  align-items: end;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--ink);
  border-radius: 12px;
  background: var(--surface);
}

.class-entry__label {
  display: flex;
  align-items: center;
  gap: 10px;
}

.class-entry__label > span {
  display: grid;
  gap: 2px;
}

.class-entry__label strong {
  font-size: 13px;
}

.class-entry__label small {
  color: var(--muted);
  font-size: 10px;
}

.class-entry label {
  display: grid;
  gap: 5px;
}

.class-entry label span {
  color: var(--muted);
  font-size: 10px;
}

.class-entry input {
  width: 100%;
  min-width: 0;
  padding: 10px 11px;
  border: 1px solid var(--line);
  border-radius: 8px;
  outline: none;
  color: var(--ink);
  background: #fff;
}

.class-entry input::placeholder {
  color: #8c8982;
}

.class-entry input:focus {
  border-color: var(--brand);
}

.class-entry button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 11px 14px;
  border: 1px solid var(--ink);
  border-radius: 8px;
  color: white;
  background: var(--ink);
  font-weight: 750;
  cursor: pointer;
}

@media (max-width: 800px) {
  .class-entry {
    grid-template-columns: 1fr 1fr;
  }

  .class-entry__label {
    grid-column: 1 / -1;
  }
}

@media (max-width: 520px) {
  .class-entry {
    grid-template-columns: 1fr;
  }
}
</style>
