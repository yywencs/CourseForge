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
    teacherName: '由后端教学班数据确定',
    credits: 0,
    schedule: '提交后按后端数据校验',
    location: '待课程查询接口返回',
    capacity: 1,
    selectedCount: 0,
    tags: ['教学班直达'],
    introduction: '课程目录接口尚未提供时，可使用教学班 ID 直接进入正式选课主链路。',
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
        <small>按后端真实 ID 提交，不依赖演示目录</small>
      </span>
    </div>
    <label>
      <span>教学班 ID</span>
      <input
        v-model="form.teachingClassId"
        inputmode="numeric"
        placeholder="例如 30001"
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
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 16px;
  background: rgba(5, 36, 29, 0.32);
  backdrop-filter: blur(12px);
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
  color: rgba(255, 255, 255, 0.62);
  font-size: 10px;
}

.class-entry label {
  display: grid;
  gap: 5px;
}

.class-entry label span {
  color: rgba(255, 255, 255, 0.64);
  font-size: 10px;
}

.class-entry input {
  width: 100%;
  min-width: 0;
  padding: 10px 11px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 9px;
  outline: none;
  color: white;
  background: rgba(255, 255, 255, 0.09);
}

.class-entry input::placeholder {
  color: rgba(255, 255, 255, 0.38);
}

.class-entry input:focus {
  border-color: var(--brand-bright);
}

.class-entry button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 11px 14px;
  border: 0;
  border-radius: 10px;
  color: #0d3d32;
  background: #a8ead8;
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
