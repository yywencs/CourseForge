<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { Link2Off, Plus } from '@lucide/vue'

import type { RoundClassBinding, SelectionRound, TeachingClass } from '@/types/catalog'

const props = defineProps<{ round: SelectionRound; teachingClasses: readonly TeachingClass[]; bindings: readonly RoundClassBinding[]; busy: boolean }>()
const emit = defineEmits<{ bind: [teachingClassId: number]; unbind: [teachingClassId: number]; close: [] }>()
const selectedClassId = shallowRef(0)
const boundIds = computed(() => new Set(props.bindings.map((item) => item.teaching_class_id)))
const candidates = computed(() => props.teachingClasses.filter((item) =>
  item.term_id === props.round.term_id && item.state === 'planned' && !boundIds.value.has(item.id),
))

function submit(): void {
  if (selectedClassId.value > 0) emit('bind', selectedClassId.value)
}
</script>

<template>
  <section class="binding-panel">
    <header><div><h2>轮次教学班</h2><p>{{ round.round_name }} · 学期 {{ round.term_id }}</p></div><button type="button" @click="emit('close')">完成</button></header>
    <form v-if="round.state === 'planned'" @submit.prevent="submit">
      <select v-model.number="selectedClassId" required><option :value="0" disabled>选择同学期、计划中的教学班</option><option v-for="item in candidates" :key="item.id" :value="item.id">{{ item.class_code }} · {{ item.course_name }}</option></select>
      <button type="submit" :disabled="busy || selectedClassId === 0"><Plus :size="16" />加入轮次</button>
    </form>
    <p v-else class="notice">轮次已进入选课流程，当前仅可查看绑定。</p>
    <div class="bindings">
      <article v-for="item in bindings" :key="item.id"><div><strong>{{ item.class_code }}</strong><span>{{ item.course_name }}</span></div><button v-if="round.state === 'planned'" type="button" :disabled="busy" aria-label="移出轮次" @click="emit('unbind', item.teaching_class_id)"><Link2Off :size="16" /></button></article>
      <p v-if="!bindings.length">还没有绑定教学班。</p>
    </div>
  </section>
</template>

<style scoped>
.binding-panel { display: grid; gap: 18px; }.binding-panel > header { display: flex; justify-content: space-between; align-items: start; }.binding-panel h2 { margin: 0; font-size: 22px; }.binding-panel header p { margin: 5px 0 0; color: var(--muted); font-size: 11px; }.binding-panel header button { border: 0; color: var(--brand); background: transparent; cursor: pointer; }
.binding-panel form { display: grid; grid-template-columns: 1fr auto; gap: 8px; }.binding-panel select { min-width: 0; padding: 11px; border: 1px solid var(--line); border-radius: 8px; background: white; }.binding-panel form button { display: flex; align-items: center; gap: 6px; padding: 0 14px; border: 1px solid var(--brand); border-radius: 8px; color: white; background: var(--brand); cursor: pointer; }.binding-panel button:disabled { opacity: .5; cursor: wait; }
.notice { margin: 0; padding: 11px; border: 1px solid var(--line-soft); border-radius: 8px; color: var(--muted); background: var(--surface-muted); font-size: 11px; }.bindings { overflow: hidden; border: 1px solid var(--line-soft); border-radius: 10px; }.bindings article { display: flex; justify-content: space-between; align-items: center; min-height: 58px; padding: 10px 13px; border-bottom: 1px solid var(--line-soft); }.bindings article:last-child { border: 0; }.bindings article div { display: grid; gap: 3px; }.bindings strong { font-family: var(--font-mono); font-size: 10px; }.bindings span, .bindings > p { color: var(--muted); font-size: 10px; }.bindings article button { display: grid; width: 32px; height: 32px; place-items: center; border: 1px solid var(--line); border-radius: 7px; color: var(--danger); background: white; cursor: pointer; }.bindings > p { margin: 0; padding: 35px; text-align: center; }
@media (max-width: 520px) { .binding-panel form { grid-template-columns: 1fr; }.binding-panel form button { min-height: 42px; justify-content: center; } }
</style>
