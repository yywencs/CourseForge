<script setup lang="ts">
import { computed, reactive, watch } from 'vue'

import type { Course, CourseDraft } from '@/types/catalog'

const props = defineProps<{ course?: Course }>()
const emit = defineEmits<{ save: [draft: CourseDraft]; cancel: [] }>()

const form = reactive({
  course_code: '', course_name: '', credits: 1, introduction: '', tags: '', video_url: '',
})
const title = computed(() => props.course ? '编辑课程' : '新增课程')

watch(
  () => props.course,
  (course) => Object.assign(form, {
    course_code: course?.course_code ?? '',
    course_name: course?.course_name ?? '',
    credits: course?.credits ?? 1,
    introduction: course?.introduction ?? '',
    tags: course?.tags.join('，') ?? '',
    video_url: course?.video_url ?? '',
  }),
  { immediate: true },
)

function submit(): void {
  emit('save', {
    course_code: form.course_code.trim(),
    course_name: form.course_name.trim(),
    credits: Number(form.credits),
    introduction: form.introduction.trim(),
    tags: form.tags.split(/[，,]/).map((item) => item.trim()).filter(Boolean),
    video_url: form.video_url.trim(),
  })
}
</script>

<template>
  <form class="editor-form" @submit.prevent="submit">
    <header><h2>{{ title }}</h2><p>课程资料可跨学期复用。</p></header>
    <div class="form-grid">
      <label><span>课程代码</span><input v-model="form.course_code" required maxlength="32" /></label>
      <label><span>课程名称</span><input v-model="form.course_name" required maxlength="128" /></label>
      <label><span>学分</span><input v-model.number="form.credits" type="number" min="0.5" step="0.5" required /></label>
      <label class="is-wide"><span>课程标签</span><input v-model="form.tags" placeholder="用逗号分隔，例如：专业核心，项目制" /></label>
      <label class="is-wide"><span>课程简介</span><textarea v-model="form.introduction" rows="4" maxlength="1000" /></label>
      <label class="is-wide"><span>课程视频地址</span><input v-model="form.video_url" type="url" placeholder="https://" /></label>
    </div>
    <footer><button type="button" class="quiet" @click="emit('cancel')">取消</button><button type="submit" class="primary">保存课程</button></footer>
  </form>
</template>

<style scoped>
.editor-form { display: grid; gap: 22px; }
.editor-form header h2 { margin: 0; font-size: 22px; }
.editor-form header p { margin: 5px 0 0; color: var(--muted); font-size: 12px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr 120px; gap: 16px; }
.form-grid label { display: grid; gap: 7px; color: var(--muted); font-size: 11px; }
.form-grid .is-wide { grid-column: 1 / -1; }
.form-grid input, .form-grid textarea { width: 100%; padding: 11px 12px; border: 1px solid var(--line); border-radius: 8px; outline: 0; color: var(--ink); background: white; }
.form-grid input:focus, .form-grid textarea:focus { border-color: var(--brand); }
.editor-form footer { display: flex; justify-content: flex-end; gap: 9px; padding-top: 16px; border-top: 1px solid var(--line-soft); }
.editor-form button { min-height: 40px; padding: 0 16px; border: 1px solid var(--line); border-radius: 8px; cursor: pointer; }
.quiet { color: var(--ink); background: white; }
.primary { border-color: var(--brand) !important; color: white; background: var(--brand); }
@media (max-width: 660px) { .form-grid { grid-template-columns: 1fr; } .form-grid .is-wide { grid-column: auto; } }
</style>
