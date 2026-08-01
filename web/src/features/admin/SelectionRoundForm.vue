<script setup lang="ts">
import { reactive, watch } from 'vue'

import type { SelectionRound, SelectionRoundDraft } from '@/types/catalog'

const props = defineProps<{ round?: SelectionRound }>()
const emit = defineEmits<{ save: [draft: SelectionRoundDraft]; cancel: [] }>()
const form = reactive({ term_id: 0, round_code: '', round_name: '', start_time: '', end_time: '' })

function localDateTime(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

watch(
  () => props.round,
  (round) => Object.assign(form, {
    term_id: round?.term_id ?? 0, round_code: round?.round_code ?? '', round_name: round?.round_name ?? '',
    start_time: localDateTime(round?.start_time), end_time: localDateTime(round?.end_time),
  }),
  { immediate: true },
)

function submit(): void {
  emit('save', {
    term_id: Number(form.term_id), round_code: form.round_code.trim(), round_name: form.round_name.trim(),
    start_time: new Date(form.start_time).toISOString(), end_time: new Date(form.end_time).toISOString(),
  })
}
</script>

<template>
  <form class="round-form" @submit.prevent="submit">
    <header><h2>{{ round ? '编辑选课轮次' : '新增选课轮次' }}</h2><p>保存后仍是计划状态，开放动作由独立业务命令完成。</p></header>
    <div>
      <label><span>学期 ID</span><input v-model.number="form.term_id" type="number" min="1" required /></label>
      <label><span>轮次代码</span><input v-model="form.round_code" required /></label>
      <label><span>轮次名称</span><input v-model="form.round_name" required /></label>
      <label><span>开始时间</span><input v-model="form.start_time" type="datetime-local" required /></label>
      <label><span>结束时间</span><input v-model="form.end_time" type="datetime-local" required /></label>
    </div>
    <footer><button type="button" @click="emit('cancel')">取消</button><button class="primary" type="submit">保存轮次</button></footer>
  </form>
</template>

<style scoped>
.round-form { display: grid; gap: 22px; }.round-form header h2 { margin: 0; font-size: 22px; }.round-form header p { margin: 5px 0 0; color: var(--muted); font-size: 12px; }
.round-form > div { display: grid; grid-template-columns: 1fr 1fr; gap: 15px; }.round-form label { display: grid; gap: 6px; color: var(--muted); font-size: 10px; }.round-form input { width: 100%; padding: 11px 12px; border: 1px solid var(--line); border-radius: 8px; outline: 0; }.round-form input:focus { border-color: var(--brand); }
.round-form footer { display: flex; justify-content: flex-end; gap: 9px; padding-top: 16px; border-top: 1px solid var(--line-soft); }.round-form button { min-height: 40px; padding: 0 16px; border: 1px solid var(--line); border-radius: 8px; background: white; cursor: pointer; }.round-form .primary { border-color: var(--brand); color: white; background: var(--brand); }
@media (max-width: 600px) { .round-form > div { grid-template-columns: 1fr; } }
</style>
