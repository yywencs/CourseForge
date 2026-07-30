<script setup lang="ts">
import { Search, ShieldCheck, Sparkles } from '@lucide/vue'
import { useQuery } from '@tanstack/vue-query'
import { computed, shallowRef } from 'vue'

import { listMyEnrollments, listMyWaitlist } from '@/api/enrollment'
import CourseCard from '@/components/CourseCard.vue'
import { useCatalogActions } from '@/composables/useCatalogActions'
import { courseCatalog } from '@/data/courseCatalog'
import { useSessionStore } from '@/stores/session'
import type { TeachingClassSummary } from '@/types/enrollment'
import CoursePreviewDialog from './CoursePreviewDialog.vue'
import TeachingClassEntryForm from './TeachingClassEntryForm.vue'

const session = useSessionStore()
const keyword = shallowRef('')
const previewCourse = shallowRef<TeachingClassSummary>()
const previewOpen = shallowRef(false)
const customCourses = shallowRef<TeachingClassSummary[]>([])
const { activeTeachingClassId, activeAction, selectionMutation, waitlistMutation } =
  useCatalogActions()

const enrollmentsQuery = useQuery({
  queryKey: computed(() => [
    'my-enrollments',
    session.studentId,
    session.context.termId,
  ]),
  queryFn: () => listMyEnrollments(session.context.termId),
  enabled: computed(
    () => session.isAuthenticated && session.context.termId > 0,
  ),
})

const waitlistQuery = useQuery({
  queryKey: computed(() => [
    'my-waitlist',
    session.studentId,
    session.context.termId,
  ]),
  queryFn: () => listMyWaitlist(session.context.termId),
  enabled: computed(
    () => session.isAuthenticated && session.context.termId > 0,
  ),
})

const courses = computed(() => [
  ...customCourses.value,
  ...courseCatalog(session.context.roundId),
])
const selectedClassIds = computed(
  () =>
    new Set(
      (enrollmentsQuery.data.value?.items ?? [])
        .filter((item) => item.state !== 'dropped')
        .map((item) => item.teaching_class_id),
    ),
)
const waitlistedClassIds = computed(
  () =>
    new Set(
      (waitlistQuery.data.value?.items ?? [])
        .filter((item) => ['waiting', 'promoting'].includes(item.state))
        .map((item) => item.teaching_class_id),
    ),
)
const filteredCourses = computed(() => {
  const normalized = keyword.value.trim().toLowerCase()
  if (!normalized) return courses.value
  return courses.value.filter((course) =>
    [
      course.courseCode,
      course.courseName,
      course.teacherName,
      String(course.id),
    ]
      .join(' ')
      .toLowerCase()
      .includes(normalized),
  )
})
const activeCount = computed(
  () =>
    (enrollmentsQuery.data.value?.items ?? []).filter(
      (item) => item.state === 'enrolled',
    ).length,
)

function openPreview(course: TeachingClassSummary): void {
  previewCourse.value = course
  previewOpen.value = true
}

function submitDirect(course: TeachingClassSummary): void {
  customCourses.value = [
    course,
    ...customCourses.value.filter((item) => item.id !== course.id),
  ]
  selectionMutation.mutate(course)
}
</script>

<template>
  <div>
    <section class="catalog-hero">
      <div class="catalog-hero__copy">
        <div class="round-kicker">
          <Sparkles :size="15" />
          轮次 {{ session.context.roundId }} · 学期 {{ session.context.termId }}
        </div>
        <h1>把想学的课，<br />放进确定的结果里。</h1>
        <p>资格校验、名额锁定和申请追踪都在一次操作后持续可见。</p>
      </div>

      <aside class="decision-rail" aria-label="选课处理链路">
        <span>当前处理链</span>
        <ol>
          <li><i>1</i><b>资格</b><small>专业 · 年级 · 先修 · 时间</small></li>
          <li><i>2</i><b>原子锁定</b><small>额度与教学班名额</small></li>
          <li><i>3</i><b>可靠落库</b><small>Stream · Confirm · MySQL</small></li>
        </ol>
        <div><ShieldCheck :size="15" /> 已选 {{ activeCount }} 门</div>
      </aside>

      <label class="course-search">
        <Search :size="19" />
        <input
          v-model="keyword"
          type="search"
          placeholder="搜索课程、教师、代码或教学班 ID"
        />
        <kbd>⌘ K</kbd>
      </label>

      <TeachingClassEntryForm
        :round-id="session.context.roundId"
        @submit="submitDirect"
      />
    </section>

    <div class="catalog-heading">
      <div>
        <span>Course ledger</span>
        <h2>本轮课程目录</h2>
      </div>
      <p>目录数据为前端展示适配层；提交时由后端重新读取课程、学分与资格，不信任页面数据。</p>
    </div>

    <p
      v-if="enrollmentsQuery.isError.value || waitlistQuery.isError.value"
      class="catalog-sync-warning"
      role="alert"
    >
      暂时无法同步本人已有选课或候补状态。提交操作仍由后端做重复选课校验，
      建议服务恢复后刷新页面再操作。
    </p>

    <div v-if="filteredCourses.length" class="course-grid">
      <CourseCard
        v-for="course in filteredCourses"
        :key="course.id"
        :course="course"
        :selected="selectedClassIds.has(course.id)"
        :waitlisted="waitlistedClassIds.has(course.id)"
        :submitting="
          activeTeachingClassId === course.id && activeAction === 'select'
        "
        :joining-waitlist="
          activeTeachingClassId === course.id && activeAction === 'waitlist'
        "
        @select="selectionMutation.mutate"
        @join-waitlist="waitlistMutation.mutate"
        @play-video="openPreview"
      />
    </div>
    <div v-else class="catalog-empty">
      <strong>没有匹配的课程</strong>
      <span>更换关键词，或使用上方教学班 ID 直达。</span>
    </div>

    <CoursePreviewDialog v-model="previewOpen" :course="previewCourse" />
  </div>
</template>

<style scoped>
.catalog-hero {
  position: relative;
  display: grid;
  overflow: hidden;
  grid-template-columns: minmax(0, 1fr) 310px;
  gap: 30px;
  padding: clamp(30px, 5vw, 58px);
  border-radius: 28px 28px 28px 8px;
  color: white;
  background:
    radial-gradient(circle at 88% 4%, rgba(84, 209, 176, 0.25), transparent 30%),
    linear-gradient(125deg, #0a3c31, #0d5a48);
  box-shadow: 0 28px 70px rgba(11, 66, 52, 0.2);
}

.catalog-hero__copy {
  align-self: center;
}

.round-kicker {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #bcebdd;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.catalog-hero h1 {
  max-width: 720px;
  margin: 17px 0 13px;
  font-family: var(--font-display);
  font-size: clamp(40px, 6vw, 68px);
  font-weight: 650;
  letter-spacing: -0.055em;
  line-height: 1.06;
}

.catalog-hero__copy p {
  margin: 0;
  color: rgba(255, 255, 255, 0.68);
  font-size: 14px;
}

.decision-rail {
  position: relative;
  padding: 21px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 18px;
  background: rgba(4, 35, 28, 0.38);
}

.decision-rail > span {
  color: rgba(255, 255, 255, 0.52);
  font-family: var(--font-mono);
  font-size: 10px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.decision-rail ol {
  display: grid;
  gap: 16px;
  margin: 18px 0;
  padding: 0;
  list-style: none;
}

.decision-rail li {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 2px 10px;
}

.decision-rail li i {
  display: grid;
  width: 23px;
  height: 23px;
  grid-row: 1 / 3;
  place-items: center;
  border-radius: 7px;
  color: #0a4738;
  background: #a6e9d6;
  font-size: 10px;
  font-style: normal;
  font-weight: 800;
}

.decision-rail li b {
  font-size: 13px;
}

.decision-rail li small {
  color: rgba(255, 255, 255, 0.5);
  font-size: 10px;
}

.decision-rail > div {
  display: flex;
  align-items: center;
  gap: 7px;
  padding-top: 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.12);
  color: #bcebdd;
  font-size: 11px;
}

.course-search {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 11px;
  padding: 8px 12px 8px 16px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 14px;
  color: var(--ink);
  background: white;
}

.course-search input {
  min-width: 0;
  padding: 8px 0;
  border: 0;
  outline: 0;
}

.course-search kbd {
  padding: 4px 7px;
  border: 1px solid #dce5e1;
  border-radius: 6px;
  color: var(--muted);
  background: #f2f6f4;
  font-family: var(--font-mono);
  font-size: 10px;
}

.catalog-hero :deep(.class-entry) {
  grid-column: 1 / -1;
}

.catalog-heading {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 24px;
  margin: 50px 0 22px;
}

.catalog-heading span {
  color: #9a650a;
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.catalog-heading h2 {
  margin: 5px 0 0;
  font-family: var(--font-display);
  font-size: 31px;
}

.catalog-heading p {
  max-width: 510px;
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
  text-align: right;
}

.course-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}

.catalog-empty {
  display: grid;
  justify-items: center;
  gap: 7px;
  padding: 70px 20px;
  border: 1px dashed #b9cec7;
  border-radius: 18px;
  color: var(--muted);
}

.catalog-sync-warning {
  margin: 0 0 18px;
  padding: 12px 14px;
  border: 1px solid #ead6a5;
  border-radius: 11px;
  color: #76500f;
  background: #fff5d9;
  font-size: 12px;
  line-height: 1.6;
}

.catalog-empty strong {
  color: var(--ink);
}

@media (max-width: 980px) {
  .course-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .catalog-hero {
    grid-template-columns: 1fr;
    padding: 28px 22px;
  }

  .catalog-hero h1 {
    font-size: 40px;
  }

  .catalog-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .catalog-heading p {
    text-align: left;
  }
}

@media (max-width: 620px) {
  .course-grid {
    grid-template-columns: 1fr;
  }
}
</style>
