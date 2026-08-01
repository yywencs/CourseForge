<script setup lang="ts">
import { Search, SlidersHorizontal } from '@lucide/vue'
import { useQuery } from '@tanstack/vue-query'
import { computed, shallowRef, watch } from 'vue'

import { listStudentCatalog } from '@/api/catalog'
import { listMyEnrollments, listMyWaitlist } from '@/api/enrollment'
import CourseCard from '@/components/CourseCard.vue'
import { useCatalogActions } from '@/composables/useCatalogActions'
import { courseCatalog, replaceCourseCatalog } from '@/data/courseCatalog'
import { useSessionStore } from '@/stores/session'
import type { TeachingClassSummary } from '@/types/enrollment'
import CourseMediaRail from './CourseMediaRail.vue'
import CoursePreviewDialog from './CoursePreviewDialog.vue'
import TeachingClassEntryForm from './TeachingClassEntryForm.vue'

const session = useSessionStore()
const keyword = shallowRef('')
const previewCourse = shallowRef<TeachingClassSummary>()
const previewOpen = shallowRef(false)
const customCourses = shallowRef<TeachingClassSummary[]>([])
const showDirectEntry = shallowRef(false)
const { activeTeachingClassId, activeAction, selectionMutation, waitlistMutation } =
  useCatalogActions()

const catalogQuery = useQuery({
  queryKey: computed(() => ['course-catalog', session.context.roundId]),
  queryFn: () => listStudentCatalog(session.context.roundId),
  enabled: computed(() => session.isAuthenticated && session.context.roundId > 0),
})

watch(
  [() => catalogQuery.data.value?.items, () => session.context.roundId],
  ([items, roundId]) => replaceCourseCatalog(items ?? [], roundId),
  { immediate: true },
)

const enrollmentsQuery = useQuery({
  queryKey: computed(() => ['my-enrollments', session.studentId, session.context.termId]),
  queryFn: () => listMyEnrollments(session.context.termId),
  enabled: computed(() => session.isAuthenticated && session.context.termId > 0),
})

const waitlistQuery = useQuery({
  queryKey: computed(() => ['my-waitlist', session.studentId, session.context.termId]),
  queryFn: () => listMyWaitlist(session.context.termId),
  enabled: computed(() => session.isAuthenticated && session.context.termId > 0),
})

const courses = computed(() => [
  ...customCourses.value,
  ...courseCatalog(session.context.roundId),
])
const selectedClassIds = computed(
  () => new Set((enrollmentsQuery.data.value?.items ?? [])
    .filter((item) => item.state !== 'dropped')
    .map((item) => item.teaching_class_id)),
)
const waitlistedClassIds = computed(
  () => new Set((waitlistQuery.data.value?.items ?? [])
    .filter((item) => ['waiting', 'promoting'].includes(item.state))
    .map((item) => item.teaching_class_id)),
)
const filteredCourses = computed(() => {
  const normalized = keyword.value.trim().toLowerCase()
  if (!normalized) return courses.value
  return courses.value.filter((course) =>
    [course.courseCode, course.courseName, course.teacherName, String(course.id)]
      .join(' ')
      .toLowerCase()
      .includes(normalized),
  )
})

function openPreview(course: TeachingClassSummary): void {
  previewCourse.value = course
  previewOpen.value = true
}

function submitDirect(course: TeachingClassSummary): void {
  customCourses.value = [course, ...customCourses.value.filter((item) => item.id !== course.id)]
  selectionMutation.mutate(course)
}
</script>

<template>
  <div class="catalog-page">
    <header class="catalog-masthead">
      <div>
        <h1>选择本学期课程</h1>
        <p>比较课程内容、教师、时间与名额，做出这轮选课决定。</p>
      </div>
      <div class="catalog-context">
        <span>学期 {{ session.context.termId }}</span>
        <strong>第 {{ session.context.roundId }} 轮选课</strong>
      </div>
    </header>

    <div class="catalog-layout">
      <main class="catalog-main">
        <div class="catalog-tools">
          <label class="course-search">
            <Search :size="21" />
            <input v-model="keyword" type="search" placeholder="搜索课程、教师、代码或教学班 ID" />
            <span>{{ filteredCourses.length }} 门课程</span>
          </label>
          <button class="direct-toggle" type="button" :aria-expanded="showDirectEntry" @click="showDirectEntry = !showDirectEntry">
            <SlidersHorizontal :size="18" />教学班直达
          </button>
        </div>

        <Transition name="entry">
          <TeachingClassEntryForm
            v-if="showDirectEntry"
            :round-id="session.context.roundId"
            @submit="submitDirect"
          />
        </Transition>

        <p v-if="enrollmentsQuery.isError.value || waitlistQuery.isError.value" class="catalog-sync-warning" role="alert">
          暂时无法同步已有选课或候补状态，请稍后刷新重试。
        </p>

        <p v-if="catalogQuery.isError.value" class="catalog-sync-warning" role="alert">
          课程目录暂时无法加载，请检查 API 和数据库迁移后重试。
        </p>

        <section class="course-ledger" aria-labelledby="course-ledger-title">
          <header>
            <h2 id="course-ledger-title">本轮课程目录</h2>
            <span>按课程信息逐项比较</span>
          </header>
          <div v-if="catalogQuery.isLoading.value" class="catalog-empty" aria-live="polite">
            <strong>正在加载课程目录</strong>
            <span>请稍候。</span>
          </div>
          <div v-else-if="filteredCourses.length" class="course-list">
            <CourseCard
              v-for="(course, index) in filteredCourses"
              :key="course.id"
              :course="course"
              :index="index + 1"
              :selected="selectedClassIds.has(course.id)"
              :waitlisted="waitlistedClassIds.has(course.id)"
              :submitting="activeTeachingClassId === course.id && activeAction === 'select'"
              :joining-waitlist="activeTeachingClassId === course.id && activeAction === 'waitlist'"
              @select="selectionMutation.mutate"
              @join-waitlist="waitlistMutation.mutate"
              @play-video="openPreview"
            />
          </div>
          <div v-else class="catalog-empty">
            <strong>没有匹配的课程</strong>
            <span>更换关键词，或使用“教学班直达”。</span>
          </div>
        </section>
      </main>

      <CourseMediaRail
        :courses="courses"
        :selected-count="selectedClassIds.size"
        :waitlist-count="waitlistedClassIds.size"
        :term-id="session.context.termId"
        :round-id="session.context.roundId"
        @play-video="openPreview"
      />
    </div>

    <CoursePreviewDialog v-model="previewOpen" :course="previewCourse" />
  </div>
</template>

<style scoped>
.catalog-page {
  display: grid;
  gap: 34px;
}

.catalog-masthead {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 40px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--line);
}

.catalog-masthead h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(38px, 5vw, 56px);
  font-variation-settings: "wdth" 82, "wght" 680;
  letter-spacing: -0.025em;
  line-height: 1.12;
}

.catalog-masthead p {
  max-width: 620px;
  margin: 22px 0 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.7;
}

.catalog-context {
  display: grid;
  min-width: 210px;
  gap: 6px;
  padding: 15px 0 4px 20px;
  border-left: 1px solid var(--line);
}

.catalog-context span {
  color: var(--muted);
  font-size: 11px;
}

.catalog-context strong {
  font-family: var(--font-display);
  font-size: 22px;
  font-variation-settings: "wdth" 76, "wght" 750;
}

.catalog-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  align-items: start;
  gap: 22px;
}

.catalog-main {
  display: grid;
  min-width: 0;
  gap: 16px;
}

.catalog-tools {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
}

.course-search {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 12px;
  min-height: 58px;
  padding: 7px 15px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface);
}

.course-search input {
  min-width: 0;
  padding: 8px 0;
  border: 0;
  outline: 0;
  background: transparent;
}

.course-search > span {
  color: var(--muted);
  font-size: 10px;
}

.direct-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 17px;
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--ink);
  background: var(--surface);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.direct-toggle:hover,
.direct-toggle[aria-expanded="true"] {
  border-color: var(--brand);
  color: var(--brand);
  background: var(--brand-pale);
}

.catalog-sync-warning {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid #e2ae63;
  border-radius: 10px;
  color: #714300;
  background: #fff4db;
  font-size: 12px;
}

.course-ledger {
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
}

.course-ledger > header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 55px;
  padding: 13px 18px;
  border-bottom: 1px solid var(--line-soft);
  color: var(--ink);
  background: var(--surface-muted);
}

.course-ledger h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 20px;
  font-variation-settings: "wdth" 75, "wght" 760;
}

.course-ledger header span {
  color: var(--muted);
  font-size: 10px;
}

.course-list {
  display: grid;
}

.catalog-empty {
  display: grid;
  justify-items: center;
  gap: 7px;
  padding: 64px 20px;
  color: var(--muted);
  font-size: 12px;
}

.catalog-empty strong {
  color: var(--ink);
  font-size: 15px;
}

.entry-enter-active,
.entry-leave-active {
  transition: opacity 180ms ease-out, transform 180ms ease-out;
}

.entry-enter-from,
.entry-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

@media (max-width: 1100px) {
  .catalog-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 680px) {
  .catalog-page {
    gap: 22px;
  }

  .catalog-masthead {
    align-items: flex-start;
    flex-direction: column;
    gap: 20px;
  }

  .catalog-masthead h1 {
    font-size: 38px;
  }

  .catalog-masthead p {
    margin-top: 15px;
    font-size: 12px;
  }

  .catalog-context {
    width: 100%;
    padding: 12px 0 0;
    border-top: 1px solid var(--line);
    border-left: 0;
  }

  .catalog-tools {
    grid-template-columns: 1fr;
  }

  .direct-toggle {
    min-height: 46px;
    justify-content: center;
  }

  .course-search > span,
  .course-ledger header span {
    display: none;
  }
}
</style>
