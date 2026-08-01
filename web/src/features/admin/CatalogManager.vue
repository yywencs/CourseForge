<script setup lang="ts">
import { Plus, RefreshCw, Search } from '@lucide/vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, shallowRef } from 'vue'

import { useCatalogAdmin } from '@/composables/useCatalogAdmin'
import type { Course, CourseDraft, SelectionRound, SelectionRoundDraft, TeachingClass, TeachingClassDraft } from '@/types/catalog'
import CatalogRecords from './CatalogRecords.vue'
import CourseForm from './CourseForm.vue'
import RoundBindingPanel from './RoundBindingPanel.vue'
import SelectionRoundForm from './SelectionRoundForm.vue'
import TeachingClassForm from './TeachingClassForm.vue'

type Section = 'courses' | 'classes' | 'rounds'
const sections: { key: Section; label: string }[] = [
  { key: 'courses', label: '课程' },
  { key: 'classes', label: '教学班' },
  { key: 'rounds', label: '选课轮次' },
]
const section = shallowRef<Section>('courses')
const keyword = shallowRef('')
const termFilter = shallowRef('')
const editorOpen = shallowRef(false)
const bindingOpen = shallowRef(false)
const editingCourse = shallowRef<Course>()
const editingClass = shallowRef<TeachingClass>()
const editingRound = shallowRef<SelectionRound>()
const managedRound = shallowRef<SelectionRound>()

const admin = useCatalogAdmin()
const normalizedKeyword = computed(() => keyword.value.trim().toLowerCase())
const termId = computed(() => Number(termFilter.value) || 0)
const visibleCourses = computed(() => admin.courses.value.filter((item) =>
  !normalizedKeyword.value || `${item.course_code} ${item.course_name}`.toLowerCase().includes(normalizedKeyword.value),
))
const visibleClasses = computed(() => admin.teachingClasses.value.filter((item) =>
  (!termId.value || item.term_id === termId.value) &&
  (!normalizedKeyword.value || `${item.class_code} ${item.course_code} ${item.course_name} ${item.teacher_name}`.toLowerCase().includes(normalizedKeyword.value)),
))
const visibleRounds = computed(() => admin.rounds.value.filter((item) =>
  (!termId.value || item.term_id === termId.value) &&
  (!normalizedKeyword.value || `${item.round_code} ${item.round_name}`.toLowerCase().includes(normalizedKeyword.value)),
))

onMounted(async () => {
  try { await admin.refreshAll() } catch (error) { showError(error, '课程配置加载失败') }
})

function openCreate(): void {
  editingCourse.value = undefined
  editingClass.value = undefined
  editingRound.value = undefined
  editorOpen.value = true
}

function editCourse(item: Course): void { editingCourse.value = item; editorOpen.value = true }
function editClass(item: TeachingClass): void { editingClass.value = item; editorOpen.value = true }
function editRound(item: SelectionRound): void { editingRound.value = item; editorOpen.value = true }

async function saveCourse(draft: CourseDraft): Promise<void> {
  try { await admin.saveCourse(draft, editingCourse.value?.id); editorOpen.value = false; ElMessage.success('课程已保存') } catch (error) { showError(error, '课程保存失败') }
}
async function saveClass(draft: TeachingClassDraft): Promise<void> {
  try { await admin.saveTeachingClass(draft, editingClass.value?.id); editorOpen.value = false; ElMessage.success('教学班已保存') } catch (error) { showError(error, '教学班保存失败') }
}
async function saveRound(draft: SelectionRoundDraft): Promise<void> {
  try { await admin.saveRound(draft, editingRound.value?.id); editorOpen.value = false; ElMessage.success('选课轮次已保存') } catch (error) { showError(error, '选课轮次保存失败') }
}

async function confirmDelete(kind: string, name: string, action: () => Promise<void>): Promise<void> {
  try {
    await ElMessageBox.confirm(`确定删除${kind}“${name}”吗？`, `删除${kind}`, { confirmButtonText: '确定删除', cancelButtonText: '取消', type: 'warning' })
    await action()
    ElMessage.success(`${kind}已删除`)
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    showError(error, `${kind}删除失败`)
  }
}

async function manageRound(item: SelectionRound): Promise<void> {
  managedRound.value = item
  bindingOpen.value = true
  try { await admin.loadBindings(item.id) } catch (error) { showError(error, '教学班绑定加载失败') }
}
async function addBinding(id: number): Promise<void> {
  try { await admin.addBinding(id); ElMessage.success('教学班已加入轮次') } catch (error) { showError(error, '加入轮次失败') }
}
async function removeBinding(id: number): Promise<void> {
  try { await admin.removeBinding(id); ElMessage.success('教学班已移出轮次') } catch (error) { showError(error, '移出轮次失败') }
}
function showError(error: unknown, fallback: string): void {
  ElMessage.error(error instanceof Error ? error.message : fallback)
}

async function refresh(): Promise<void> {
  try { await admin.refreshAll(); ElMessage.success('数据已刷新') } catch (error) { showError(error, '数据刷新失败') }
}
</script>

<template>
  <section class="catalog-manager" :aria-busy="admin.isLoading.value">
    <nav class="catalog-tabs" aria-label="课程配置分类">
      <button v-for="item in sections" :key="item.key" type="button" :class="{ active: section === item.key }" @click="section = item.key">{{ item.label }}</button>
    </nav>

    <div class="manager-tools">
      <label class="search"><Search :size="17" /><input v-model="keyword" type="search" :placeholder="section === 'courses' ? '搜索课程代码或名称' : '搜索代码或名称'" /></label>
      <label v-if="section !== 'courses'" class="term-filter"><span>学期</span><input v-model="termFilter" inputmode="numeric" placeholder="全部" /></label>
      <button type="button" class="refresh" :disabled="admin.isLoading.value" @click="refresh"><RefreshCw :size="16" />刷新</button>
      <button type="button" class="add" @click="openCreate"><Plus :size="17" />{{ section === 'courses' ? '新增课程' : section === 'classes' ? '新增教学班' : '新增轮次' }}</button>
    </div>

    <CatalogRecords
      :mode="section" :courses="visibleCourses" :teaching-classes="visibleClasses" :rounds="visibleRounds"
      @edit-course="editCourse" @edit-class="editClass" @edit-round="editRound" @manage-round="manageRound"
      @delete-course="(item) => confirmDelete('课程', item.course_name, () => admin.removeCourse(item.id))"
      @delete-class="(item) => confirmDelete('教学班', item.class_code, () => admin.removeTeachingClass(item.id))"
      @delete-round="(item) => confirmDelete('选课轮次', item.round_name, () => admin.removeRound(item.id))"
    />

    <ElDialog v-model="editorOpen" width="min(720px, calc(100vw - 28px))" :show-close="false" destroy-on-close align-center>
      <CourseForm v-if="section === 'courses'" :course="editingCourse" @save="saveCourse" @cancel="editorOpen = false" />
      <TeachingClassForm v-else-if="section === 'classes'" :courses="admin.courses.value" :teaching-class="editingClass" @save="saveClass" @cancel="editorOpen = false" />
      <SelectionRoundForm v-else :round="editingRound" @save="saveRound" @cancel="editorOpen = false" />
    </ElDialog>

    <ElDialog v-model="bindingOpen" width="min(620px, calc(100vw - 28px))" :show-close="false" destroy-on-close align-center>
      <RoundBindingPanel v-if="managedRound" :round="managedRound" :teaching-classes="admin.teachingClasses.value" :bindings="admin.bindings.value" :busy="admin.isLoading.value" @bind="addBinding" @unbind="removeBinding" @close="bindingOpen = false" />
    </ElDialog>
  </section>
</template>

<style scoped>
.catalog-manager { display: grid; gap: 16px; }.catalog-tabs { display: flex; gap: 3px; width: fit-content; padding: 3px; border: 1px solid var(--line); border-radius: 9px; background: var(--surface-muted); }.catalog-tabs button { min-height: 36px; padding: 0 15px; border: 0; border-radius: 6px; color: var(--muted); background: transparent; font-size: 11px; cursor: pointer; }.catalog-tabs button.active { color: var(--ink); background: white; box-shadow: 0 1px 2px rgb(0 0 0 / 7%); }
.manager-tools { display: grid; grid-template-columns: minmax(220px, 1fr) 150px auto auto; gap: 8px; }.search, .term-filter { display: flex; align-items: center; gap: 8px; min-height: 43px; padding: 0 12px; border: 1px solid var(--line); border-radius: 8px; background: white; }.search input, .term-filter input { min-width: 0; width: 100%; border: 0; outline: 0; background: transparent; }.term-filter span { color: var(--muted); font-size: 9px; }.manager-tools > button { display: flex; align-items: center; justify-content: center; gap: 6px; min-height: 43px; padding: 0 13px; border: 1px solid var(--line); border-radius: 8px; background: white; font-size: 11px; font-weight: 700; cursor: pointer; }.manager-tools .add { border-color: var(--brand); color: white; background: var(--brand); }.manager-tools button:disabled { opacity: .5; cursor: wait; }
@media (max-width: 720px) { .manager-tools { grid-template-columns: 1fr 1fr; }.search { grid-column: 1 / -1; } } @media (max-width: 460px) { .manager-tools { grid-template-columns: 1fr; }.search { grid-column: auto; } }
</style>
