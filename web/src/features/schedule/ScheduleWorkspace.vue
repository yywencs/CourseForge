<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { CalendarX2, MapPin } from '@lucide/vue'
import { computed } from 'vue'

import { listMyEnrollments } from '@/api/enrollment'
import { findCourse } from '@/data/courseCatalog'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const weekdays = ['周一', '周二', '周三', '周四', '周五']
const periods = [
  { label: '1–2 节', start: 1, end: 2 },
  { label: '3–4 节', start: 3, end: 4 },
  { label: '5–6 节', start: 5, end: 6 },
  { label: '7–8 节', start: 7, end: 8 },
]

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

const scheduledCourses = computed(() =>
  (enrollmentsQuery.data.value?.items ?? [])
    .filter((item) => item.state === 'enrolled')
    .map((item) => ({
      enrollment: item,
      course: findCourse(item.teaching_class_id, item.round_id),
    }))
    .filter((item) => item.course?.dayOfWeek && item.course.startSection),
)

const unscheduledCourses = computed(() =>
  (enrollmentsQuery.data.value?.items ?? [])
    .filter((item) => item.state === 'enrolled')
    .filter((item) => !findCourse(item.teaching_class_id)?.dayOfWeek),
)

function courseAt(dayIndex: number, startSection: number) {
  return scheduledCourses.value.find(
    (item) =>
      item.course?.dayOfWeek === dayIndex + 1 &&
      item.course.startSection === startSection,
  )?.course
}
</script>

<template>
  <div class="schedule-workspace">
    <div v-if="enrollmentsQuery.isLoading.value" class="schedule-state">
      正在生成课表…
    </div>
    <div
      v-else-if="enrollmentsQuery.isError.value"
      class="schedule-state schedule-state--error"
      role="alert"
    >
      {{
        enrollmentsQuery.error.value instanceof Error
          ? enrollmentsQuery.error.value.message
          : '课表读取失败，请稍后刷新'
      }}
    </div>
    <div v-else class="schedule-scroll">
      <div class="schedule-grid">
        <div class="schedule-grid__corner">节次</div>
        <div v-for="day in weekdays" :key="day" class="schedule-grid__head">
          {{ day }}
        </div>
        <template v-for="period in periods" :key="period.start">
          <div class="schedule-grid__period">{{ period.label }}</div>
          <div
            v-for="(day, dayIndex) in weekdays"
            :key="`${period.start}-${day}`"
            class="schedule-grid__cell"
          >
            <article
              v-if="courseAt(dayIndex, period.start)"
              class="schedule-course"
            >
              <span>{{ courseAt(dayIndex, period.start)?.courseCode }}</span>
              <strong>{{ courseAt(dayIndex, period.start)?.courseName }}</strong>
              <small>
                <MapPin :size="12" />
                {{ courseAt(dayIndex, period.start)?.location }}
              </small>
            </article>
          </div>
        </template>
      </div>
    </div>

    <aside v-if="unscheduledCourses.length" class="unscheduled-panel">
      <CalendarX2 :size="20" />
      <div>
        <strong>{{ unscheduledCourses.length }} 门课程暂未排入课表</strong>
        <p>
          后端选课记录目前只返回教学班 ID，课程时间查询接口接入后会自动补齐未知教学班。
        </p>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.schedule-workspace {
  display: grid;
  gap: 16px;
}

.schedule-scroll {
  overflow-x: auto;
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.schedule-grid {
  display: grid;
  min-width: 780px;
  grid-template-columns: 90px repeat(5, minmax(130px, 1fr));
}

.schedule-grid__corner,
.schedule-grid__head,
.schedule-grid__period,
.schedule-grid__cell {
  min-height: 96px;
  padding: 12px;
  border-right: 1px solid var(--line-soft);
  border-bottom: 1px solid var(--line-soft);
}

.schedule-grid__head,
.schedule-grid__corner {
  min-height: 48px;
  color: var(--muted);
  font-size: 11px;
  font-weight: 750;
}

.schedule-grid__period {
  color: var(--muted);
  font-size: 11px;
}

.schedule-course {
  display: flex;
  height: 100%;
  flex-direction: column;
  gap: 5px;
  padding: 11px;
  border-left: 3px solid var(--brand-bright);
  border-radius: 8px;
  color: #164b3e;
  background: #dcf0e8;
}

.schedule-course > span {
  font-family: var(--font-mono);
  font-size: 9px;
}

.schedule-course strong {
  font-size: 12px;
}

.schedule-course small {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: auto;
  color: #55736a;
  font-size: 9px;
}

.schedule-state {
  padding: 60px;
  color: var(--muted);
  text-align: center;
}

.schedule-state--error {
  color: var(--danger);
}

.unscheduled-panel {
  display: flex;
  gap: 12px;
  padding: 16px;
  border: 1px solid #ead6a5;
  border-radius: 12px;
  color: #76500f;
  background: #fff5d9;
}

.unscheduled-panel strong {
  font-size: 12px;
}

.unscheduled-panel p {
  margin: 4px 0 0;
  font-size: 11px;
  line-height: 1.5;
}
</style>
